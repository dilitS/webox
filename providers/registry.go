package providers

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Factory builds a [HostingProvider] from a normalised [ProviderConfig].
// Adapters register themselves via [Register] in their init() block.
//
// Factories MUST NOT perform I/O at construction time — they may only
// validate adapter-specific properties and reject obvious misuse via
// errors wrapping ErrInvalidProviderConfig. The wizard / status loop
// owns the lifecycle of the returned HostingProvider, including
// dialling SSH and warming caches.
type Factory func(cfg ProviderConfig) (HostingProvider, error)

// registry is the in-process catalogue of provider factories. It is a
// package-level singleton guarded by [registryMu]; tests that need a
// pristine registry use [WithRegistry] to swap in a private map.
//
// Adapters register at init() time, so the singleton is the only
// pragmatic option for production code — but every direct user goes
// through [Register] / [New] / [Names], never through the variable
// itself, so the test seam stays narrow.
var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

// aliasPattern is the validation regex shared with `config.Profile.Alias`.
// Keeping the constraint identical means the registry can rely on the
// upstream JSON schema rejecting bad aliases before construction, while
// still defending in depth when callers build [ProviderConfig] in
// memory (tests, future REST surface, …).
var aliasPattern = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

// Register adds factory under providerType. Adapters call this from
// their package init(), so duplicate registration during normal
// operation is almost always a code bug (two init() blocks claiming
// the same name) — we surface it as a typed error rather than
// panicking, so tests can assert on the behavior without the process
// crashing mid-suite.
//
// Empty type / nil factory are rejected with ErrInvalidProviderConfig:
// they are programmer errors that we want to surface at boot, not
// silently swallow.
func Register(providerType string, factory Factory) error {
	providerType = strings.TrimSpace(providerType)
	if providerType == "" {
		return fmt.Errorf("%w: provider type is empty", ErrInvalidProviderConfig)
	}
	if factory == nil {
		return fmt.Errorf("%w: factory for %q is nil", ErrInvalidProviderConfig, providerType)
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	if _, ok := registry[providerType]; ok {
		return fmt.Errorf("%w: %q", ErrProviderAlreadyRegistered, providerType)
	}
	registry[providerType] = factory
	return nil
}

// Unregister removes providerType from the registry. It exists for
// tests that exercise adapter loading semantics; production code MUST
// NOT call it. Returns true when an entry was removed, false otherwise
// — letting test cleanup loops be idempotent.
func Unregister(providerType string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, ok := registry[providerType]; !ok {
		return false
	}
	delete(registry, providerType)
	return true
}

// Names returns the sorted list of registered provider types. It is
// useful for `webox doctor`, the wizard's provider picker, and CLI
// help text. The result is a fresh slice — callers may mutate it
// safely.
func Names() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// New looks up the factory for cfg.Type, validates the shared invariants
// (alias / host / user / port / Properties map), normalises Port to
// [DefaultSSHPort] when zero, and returns the constructed adapter.
//
// Validation order is deliberate: registry lookup runs FIRST so that a
// completely bogus profile type (e.g. typo "smolhost") surfaces as
// ErrUnknownProvider — actionable for the user — instead of being
// drowned by ErrInvalidProviderConfig on a field they did not edit.
func New(cfg ProviderConfig) (HostingProvider, error) {
	cfg.Type = strings.TrimSpace(cfg.Type)
	if cfg.Type == "" {
		return nil, fmt.Errorf("%w: type is required", ErrInvalidProviderConfig)
	}

	registryMu.RLock()
	factory, ok := registry[cfg.Type]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q (registered: %v)", ErrUnknownProvider, cfg.Type, Names())
	}

	normalised, err := validateConfig(cfg)
	if err != nil {
		return nil, err
	}

	provider, err := factory(normalised)
	if err != nil {
		return nil, fmt.Errorf("provider %q factory: %w", normalised.Type, err)
	}
	return provider, nil
}

// validateConfig enforces the shared cross-adapter invariants. Adapter
// factories receive a normalised cfg (Port defaulted, Properties
// non-nil) so they only need to validate adapter-specific keys.
func validateConfig(cfg ProviderConfig) (ProviderConfig, error) {
	cfg.Alias = strings.TrimSpace(cfg.Alias)
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.User = strings.TrimSpace(cfg.User)

	if cfg.Alias == "" {
		return ProviderConfig{}, fmt.Errorf("%w: alias is required", ErrInvalidProviderConfig)
	}
	if !aliasPattern.MatchString(cfg.Alias) {
		return ProviderConfig{}, fmt.Errorf("%w: alias %q does not match %s", ErrInvalidProviderConfig, cfg.Alias, aliasPattern.String())
	}
	if cfg.Host == "" {
		return ProviderConfig{}, fmt.Errorf("%w: host is required", ErrInvalidProviderConfig)
	}
	if cfg.User == "" {
		return ProviderConfig{}, fmt.Errorf("%w: user is required", ErrInvalidProviderConfig)
	}
	if cfg.Port == 0 {
		cfg.Port = DefaultSSHPort
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return ProviderConfig{}, fmt.Errorf("%w: port %d out of range [1,65535]", ErrInvalidProviderConfig, cfg.Port)
	}
	if cfg.Properties == nil {
		cfg.Properties = map[string]string{}
	}
	return cfg, nil
}
