# Sprint 15 — v0.2 Foundation Plan (drugi provider, OAuth PoC, schema v3, i18n) — **SUPERSEDED**

> **STATUS: SUPERSEDED** (2026-05-25) — Po retrospektywie post-Sprint 14 i strategicznej decyzji „launch readiness → public OSS launch → cPanel adapter ships v0.2", **Sprint 15 został przeplanowany** jako [Launch Readiness](./sprint-15-launch-readiness.md). Content tego planu został rozproszony do:
>
> - **cPanel adapter** → [Sprint 17](./sprint-17-cpanel-adapter.md) (part 1 — skeleton + read-only ops) + [Sprint 18](./sprint-18-cpanel-polish.md) (part 2 — mutating ops + v0.2.0-rc1 cut).
> - **OAuth Device Flow PoC** → [Sprint 20+ ścieżka B](./sprint-20-plus-options.md) (default jeśli launch nie daje jasnego sygnału na DirectAdmin).
> - **`config.json` schema v3** → wbudowane w Sprint 17 jako migracja `cpanel`-specific fields (lub Sprint 18 wizard integration).
> - **ADR-0010 i18n migration plan** → orphan, nie blocker dla v0.2 GA; możliwe Sprint 20+ B (Quality Polish) lub Sprint 21+.
>
> Ten plik **zostaje** jako reference historyczny — content nadal może być źródłem akceptacji dla rozproszonych sprintów. **Nie planuj wykonania tego sprintu w tej formie.**
>
> Patrz [docs/sprints/README.md](./README.md) dla nowej tabeli sprintów + [.local/notes/2026-05-25-initial-launch-strategy.md](../../.local/notes/2026-05-25-initial-launch-strategy.md) (gitignored) dla pełnego rationale decyzji.
>
> ---
>
> **(Original plan body below, kept for reference. Daty NIE są już aktualne.)**

> **Daty (HISTORICAL):** 2026-07-01 → 2026-07-21 (planowane 3 tygodnie solo) · **Czas:** ~36-44h skupienia
>
> **Cel:** zbudować fundament pod v0.2 release: drugi provider (cPanel) jako walidacja Provider Pattern, OAuth Device Flow PoC za feature flagą, schema migracja `config.json` v2→v3 oraz ADR dla planowanej migracji i18n. **Bez** cofania się po Sprint 14 architecture-hardening — Sprint 15 buduje *na* zielonej bazie surfaces / per-tile scroll / SSH semaphore / strukturalnego tracingu.
>
> Sprint 14 zamknął god-package `tui/`, dodał per-tile scroll, SSH inflight semaphore, host-key modal i `--debug-trace`. Sprint 15 wykorzystuje tę bazę żeby dorzucić **produktową** wartość v0.2: drugi provider udowadnia że Provider Pattern działa, OAuth PoC sprawdza ścieżkę dalszej automatyzacji bez `gh` CLI, schema v3 przygotowuje miejsce na opcjonalne pola DB / monitoring TTL, a i18n ADR konsoliduje plan stopniowej migracji EN/PL+.

---

## TL;DR

Sprint 15 zbiera **cztery sztywno-zafreezowane tematy** które opcujemy w v0.2:

1. **cPanel adapter** — implementacja Provider Pattern w innym ekosystemie (UAPI + SSH).
2. **OAuth Device Flow PoC** — ścieżka GitHub auth bez wymagania `gh` CLI lub PAT (za `WEBOX_EXPERIMENTAL=1`).
3. **`config.json` schema v3** — opcjonalne pola DB (driver/host/port/sslmode), walidacja, migracja v2→v3 z LIFO rollback.
4. **ADR-0010: i18n migration plan** — strategia gradualnej migracji widoków na `i18n` package, kolejność migracji, rollout testowy.

**Nie robimy w tym sprincie:**

- Trzeci provider (DirectAdmin / CyberPanel) — Sprint 16+.
- DAG-based rollback engine — Sprint 16+.
- In-app updater — v0.3+.
- Multi-provider dashboard agregator — v0.2 (po Sprincie 17, post-cPanel stabilizacji).
- Sound Engine, fast-chord bindings — v0.2 (Sprint 17+).

---

## Pre-flight Checklist

- [ ] Sprint 14 zakończony z zielonym `make ci` + zielonym `make bench-check`.
- [ ] `v0.1.0` GA wytagowane lub świadoma decyzja o `v0.1.1` patch z domkniętymi sprintami 13-14.
- [ ] `docs/AUDIT.md` przejrzany — żadne "carry-over to Sprint 15" nie zostało zaniedbane.
- [ ] `docs/PRD.md §6` mapuje aktywne ficzery v0.2 na P0/P1.
- [ ] `docs/SECURITY.md §10` rozszerzone o sekcję OAuth Device Flow (token storage, scope limitation, refresh).

---

## Tasks

