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

## Sprint 22 mutating fixtures

Same source provenance as the read-only fixtures (research-derived from api.docs.cpanel.net plus published cPanel error catalogs). Replaced one-for-one when TASK-22.0 onboards the live test account.

| Fixture | Module / Function | Purpose |
|---|---|---|
| `mut_add_addon_domain_ok.json` | DomainInfo::add_addon_domain | Happy path; status:1 + success message. |
| `mut_add_addon_domain_exists.json` | DomainInfo::add_addon_domain | "already exists" → maps onto `ErrResourceExists`. |
| `mut_del_domain_ok.json` | DomainInfo::del_domain | Happy path delete. |
| `mut_del_domain_not_found.json` | DomainInfo::del_domain | "does not exist" → maps onto `ErrResourceNotFound` (idempotency). |
| `mut_create_passenger_app_ok.json` | PassengerApps::create_application | Returns the registered app metadata. |
| `mut_create_passenger_app_exists.json` | PassengerApps::create_application | "already in use" → `ErrResourceExists`. |
| `mut_restart_passenger_app_ok.json` | PassengerApps::restart_application | Graceful restart success. |
| `mut_create_mysql_db_ok.json` | Mysql::create_database | Account-prefixed DB created. |
| `mut_create_mysql_db_exists.json` | Mysql::create_database | "already exists" → `ErrResourceExists`. |
| `mut_create_mysql_user_ok.json` | Mysql::create_user | Account-prefixed user created. |
| `mut_set_mysql_privileges_ok.json` | Mysql::set_privileges_on_database | Privilege grant success. |
| `mut_start_autossl_ok.json` | SSL::start_autossl_check | AutoSSL triggered (async). |
| `mut_install_ssl_ok.json` | SSL::install_ssl | byo-cert install success. |
| `mut_delete_ssl_not_found.json` | SSL::delete_ssl | "no SSL installed" → `ErrResourceNotFound`. |

When the real account lands, replace these byte-for-byte with the live capture and rerun `WEBOX_CPANEL_MUTATIONS=1 go test ./providers/cpanel/uapi/...` to confirm the parser still passes. The transport never logs Authorization headers, so capturing live responses is safe to commit as long as the WHM account is dedicated to fixture work and **doesn't host customer data**.
