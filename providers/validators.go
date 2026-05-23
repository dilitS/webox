package providers

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ErrUnknownValidator is returned by [PlanValidatorsFor] when the
// requested provider type has no validator set registered. Wizard
// code branches on this via [errors.Is] to fail closed rather than
// fall back to a permissive default.
var ErrUnknownValidator = errors.New("provider: plan validators not registered")

// ErrInvalidValidatorSet is returned by [RegisterPlanValidators] when
// any of the required validator functions are nil. Plan-level
// validation is a security-critical path: a missing validator silently
// degrades defense-in-depth, so we refuse to register a partial set.
var ErrInvalidValidatorSet = errors.New("provider: plan validator set is incomplete")

// PlanValidators is the per-provider bundle of input validators the
// wizard runs before any provider call. Each function returns nil on
// success and a wrapped error on failure; the wizard further wraps
// the result in `wizard.ErrInvalidPlan` so call sites only need a
// single sentinel comparison.
//
// Decoupling the wizard from provider packages this way fulfils the
// AGENTS.md §2.2 rule that "business logic never knows a provider by
// name". Adapters register their validators alongside the factory in
// init(); the wizard resolves them through [PlanValidatorsFor].
type PlanValidators struct {
	// ValidateDomain checks that the fully-qualified subdomain is
	// safe to substitute into the panel's CLI commands. Adapters
	// MUST reject shell metacharacters, embedded whitespace, and
	// any pattern that would let an injected value escape its
	// argument slot.
	ValidateDomain func(domain string) error

	// ValidateNodeVersion checks that the Node.js version string is
	// safe to forward to the panel. Adapters typically accept a
	// short alphanumeric token (`22`, `v22.4.1`) and reject any
	// shell-like input.
	ValidateNodeVersion func(version string) error

	// ValidateDBName checks that the database identifier is safe to
	// substitute into `<panel> mysql add` / `pgsql add`. Adapters
	// constrain identifiers to lowercase alphanumerics + `_` with a
	// hard length cap so the value round-trips cleanly through
	// `config.json` and the panel.
	ValidateDBName func(name string) error
}

// IsComplete reports whether every required validator is set. A nil
// validator in any slot would silently bypass an entire layer of
// defense, so the registry rejects partial sets at registration
// time.
func (v PlanValidators) IsComplete() bool {
	return v.ValidateDomain != nil && v.ValidateNodeVersion != nil && v.ValidateDBName != nil
}

// validatorRegistry mirrors [registry] but for plan-level validation
// functions. Keeping a parallel structure avoids growing
// `HostingProvider` with non-I/O methods that every future provider
// would have to implement on the receiver instead of as plain
// functions.
var (
	validatorMu sync.RWMutex
	validators  = map[string]PlanValidators{}
)

// RegisterPlanValidators stores set under providerType so the wizard
// can resolve adapter-specific input validators without importing the
// adapter package. Adapters call this from init() right after
// [Register]; double-registration is rejected because it is almost
// always a code bug (two init() blocks).
func RegisterPlanValidators(providerType string, set PlanValidators) error {
	providerType = strings.TrimSpace(providerType)
	if providerType == "" {
		return fmt.Errorf("%w: provider type is empty", ErrInvalidProviderConfig)
	}
	if !set.IsComplete() {
		return fmt.Errorf("%w: %q is missing one of ValidateDomain/ValidateNodeVersion/ValidateDBName", ErrInvalidValidatorSet, providerType)
	}

	validatorMu.Lock()
	defer validatorMu.Unlock()
	if _, ok := validators[providerType]; ok {
		return fmt.Errorf("%w: validators %q already registered", ErrProviderAlreadyRegistered, providerType)
	}
	validators[providerType] = set
	return nil
}

// UnregisterPlanValidators removes set under providerType. Exists for
// tests that exercise registration semantics; production code MUST
// NOT call it. Returns true when an entry was removed, false
// otherwise so test cleanup loops are idempotent.
func UnregisterPlanValidators(providerType string) bool {
	validatorMu.Lock()
	defer validatorMu.Unlock()
	if _, ok := validators[providerType]; !ok {
		return false
	}
	delete(validators, providerType)
	return true
}

// PlanValidatorsFor returns the registered [PlanValidators] for
// providerType. The returned set is a value copy so callers cannot
// mutate the registry through it.
func PlanValidatorsFor(providerType string) (PlanValidators, error) {
	providerType = strings.TrimSpace(providerType)
	if providerType == "" {
		return PlanValidators{}, fmt.Errorf("%w: provider type is empty", ErrInvalidProviderConfig)
	}

	validatorMu.RLock()
	defer validatorMu.RUnlock()
	set, ok := validators[providerType]
	if !ok {
		return PlanValidators{}, fmt.Errorf("%w: %q", ErrUnknownValidator, providerType)
	}
	return set, nil
}
