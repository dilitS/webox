package api

// Endpoint is a typed wrapper for the path under `/api/` the
// client requests. Keeping the surface as named constants makes
// the call sites greppable and lets static analysers catch typos
// at compile-time rather than HTTP 404.
//
// DA's Live API is documented at https://docs.directadmin.com/developer/api/
// and bundled as a `swagger.json` on every install under
// `/static/swagger.json`. The constants below cover the read-only
// surface Sprint 23 needs; mutating endpoints land in Sprint 24+.
//
// We intentionally pin the unversioned `/api/...` path (no `/v1/`
// or `/v2/` prefix) because DA's documentation and the bundled
// Swagger spec disagree on versioning across releases. The
// unversioned path is what `da-cli` itself uses internally and
// what's been stable across DA 1.5x → 1.6x.
type Endpoint string

// Read-only endpoints. Each maps to a documented Live API call
// that returns a JSON array (or a Swagger-defined object wrapper
// — the decoder handles both shapes).
const (
	// EndpointListDomains lists the authenticated user's owned
	// domains (primary + addon). Maps to GET /api/users/{user}/domains
	// per DA's User API surface. Response: array of domain
	// descriptors or `{"domains":[...]}` wrapper depending on
	// DA version (decoder is shape-tolerant).
	EndpointListDomains Endpoint = "users/%s/domains"

	// EndpointListSubdomains lists subdomains owned by the
	// authenticated user. Maps to GET /api/users/{user}/subdomains.
	// Distinct endpoint from EndpointListDomains because DA
	// treats subdomains as first-class entities with separate
	// permissions and SSL policy.
	EndpointListSubdomains Endpoint = "users/%s/subdomains"

	// EndpointListDatabases lists the MySQL / MariaDB databases
	// owned by the authenticated user. DA Live API exposes a
	// flat list; SSH fallback (TASK-23.2) parses the legacy
	// `CMD_API_DATABASES` text format for older installs.
	EndpointListDatabases Endpoint = "users/{user}/databases"

	// EndpointListSSLCertificates lists installed SSL certs
	// (manual + Let's Encrypt). Useful for `webox doctor
	// directadmin` to surface expiring certs in the status feed.
	EndpointListSSLCertificates Endpoint = "users/{user}/ssl/certificates"

	// EndpointWhoami is a cheap auth-only probe. The doctor CLI
	// calls this first to surface auth failures with `AUTH_FAILED`
	// before issuing per-section probes. Per DA docs: GET /api/whoami
	// returns the authenticated user's id + scope info.
	EndpointWhoami Endpoint = "whoami"
)

// Domain describes a row in the EndpointListDomains response. DA
// returns a mixture of strings (domain name) and structured
// objects depending on version; the decoder normalises both into
// this shape with `Name` always populated.
//
// PHPSelector, SSL, etc. are populated when the row carries them;
// callers should NEVER rely on `nil`/`""` to mean "feature not
// available" — DA omits unset fields from the JSON to save bytes,
// so an absent field could mean "off" or "panel doesn't surface
// this field". The doctor CLI surfaces this distinction explicitly.
type Domain struct {
	Name           string `json:"domain"`
	Primary        bool   `json:"is_primary,omitempty"`
	SSLEnabled     bool   `json:"ssl,omitempty"`
	BandwidthBytes int64  `json:"bandwidth_bytes,omitempty"`
	QuotaBytes     int64  `json:"quota_bytes,omitempty"`
	Suspended      bool   `json:"suspended,omitempty"`
}

// Subdomain describes a row in the EndpointListSubdomains response.
// DA treats subdomains as ordinary domains for SSL/quota purposes
// but tracks them separately so primary-vs-addon distinction
// stays clean.
type Subdomain struct {
	Name       string `json:"subdomain"`
	Parent     string `json:"parent_domain"`
	SSLEnabled bool   `json:"ssl,omitempty"`
	Suspended  bool   `json:"suspended,omitempty"`
}

// Database describes a row in EndpointListDatabases.
//
// `User` is the OS-level MySQL user, NOT a Webox-managed
// credential. DA prefixes every MySQL user with the cPanel user
// (e.g. `alice_shopuser`) per its 8-char-prefix policy; the
// adapter's validator (Sprint 24) will enforce this on creation.
type Database struct {
	Name   string `json:"name"`
	User   string `json:"user,omitempty"`
	SizeMB int64  `json:"size_mb,omitempty"`
}

// SSLCertificate describes a row in EndpointListSSLCertificates.
// `LetsEncrypt` is true when DA's Let's Encrypt integration owns
// the cert; manual installs leave it false.
type SSLCertificate struct {
	Domain      string `json:"domain"`
	Issuer      string `json:"issuer,omitempty"`
	NotAfter    string `json:"not_after,omitempty"` // RFC 3339 in newer DA; "YYYY-MM-DD" in older.
	LetsEncrypt bool   `json:"letsencrypt,omitempty"`
}

// WhoamiResponse is the small payload from EndpointWhoami. DA
// returns the authenticated user's id + scope; we use it as a
// cheap "is the token still valid?" probe.
type WhoamiResponse struct {
	Username string   `json:"username"`
	UserType string   `json:"user_type,omitempty"` // "user", "reseller", "admin"
	Scopes   []string `json:"scopes,omitempty"`
}

// envelope is the generic shape DA wraps responses in when they
// don't return a top-level array. Some endpoints return
// `{"data": [...]}`, some return `{"users": [...]}`, some return
// the bare array. The decoder tries the wrapper shape first; if
// that fails, the raw body is re-parsed as the array directly.
type envelope[T any] struct {
	Data    []T  `json:"data,omitempty"`
	Success bool `json:"success,omitempty"`
}
