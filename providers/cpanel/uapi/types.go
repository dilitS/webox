package uapi

// envelope is the shape every UAPI v1 response shares. cPanel's
// documented schema (api.docs.cpanel.net) sets `status` to 1 on
// success, 0 on failure, with `errors` populated on the failure
// path. The transport decodes this envelope and hands `data` /
// `metadata` to the module-specific decoder so each typed response
// stays focused on the module's data, not the boilerplate.
//
// data is decoded as json.RawMessage so module decoders can use
// their own typed shape without paying for the runtime cost of
// re-encoding/decoding a generic map.
type envelope struct {
	// Result wraps the top-level fields cPanel returns for every
	// UAPI call. Embedding it here matches the JSON layout
	// directly without an extra unmarshal step.
	Result resultEnvelope `json:"result"`
}

// resultEnvelope is the inner object cPanel wraps every UAPI
// response in. Status is 1 on success, 0 on failure; Errors is
// populated only on the failure path. Data carries the typed
// payload as json.RawMessage so module decoders can unmarshal into
// their own shape without paying for an extra round trip through
// a generic map.
type resultEnvelope struct {
	Status   int            `json:"status"`
	Errors   []string       `json:"errors,omitempty"`
	Warnings []string       `json:"warnings,omitempty"`
	Messages []string       `json:"messages,omitempty"`
	Data     jsonRawMessage `json:"data,omitempty"`
	Metadata jsonRawMessage `json:"metadata,omitempty"`
}

// jsonRawMessage is an aliased type so the package never imports
// `encoding/json` from any non-transport file. transport.go owns
// every JSON encode/decode boundary.
type jsonRawMessage []byte

// UnmarshalJSON / MarshalJSON make jsonRawMessage drop-in
// compatible with json.RawMessage without making the public API
// surface depend on encoding/json types.
func (j *jsonRawMessage) UnmarshalJSON(data []byte) error {
	if j == nil {
		return nil
	}
	*j = append((*j)[:0], data...)
	return nil
}

// MarshalJSON returns the raw bytes verbatim (or `null` when nil),
// matching encoding/json.RawMessage semantics exactly.
func (j jsonRawMessage) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("null"), nil
	}
	return j, nil
}

// Module enumerates the four read-only modules Sprint 21 ships. The
// type is a typed string so callers can't accidentally pass a
// mutating module like `Email`/`Cron`/`Pkg`. Adding a new module
// here requires updating the public read-only client too — the type
// system stops silent expansion.
type Module string

// Recognised UAPI modules. Sprint 21 scope: DomainInfo,
// PassengerApps, Mysql, SSL. Sprint 22 adds SubDomain for the
// addsubdomain / delsubdomain pair (the addon-domain endpoints
// live under DomainInfo). Each maps to a small set of functions
// documented inline next to the typed response below.
const (
	ModuleDomainInfo    Module = "DomainInfo"
	ModulePassengerApps Module = "PassengerApps"
	ModuleMysql         Module = "Mysql"
	ModuleSSL           Module = "SSL"
	ModuleSubDomain     Module = "SubDomain"
)

// AllReadOnlyModules returns the closed set of modules the Sprint 21
// read-only client supports. Useful for table-driven tests and for
// the future `webox doctor cpanel` to enumerate every probe it
// performs.
func AllReadOnlyModules() []Module {
	return []Module{ModuleDomainInfo, ModulePassengerApps, ModuleMysql, ModuleSSL}
}

// Function is a typed function name within a module. We use Go
// constants instead of free-form strings so a typo (`list_certs` vs
// `list_keys`) is a compile error rather than a runtime 400.
type Function string

