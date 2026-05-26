package uapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
)

// EnvMutationsAllowed is the env-var name that gates every method
// on the [Mutator] surface. Operators MUST set
// `WEBOX_CPANEL_MUTATIONS=1` to opt in; absent or any other value
// makes every method return [ErrMutationsDisabled] without making
// the underlying request. This is defence in depth — the adapter
// layer (providers/cpanel) ALSO checks its own properties bag, but
// the transport-level guard means a misconfigured caller cannot
// silently bypass the adapter.
const EnvMutationsAllowed = "WEBOX_CPANEL_MUTATIONS"

// MutationsAllowed reports whether the operator has opted into
// mutating cPanel ops via the [EnvMutationsAllowed] env var. The
// constructor for every mutator implementation calls this; method
// callers MAY call it directly when they want to render a remediation
// hint before attempting a call (e.g. "set WEBOX_CPANEL_MUTATIONS=1
// to enable").
func MutationsAllowed() bool {
	return strings.TrimSpace(os.Getenv(EnvMutationsAllowed)) == "1"
}

// CreateAddonDomainArgs is the typed input for
// [Mutator.AddAddonDomain]. The adapter has already validated every
// field at the public boundary; the struct's [Validate] runs a
// shape check here so a hand-built args struct cannot smuggle shell
// metacharacters or empty values into the transport.
type CreateAddonDomainArgs struct {
	// NewDomain is the fully-qualified top-level domain to bind,
	// e.g. "client.example.com".
	NewDomain string
	// Subdomain is the auto-generated subdomain handle cPanel
	// uses internally (typically the same as NewDomain with `.`
	// replaced by `-`). The adapter computes a default when the
	// caller leaves this empty.
	Subdomain string
	// Dir is the document root relative to the account home
	// (e.g. "public_html/client").
	Dir string
}

// CreateSubdomainArgs is the typed input for [Mutator.AddSubdomain].
type CreateSubdomainArgs struct {
	// Domain is the leftmost label, e.g. "api".
	Domain string
	// RootDomain is the parent zone, e.g. "example.com".
	RootDomain string
	// Dir is the document root relative to the account home.
	Dir string
}

// CreatePassengerAppArgs is the typed input for
// [Mutator.CreatePassengerApp].
type CreatePassengerAppArgs struct {
	// Name is the application handle cPanel displays in the
	// Application Manager. Limited to lowercase alphanumerics
	// plus `-` / `_`.
	Name string
	// Path is the absolute on-disk path of the application root
	// (e.g. "/home/user/nodejs/myapp").
	Path string
	// Domain is the FQDN the application serves.
	Domain string
	// BaseURI is the URL prefix the application is mounted at
	// (typically "/").
	BaseURI string
	// DeploymentMode is "production" or "development".
	DeploymentMode string
	// Envvars is the environment variable bundle (NODE_ENV,
	// PORT, etc.). Keys are flattened as `envvar.<KEY>=<VALUE>`
	// query params; cPanel UAPI documents this convention
	// explicitly for PassengerApps.create_application.
	Envvars map[string]string
}

// EditPassengerAppArgs is the typed input for
// [Mutator.EditPassengerApp]. The Path selects the existing
// application; every other field overrides the current value.
type EditPassengerAppArgs struct {
	Path           string
	Domain         string
	BaseURI        string
	DeploymentMode string
	Envvars        map[string]string
}

// MysqlPrivilegesArgs is the typed input for
// [Mutator.SetMysqlPrivileges].
type MysqlPrivilegesArgs struct {
	Database   string
	User       string
	Privileges []string // e.g. ["ALL PRIVILEGES"] or ["SELECT","INSERT"]
}

// InstallSSLArgs is the typed input for [Mutator.InstallSSL]. The
// adapter uses [Mutator.StartAutoSSL] instead on AutoSSL-enabled
// hosts; manual install is the escape hatch for byo-cert flows.
type InstallSSLArgs struct {
	Domain   string
	Cert     string // PEM-encoded certificate
	Key      string // PEM-encoded private key
	CABundle string // optional intermediates chain
}

