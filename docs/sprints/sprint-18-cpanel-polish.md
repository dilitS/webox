# Sprint 18 — cPanel Adapter (Part 2: mutations + v0.2.0-rc1)

> **Daty:** TBD (po Sprint 17) → +10-12 dni · **Czas:** ~25-30h
>
> **Cel:** dokończyć cPanel adapter (CreateSubdomain, CreateDatabase, IssueSSL) z pełnym wizard flow + LIFO rollback + E2E test against real account. Wyciąć `v0.2.0-rc1` z dwoma verified providerami. **Po tym sprintcie Webox jest realnym, dwu-providerowym narzędziem.**

---

## TL;DR

Sprint 17 dał `providers/cpanel/` z read-only ops. Sprint 18 zamyka mutating ops i pakuje wszystko w v0.2.0-rc1.

Scope:

- **CreateSubdomain** flow: Application Manager + Passenger config + AutoSSL kick-off.
- **CreateDatabase** flow: UAPI Mysql + user creation + privileges.
- **IssueSSL** flow: AutoSSL preferred, Let's Encrypt fallback (via UAPI SSL).
- **RemoveSubdomain / RemoveDatabase / RemoveSSL** — idempotent (essential dla LIFO rollback).
- Wizard flow integration: cPanel jako wybór w wizard step „Choose provider".
- E2E test: `internal/e2e/cpanel_*_test.go` z mock + jeden manual real-account run.
- README badge: **two verified providers**.
- `v0.2.0-rc1` cut.
- Polish: error messages EN-only, error path docs.

**Nie robimy w Sprint 18:**

- DirectAdmin / CyberPanel — Sprint 20+ (decision-gated).
- Preset registry — Sprint 19.
- `webox auth login` OAuth GitHub Device Flow — Sprint 21+.

---

## Pre-flight Checklist

- [ ] Sprint 17 zamknięty, TASK-17.1-17.10 done.
- [ ] cPanel test account nadal aktywny + Application Manager / Passenger working.
- [ ] `webox doctor cpanel --preset cpanel-generic` zwraca `verified` lub `partial-acceptable`.

---

## Taski (rough outline)

### TASK-18.1 — `CreateSubdomain` flow

- **Estymata:** L
- **Acceptance Criteria (rough):**
  - [ ] UAPI `SubDomain/addsubdomain` → wait for DNS propagation (poll 30s).
  - [ ] UAPI `PassengerApps/create_application` z {appname, deploy path, startup file, node version}.
  - [ ] Verify Passenger config writing (~/.htaccess w app root z `PassengerStartupFile`).
  - [ ] Edge cases: subdomain already exists (idempotent — return existing), Application Manager exhausted slots (concrete error).

### TASK-18.2 — `CreateDatabase` flow

- **Estymata:** M
- **Acceptance Criteria (rough):**
  - [ ] UAPI `Mysql/create_database` + `Mysql/create_user` + `Mysql/set_privileges_on_database`.
  - [ ] Random password gen via `crypto/rand` (NOT secret-in-config — store in keyring).
  - [ ] Return connection string scrubbed for log: `mysql://<user>:[REDACTED]@<host>/<db>`.

### TASK-18.3 — `IssueSSL` flow

- **Estymata:** M
- **Acceptance Criteria (rough):**
  - [ ] AutoSSL preferred: UAPI `SSL/start_autossl_check` na konkretną domenę.
  - [ ] Fallback Let's Encrypt: UAPI `SSL/install_letsencrypt_cert` (jeśli plugin obecny).
  - [ ] Polling cert issuance (5 min budget).
  - [ ] Verify cert installed via `SSL/installed_hosts`.

### TASK-18.4 — `Remove*` idempotent operations (LIFO rollback support)

- **Estymata:** M
- **Acceptance Criteria (rough):**
  - [ ] `RemoveSubdomain` → UAPI `SubDomain/delsubdomain` + `PassengerApps/delete_application`. Brak zasobu = sukces.
  - [ ] `RemoveDatabase` → UAPI `Mysql/delete_user` + `Mysql/delete_database`. Brak zasobu = sukces.
  - [ ] `RemoveSSL` → UAPI `SSL/delete_ssl` jeśli AutoSSL nie zarządzana.
  - [ ] Wszystkie operacje weryfikowalne dla `pending_cleanups.json` resume.

