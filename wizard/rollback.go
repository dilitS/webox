package wizard

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"

	"github.com/dilitS/webox/providers"
)

// secretShapePattern is the union of all secret-shaped tokens
// [config.Validate] rejects in `config.json`, plus the conventional
// `key=value` shapes (`password=`, `passwd=`) the wizard might
// accidentally serialise from a free-form panel response. The pattern
// is intentionally a superset of the redactor list — false positives
// here cost developer time, false negatives leak credentials.
var secretShapePattern = regexp.MustCompile(`(?i)(ghp_[A-Za-z0-9]|ghs_[A-Za-z0-9]|github_pat_[A-Za-z0-9]|\bsk-[A-Za-z0-9]|BEGIN [A-Z ]*PRIVATE KEY|password\s*=|passwd\s*=)`)

// isSecretShaped reports whether v matches any of the project-wide
// secret-shape rules. Returns false for empty strings so genuine
// missing fields surface as their own validation error.
func isSecretShaped(v string) bool {
	if v == "" {
		return false
	}
	return secretShapePattern.MatchString(v)
}

// PersistFunc is the seam [Stack] uses to write snapshots to disk.
// A nil PersistFunc is allowed — in-memory only stacks are useful for
// happy-path tests that do not exercise resume.
//
// Implementations MUST be context-aware and MUST handle the empty
// slice by removing the snapshot file (clean state). [NewFilePersister]
// supplies the canonical implementation; tests inject a recorder.
type PersistFunc func(ctx context.Context, steps []CleanupStep) error

// StepRunner executes a single cleanup step. Returned errors are
// captured per-step by [Stack.Rollback]; execution continues so the
// operator sees the full picture instead of halting on the first
// failure.
type StepRunner func(ctx context.Context, step CleanupStep) error

// Stack is the LIFO rollback stack from DESIGN §10.0. Push/Pop are
// goroutine-safe, snapshotting happens inside the lock so concurrent
// Push calls cannot interleave with persistence.
//
// The zero value is NOT usable — construct via [NewStack].
type Stack struct {
	mu       sync.Mutex
	steps    []CleanupStep
	persist  PersistFunc
	wizardID string
}

// NewStack returns a Stack with persist as the seam used to checkpoint
// after every Push/Pop. wizardID is embedded in the persisted snapshot
// so concurrent or resumed wizards can be distinguished in the resume
// UI; an empty wizardID is allowed in tests.
func NewStack(persist PersistFunc, wizardID string) *Stack {
	return &Stack{persist: persist, wizardID: wizardID}
}

// WizardID returns the identifier the stack persists alongside each
// snapshot.
func (s *Stack) WizardID() string { return s.wizardID }

// Len returns the current step count. Safe to call concurrently.
func (s *Stack) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.steps)
}

// Steps returns a defensive copy of the stack (oldest first). The
// rollback UI uses this to render "what we'll undo" before the user
// confirms.
func (s *Stack) Steps() []CleanupStep {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]CleanupStep, len(s.steps))
	copy(out, s.steps)
	return out
}

// Push appends step to the stack and persists the new snapshot
// atomically (when persist is non-nil). Validation runs first so a
// rejected step never leaks into the in-memory stack OR the on-disk
// snapshot.
//
// On persist failure the in-memory state is rolled back to its
// pre-push form so the caller can retry with a different persistor or
// surrender; the wizard treats persist failures as fatal because the
// LIFO contract is "every push is recoverable across crashes".
func (s *Stack) Push(ctx context.Context, step CleanupStep) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateStep(step); err != nil {
		return err
	}

	s.mu.Lock()
	s.steps = append(s.steps, step)
	snapshot := append([]CleanupStep(nil), s.steps...)
	s.mu.Unlock()

	if s.persist == nil {
		return nil
	}
	if err := s.persist(ctx, snapshot); err != nil {
		s.mu.Lock()
		if len(s.steps) > 0 {
			s.steps = s.steps[:len(s.steps)-1]
		}
		s.mu.Unlock()
		return fmt.Errorf("wizard: persist snapshot: %w", err)
	}
	return nil
}

// Pop removes and returns the most recent step. The boolean is false
// when the stack is empty. Persistence happens with the post-pop
// snapshot so a crash between Pop and Rollback never reruns the same
// step twice; the panel's idempotent Remove* covers the worst case.
func (s *Stack) Pop(ctx context.Context) (CleanupStep, bool, error) {
	if err := ctx.Err(); err != nil {
		return CleanupStep{}, false, err
	}

	s.mu.Lock()
	if len(s.steps) == 0 {
		s.mu.Unlock()
		return CleanupStep{}, false, nil
	}
	last := s.steps[len(s.steps)-1]
	s.steps = s.steps[:len(s.steps)-1]
	snapshot := append([]CleanupStep(nil), s.steps...)
	s.mu.Unlock()

	if s.persist != nil {
		if err := s.persist(ctx, snapshot); err != nil {
			return last, true, fmt.Errorf("wizard: persist after pop: %w", err)
		}
	}
	return last, true, nil
}

