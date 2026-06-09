package cpanel

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dilitS/webox/providers"
)

// providerName is the registered type token shared with
// `config.Profile.Type` and the preset registry's `provider_type`
// field. Exported as a constant so the wizard, doctor, and tests
// reference it without leaking stringly-typed literals.
const providerName = "cpanel"

// RestartMethod enumerates the supported Passenger restart paths.
// Sprint 22 ships `passenger` (UAPI restart_application) as the
// only supported value; `app_manager` is reserved for a future
// hoster-specific shim. Anything else surfaces as
// [providers.ErrInvalidProviderConfig] at construction.
const (
	restartPassenger  = "passenger"
	restartAppManager = "app_manager"
)

// SSLProvider enumerates the supported SSL provisioning paths.
// `autossl` is the cPanel default (and the only one most shared
// hosts allow); `manual` is the byo-cert escape hatch.
const (
	sslAutoSSL = "autossl"
	sslManual  = "manual"
)

// NodeSelector enumerates the Node.js runtime owners observed on
// real cPanel hosts. The constant influences which app-root
// template the adapter resolves and (in future sprints) which
// selector binary the doctor probes.
const (
	nodeSelectorCloudlinux = "cloudlinux_selector"
	nodeSelectorAppManager = "app_manager"
	nodeSelectorNone       = "none"
)

// DomainKind enumerates how Webox should treat the wizard's domain
// input. `addon` calls DomainInfo::add_addon_domain (the default
// for the operator's primary cPanel account); `subdomain` calls
// SubDomain::addsubdomain. The decision matters for billing
// (addon domains count against the account's addon-domain quota)
// and for DNS (subdomains inherit the parent zone).
const (
	domainKindAddon     = "addon"
	domainKindSubdomain = "subdomain"
)

// Default template values match the `cpanel-cloudlinux-selector`
// preset shipped in Sprint 19. Operators with a different host
// layout override via the Properties bag at adapter construction.
const (
	defaultAppRootTemplate    = "/home/{user}/nodejs/{app_root}"
	defaultDeployPathTemplate = "/home/{user}/nodejs/{app_root}/public"
	defaultLogPathTemplate    = "/home/{user}/nodejs/{app_root}/logs"
	defaultAPIPort            = 2083
)

// Min/max API port — cPanel UAPI listens on 2083 by default but
// some white-label hosters re-expose it on 2087 (WHM port) or a
// custom port. We clamp to TCP-valid range only; the registry's
// shared port validator already enforces 1..65535.
const (
	minAPIPort = 1
	maxAPIPort = 65535
)

// Properties is the typed projection of `ProviderConfig.Properties`
// for cPanel. Parsing happens once during construction so every
// adapter method reads typed values — same shape as the smallhost
// adapter, keeping the two adapters reviewable side-by-side.
type Properties struct {
	// RestartMethod selects how [Provider.RestartNodeApp]
	// talks to the panel. Sprint 22: only `passenger` is
	// supported.
	RestartMethod string

	// NodeSelector selects which Node.js runtime owner the
	// adapter assumes. Influences app-root template resolution.
	NodeSelector string

	// SSLProvider selects [Provider.SetupSSL] strategy. Sprint
	// 22: `autossl` (default) or `manual` (byo-cert).
	SSLProvider string

	// DomainKind selects whether [Provider.CreateSubdomain]
	// calls DomainInfo::add_addon_domain or
	// SubDomain::addsubdomain. Default: `addon` (the wizard's
	// most common case).
	DomainKind string

	// AppRootTemplate is the on-disk root for Passenger apps.
	// Placeholders: `{user}` (account login), `{app_root}` (the
	// adapter-derived per-app slug).
	AppRootTemplate string

	// DeployPathTemplate is the path the deploy workflow rsyncs
	// build artefacts into.
	DeployPathTemplate string

	// LogPathTemplate is the directory containing the app's
	// stdout/stderr logs.
	LogPathTemplate string

	// APIPort is the HTTPS UAPI port. Defaults to 2083.
	APIPort int

	// AutoSSLPollSeconds is the post-AutoSSL settle delay before
	// the wizard's status loop checks for cert installation.
	// Sprint 22 default: 0 (no automatic polling); the SetupSSL
	// implementation just triggers AutoSSL and returns.
	AutoSSLPollSeconds int
}

