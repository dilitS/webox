# Provider: directadmin

> Status: **READ-ONLY CLIENT SHIPPED (Sprint 23, v0.2 path)** · Ostatnia aktualizacja: 2026-05-27 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [providers/smallhost.md](./smallhost.md) (wzorzec), [providers/cpanel.md](./cpanel.md) (siblingsprintów 21+22), [CONTRIBUTING §3](../CONTRIBUTING.md#3-jak-doda%C4%87-nowy-provider).

## STATUS

**READ-ONLY CLIENT + DIAGNOSTIC CLI SHIPPED** w Sprincie 23 ([plan](../sprints/sprint-23-second-provider-or-launch.md), [retro](../retros/2026-05-27-sprint-23.md)). Aktualnie:

| Komponent | Status | Lokalizacja |
|---|---|---|
| Read-only Live API client | ✅ ✅ | [`providers/directadmin/api/client.go`](../../providers/directadmin/api/client.go) |
| SSH fallback (loopback curl) | ✅ | [`providers/directadmin/api/ssh.go`](../../providers/directadmin/api/ssh.go) |
| Composite (HTTPS-first, SSH fallover) | ✅ | [`providers/directadmin/api/composite.go`](../../providers/directadmin/api/composite.go) |
| `webox doctor directadmin` CLI | ✅ | [`cmd/webox/directadmin.go`](../../cmd/webox/directadmin.go) |
| `directadmin-generic` preset graduate research → candidate | ✅ | [`assets/provider-presets/directadmin-generic.json`](../../assets/provider-presets/directadmin-generic.json) |
| `providers.HostingProvider` adapter | ⏭️ Sprint 24 | — |
| Mutating client + env-var guard | ⏭️ Sprint 24 | — |
| Live test account + live fixture capture | ⏭️ Sprint 24 (operator-gated) | — |
| Wizard integration | ⏭️ Sprint 24 | — |

> Oryginalny PRD zawierał implementację `directadmin.go` napisaną z głowy, z `DA_API_KEY` jako **string literal w kodzie** — to było jednocześnie błędem implementacyjnym (sekret w pliku źródłowym!) i nieweryfikowalną propozycją API. Usunięte. Patrz [CHANGES.md poprawki 6.8 + 6.9](../../CHANGES.md#1-poprawki-merytoryczne-z-tabeli-6-briefu).

## TL;DR

DirectAdmin to popularny panel hostingowy w Europie i Azji, z dwoma API: **Legacy `/CMD_API_*`** (URL-encoded lub `?json=yes`) oraz nowsze **`/api/*`** (JSON, Swagger). Webox będzie celował w nowsze API. Autentykacja przez user/password lub API key. **API key trzymany w keyringu**, nigdy w kodzie ani configu — to absolutna reguła.

## Spis treści

1. [Źródła i oficjalna dokumentacja](#1-%C5%BAr%C3%B3d%C5%82a-i-oficjalna-dokumentacja)
2. [Charakterystyka panelu](#2-charakterystyka-panelu)
3. [Autentykacja i sekret API](#3-autentykacja-i-sekret-api)
4. [Otwarte pytania przed implementacją](#4-otwarte-pytania-przed-implementacj%C4%85)
5. [Wstępne mapowanie metod interfejsu](#5-wst%C4%99pne-mapowanie-metod-interfejsu)
6. [Specyficzne properties](#6-specyficzne-properties)
7. [Ścieżki plików — hipoteza](#7-%C5%9Bcie%C5%BCki-plik%C3%B3w--hipoteza)
8. [Edge cases znane z dokumentacji](#8-edge-cases-znane-z-dokumentacji)
9. [Plan testów przed implementacją](#9-plan-test%C3%B3w-przed-implementacj%C4%85)

---

## 1. Źródła i oficjalna dokumentacja

| Zasób | Link | Uwagi |
|---|---|---|
| DirectAdmin API Access | https://directadmin.com/api.php | Overview dwóch API modes. |
| Legacy API | https://docs-dev.directadmin.com/developer/api/legacy-api.html | `CMD_API_*` endpoints. |
| New JSON API (Swagger) | https://docs.directadmin.com/developer/api/ + `https://<your-host>:2222/static/swagger.json` | OpenAPI 2.0 spec bundled with server. |
| Examples (curl, bash, PHP, Python) | https://docs.directadmin.com/developer/api/examples.html | Przykłady wywołań. |
| Admin SSL | https://docs.directadmin.com/webservices/ssl/cmd_admin_ssl.html | SSL management. |

## 2. Charakterystyka panelu

- **Producent:** JBMC Software / DirectAdmin.
- **Model:** Admin / Reseller / User hierarchy. Webox planuje na poziomie User.
- **Port standardowy:** 2222 (HTTPS).
- **Dwa API:**
  - **Legacy `/CMD_API_*`** — URL-encoded, z `?json=yes` zwraca JSON. Tabela commands w Legacy API docs.
  - **`/api/*`** — JSON-only, OpenAPI 2.0 (Swagger). Aktywnie rozwijane.
- **Wersje Node:** zwykle przez Node.js wrapper (custom builds DirectAdmin) lub system-wide nvm. `[TO BE VERIFIED]` — jak konfigurowane per domain.
- **Restart aplikacji:** `[TO BE VERIFIED]` — Passenger lub supervisor. Brak natywnego `restart` w API legacy.
- **SSL:** Let's Encrypt integration przez `CMD_API_SSL` lub `/api/ssl/letsencrypt`. Dobrze udokumentowane.

## 3. Autentykacja i sekret API

> **Reguła twarda:** API key DirectAdmin **NIGDY** nie zapisywany w `config.json` ani w kodzie źródłowym. Zawsze w keyring pod kluczem `webox-api-<profile_alias>`.

| Mechanizm | Kiedy | Storage |
|---|---|---|
| User/password | Pierwsze połączenie (gdy user nie ma jeszcze API key) | nigdy nie storowany; tylko użyty raz do wygenerowania klucza |
| **API key** (Login Key) | Po pierwszym setupie | `webox-api-<profile_alias>` w keyring |
| Session cookie | Sesja przeglądarki | webox nie używa |

Pierwszy run profilu `directadmin`:

1. Webox prosi o user + password.
2. Webox woła `/api/login-keys` (lub legacy `CMD_API_LOGIN_KEYS`) z user/password → otrzymuje API key.
3. API key zapisywany do keyringa pod `webox-api-<alias>`.
4. User/password są trzymane krótko w `memguard.LockedBuffer`, a po użyciu niszczone przez `Destroy()` zgodnie z ograniczeniami opisanymi w `SECURITY.md`.

API key można rotować w `/settings → Provider → Rotate API key`.

## 4. Otwarte pytania przed implementacją

- [ ] **Wybór API:** Legacy `/CMD_API_*` czy nowy `/api/*`? Plan: nowy, ale **TO BE VERIFIED**: czy wszystkie potrzebne operacje są w nowym (lista P0 — patrz §5).
- [ ] **Czy server udostępnia Swagger pod `/static/swagger.json` na każdej instalacji?** — może być wyłączone przez admina.
- [ ] **Format JSON response w legacy API** (`?json=yes`) — jak różni się od nowego.
- [ ] **Tworzenie subdomeny vs addon vs parked** — w DA `CMD_API_DOMAIN action=create` tworzy główną domenę usera; subdomena to `CMD_API_SUBDOMAINS action=create`. **TO BE VERIFIED** — który endpoint pasuje do "subdomeny aplikacji" w webox sensie.
- [ ] **Node.js application setup** — czy DA ma natywne Node Application Manager (jak cPanel), czy wymaga ręcznej konfiguracji Apache/Nginx + Passenger.
- [ ] **SSL Let's Encrypt CMD** — `CMD_API_SSL` z `action=save` i `letsencrypt=1` — `[TO BE VERIFIED]` w stosunku do nowego API.
- [ ] **Database user creation** — czy `CMD_API_DATABASES` tworzy bazę + user razem czy osobno.
- [ ] **Restart mechanism** — brakuje natywnego `restart` w API. Hipoteza: `touch ~/domains/<DOMAIN>/.restart-trigger` (jeśli Passenger), lub HTTP request do panelu, lub kill+spawn przez supervisor. **TO BE VERIFIED**.
- [ ] **Listing zasobów dla detekcji stale** — `CMD_API_SHOW_DOMAINS` (legacy). Nowsze API: `/api/v1/users/<USER>/domains`. **TO BE VERIFIED**.
- [ ] **Rate limits** — czy są dla API.
- [ ] **Reverse proxy do Node:** czy DA automatycznie konfiguruje Apache/Nginx jako proxy do localhost:<PORT>, czy webox musi to robić ręcznie.

## 5. Wstępne mapowanie metod interfejsu

> Wszystkie pozycje **TO BE VERIFIED**.

| Metoda interfejsu | Nowe API (preferowane) | Legacy API (fallback) | Status |
|---|---|---|---|
| `CreateSubdomain(ctx, domain, nodeVersion)` | `POST /api/v1/users/<USER>/subdomains` | `POST /CMD_API_SUBDOMAINS?action=create&subdomain=...&domain=...` | **TO BE VERIFIED** |
| `SetupSSL(ctx, domain)` | `POST /api/v1/ssl/letsencrypt` | `POST /CMD_API_SSL?action=save&domain=...&letsencrypt=1` | **TO BE VERIFIED** |
| `CreateDatabase(ctx, "mysql", dbName)` | `POST /api/v1/databases` | `POST /CMD_API_DATABASES?action=create&name=...&user=...&passwd=...` | **TO BE VERIFIED** |
| `RestartNodeApp(ctx, domain)` | brak natywny | brak natywny | **TO BE VERIFIED** — możliwy fallback `touch tmp/restart.txt` |
| `GetDeployPath(domain)` | `/home/<USER>/domains/<DOMAIN>/public_html` (PHP) lub `/home/<USER>/<app>/public/` (Node) | jw. | **TO BE VERIFIED** |
| `GetLogPath(domain)` | `/home/<USER>/domains/<DOMAIN>/logs` | jw. | **TO BE VERIFIED** |
| `CheckStatus(ctx)` | `GET /api/v1/users/<USER>` | `GET /CMD_API_SHOW_USER_DOMAINS?json=yes` | **TO BE VERIFIED** |
| `ListSubdomains(ctx)` | `GET /api/v1/users/<USER>/subdomains` | `GET /CMD_API_SHOW_DOMAINS?json=yes` | **TO BE VERIFIED** |
| `RemoveSubdomain(ctx, domain)` | `DELETE /api/v1/users/<USER>/subdomains/<DOMAIN>` | `POST /CMD_API_SUBDOMAINS?action=delete&subdomain=...` | **TO BE VERIFIED** |
| `RemoveDatabase(ctx, dbName)` | `DELETE /api/v1/databases/<DB>` | `POST /CMD_API_DATABASES?action=delete&name=...` | **TO BE VERIFIED** |
| `RemoveSSL(ctx, domain)` | `[TO BE VERIFIED]` | `[TO BE VERIFIED]` | **TO BE VERIFIED** |

## 6. Specyficzne properties

| Klucz | Wartości | Cel |
|---|---|---|
| `api_version` | `"new"` (domyślne) / `"legacy"` | Wybór API. |
| `restart_method` | `"passenger"` / `"supervisor"` / `"systemd_user"` | Adapter wybiera. |
| `app_entrypoint` | `"app.js"` / `"index.js"` | Restart przez Passenger lub similar. |
| `db_user_prefix_max_len` | `"8"` (typical DA limit) | Walidacja długości nazwy DB. |
| `api_port` | `"2222"` (default) | Port API. |
| `node_setup_strategy` | `"passenger"` / `"nginx_proxy"` / `"system_node"` | Jak Node jest hostowany. |

## 7. Ścieżki plików — hipoteza

```
/home/<USER>/
├── domains/
│   └── <DOMAIN>/
│       ├── public_html/        # PHP / static
│       ├── private_html/       # SSL-only content
│       └── logs/
└── <node-app-name>/            # jeśli Node aplikacja
    ├── public/
    ├── tmp/                    # restart trigger
    └── app.js
```

`[TO BE VERIFIED]` — czy DA standardowo tworzy `public_nodejs/` jak small.pl, czy używa innego layoutu.

## 8. Edge cases znane z dokumentacji

- **Legacy API zwraca URL-encoded values** bez `?json=yes`. Parser musi obsłużyć obie ścieżki.
- **Database user prefix:** zwykle 8 znaków limit prefiksu — co psuje schemat naming'u (np. `verylongusername_db` → DA odrzuca).
- **Subdomain vs domain:** różne endpointy. Webox musi rozumieć, że "subdomena projektu" w jego sensie to "subdomain" w DA, nie "addon domain".
- **SSL z `letsencrypt=1`** wymaga, by DNS już wskazywał na serwer (jak wszędzie).
- **User mode dwa typy domen:** "primary domain" (jedna per konto) i subdomeny. Webox traktuje wszystko jak subdomeny, ale `[TO BE VERIFIED]` że to bezpieczne założenie.

## 9. Plan testów przed implementacją

> Status: **częściowo wykonany w Sprincie 23.** Read-only ścieżka pokryta przez research-derived fixture'y; pełna weryfikacja Live API + Legacy mapping czeka na live test account (Sprint 24, mirror TASK-22.0 z cpanela).

1. ✅ **Public Swagger spec wykorzystany jako fixture base** — `providers/directadmin/api/testdata/` zawiera 9 golden fixture'ów z DA Live API docs + Swagger spec, plus 3 wire-shape warianty wrapper'a (`{"domains":[...]}`, `{"data":[...]}`, bare array).
2. ⏭️ **Sprint 24:** pozyskać live DA test account. Następnie:
   - Ściągnąć `swagger.json` z testowego serwera — porównać kompletność z założeniami Sprintu 23.
   - Wykonać każdą z planowanych operacji (§5) zarówno w Live API jak i legacy `CMD_API_*`.
   - Zapisać sanitised responses w `providers/directadmin/api/testdata/live/` (gitignored) za pomocą `scripts/smoke-directadmin.sh` (mirror `smoke-cpanel.sh`).
   - Po manualnej redacji promować wybrane payloady do `providers/directadmin/api/testdata/` jako live-captured fixture'y; status preset'u flip z `candidate` na `verified`.
3. ⏭️ **Sprint 24+:** mutating client (`Mutator` interface) + adapter implementation (`providers/directadmin/directadmin.go`) + wizard integration.

Bez kompletu fixture'ów + mutating client + adapter — `webox` nie zaproponuje DirectAdmin w Provider Catalog wizard'a. Patrz [CONTRIBUTING §3.4](../CONTRIBUTING.md#34-krok-4--testy).

## 10. Implementation notes (Sprint 23)

### 10.1 Co zostało zaimplementowane

- **Live API client (`providers/directadmin/api/`)** — HTTPS-only transport z Basic-auth nagłówkiem `Authorization: Basic <user:loginkey>`, retry policy (500 ms × 2ⁿ × 3 attempts), 4 MiB body cap, 30s default timeout. 9 typed error sentinels (`ErrAuthenticationFailed`, `ErrRateLimited`, `ErrAPIDisabled`, etc.); decoder akceptuje 3 wire shape'y (wrapper key, `{"data":[...]}`, bare array).
- **SSH fallback** — shells out to `curl -sk --user <user>:<key> https://localhost:<port>/api/<path> --write-out '\n%{http_code}'` na zdalnym boxie. Rozwiązuje case: operator może zassh-ować się do hosta, ale jego machine nie ma dostępu do panelu :2222 (restrictive firewall, NAT, IP allowlist).
- **Composite** — generic `Composite{Primary, Secondary}` z fall-over WYŁĄCZNIE na `ErrTransportUnavailable`. Auth / rate-limit / API-disabled surface verbatim, bo SSH fallback uderza w ten sam endpoint z tym samym kluczem.
- **`webox doctor directadmin`** — 5-sekcyjny doctor (Whoami / Domains / Subdomains / Databases / SSLCertificates), JSON + text output, exit codes 0/1/2 (OK-DEGRADED / BLOCKED / misuse). Status taxonomy: OK / DISABLED / AUTH_FAILED / UNREACHABLE / FAILED.

### 10.2 Otwarte decyzje (do Sprint 24)

- **Legacy `/CMD_API_*` adapter** — Sprint 23 nie zaimplementował. `ErrAPIDisabled` surface'uje 404/503 z Live API, ale legacy CMD ciągle działa na starszych installach. Decision pending: czy adapter na legacy CMD wchodzi w v0.4+ (potencjalny ADR), czy odsuwamy do v1.0.
- **Mutating endpoints** — Sprint 24 (TASK-24.1+). Planowane: `CreateAddonDomain` (`/api/users/<u>/domains`), `CreateSubdomain` (`/api/users/<u>/subdomains`), `InstallSSL` (`/api/ssl/letsencrypt`), `CreateDatabase` (`/api/users/<u>/databases`). Wszystkie pod env-var guardem `WEBOX_DIRECTADMIN_MUTATIONS=1` (mirror cpanel pattern).
- **Database user prefix limit (8 chars)** — DA caps MySQL user prefix at 8 chars, co psuje "verylongusername_db" naming policy. Adapter (Sprint 24) musi enforce'ować w validatorze: `dbUserName := truncate(username, 8) + "_" + suffix`.
- **Nginx Unit vs Passenger detection** — niektóre DA installs używają Nginx Unit zamiast Passenger. Adapter musi runtime-detect (Sprint 24); probe `command -v passenger-config` versus `systemctl status unit` na hoście.