// Recognised UAPI functions inside the four Sprint 21 modules. The
// inline comments document the function's read-only nature; the
// type system also refuses to compile a Call() against any other
// function for these modules.
const (
	// FunctionDomainInfoList — returns the account's domains
	// (primary + addon + sub) with document roots and main
	// domain flags. Read-only.
	FunctionDomainInfoList Function = "list_domains"
	// FunctionPassengerAppsList — returns the active Passenger
	// applications with name, deployment paths, environment,
	// and the controlling user. Read-only.
	FunctionPassengerAppsList Function = "list_applications"
	// FunctionMysqlListDatabases — returns the MySQL databases
	// available to the account, including disk usage.
	// Read-only.
	FunctionMysqlListDatabases Function = "list_databases"
	// FunctionSSLListKeys — returns the SSL key pairs that exist
	// for the account; consumed by the cockpit's SSL tile. Each
	// key has a friendly name, modulus length, and the matching
	// CRT's NotAfter date for renewal scheduling. Read-only.
	FunctionSSLListKeys Function = "list_keys"
)

// Sprint 22 mutating function names. Every constant below maps
// 1:1 to a documented UAPI endpoint (api.docs.cpanel.net). The
// type system refuses to substitute these into [Mutator] calls
// outside the expected module — keeping a typo (`creat_database`
// vs `create_database`) a compile error rather than a 400 at
// runtime.
const (
	// FunctionDomainInfoAddAddonDomain provisions a new top-level
	// domain as an "addon" under the account. Args: `newdomain`,
	// `subdomain`, `dir`. cPanel UAPI v1 reference:
	// DomainInfo::add_addon_domain.
	FunctionDomainInfoAddAddonDomain Function = "add_addon_domain"

	// FunctionDomainInfoDelDomain removes a previously-provisioned
	// addon or subdomain. Args: `domain`. Idempotent on the panel
	// side; this client maps the "domain not found" error onto
	// [ErrResourceNotFound] so the adapter can treat it as nil.
	FunctionDomainInfoDelDomain Function = "del_domain"

	// FunctionSubdomainAdd provisions a subdomain under one of
	// the account's primary domains. Args: `domain` (the leftmost
	// label), `rootdomain`, `dir`.
	FunctionSubdomainAdd Function = "addsubdomain"

	// FunctionSubdomainDel removes a subdomain. Args: `domain`
	// (the fully-qualified subdomain). Idempotent.
	FunctionSubdomainDel Function = "delsubdomain"

	// FunctionPassengerAppsCreate registers a Passenger / Node.js
	// application against the panel. Args: `name`, `path`,
	// `domain`, `deployment_mode`, `base_uri`, plus optional
	// `envvars` flattened as `envvar.<key>=<value>` query params.
	FunctionPassengerAppsCreate Function = "create_application" //nolint:gosec // G101: UAPI function name, not a credential.

	// FunctionPassengerAppsEdit updates an existing application
	// (typically `envvars`). Same args as create + the existing
	// `path` selector.
	FunctionPassengerAppsEdit Function = "edit_application"

	// FunctionPassengerAppsRestart triggers a Passenger graceful
	// restart for the application at the given `path`.
	FunctionPassengerAppsRestart Function = "restart_application" //nolint:gosec // G101: UAPI function name, not a credential.

	// FunctionPassengerAppsDelete deregisters the application at
	// `path`. Does NOT remove the on-disk source tree — adapter
	// callers MUST orchestrate filesystem cleanup separately
	// when the rollback path requires it.
	FunctionPassengerAppsDelete Function = "delete_application" //nolint:gosec // G101: UAPI function name, not a credential.

	// FunctionMysqlCreateDatabase provisions a fresh MySQL
	// database. cPanel prefixes the database name with the
	// account user; this client passes the operator-supplied
	// name verbatim and the adapter handles the prefix policy.
	FunctionMysqlCreateDatabase Function = "create_database"

	// FunctionMysqlDeleteDatabase drops a database. Idempotent.
	FunctionMysqlDeleteDatabase Function = "delete_database"

	// FunctionMysqlCreateUser provisions a MySQL user with the
	// supplied password. The password travels as a query param;
	// the transport's User-Agent and Authorization headers are
	// the only items logged by the project's redactor, and
	// `webox doctor` never echoes the password back to stderr.
	FunctionMysqlCreateUser Function = "create_user"

	// FunctionMysqlDeleteUser drops a MySQL user. Idempotent.
	FunctionMysqlDeleteUser Function = "delete_user"

	// FunctionMysqlSetPrivileges grants the supplied list of
	// privileges (`ALL PRIVILEGES`, `SELECT,INSERT`, …) on the
	// (database, user) pair. UAPI accepts the list as a single
	// comma-separated string.
	FunctionMysqlSetPrivileges Function = "set_privileges_on_database"

	// FunctionSSLInstallSSL installs a supplied PEM cert + key
	// (+ optional CA chain) on the given host. The adapter does
	// not call this directly in Sprint 22 — see [FunctionSSLStartAutoSSL]
	// for the AutoSSL path, which is the default cPanel SSL
	// provider on shared hosting.
	FunctionSSLInstallSSL Function = "install_ssl"

	// FunctionSSLStartAutoSSL triggers AutoSSL provisioning
	// for the given account. cPanel orchestrates Let's Encrypt
	// behind the scenes; the call returns immediately while the
	// cert request runs asynchronously. Callers poll
	// [Reader.ListSSLKeys] (or follow up with a status probe) to
	// confirm the cert landed.
	FunctionSSLStartAutoSSL Function = "start_autossl_check"

	// FunctionSSLDeleteSSL revokes the installed cert for the
	// given host. Idempotent.
	FunctionSSLDeleteSSL Function = "delete_ssl"
)

