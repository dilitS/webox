# Sprint 17 — cPanel Adapter MVP (Part 1: skeleton + read-only ops)

> **Daty:** TBD (po Sprint 16) → +12 dni · **Czas:** ~25-30h
>
> **Cel:** dostarczyć **drugi działający adapter** w core Webox. cPanel UAPI + SSH fallback, focus na operacjach **odczytowych i bezpiecznych** (list, status, restart). Mutacyjne operacje (CreateSubdomain z Application Manager, CreateDatabase, IssueSSL) idą do Sprint 18. **To jest moment, w którym Webox przestaje być projektem niszowym.**

---

## TL;DR

Po Sprint 16 mamy: cPanel test account, captured fixtures, rozszerzone `docs/providers/cpanel.md`. Sprint 17 robi z tego realny pakiet `providers/cpanel/` + rejestrację w core.

Scope sprint 17 (read-only + safe writes):

- `providers/cpanel/` pakiet, mirror struktury `providers/smallhost/`.
- UAPI client z token auth (HTTPS `:2083`) — preferowany happy path.
- SSH `uapi --output=json` fallback — gdy token disabled przez hostera.
- **ListProjects** (`PassengerApps list_applications`).
- **GetStatus** (cPanel app status + Passenger health).
- **GetLogs** (`tail -n N` via SSH na `~/<app>/logs/error_log`).
- **Restart** (`passenger-config restart-app <path>` lub `touch tmp/restart.txt`).
- Output parsers + golden files + malicious fixtures.
- Pierwsze 2 presety: `cpanel-generic`, `cpanel-<hoster>` z konta testowego.
- `webox doctor cpanel --preset <id>` capability probe (minimum: version, Application Manager available, Passenger version).

Scope sprint 18 (mutating ops + polish):

