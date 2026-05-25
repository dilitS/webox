// Package uapi is the typed cPanel UAPI client surfaced by Webox's
// cPanel adapter (Sprint 21 TASK-21.1). It is intentionally minimal:
// read-only operations against the four modules the Sprint 21 plan
// enumerates, HTTPS-only transport, basic auth via `cpanel:<token>`,
// and rate-limit-aware retry. Mutating endpoints are deliberately
// not callable from [Client]; the parallel [MutatingClient] interface
// returns [ErrSprintScopeNotMutable] so the type system enforces the
// "no destructive ops in v0.2-rc" guardrail.
//
// The transport never logs Authorization headers, never persists
// response bodies to disk, and never accepts plain HTTP (the
// constructor refuses non-https schemes). Per-call context cancels
// the underlying HTTP request immediately.
//
// Fixture set under testdata/ is research-derived (cPanel UAPI v1
// public docs) until Sprint 21 TASK-21.7 onboards a real test
// account; once fixtures land from a live account they replace the
// research-derived shapes one-for-one. The fixtures intentionally
// cover both the happy path (HTTP 200 + `result.status: 1`) and the
// edge cases the parser is expected to handle: 401 missing token,
// 429 rate limit, malformed JSON body, and module/function disabled.
package uapi
