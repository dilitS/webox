package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
)

// Reader is the closed read-only interface implemented by the
// HTTPS Client, the SSH fallback (TASK-23.2), and the composite
// layer that prefers HTTPS with SSH fall-over. Callers (the
// `webox doctor directadmin` CLI; the Sprint 24+ adapter) depend
// only on this interface, never on the concrete types — so the
// transport choice stays a runtime decision driven by the
// operator's flags.
//
// Method-count discipline: 5 methods is the budget. Adding a 6th
// requires a sprint-plan note + an updated `var _ Reader = (*X)(nil)`
// assertion in every implementation. Sprint 23 covers the
// read-only diagnostics; mutating methods land in a separate
// `Mutator` interface in Sprint 24 to keep the read/write
// surfaces clearly partitioned (same shape as cpanel/uapi).
type Reader interface {
	Whoami(ctx context.Context) (*WhoamiResponse, error)
	ListDomains(ctx context.Context) ([]Domain, error)
	ListSubdomains(ctx context.Context) ([]Subdomain, error)
	ListDatabases(ctx context.Context) ([]Database, error)
	ListSSLCertificates(ctx context.Context) ([]SSLCertificate, error)

	// Transport returns a short, stable label for the transport
	// powering this Reader. Used by `webox doctor directadmin` to
	// render the transport hint next to each section without a
	// runtime type-switch in the caller. Stable values: "HTTPS",
	// "SSH", "HTTPS+SSH" (composite with both legs wired), "?"
	// (composite with neither wired — a programmer error).
	Transport() string
}

// Client is the HTTPS implementation of Reader. Constructed via
// [NewClient]; do not instantiate the struct directly because the
// constructor performs critical URL / credential validation that
// the type system can't enforce.
//
// Concurrency: safe for parallel use across goroutines once
// constructed. The underlying http.Client owns its own mutexes,
// and the transport struct treats user/loginKey as immutable
// after construction.
type Client struct {
	t *transport
}

// Compile-time guarantee that Client satisfies the Reader contract.
// The assertion will refuse to build if a method gets added to
// Reader without a matching Client method.
var _ Reader = (*Client)(nil)

// NewClient builds an HTTPS DA Live API client.
//
//   - baseURL must be `https://<host>:<port>` (typically port 2222).
//     The `/api/` prefix is appended by the transport for every call.
//   - user is the panel account login (NOT email).
//   - loginKey is the DA "Login Key" generated under
//     Manage Login Keys; never the account password.
//   - httpClient is optional. Pass nil to get the package's default
//     30s-timeout client; pass a configured one to share connection
//     pools or pin TLS roots.
//
// Returns ErrMissingCredentials if user or loginKey is empty;
// ErrInvalidEndpoint if baseURL is malformed or uses http://.
func NewClient(baseURL, user, loginKey string, httpClient *http.Client) (*Client, error) {
	tr, err := newTransport(baseURL, user, loginKey, httpClient)
	if err != nil {
		return nil, err
	}
	return &Client{t: tr}, nil
}

// Whoami issues an authenticated probe against the cheapest
// endpoint DA exposes. `webox doctor directadmin` calls it first
// to surface auth failures before the per-section probes hit
// the wire.
func (c *Client) Whoami(ctx context.Context) (*WhoamiResponse, error) {
	body, err := c.t.call(ctx, c.t.endpointURL(EndpointWhoami))
	if err != nil {
		return nil, err
	}
	out := &WhoamiResponse{}
	if err := json.Unmarshal(body, out); err != nil {
		return nil, fmt.Errorf("%w: whoami: %w", ErrMalformedResponse, err)
	}
	return out, nil
}