// Validate runs the shape check the transport relies on before
// building the request. Empty / oversized / control-character
// inputs surface as [ErrInvalidArgs] with a wrapped message
// identifying the offending field.
func (a CreateAddonDomainArgs) Validate() error {
	if err := mustNonEmpty("new_domain", a.NewDomain); err != nil {
		return err
	}
	if err := mustNonEmpty("dir", a.Dir); err != nil {
		return err
	}
	if err := mustNotControl("new_domain", a.NewDomain); err != nil {
		return err
	}
	if err := mustNotControl("subdomain", a.Subdomain); err != nil {
		return err
	}
	if err := mustNotControl("dir", a.Dir); err != nil {
		return err
	}
	return nil
}

// Validate runs the shape check for the subdomain creation args.
func (a CreateSubdomainArgs) Validate() error {
	if err := mustNonEmpty("domain", a.Domain); err != nil {
		return err
	}
	if err := mustNonEmpty("rootdomain", a.RootDomain); err != nil {
		return err
	}
	if err := mustNonEmpty("dir", a.Dir); err != nil {
		return err
	}
	if err := mustNotControl("domain", a.Domain); err != nil {
		return err
	}
	if err := mustNotControl("rootdomain", a.RootDomain); err != nil {
		return err
	}
	if err := mustNotControl("dir", a.Dir); err != nil {
		return err
	}
	return nil
}

// Validate runs the shape check for the Passenger app creation
// args. Envvars are validated key-by-key so a single bad pair
// surfaces with its key name rather than as a generic "invalid
// args" message.
func (a CreatePassengerAppArgs) Validate() error {
	if err := mustNonEmpty("name", a.Name); err != nil {
		return err
	}
	if err := mustNonEmpty("path", a.Path); err != nil {
		return err
	}
	if err := mustNonEmpty("domain", a.Domain); err != nil {
		return err
	}
	if err := mustNotControl("name", a.Name); err != nil {
		return err
	}
	if err := mustNotControl("path", a.Path); err != nil {
		return err
	}
	if err := mustNotControl("domain", a.Domain); err != nil {
		return err
	}
	if err := mustNotControl("base_uri", a.BaseURI); err != nil {
		return err
	}
	if err := mustNotControl("deployment_mode", a.DeploymentMode); err != nil {
		return err
	}
	for k, v := range a.Envvars {
		if err := mustNotControl("envvar."+k, v); err != nil {
			return err
		}
		if err := mustNotControl("envvar key", k); err != nil {
			return err
		}
	}
	return nil
}

// Validate runs the shape check for the Passenger app edit args.
// The Path selector is the only mandatory field; every other slot
// MAY be empty (meaning "keep the current value").
func (a EditPassengerAppArgs) Validate() error {
	if err := mustNonEmpty("path", a.Path); err != nil {
		return err
	}
	if err := mustNotControl("path", a.Path); err != nil {
		return err
	}
	if err := mustNotControl("domain", a.Domain); err != nil {
		return err
	}
	if err := mustNotControl("base_uri", a.BaseURI); err != nil {
		return err
	}
	if err := mustNotControl("deployment_mode", a.DeploymentMode); err != nil {
		return err
	}
	for k, v := range a.Envvars {
		if err := mustNotControl("envvar."+k, v); err != nil {
			return err
		}
		if err := mustNotControl("envvar key", k); err != nil {
			return err
		}
	}
	return nil
}

// Validate runs the shape check for the privilege grant args.
func (a MysqlPrivilegesArgs) Validate() error {
	if err := mustNonEmpty("dbname", a.Database); err != nil {
		return err
	}
	if err := mustNonEmpty("user", a.User); err != nil {
		return err
	}
	if len(a.Privileges) == 0 {
		return fmt.Errorf("%w: privileges must include at least one entry", ErrInvalidArgs)
	}
	for _, p := range a.Privileges {
		if err := mustNotControl("privilege", p); err != nil {
			return err
		}
	}
	return nil
}