### TASK-15.1 — cPanel adapter (UAPI + SSH hybrid)

- **Estymata:** L
- **Zależności:** Sprint 03 `HostingProvider` interface, Sprint 14 inflight limiter (cPanel rate limits są ostre).
- **Acceptance Criteria:**
  - [ ] `providers/cpanel/` z full `HostingProvider` impl (CreateSubdomain, CreateMySQL, CreateMySQLUser, GrantPrivileges, RemoveSubdomain, RemoveMySQL, RemoveMySQLUser, Status, Restart, ListProjects, GetSSL).
  - [ ] UAPI client w `providers/cpanel/uapi.go`: token auth, request signing, redactor in/out, idempotent retries (`429` + `Retry-After`).
  - [ ] SSH operacje (deploy key install, `git pull`, `npm install`, `pm2 restart`) przez wspólny `ssh.Pool` + `ssh.ExecWithRetryLimited` (TASK-14.3).
  - [ ] Fixture'y: `testing/fixtures/cpanel/` z każdym znanym typem odpowiedzi UAPI (success, exists, rate-limited, auth-failed, invalid-domain).
  - [ ] Per-call golden test (`*_test.go`) dla każdego parsera UAPI; parser MUSI być test-first (TDD obowiązkowe).
  - [ ] `providers/cpanel/cpanel.md` zgodnie z [smallhost.md](../providers/smallhost.md) wzorcem.
  - [ ] Provider zarejestrowany w `providers.Register("cpanel", New)` ale za `WEBOX_EXPERIMENTAL=1` flag (operator widzi go w `webox doctor providers --experimental`).
  - [ ] Coverage ≥ 80 % w `providers/cpanel/...`.
- **Docs:** [adr/0003-provider-pattern.md](../adr/0003-provider-pattern.md), [providers/cpanel.md](../providers/cpanel.md) (rozbudować z notatek badawczych do pełnego designu).

### TASK-15.2 — OAuth Device Flow PoC za `WEBOX_EXPERIMENTAL=1`

