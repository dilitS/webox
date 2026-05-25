# cPanel UAPI golden fixtures

Source: cPanel UAPI v1 public docs (api.docs.cpanel.net) — research-derived. These shapes will be replaced one-for-one with captures from the real test account onboarded by **TASK-21.7**; until then they cover the full happy + edge path matrix exercised by `client_test.go`:

| Fixture | Module / Function | Purpose |
|---|---|---|
| `list_domains_ok.json` | DomainInfo::list_domains | Happy path with primary, sub, addon, parked. |
| `list_passenger_apps_ok.json` | PassengerApps::list_applications | Modern `applications: []` shape. |
| `list_passenger_apps_legacy.json` | PassengerApps::list_applications | Legacy map-keyed shape. |
| `list_mysql_databases_ok.json` | Mysql::list_databases | Two DBs with disk usage + user count. |
| `list_ssl_keys_ok.json` | SSL::list_keys | Two keys with `not_after` for renewal scheduling. |
| `error_module_denied.json` | (any) | `status:0` with the canonical "feature disabled" message. |
| `error_invalid_envelope.json` | (any) | `status:0` with a generic error (not disabled). |

When the real account lands, replace these byte-for-byte with the live capture and rerun `go test ./providers/cpanel/uapi/...` to confirm the parser still passes. The transport never logs Authorization headers, so capturing live responses is safe to commit as long as the WHM account is dedicated to fixture work and **doesn't host customer data**.