// LoadSnapshot rehydrates the stack from a deserialised
// [PendingCleanups] snapshot. The wizard uses this to resume a crashed
// run. The current in-memory steps are replaced wholesale; callers
// MUST be sure the stack is empty (or that they want the discard).
//
// LoadSnapshot does NOT persist — the caller already holds the source
// snapshot on disk; rewriting it would be a no-op at best, a partial
// write at worst.
func (s *Stack) LoadSnapshot(steps []CleanupStep) error {
	for _, step := range steps {
		if err := validateStep(step); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.steps = append(s.steps[:0:0], steps...)
	s.mu.Unlock()
	return nil
}

// Rollback pops every step and runs each through run, in reverse push
// order, collecting per-step results. Context cancellation stops the
// loop AFTER the in-flight pop returns; the operator can see how far
// the rollback got from the returned results slice.
//
// On a single-step rollback failure the loop continues — the panel
// Remove* methods are idempotent by contract, so a transient failure
// is the user's signal to retry rather than evidence the resource is
// gone. Aggregated errors are returned via [errors.Join].
func (s *Stack) Rollback(ctx context.Context, run StepRunner) ([]CleanupResult, error) {
	if run == nil {
		return nil, fmt.Errorf("%w: runner is nil", ErrInvalidStep)
	}

	var (
		results []CleanupResult
		errs    []error
	)
	for {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			return results, errors.Join(errs...)
		}
		step, ok, popErr := s.Pop(ctx)
		if popErr != nil {
			errs = append(errs, popErr)
			return results, errors.Join(errs...)
		}
		if !ok {
			break
		}
		runErr := run(ctx, step)
		results = append(results, CleanupResult{Step: step, Err: runErr})
		if runErr != nil {
			errs = append(errs, fmt.Errorf("rollback %s: %w", step.Name, runErr))
		}
	}
	if len(errs) == 0 {
		return results, nil
	}
	return results, errors.Join(errs...)
}

// MakeStepRunner returns a StepRunner that dispatches each [CleanupStep]
// to the matching [providers.HostingProvider] Remove* method. Unknown
// kinds surface as [ErrUnsupportedKind] so a snapshot written by a
// newer Webox release never silently no-ops on an older binary.
//
// The runner trusts the dispatcher target — it does NOT re-validate
// Params, because [Stack.Push] already rejected malformed steps and
// the on-disk snapshot is owner-only (`0600`).
func MakeStepRunner(provider providers.HostingProvider) StepRunner {
	return func(ctx context.Context, step CleanupStep) error {
		if provider == nil {
			return fmt.Errorf("%w: provider is nil", ErrInvalidStep)
		}
		switch step.Kind {
		case ResourceSubdomain:
			return provider.RemoveSubdomain(ctx, step.Params["domain"])
		case ResourceSSL:
			return provider.RemoveSSL(ctx, step.Params["domain"])
		case ResourceDatabase:
			return provider.RemoveDatabase(ctx, step.Params["dbKind"], step.Params["dbName"])
		default:
			return fmt.Errorf("%w: %q", ErrUnsupportedKind, step.Kind)
		}
	}
}

// validateStep enforces the structural invariants and the
// no-secret-in-params rule. Pure function — every input is plain Go
// strings, no I/O.
func validateStep(step CleanupStep) error {
	if step.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidStep)
	}
	switch step.Kind {
	case ResourceSubdomain, ResourceSSL, ResourceDatabase:
	default:
		return fmt.Errorf("%w: kind %q is not supported", ErrInvalidStep, step.Kind)
	}
	for k, v := range step.Params {
		if isSecretShaped(v) {
			return fmt.Errorf("%w: %q", ErrSecretInCleanup, k)
		}
	}
	switch step.Kind {
	case ResourceSubdomain, ResourceSSL:
		if step.Params["domain"] == "" {
			return fmt.Errorf("%w: %s requires params[domain]", ErrInvalidStep, step.Kind)
		}
	case ResourceDatabase:
		if step.Params["dbName"] == "" || step.Params["dbKind"] == "" {
			return fmt.Errorf("%w: database requires params[dbKind] and params[dbName]", ErrInvalidStep)
		}
	}
	return nil
}