// Validate runs the shape check for the manual SSL install args.
func (a InstallSSLArgs) Validate() error {
	if err := mustNonEmpty("domain", a.Domain); err != nil {
		return err
	}
	if err := mustNonEmpty("cert", a.Cert); err != nil {
		return err
	}
	if err := mustNonEmpty("key", a.Key); err != nil {
		return err
	}
	return nil
}

// Mutator is the Sprint-22 typed mutating surface. Every method
// is implementation-agnostic — the adapter consumes [Mutator] and
// receives whichever transport (HTTPS, SSH, composite) the wiring
// step assembled.
//
// Every method short-circuits when the env-var guard
// [MutationsAllowed] returns false, surfacing [ErrMutationsDisabled]
// before any I/O. The check happens at method entry so a
// long-running adapter process cannot silently start mutating just
// because the operator updated the env after startup; restart is
// required.
type Mutator interface {
	// Domain ops.
	AddAddonDomain(ctx context.Context, args CreateAddonDomainArgs) error
	AddSubdomain(ctx context.Context, args CreateSubdomainArgs) error
	DeleteDomain(ctx context.Context, domain string) error
	DeleteSubdomain(ctx context.Context, fqSubdomain string) error

	// Passenger app ops.
	CreatePassengerApp(ctx context.Context, args CreatePassengerAppArgs) error
	EditPassengerApp(ctx context.Context, args EditPassengerAppArgs) error
	RestartPassengerApp(ctx context.Context, appPath string) error
	DeletePassengerApp(ctx context.Context, appPath string) error

	// MySQL ops.
	CreateMysqlDatabase(ctx context.Context, dbName string) error
	DeleteMysqlDatabase(ctx context.Context, dbName string) error
	CreateMysqlUser(ctx context.Context, user, password string) error
	DeleteMysqlUser(ctx context.Context, user string) error
	SetMysqlPrivileges(ctx context.Context, args MysqlPrivilegesArgs) error

	// SSL ops.
	InstallSSL(ctx context.Context, args InstallSSLArgs) error
	StartAutoSSL(ctx context.Context, domain string) error
	DeleteSSL(ctx context.Context, host string) error
}

// argsForAddAddonDomain renders the typed args into the
// query-param map the transport expects. Pulled out as a free
// function so HTTPS and SSH mutators share one source of truth.
func argsForAddAddonDomain(a CreateAddonDomainArgs) map[string]string {
	out := map[string]string{
		"newdomain": a.NewDomain,
		"dir":       a.Dir,
	}
	if a.Subdomain != "" {
		out["subdomain"] = a.Subdomain
	}
	return out
}

func argsForAddSubdomain(a CreateSubdomainArgs) map[string]string {
	return map[string]string{
		"domain":     a.Domain,
		"rootdomain": a.RootDomain,
		"dir":        a.Dir,
	}
}

func argsForDeleteDomain(domain string) map[string]string {
	return map[string]string{"domain": domain}
}

func argsForCreatePassengerApp(a CreatePassengerAppArgs) map[string]string {
	out := map[string]string{
		"name":   a.Name,
		"path":   a.Path,
		"domain": a.Domain,
	}
	if a.BaseURI != "" {
		out["base_uri"] = a.BaseURI
	}
	if a.DeploymentMode != "" {
		out["deployment_mode"] = a.DeploymentMode
	}
	for k, v := range a.Envvars {
		out["envvar."+k] = v
	}
	return out
}

func argsForEditPassengerApp(a EditPassengerAppArgs) map[string]string {
	out := map[string]string{"path": a.Path}
	if a.Domain != "" {
		out["domain"] = a.Domain
	}
	if a.BaseURI != "" {
		out["base_uri"] = a.BaseURI
	}
	if a.DeploymentMode != "" {
		out["deployment_mode"] = a.DeploymentMode
	}
	for k, v := range a.Envvars {
		out["envvar."+k] = v
	}
	return out
}