// DomainInfoListResponse is the typed payload returned by
// `Client.ListDomains`. Field documentation mirrors
// api.docs.cpanel.net; comment changes propagate to the README under
// docs/providers/preconfiguration-vision.md.
type DomainInfoListResponse struct {
	// MainDomain is the primary domain associated with the account.
	MainDomain string `json:"main_domain"`
	// SubDomains is the slice of non-primary subdomains under
	// the main domain.
	SubDomains []string `json:"sub_domains"`
	// AddonDomains is the slice of additional top-level domains
	// pointed at the account.
	AddonDomains []string `json:"addon_domains"`
	// ParkedDomains is the slice of alias / parked domains.
	ParkedDomains []string `json:"parked_domains"`
}

// PassengerAppsListResponse is the typed payload returned by
// `Client.ListPassengerApps`.
type PassengerAppsListResponse struct {
	Applications []PassengerApp `json:"applications"`
}

// PassengerApp is one Passenger / Node.js application row.
type PassengerApp struct {
	Name         string            `json:"name"`
	Path         string            `json:"path"`
	Domain       string            `json:"domain"`
	Enabled      bool              `json:"enabled"`
	Status       string            `json:"status"`
	BaseURI      string            `json:"base_uri"`
	EnvironmentX map[string]string `json:"envvars,omitempty"`
}

// MysqlListDatabasesResponse is the typed payload returned by
// `Client.ListMysqlDatabases`.
type MysqlListDatabasesResponse struct {
	Databases []MysqlDatabase `json:"databases"`
}

// MysqlDatabase is one MySQL database row. cPanel exposes both
// `name` and `db` aliases; the decoder maps both to Name so callers
// can ignore the historical inconsistency.
type MysqlDatabase struct {
	Name      string `json:"name"`
	DiskUsage int64  `json:"disk_usage"`
	UserCount int    `json:"user_count,omitempty"`
}

// SSLListKeysResponse is the typed payload returned by
// `Client.ListSSLKeys`.
type SSLListKeysResponse struct {
	Keys []SSLKey `json:"keys"`
}

// SSLKey is one SSL key/cert pair row. The NotAfter field is the
// caller's hook for SSL-renewal scheduling — empty means cPanel did
// not provide an explicit not-after, in which case the cockpit
// falls back to the `friendly_name` ageing heuristic.
type SSLKey struct {
	FriendlyName  string `json:"friendly_name"`
	Modulus       string `json:"modulus,omitempty"`
	ModulusLength int    `json:"modulus_length,omitempty"`
	HostName      string `json:"host"`
	NotAfter      string `json:"not_after,omitempty"`
}
