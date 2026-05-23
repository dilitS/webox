package smallhost

import (
	"fmt"
	"strconv"

	"github.com/dilitS/webox/providers"
)

// providerName is the registered type token shared with `config.Profile.Type`
// and `docs/providers/smallhost.md`. It is exported as a constant so
// the wizard and `webox doctor` can reference it without leaking
// stringly-typed literals across the codebase.
const providerName = "smallhost"

// restartMethodDevil is the only restart_method supported in MVP. The
// constant exists so the parser, doctor, and future providers (`cpanel`
// with `passenger` / `app_manager`) share a single source of truth.
const restartMethodDevil = "devil"

// Default values mirror the conservative numbers from DESIGN §5.3.
// They are package-private constants because adapters MUST own the
// final say on what is sane for their panel — only the registry-level
// invariants live in `providers/`.
const (
	defaultSSHPoolMax = 3
	minSSHPoolMax     = 1
	maxSSHPoolMax     = 16
)

// Properties is the typed projection of `ProviderConfig.Properties`
// for small.pl. Parsing happens once during construction so every
// adapter method reads typed values; this also moves all "unknown
// key", "bad value" errors to a single, testable code path.
type Properties struct {
	// RestartMethod selects how RestartNodeApp talks to the panel.
	// MVP only supports "devil" — other values surface as
	// ErrInvalidProviderConfig at construction.
	RestartMethod string

	// SSHPoolMax is the per-target connection cap used by
	// `ssh.Pool`. Zero in the parsed result means "use default";
	// the field is populated with the resolved number for
	// observability.
	SSHPoolMax int

	// LegacyAlgorithmCompat opts the SSH client into the legacy
	// `ssh-rsa` (SHA-1) host key algorithm — required for the
	// handful of small.pl edges that still negotiate it. Default
	// false matches the conservative SECURITY §5.5 stance.
	LegacyAlgorithmCompat bool
}

// Provider is the small.pl HostingProvider implementation. It is
// constructed via [New] and registered with `providers.Register` in
// init(); business logic NEVER instantiates it directly.
//
// The executor field is the SSH-command seam ([Executor]). Tests
// install a fake via [Provider.SetExecutor]; the wizard installs a
// real one over `ssh.Pool` via [NewSSHExecutor]. Until SetExecutor
// runs, every method that needs SSH returns ErrUnknownOutputFormat
// wrapped with a "not configured" diagnostic — fail-closed.
type Provider struct {
	cfg      providers.ProviderConfig
	props    Properties
	executor Executor
}

// Name satisfies [providers.HostingProvider]. It returns the
// registered provider type rather than the alias so logs are
// resilient to alias renames during the MVP rollout.
func (p *Provider) Name() string { return providerName }

// Config exposes the normalised configuration for tests and the
// debug TUI. Returning a copy keeps callers from mutating the
// provider's internal state.
func (p *Provider) Config() providers.ProviderConfig { return p.cfg }

// Properties exposes the parsed properties bag for tests and the
// debug TUI. Same value-copy contract as [Config].
func (p *Provider) Properties() Properties { return p.props }

// New is the [providers.Factory] registered under "smallhost". It
// receives a config that the registry already validated for shared
// invariants (alias / host / user / port / Properties non-nil) and
// runs the adapter-specific checks (properties bag, restart method).
func New(cfg providers.ProviderConfig) (providers.HostingProvider, error) {
	if cfg.Type != providerName {
		return nil, fmt.Errorf("%w: smallhost factory invoked with type %q", providers.ErrInvalidProviderConfig, cfg.Type)
	}

	props, err := parseProperties(cfg.Properties)
	if err != nil {
		return nil, err
	}
	return &Provider{cfg: cfg, props: props}, nil
}

// parseProperties is the single seam where stringly-typed map entries
// become a typed [Properties] value. It is intentionally pure (no I/O,
// no logging) so tests can table-drive it without touching the
// registry / ssh layer.
func parseProperties(raw map[string]string) (Properties, error) {
	out := Properties{
		RestartMethod: restartMethodDevil,
		SSHPoolMax:    defaultSSHPoolMax,
	}

	if v, ok := raw["restart_method"]; ok {
		if v != restartMethodDevil {
			return Properties{}, fmt.Errorf("%w: restart_method %q is not supported (smallhost only supports %q)", providers.ErrInvalidProviderConfig, v, restartMethodDevil)
		}
		out.RestartMethod = v
	}

	if v, ok := raw["ssh_pool_max"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Properties{}, fmt.Errorf("%w: ssh_pool_max %q is not an integer", providers.ErrInvalidProviderConfig, v)
		}
		if n < minSSHPoolMax || n > maxSSHPoolMax {
			return Properties{}, fmt.Errorf("%w: ssh_pool_max %d out of range [%d,%d]", providers.ErrInvalidProviderConfig, n, minSSHPoolMax, maxSSHPoolMax)
		}
		out.SSHPoolMax = n
	}

	if v, ok := raw["ssh_algorithms_legacy_compat"]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Properties{}, fmt.Errorf("%w: ssh_algorithms_legacy_compat %q is not a boolean", providers.ErrInvalidProviderConfig, v)
		}
		out.LegacyAlgorithmCompat = b
	}

	return out, nil
}

func init() {
	if err := providers.Register(providerName, New); err != nil {
		panic(fmt.Sprintf("smallhost: register: %v", err))
	}
}