func argsForRestartPassengerApp(path string) map[string]string {
	return map[string]string{"path": path}
}

func argsForDeletePassengerApp(path string) map[string]string {
	return map[string]string{"path": path}
}

func argsForCreateMysqlDatabase(name string) map[string]string {
	return map[string]string{"name": name}
}

func argsForDeleteMysqlDatabase(name string) map[string]string {
	return map[string]string{"name": name}
}

func argsForCreateMysqlUser(user, password string) map[string]string {
	return map[string]string{"name": user, "password": password}
}

func argsForDeleteMysqlUser(user string) map[string]string {
	return map[string]string{"name": user}
}

func argsForSetMysqlPrivileges(a MysqlPrivilegesArgs) map[string]string {
	return map[string]string{
		"user":       a.User,
		"database":   a.Database,
		"privileges": strings.Join(a.Privileges, ","),
	}
}

func argsForInstallSSL(a InstallSSLArgs) map[string]string {
	out := map[string]string{
		"domain": a.Domain,
		"cert":   a.Cert,
		"key":    a.Key,
	}
	if a.CABundle != "" {
		out["cabundle"] = a.CABundle
	}
	return out
}

func argsForStartAutoSSL(domain string) map[string]string {
	return map[string]string{"domain": domain}
}

func argsForDeleteSSL(host string) map[string]string {
	return map[string]string{"host": host}
}

// mustNonEmpty wraps the empty-string check around
// [ErrInvalidArgs] with the field name. Pulled out as a helper so
// Validate methods read top-to-bottom without a stack of
// `if x == "" { return ... }` blocks.
func mustNonEmpty(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: %s is required", ErrInvalidArgs, field)
	}
	return nil
}

// mustNotControl rejects ASCII control characters (newline,
// carriage return, NUL, etc.) so a stray byte cannot reach the
// transport. Real cPanel inputs never include control characters;
// surfacing them as [ErrInvalidArgs] is a defence-in-depth check.
func mustNotControl(field, value string) error {
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: %s contains control character", ErrInvalidArgs, field)
		}
	}
	return nil
}

// classifyMutationError maps the package-level transport / API
// errors onto the panel-level idempotency sentinels (resource
// exists / not found). This lets the adapter call sites use a
// simple errors.Is check rather than scraping error messages.
//
// The mapping is intentionally narrow: only well-documented
// cPanel error phrases participate (verified against
// api.docs.cpanel.net public examples). Anything else surfaces
// verbatim so silent misclassification cannot mask a real bug.
func classifyMutationError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrResourceExists) || errors.Is(err, ErrResourceNotFound) {
		return err
	}
	if !errors.Is(err, ErrAPIResultFailure) {
		return err
	}
	low := strings.ToLower(err.Error())
	switch {
	case strings.Contains(low, "already exists"),
		strings.Contains(low, "already in use"),
		strings.Contains(low, "is in use"),
		strings.Contains(low, "duplicate"):
		return fmt.Errorf("%w: %w", ErrResourceExists, err)
	case strings.Contains(low, "not exist"),
		strings.Contains(low, "no such"),
		strings.Contains(low, "not found"),
		strings.Contains(low, "does not exist"),
		// SSL.delete_ssl returns "There is no SSL certificate
		// installed for ..." when called on a host that never
		// had a cert; this is the documented idempotency
		// signal even though the wording is unusual.
		strings.Contains(low, "no ssl certificate"),
		strings.Contains(low, "is not installed"):
		return fmt.Errorf("%w: %w", ErrResourceNotFound, err)
	}
	return err
}

// sortedKeys returns the map keys in deterministic order. Used by
// fixture-driven tests so a `map[string]string` arg renders
// identically across runs.
func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