// maxAutoSSLPollSeconds is the upper bound on the post-AutoSSL
// settle delay. Ten minutes is more than enough for cPanel to
// finish a Let's Encrypt request; anything larger is almost
// certainly a config typo (the wizard surfaces a hint).
const maxAutoSSLPollSeconds = 600

// parseProperties is the single seam where the stringly-typed map
// entries become a typed [Properties] value. Pure: no I/O, no
// logging — safe for table-driven tests in the package's
// `config_test.go`.
func parseProperties(raw map[string]string) (Properties, error) {
	out := defaultProperties()
	if err := applyEnumProperty(&out.RestartMethod, raw, "restart_method", restartPassenger, restartAppManager); err != nil {
		return Properties{}, err
	}
	if err := applyEnumProperty(&out.NodeSelector, raw, "node_selector", nodeSelectorCloudlinux, nodeSelectorAppManager, nodeSelectorNone); err != nil {
		return Properties{}, err
	}
	if err := applyEnumProperty(&out.SSLProvider, raw, "ssl_provider", sslAutoSSL, sslManual); err != nil {
		return Properties{}, err
	}
	if err := applyEnumProperty(&out.DomainKind, raw, "domain_kind", domainKindAddon, domainKindSubdomain); err != nil {
		return Properties{}, err
	}
	applyTemplateProperty(&out.AppRootTemplate, raw, "app_root_template")
	applyTemplateProperty(&out.DeployPathTemplate, raw, "deploy_path_template")
	applyTemplateProperty(&out.LogPathTemplate, raw, "log_path_template")
	if err := applyIntProperty(&out.APIPort, raw, "api_port", minAPIPort, maxAPIPort); err != nil {
		return Properties{}, err
	}
	if err := applyIntProperty(&out.AutoSSLPollSeconds, raw, "autossl_poll_seconds", 0, maxAutoSSLPollSeconds); err != nil {
		return Properties{}, err
	}
	return out, nil
}

// defaultProperties returns the typed defaults — pulled out so the
// happy-path parser reads top-to-bottom and the cyclomatic count
// stays well under the lint gate.
func defaultProperties() Properties {
	return Properties{
		RestartMethod:      restartPassenger,
		NodeSelector:       nodeSelectorCloudlinux,
		SSLProvider:        sslAutoSSL,
		DomainKind:         domainKindAddon,
		AppRootTemplate:    defaultAppRootTemplate,
		DeployPathTemplate: defaultDeployPathTemplate,
		LogPathTemplate:    defaultLogPathTemplate,
		APIPort:            defaultAPIPort,
	}
}

// applyEnumProperty sets *dst to raw[key] when present and the value
// is in allowed; returns ErrInvalidProviderConfig otherwise.
func applyEnumProperty(dst *string, raw map[string]string, key string, allowed ...string) error {
	v, ok := raw[key]
	if !ok {
		return nil
	}
	for _, a := range allowed {
		if v == a {
			*dst = v
			return nil
		}
	}
	return fmt.Errorf("%w: %s %q not in %v", providers.ErrInvalidProviderConfig, key, v, allowed)
}

// applyTemplateProperty overrides *dst with raw[key] when the value
// is present and non-blank. Blank strings preserve the default —
// matches the smallhost adapter's behaviour.
func applyTemplateProperty(dst *string, raw map[string]string, key string) {
	if v, ok := raw[key]; ok && strings.TrimSpace(v) != "" {
		*dst = v
	}
}

// applyIntProperty parses raw[key] as an integer and clamps it to
// [lo, hi]. Returns ErrInvalidProviderConfig on parse failure or
// out-of-range value. Missing key = no-op (leave default).
func applyIntProperty(dst *int, raw map[string]string, key string, lo, hi int) error {
	v, ok := raw[key]
	if !ok {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("%w: %s %q is not an integer",
			providers.ErrInvalidProviderConfig, key, v)
	}
	if n < lo || n > hi {
		return fmt.Errorf("%w: %s %d out of range [%d,%d]",
			providers.ErrInvalidProviderConfig, key, n, lo, hi)
	}
	*dst = n
	return nil
}