// ListDomains returns the user's owned domains (primary + addon).
// The result is stable-sorted by Name so test fixtures stay
// identical across DA versions even if the panel reshuffles the
// underlying storage order.
func (c *Client) ListDomains(ctx context.Context) ([]Domain, error) {
	body, err := c.t.call(ctx, c.t.endpointURL(EndpointListDomains, c.t.user))
	if err != nil {
		return nil, err
	}
	out, err := decodeList[Domain](body, "domains")
	if err != nil {
		return nil, fmt.Errorf("%w: list_domains: %w", ErrMalformedResponse, err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ListSubdomains returns the user's subdomains.
func (c *Client) ListSubdomains(ctx context.Context) ([]Subdomain, error) {
	body, err := c.t.call(ctx, c.t.endpointURL(EndpointListSubdomains, c.t.user))
	if err != nil {
		return nil, err
	}
	out, err := decodeList[Subdomain](body, "subdomains")
	if err != nil {
		return nil, fmt.Errorf("%w: list_subdomains: %w", ErrMalformedResponse, err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ListDatabases returns the MySQL / MariaDB databases owned by
// the user. The DA Live API uses a flat array shape; the decoder
// accepts both bare arrays and wrapper objects in case a future
// DA version changes the response shape.
func (c *Client) ListDatabases(ctx context.Context) ([]Database, error) {
	// DA's documented endpoint actually uses literal `{user}`
	// substitution in some installs, but the canonical path is
	// `/api/users/<user>/databases`. We hand the username
	// through %s and ignore the documented placeholder.
	const databasesPath Endpoint = "users/%s/databases"
	body, err := c.t.call(ctx, c.t.endpointURL(databasesPath, c.t.user))
	if err != nil {
		return nil, err
	}
	out, err := decodeList[Database](body, "databases")
	if err != nil {
		return nil, fmt.Errorf("%w: list_databases: %w", ErrMalformedResponse, err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ListSSLCertificates returns the SSL certs installed for the
// user's domains.
func (c *Client) ListSSLCertificates(ctx context.Context) ([]SSLCertificate, error) {
	const sslPath Endpoint = "users/%s/ssl/certificates"
	body, err := c.t.call(ctx, c.t.endpointURL(sslPath, c.t.user))
	if err != nil {
		return nil, err
	}
	out, err := decodeList[SSLCertificate](body, "certificates")
	if err != nil {
		return nil, fmt.Errorf("%w: list_ssl_certificates: %w", ErrMalformedResponse, err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Domain < out[j].Domain })
	return out, nil
}

// Transport satisfies [Reader] and returns the constant "HTTPS"
// (this client only ever issues HTTPS requests against the Live
// API surface). The constant lives on the implementation, not the
// caller, so transport identity stays owned by the type that
// implements it.
func (*Client) Transport() string { return "HTTPS" }

// decodeList is the generic shape-tolerant decoder DA's responses
// need. Three shapes seen in the wild:
//
//  1. Wrapper object with a typed array under a known key:
//     `{"domains":[{...},{...}], "success":true}`. Modern DA.
//  2. Wrapper under a generic "data" key:
//     `{"data":[{...},{...}], "success":true}`. Some installs.
//  3. Bare top-level array: `[{...},{...}]`. Legacy CMD-aliased
//     Live API endpoints.
//
// The decoder tries (1) with the caller-supplied wrapperKey,
// then (2) with the generic key, then (3) as a bare array. The
// first successful parse wins. If none of the three shapes
// decode, the function returns the wrapped json error so the
// caller can attach the endpoint name + ErrMalformedResponse.
//
// We use sub-decoders (one per shape) rather than a single
// reflect-based attempt because the json package's default
// behaviour silently zeroes fields on the unmatched shape, which
// would mask a genuine schema drift.
func decodeList[T any](body []byte, wrapperKey string) ([]T, error) {
	if len(body) == 0 {
		return nil, nil
	}
	// Shape 1: typed wrapper key.
	if wrapperKey != "" {
		var typed map[string]json.RawMessage
		if err := json.Unmarshal(body, &typed); err == nil {
			if raw, ok := typed[wrapperKey]; ok {
				var out []T
				if err := json.Unmarshal(raw, &out); err == nil {
					return out, nil
				}
			}
		}
	}
	// Shape 2: generic "data" wrapper.
	var env envelope[T]
	if err := json.Unmarshal(body, &env); err == nil && env.Data != nil {
		return env.Data, nil
	}
	// Shape 3: bare array.
	var bare []T
	if err := json.Unmarshal(body, &bare); err == nil {
		return bare, nil
	}
	return nil, fmt.Errorf("%w: wrapper key %q, data wrapper, bare array", errUnsupportedListShape, wrapperKey)
}

// errUnsupportedListShape is the typed sentinel decodeList wraps
// when none of the three documented body shapes parse. Callers
// see this through the `%w: list_<endpoint>:` wrapping in each
// List* method; tests can `errors.Is` against it if needed.
var errUnsupportedListShape = errors.New("unsupported list response shape")
