package tui

import (
	"context"
	"sync"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/providers"
	_ "github.com/dilitS/webox/providers/smallhost" // register smallhost factory
	"github.com/dilitS/webox/wizard"
)

// WizardRunner is the side-effect seam the TUI uses to talk to
// providers. Production wiring (see [DefaultWizardRunner]) builds the
// real provider over `ssh.Pool`; tests substitute an in-memory fake
// so wizard transitions stay deterministic and offline.
type WizardRunner interface {
	// Preflight verifies the named profile is reachable. The TUI
	// surfaces a non-nil err inside the Domain step rather than
	// aborting to Dashboard.
	Preflight(ctx context.Context, profile config.Profile) (*providers.ProviderStatus, error)

	// CheckDomainAvailable returns ErrSubdomainExists when the
	// domain already lives on the panel; nil when the wizard may
	// proceed.
	CheckDomainAvailable(ctx context.Context, profile config.Profile, domain string) error

	// Execute provisions the plan and records cleanup steps on
	// stack. Returns [wizard.ExecutionFailedError] on partial
	// failure; the caller drives rollback or "keep and exit" from
	// the failure UI.
	Execute(ctx context.Context, profile config.Profile, plan wizard.ProvisionPlan, stack *wizard.Stack) (*wizard.ProvisionReport, error)

	// Rollback runs every cleanup step pushed by Execute, in
	// reverse order. Idempotent: missing panel resources are
	// success.
	Rollback(ctx context.Context, profile config.Profile, stack *wizard.Stack) ([]wizard.CleanupResult, error)

	// RestartApp restarts the Node.js application bound to domain.
	// Wraps [providers.HostingProvider.RestartNodeApp] but keeps the
	// runner as the only TUI-facing seam so tests stay deterministic.
	RestartApp(ctx context.Context, profile config.Profile, domain string) error

	// RenewSSL re-runs the panel's "ssl www add" call. small.pl's
	// Devil treats repeat invocations as renew, so the wizard and
	// dashboard share the same provider method. Returns the same
	// sentinel set as the wizard SSL step.
	RenewSSL(ctx context.Context, profile config.Profile, domain string) error

	// TailLog returns the last `lines` log entries for the project.
	// The runner clamps the line count via the provider so a UI bug
	// cannot ship 1 GiB of log to the local process.
	TailLog(ctx context.Context, profile config.Profile, domain string, lines int) ([]byte, error)

	// ListProviderSubdomains is the read-only enumeration used by
	// the dashboard import flow. Returns the panel snapshot so the
	// caller can diff against `config.json` without making any
	// state-changing call.
	ListProviderSubdomains(ctx context.Context, profile config.Profile) ([]providers.Subdomain, error)
}

// providerProvider is the seam used by [DefaultWizardRunner] to
// construct a [providers.HostingProvider] from a [config.Profile].
// Production wires `providers.New` directly; tests override with a
// fake to bypass the global factory registry.
type providerProvider func(config.Profile) (providers.HostingProvider, error)

// DefaultWizardRunner returns a runner that constructs the panel
// provider on demand via [providers.New]. The runner does NOT cache
// providers across calls — each step opens a fresh adapter because
// the smallhost provider methods are stateless (the adapter uses an
// executor seam set on every call).
//
// A future revision can carry a long-lived `ssh.Pool` so deploy
// flows reuse connections across the wizard + status loop.
func DefaultWizardRunner() WizardRunner {
	return &defaultRunner{provider: newProviderFromProfile}
}

// NewTestWizardRunner builds a runner with a custom provider seam.
// Used by tests; not exported for non-test callers.
func NewTestWizardRunner(p providerProvider) WizardRunner {
	return &defaultRunner{provider: p}
}

type defaultRunner struct {
	provider providerProvider
}

// Preflight constructs the panel provider via the configured factory
// and delegates to [wizard.Preflight].
func (r *defaultRunner) Preflight(ctx context.Context, profile config.Profile) (*providers.ProviderStatus, error) {
	p, err := r.provider(profile)
	if err != nil {
		return nil, err
	}
	return wizard.Preflight(ctx, p)
}

