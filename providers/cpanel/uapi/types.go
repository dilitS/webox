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
// PassengerApps, Mysql, SSL. Each maps to a small set of read-only
// functions documented inline next to the typed response below.
const (
	ModuleDomainInfo    Module = "DomainInfo"
	ModulePassengerApps Module = "PassengerApps"
	ModuleMysql         Module = "Mysql"
	ModuleSSL           Module = "SSL"
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
