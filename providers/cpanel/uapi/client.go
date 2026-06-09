package uapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Client is the read-only UAPI client. Construction validates the
// endpoint URL and credentials up front so a caller that
// successfully gets a *Client knows it can issue calls without
// runtime configuration errors. Every method takes a context so
// the caller controls timeouts and cancellation — the package
// itself never sets a context deadline.
type Client struct {
	transport *transport
}

// NewClient validates the endpoint (must be https://) and
// credentials, then returns a ready-to-use client. The optional
// httpClient lets callers wire their own *http.Client (proxy,
// custom TLS roots, etc.); nil keeps the package default with a
// 30 s overall timeout.
func NewClient(baseURL, user, token string, httpClient *http.Client) (*Client, error) {
	tr, err := newTransport(baseURL, user, token, httpClient)
	if err != nil {
		return nil, err
	}
	return &Client{transport: tr}, nil
}

// ListDomains calls UAPI DomainInfo::list_domains. Returns a typed
// [DomainInfoListResponse] on success; on failure the error wraps
// one of the package-level sentinels (ErrAuthenticationFailed,
// ErrRateLimited, ErrServerError, ErrMalformedResponse,
// ErrModuleFunctionDenied).
func (c *Client) ListDomains(ctx context.Context) (*DomainInfoListResponse, error) {
	env, err := c.transport.call(ctx, ModuleDomainInfo, FunctionDomainInfoList)
	if err != nil {
		return nil, err
	}
	out := &DomainInfoListResponse{}
	if err := json.Unmarshal(env.Result.Data, out); err != nil {
		return nil, fmt.Errorf("%w: DomainInfo.list_domains: %w", ErrMalformedResponse, err)
	}
	return out, nil
}

// ListPassengerApps calls UAPI PassengerApps::list_applications.
// Returns a typed [PassengerAppsListResponse] on success. cPanel
// historically returned the applications as an object keyed by app
// name; recent versions return an array. The decoder accepts both
// shapes transparently.
func (c *Client) ListPassengerApps(ctx context.Context) (*PassengerAppsListResponse, error) {
	env, err := c.transport.call(ctx, ModulePassengerApps, FunctionPassengerAppsList)
	if err != nil {
		return nil, err
	}
	apps, err := decodePassengerApps(env.Result.Data)
	if err != nil {
		return nil, fmt.Errorf("%w: PassengerApps.list_applications: %w", ErrMalformedResponse, err)
	}
	return &PassengerAppsListResponse{Applications: apps}, nil
}

// ListMysqlDatabases calls UAPI Mysql::list_databases. Returns a
// typed [MysqlListDatabasesResponse] on success.
func (c *Client) ListMysqlDatabases(ctx context.Context) (*MysqlListDatabasesResponse, error) {
	env, err := c.transport.call(ctx, ModuleMysql, FunctionMysqlListDatabases)
	if err != nil {
		return nil, err
	}
	dbs, err := decodeMysqlDatabases(env.Result.Data)
	if err != nil {
		return nil, fmt.Errorf("%w: Mysql.list_databases: %w", ErrMalformedResponse, err)
	}
	return &MysqlListDatabasesResponse{Databases: dbs}, nil
}

// ListSSLKeys calls UAPI SSL::list_keys. Returns a typed
// [SSLListKeysResponse] on success.
func (c *Client) ListSSLKeys(ctx context.Context) (*SSLListKeysResponse, error) {
	env, err := c.transport.call(ctx, ModuleSSL, FunctionSSLListKeys)
	if err != nil {
		return nil, err
	}
	keys, err := decodeSSLKeys(env.Result.Data)
	if err != nil {
		return nil, fmt.Errorf("%w: SSL.list_keys: %w", ErrMalformedResponse, err)
	}
	return &SSLListKeysResponse{Keys: keys}, nil
}

// Transport satisfies [Reader] and returns the constant "HTTPS"
// (this client only ever issues HTTPS requests against the UAPI
// surface). The constant lives here rather than in callers to keep
// transport identity owned by the implementation, not the consumer.
func (*Client) Transport() string { return "HTTPS" }

// MutatingClient is the placeholder interface for the eventually
// mutating UAPI surface (account, email, cron, etc.). Sprint 21
// deliberately leaves the method body returning
// [ErrSprintScopeNotMutable] so the type system enforces the
// "no destructive ops in v0.2-rc" guardrail. Sprint 22 wires the
// real implementation behind a second guard (env var + ADR sign-off).
type MutatingClient interface {
	// Call issues a single mutating UAPI request. Sprint 21:
	// always returns ErrSprintScopeNotMutable.
	Call(ctx context.Context, module Module, function Function, args map[string]string) error
}

// stubMutatingClient is the canonical Sprint-21 implementation: it
// always returns the typed scope error. New Sprint-22 callers can
// detect a stub via errors.Is(err, ErrSprintScopeNotMutable) and
// degrade gracefully.
type stubMutatingClient struct{}

// NewStubMutatingClient returns a MutatingClient that always
// returns [ErrSprintScopeNotMutable]. Sprint 22 replaces this with
// the real implementation; in the meantime callers that need to
// satisfy the interface in tests can use this stub directly.
func NewStubMutatingClient() MutatingClient { return stubMutatingClient{} }

// Call always returns ErrSprintScopeNotMutable on the Sprint-21
// stub. The receiver is value-typed (no state) so test code can
// allocate one on the fly without bookkeeping.
func (stubMutatingClient) Call(_ context.Context, _ Module, _ Function, _ map[string]string) error {
	return ErrSprintScopeNotMutable
}