- **Estymata:** M
- **Zależności:** brak (działa równolegle do reszty).
- **Acceptance Criteria:**
  - [ ] `services/github/auth/device.go` — implementacja [Device Authorization Grant (RFC 8628)](https://datatracker.ietf.org/doc/html/rfc8628) dla GitHub.
  - [ ] CLI: `webox auth login github --device` (za `WEBOX_EXPERIMENTAL=1`); flow:
    1. POST do `https://github.com/login/device/code`.
    2. UI w TUI: kod + URL + QR ASCII (opcjonalnie) + spinner pollingu.
    3. POST do `https://github.com/login/oauth/access_token` co `interval` sek.
    4. Po success: token zapisany w keyringu pod kluczem `webox.github.oauth.<user_login>` (NIGDY w `config.json`).
  - [ ] Token refresh: webox prowadzi background refresh przed `expires_at-30s` (lumberjack-style, jeden goroutine na proces).
  - [ ] Scope: `repo` + `workflow` (wystarczy do current funkcjonalności GHA + secrets).
  - [ ] Nigdzie nie loggujemy access_token; redaktor `internal/log/redact.go` dostaje wzorzec `gho_[A-Za-z0-9]{36}` (test pin).
  - [ ] PoC scope: TYLKO login flow + token storage. Refresh + expire handling = Sprint 16.
- **Docs:** [adr/0011-oauth-device-flow.md](../adr/0011-oauth-device-flow.md) (do utworzenia), [SECURITY.md §10.2](../SECURITY.md).

### TASK-15.3 — `config.json` schema v3 + migracja v2→v3 + opcjonalne DB

- **Estymata:** M
- **Zależności:** Sprint 01 config foundation, Sprint 14 atomic save (już mamy).
- **Acceptance Criteria:**
  - [ ] `config/schema/v3.json` — JSON Schema z opcjonalnymi polami:
    - `projects[].database` (oneOf: `mysql_local`, `postgres_remote`, `none`).
    - `projects[].monitoring_ttl_seconds` (override globalnego 5s).
    - `projects[].health_check_url` (override domyślnego `/`).
  - [ ] `config/migrate_v2_to_v3.go` — czysta funkcja `(v2 *ConfigV2) (*ConfigV3, error)` z LIFO rollback (jeśli walidacja v3 padnie).
  - [ ] Walidator: każde nowe pole ma test (`config/validate_v3_test.go`) — pozytywny + negatywny case (`monitoring_ttl_seconds = 0` → reject, ujemny → reject, > 1h → reject, etc.).
  - [ ] Backwards compat: schema_version brak w pliku → traktuj jako v2 → migruj. v3 → zostaw.
  - [ ] Forwards compat: schema_version > v3 → reject z komunikatem `webox is too old, please update`.
  - [ ] Migracja jest **idempotentna**: dwukrotne uruchomienie nie zmienia output.
  - [ ] `webox doctor config` raportuje `schema_version`, ostrzeżenia / rejecty.
  - [ ] Coverage ≥ 80 % w `config/...` (już mamy 84 % po Sprint 13; nie regresować).
- **Docs:** [DESIGN.md §6](../DESIGN.md#6-model-danych-i-atomowo%C5%9B%C4%87-zapisu-configjson) (rozbudować o v3), [adr/0009-config-schema-v3.md](../adr/0009-config-schema-v3.md) (do utworzenia).

### TASK-15.4 — ADR-0010: i18n migration plan (gradual view-by-view)

- **Estymata:** S
- **Zależności:** brak (czysto dokumentacyjny).
- **Acceptance Criteria:**
  - [ ] `docs/adr/0010-i18n-migration-plan.md` — pełny ADR.
  - [ ] Sekcje:
    1. Status quo: `i18n` package istnieje, ale wiele widoków nadal hardkoduje teksty PL.
    2. Decyzja: stopniowa migracja widok-po-widoku, zamiast big-bang.
    3. Kolejność migracji (PRIORYTY): Init Wizard → Project Wizard → Dashboard footer hints → Project Detail → Live Logs → CICD → Topology → Modaly.
    4. Definition of Done per widok: każdy `string literal` w `tui/views/<view>.go` przenoszony do `i18n/<view>.<lang>.json`; test snapshot covers EN+PL+jeden community pack (np. DE).
    5. Rollout: per-sprint 1-2 widoki, z release notes wymieniającymi które widoki przeszły migrację.
    6. Pattern: `strings.go` w pakiecie widoku — wszystkie klucze typed; `i18n.T(ctx, key)` zamiast `i18n.T(string)` (typo guard).
    7. Risk: tłumacze z poza EN/PL muszą mieć schema gdzie jakie klucze są dostępne — generujemy `docs/i18n/keys.md` z `go generate`.
  - [ ] Mapowanie sprint → widok do migracji w sprint-15+ planach.
- **Docs:** [adr/0006-jezyk-interfejsu-en-domyslny.md](../adr/0006-jezyk-interfejsu-en-domyslny.md) (rationale dla EN default).

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| cPanel UAPI rate limits są agresywniejsze niż small.pl/Devil — naive port może błyskawicznie zatrzymać operatora. | H | TASK-15.1 wymusza `ssh.InflightLimiter` (TASK-14.3) + dedykowany HTTP `RateLimiter` z `Retry-After`. |
| OAuth Device Flow PoC zacznie być traktowany jako produkcyjny → migracja `gh` CLI users zaczyna pchać się do v0.2.0. | M | AC explicit: "PoC scope, refresh + expire handling = Sprint 16". `WEBOX_EXPERIMENTAL=1` flag obowiązkowy. |
| Schema v3 migracja zepsuje istniejące produkcyjne `config.json`. | H | Migracja jest idempotentna + LIFO rollback. CI test: zaczyna od `config_v2_fixture.json` → migruje → migruje ponownie → output identyczny. |
| ADR-0010 zacznie być postrzegany jako "robimy migrację teraz". | L | Sprint 15 dostarcza tylko ADR. Pierwsza migracja widoku → Sprint 16. |

---

## Dependencies signoff

Sprint 15 może wymagać:

- `golang.org/x/oauth2` — **NOWA zależność**. Wymaga sign-off maintainera (AGENTS.md §8.2). Standardowa biblioteka Go ekosystemu, używana przez `gh` CLI sam.

**Inne nowe zależności:** żadne; cPanel adapter używa istniejących `net/http` + `ssh.Pool`.

---

## Outcome (wypełnij po sprincie)

- ✅ Done: ...
- ⏭️ Carry-over → Sprint 16: ...
- 📌 Decyzje:
  - cPanel adapter merged jako experimental: TAK / NIE
  - OAuth PoC działa: TAK / NIE
  - Schema v3 zaktywowana: TAK / NIE
  - ADR-0010 merged: TAK / NIE
- 🧠 Surprises: ...
- 📊 Metryki:
  - Coverage end-of-sprint: ?% (target ≥ 75 % po wprowadzeniu cpanel/)
  - Open issues po sprincie: ?
  - Bench worst ns/op: ?
- 🔒 Security validation:
  - [ ] `govulncheck` zielony
  - [ ] OAuth token NIGDZIE w `config.json` ani w trace.jsonl
  - [ ] cPanel UAPI token NIGDZIE w log / error message (regex pin w redaktorze)
  - [ ] Migracja v2→v3 nie usuwa pól, które były w v2
- ➡️ Następny sprint: `sprint-16-cpanel-stabilization-plus-oauth-refresh.md`

---

## Retro Link

`docs/retros/<data>-sprint-15.md` (do utworzenia po zakończeniu sprintu)