// CheckDomainAvailable forwards to [wizard.CheckSubdomainAvailable]
// after constructing the provider for profile.
func (r *defaultRunner) CheckDomainAvailable(ctx context.Context, profile config.Profile, domain string) error {
	p, err := r.provider(profile)
	if err != nil {
		return err
	}
	return wizard.CheckSubdomainAvailable(ctx, p, domain)
}

// Execute constructs the provider and dispatches plan through
// [wizard.Execute].
func (r *defaultRunner) Execute(ctx context.Context, profile config.Profile, plan wizard.ProvisionPlan, stack *wizard.Stack) (*wizard.ProvisionReport, error) {
	p, err := r.provider(profile)
	if err != nil {
		return nil, err
	}
	return wizard.Execute(ctx, p, plan, stack)
}

// Rollback drives [Stack.Rollback] against a provider built from
// profile.
func (r *defaultRunner) Rollback(ctx context.Context, profile config.Profile, stack *wizard.Stack) ([]wizard.CleanupResult, error) {
	p, err := r.provider(profile)
	if err != nil {
		return nil, err
	}
	return stack.Rollback(ctx, wizard.MakeStepRunner(p))
}

// RestartApp resolves the provider and forwards to RestartNodeApp.
func (r *defaultRunner) RestartApp(ctx context.Context, profile config.Profile, domain string) error {
	p, err := r.provider(profile)
	if err != nil {
		return err
	}
	return p.RestartNodeApp(ctx, domain)
}

// RenewSSL resolves the provider and forwards to SetupSSL. Devil
// treats SetupSSL as idempotent renew.
func (r *defaultRunner) RenewSSL(ctx context.Context, profile config.Profile, domain string) error {
	p, err := r.provider(profile)
	if err != nil {
		return err
	}
	return p.SetupSSL(ctx, domain)
}

// TailLog resolves the provider and forwards to TailLog.
func (r *defaultRunner) TailLog(ctx context.Context, profile config.Profile, domain string, lines int) ([]byte, error) {
	p, err := r.provider(profile)
	if err != nil {
		return nil, err
	}
	return p.TailLog(ctx, domain, lines)
}

// ListProviderSubdomains resolves the provider and forwards to
// ListSubdomains.
func (r *defaultRunner) ListProviderSubdomains(ctx context.Context, profile config.Profile) ([]providers.Subdomain, error) {
	p, err := r.provider(profile)
	if err != nil {
		return nil, err
	}
	return p.ListSubdomains(ctx)
}

// newProviderFromProfile constructs a [providers.HostingProvider]
// from the persisted profile. The smallhost executor is wired with a
// nil pool because no provider call goes over a live network yet —
// the wizard surfaces "ssh disconnected" at preflight when the pool
// dependency is plumbed in. For now, integration tests install a
// fake runner; the default runner stays compiled but guarded by the
// executor-not-configured sentinel.
func newProviderFromProfile(profile config.Profile) (providers.HostingProvider, error) {
	return providers.New(providers.ProviderConfig{
		Alias:      profile.Alias,
		Type:       profile.Type,
		Host:       profile.Host,
		Port:       profile.Port,
		User:       profile.User,
		Properties: cloneProps(profile.Properties),
	})
}

func cloneProps(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// wizardStackSlot is the model-held handle to a wizard execution's
// LIFO stack. It is a pointer holder so successive Update calls can
// share the same stack value through the model copy (the stack
// itself is heap-allocated and uses its own mutex).
type wizardStackSlot struct {
	mu    sync.Mutex
	stack *wizard.Stack
}

func newStackSlot() *wizardStackSlot { return &wizardStackSlot{} }

// set replaces the held stack reference. Safe to call from any
// goroutine; the wizard commands share one slot per project flow.
func (s *wizardStackSlot) set(st *wizard.Stack) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stack = st
}

// get returns the current stack pointer, or nil if Execute has not
// run yet.
func (s *wizardStackSlot) get() *wizard.Stack {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stack
}

// ProfileByAlias returns the profile from cfg with matching alias.
// Returns the zero profile and false when no match — the wizard
// treats the false branch as a fatal error.
func ProfileByAlias(cfg *config.Config, alias string) (config.Profile, bool) {
	if cfg == nil {
		return config.Profile{}, false
	}
	for _, p := range cfg.Profiles {
		if p.Alias == alias {
			return p, true
		}
	}
	return config.Profile{}, false
}