### TASK-18.5 — Wizard integration

- **Estymata:** S
- **Acceptance Criteria (rough):**
  - [ ] Wizard step „Choose provider" → cPanel teraz jako pełnoprawny wybór (bez `experimental` flag, ale z badge `verified`).
  - [ ] Provider profile creation flow dla cPanel: host, port (`:2083`), user, token, SSH key path.
  - [ ] Walidacja token via `webox doctor cpanel` przed zapisem profilu.

### TASK-18.6 — E2E test against real account

- **Estymata:** L
- **Acceptance Criteria (rough):**
  - [ ] `internal/e2e/cpanel_create_remove_test.go` — pełny flow: create subdomain → DB → SSL → restart → remove all.
  - [ ] Run manualny: `make test-integration WEBOX_TEST_PROVIDER=cpanel WEBOX_TEST_PROFILE=test-cpanel`.
  - [ ] Cleanup gwarantowany nawet jeśli test fail (defer).
  - [ ] Nigdy nie commitowane: real credentials. Test pattern: `t.Skip` jeśli env vars nie ustawione.

### TASK-18.7 — Documentation + README update

- **Estymata:** S
- **Acceptance Criteria (rough):**
  - [ ] `docs/providers/cpanel.md` upgrade z `Status: Research` → `Status: Stable`.
  - [ ] `README.md` Hero section update: „**Currently supports**: small.pl/Devil, cPanel."
  - [ ] `docs/ROADMAP.md` § v0.2 zamknięcie.
  - [ ] CHANGELOG `[Unreleased]` → przygotować `[v0.2.0-rc1]` z `Added: cPanel provider`.

### TASK-18.8 — Release v0.2.0-rc1

- **Estymata:** S
- **Acceptance Criteria (rough):**
  - [ ] Pre-release checklist (`.cursor/skills/release-check/SKILL.md`).
  - [ ] Bump version, tag `v0.2.0-rc1`, push.
  - [ ] GoReleaser snapshot → GitHub Release draft.
  - [ ] Asciinema update z nowym demo (Tab between providers).

### TASK-18.9 — Re-announcement w community

- **Estymata:** S
- **Acceptance Criteria (rough):**
  - [ ] Post w r/selfhosted (nie HN — HN tylko raz na major version): „Update: Webox now supports cPanel."
  - [ ] Tweet/X thread z before/after asciinema.
  - [ ] Update na partnership outreach status — jeśli H88 odpowiedział, pokazać im konkret.
  - [ ] Issue update na `good-first-issue#1 (cPanel skeleton)` → closed jeśli zrobiłeś sam, otherwise pair-completed.

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| AutoSSL nie issue dla domeny zonal mismatch | M | Let's Encrypt fallback (TASK-18.3). |
| `pending_cleanups.json` schema musi się rozszerzyć dla cPanel-specific cleanup info | M | Migracja v2 → v3 jeśli potrzeba. Trigger ADR. |
| Test account expire w środku sprintu | M | Renew 14 days before expiry. Plan: extend by 3 months kiedy zacząłeś Sprint 16. |
| `v0.2.0-rc1` cut zostawi za sobą nieukończone polish — push to v0.2.0 lub v0.2.1? | M | RC1 jest acceptable z 1-2 known issues, jak długo nie są blokerami. Bug bash w Sprint 19 lub osobny sprint pomiędzy. |

---

## Outcome (wypełnij po sprincie)

- ✅ Done: ...
- ⏭️ Carry-over: ...
- 📌 Decyzje: ...
- 🧠 Surprises: ...
- 📊 Metryki:
  - v0.2.0-rc1 cut date: ?
  - cPanel coverage: ?
  - E2E pass rate: ?
- ➡️ Następny sprint: `sprint-19-preset-registry.md`

---

## Retro Link

`docs/retros/<data>-sprint-18.md`
