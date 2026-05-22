# Provider: directadmin

> Status: **RESEARCH / PLANNED POST-MVP (v0.3)** · Ostatnia aktualizacja: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [providers/smallhost.md](./smallhost.md) (wzorzec), [providers/cpanel.md](./cpanel.md) (drugi research), [CONTRIBUTING §3](../CONTRIBUTING.md#3-jak-doda%C4%87-nowy-provider).

## STATUS

**RESEARCH — implementacja zaplanowana na v0.3** (po cPanel w v0.2, jeśli ROADMAP §4 nie zostanie zmieniony). Ten dokument zawiera **wyłącznie notatki badawcze** i otwarte pytania. Wszystkie hipotezy `[TO BE VERIFIED]` wymagają potwierdzenia.

> Oryginalny PRD zawierał implementację `directadmin.go` napisaną z głowy, z `DA_API_KEY` jako **string literal w kodzie** — to było jednocześnie błędem implementacyjnym (sekret w pliku źródłowym!) i nieweryfikowalną propozycją API. Usunięte. Patrz [CHANGES.md poprawki 6.8 + 6.9](../../CHANGES.md#1-poprawki-merytoryczne-z-tabeli-%C2%A76-briefu).

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
4. User/password są **wycierane z pamięci** (`zerocopy.Wipe`).

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

1. Pozyskać dostęp do testowego konta DirectAdmin (sandbox / trial / partner program).
2. Sciągnąć `swagger.json` z testowego serwera — zbadać kompletność nowego API.
3. Manualnie wykonać każdą z planowanych operacji (§5) zarówno w API legacy jak i nowym.
4. Zapisać responses jako fixture'y w `testing/fixtures/directadmin/` (sanityzowane).
5. Każdy fixture ma `*.fixture.md` z opisem pochodzenia.
6. Iteracja §4 → coroczna aktualizacja.
7. Po weryfikacji wszystkich punktów §4 — promocja statusu z **RESEARCH** na **PLANNED**.

Bez kompletu fixture'ów — adapter nie wchodzi do `main`. Patrz [CONTRIBUTING §3.4](../CONTRIBUTING.md#34-krok-4--testy).
