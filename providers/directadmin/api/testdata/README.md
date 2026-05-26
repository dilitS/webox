# DirectAdmin Live API fixtures

This directory holds golden JSON responses the read-only client
suite (`client_test.go`, `transport_test.go`) parses to assert
the decoder's shape tolerance.

## Provenance

| Fixture | Source | Status |
|---|---|---|
| `whoami_ok.json` | Constructed from DA Live API Swagger spec (public). | research |
| `list_domains_ok.json` | Constructed from DA Live API Swagger spec. | research |
| `list_domains_legacy_wrapper.json` | Constructed; documents the legacy `{"data": [...]}` wrapper shape some installs return. | research |
| `list_domains_bare_array.json` | Constructed; documents the bare top-level array shape. | research |
| `list_subdomains_ok.json` | Constructed from Swagger spec. | research |
| `list_databases_ok.json` | Constructed; reflects DA's user-prefixed database naming. | research |
| `list_ssl_certificates_ok.json` | Constructed; documents both LetsEncrypt + manual cert rows. | research |
| `error_api_disabled_503.json` | Constructed; matches DA's canonical "Live API disabled" 503 body. | research |
| `error_auth_failed.json` | Constructed; minimal 401 body. | research |

## Live capture

Once Sprint 24 procures a DA test account, the operator will
replace each `research`-status fixture with a live-captured
equivalent via `scripts/smoke-directadmin.sh` (mirror of
`smoke-cpanel.sh`). The status field will then flip to
`live-captured: <YYYY-MM-DD> <da-version>`.

## What's NOT here

- **Mutating ops** — Sprint 24+.
- **Legacy `/CMD_API_*` text format** — out of scope; `ErrAPIDisabled` surfaces the degraded panel up to the doctor CLI.