- CreateSubdomain + Application Manager flow.
- CreateDatabase (UAPI Mysql).
- IssueSSL (AutoSSL + Let's Encrypt fallback).
- E2E test against real account.
- README badge update: „**verified providers: smallhost, cPanel**".
- v0.2.0-rc1 cut.

**Nie robimy w Sprint 17:**

- Mutating operations cPanel (Sprint 18).
- DirectAdmin / CyberPanel — Sprint 20+ (decision-gated).
- Preset registry foundation — Sprint 19.

---

## Pre-flight Checklist

- [ ] Sprint 16 zamknięty, cPanel test account ma SSH + UAPI access verified.
- [ ] `docs/providers/cpanel.md` zawiera real-world findings z TASK-16.5.
- [ ] Fixtures w `testing/fixtures/cpanel/` z minimum 10 captured outputs.
- [ ] `make ci` + `make bench-check` zielone na `main`.

---

## Taski (rough outline — uszczegóławiamy w sprint-planning po retro 16)

### TASK-17.1 — `providers/cpanel/` package skeleton (z `webox provider new`)

- **Estymata:** S
- **Acceptance Criteria (rough):**
  - [ ] `webox provider new cpanel --preset cpanel-uapi` generuje pakiet.
  - [ ] `providers/cpanel/provider.go` — factory + struct CPanelProvider + 9 metod jako TODO stubs zwracające `ErrNotImplemented`.
  - [ ] `providers/imports.go` registers cpanel.
  - [ ] `go build ./...` + lint clean.

### TASK-17.2 — UAPI HTTP client (token auth, `:2083`)

- **Estymata:** L
- **Acceptance Criteria (rough):**
  - [ ] `providers/cpanel/uapi/client.go` — wrapper around `net/http.Client` z TLS verification (no `InsecureSkipVerify`).
  - [ ] Token auth header: `Authorization: cpanel <user>:<token>`.
  - [ ] Methods: `Get(ctx, module, function, params) (*UAPIResponse, error)`, `Post(ctx, ...)`.
  - [ ] Response parsing: `UAPIResponse{Status int, Errors []string, Messages []string, Data any, Metadata ...}`.
  - [ ] Rate limit handling: 429 → exponential backoff (2 retries, 1s/2s).
  - [ ] Unit tests against `httptest.Server` z captured fixtures.
  - [ ] **Secret discipline:** token NIGDY w log, error message, stack trace. Redactor regex pattern `cpanel\s+\w+:\w+` extension.

### TASK-17.3 — SSH `uapi` fallback transport

- **Estymata:** M
- **Acceptance Criteria (rough):**
  - [ ] `providers/cpanel/ssh/client.go` — wrapper that wraps `ssh/pool.Pool`.
  - [ ] Command building: `uapi --output=json <module> <function> param=value`.
  - [ ] Output parsing identical to HTTP client (same `UAPIResponse` shape).
  - [ ] Decision matrix in `providers/cpanel/transport.go`: prefer HTTP, fallback SSH when token missing / 401 / Application Manager indicates `disabled in Feature Manager`.
  - [ ] Unit tests against `sshmock` z captured fixtures.

### TASK-17.4 — `ListProjects` implementation

- **Estymata:** M
- **Acceptance Criteria (rough):**
  - [ ] `CPanelProvider.ListProjects(ctx)` calls `PassengerApps/list_applications`.
  - [ ] Output → `[]providers.Project` z mapping: name, path, node version, status (running/stopped).
  - [ ] Empty list (no Applications) → return `nil, nil` (idempotent).
  - [ ] **Edge cases tested:**
    - User account bez żadnych apps.
    - Application Manager disabled w WHM → konkretny error `ErrApplicationManagerDisabled`.
    - Stary cPanel < 11.108 (brak PassengerApps endpointu) → fallback do SSH ls `~/`.
  - [ ] Coverage ≥ 80%.

### TASK-17.5 — `GetStatus` implementation

- **Estymata:** M
- **Acceptance Criteria (rough):**
  - [ ] HTTP ping (200/non-200), SSL cert info (UAPI `SSL/installed_hosts`), Node version (Passenger metadata), last deploy (mtime of `~/<app>/current/`).
  - [ ] Wszystko parallel via `errgroup`, with context cancellation.
  - [ ] Cached przez `status/cache` (5 sek TTL — patrz [DESIGN §8](../DESIGN.md#8-tr%C3%B3jpoziomowy-status-cache-stale-while-revalidate)).

### TASK-17.6 — `GetLogs` implementation

- **Estymata:** M
- **Acceptance Criteria (rough):**
  - [ ] SSH tail `~/<app>/logs/error_log` ostatnich N linii (default 200).
  - [ ] Path resolution z UAPI metadata albo SSH `ls -la ~/`.
  - [ ] **Defensywne parsing:** stripping ANSI, ograniczanie linii do 4KB każda (DoS protection).
  - [ ] Live tail (`tail -f`) — używa istniejącego `services/sshtail/` (Sprint 09).

### TASK-17.7 — `Restart` implementation

- **Estymata:** M
- **Acceptance Criteria (rough):**
  - [ ] **Preferred:** `passenger-config restart-app ~/<app>` przez SSH.
  - [ ] **Fallback:** `touch ~/<app>/tmp/restart.txt` (Passenger graceful restart).
  - [ ] Verification: `GetStatus` po 3 sek pokazuje running + new uptime.
  - [ ] **NIE używamy** UAPI Application Manager restart (różne hostery różnie się zachowują).
  - [ ] Idempotency: restart of stopped app → start it.

### TASK-17.8 — `webox doctor cpanel` capability probe

- **Estymata:** M
- **Acceptance Criteria (rough):**
  - [ ] Sub-command `webox doctor cpanel --profile <name>` runs probes z `docs/providers/cpanel.md §3.3`.
  - [ ] Output: human-readable + `--json` machine-readable.
  - [ ] Diagnoses: Application Manager available, UAPI token works, Passenger version, Node versions available, AutoSSL available, MySQL available, deploy path writable.
  - [ ] Exit codes: 0 = all green, 1 = warnings, 2 = blocking issues.

### TASK-17.9 — `webox doctor cpanel --preset <id>` preset match

- **Estymata:** S
- **Acceptance Criteria (rough):**
  - [ ] Preset metadata loaded from `assets/provider-presets/cpanel-*.json` (foundation only — pełny registry w Sprint 19).
  - [ ] Match real probe results vs preset expectations → `verified` / `partial` / `unsupported`.
  - [ ] Stored result in `~/.cache/webox/preset-match-cpanel.json`.

### TASK-17.10 — Sprint 17 retro + Sprint 18 detailed planning

- **Estymata:** S
- **Acceptance Criteria (rough):**
  - [ ] Retro: rzeczywista velocity Provider Pattern at scale (jak długo zajęło drugiemu adapterowi vs Sprint 03 dla smallhost? Sygnał dla v0.3 estymowania).
  - [ ] Decyzje:
    - Czy mutating ops (Sprint 18) zostają w `providers/cpanel/` czy idą do separate `providers/cpanel/mutations.go` (separation of concerns)?
    - Czy potrzeba ADR-0010 dla wzorca `transport.go` (HTTP-preferred-SSH-fallback)?

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| UAPI auth break w środku sprintu (hoster wyłącza token) | H | SSH fallback (TASK-17.3) gotowy wcześniej, nie dopiero w sprintcie 18. |
| Drugi adapter ujawnia, że `HostingProvider` interface ma luki | M | Patrz ADR-0003 — interfejs jest świadomie zaprojektowany "second-provider-first". Ale spike w razie naprawdę dużej luki: 4h timebox + decyzja czy ADR-0011. |
| Application Manager / Passenger zachowuje się inaczej u różnych hosterów | H | TASK-17.5/17.7 testujemy na **dwóch** hosterach jeśli budget pozwoli — Krystal + HostArmada. |
| Pierwszy zewnętrzny kontrybutor (z launchu) zaczyna cPanel PR równolegle | M | Pair-review commitment (TASK-15.3). Sprint 17 może się skrócić do skeleton + transport jeśli kontrybutor robi metody. |

---

## Outcome (wypełnij po sprincie)

- ✅ Done: ...
- ⏭️ Carry-over → Sprint 18: ...
- 📌 Decyzje:
  - Mutating ops module separation: TAK / NIE
  - ADR-0010 / 0011 needed: ...
- 🧠 Surprises: ...
- 📊 Metryki:
  - cPanel coverage: ?%
  - Real account ListProjects/GetStatus/GetLogs/Restart pass rate: ?
  - Average UAPI call latency (P50/P95): ?
- ➡️ Następny sprint: `sprint-18-cpanel-polish.md`

---

## Retro Link

`docs/retros/<data>-sprint-17.md`
