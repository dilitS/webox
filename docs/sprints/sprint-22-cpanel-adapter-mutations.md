# Sprint 22 — cPanel Adapter (part 2) + v0.2.0-rc1

> **Daty:** 2026-06-23 → 2026-07-06 (2 tygodnie) · **Cel:** Mutating cPanel ops (createProject, restartApp, addSSLDomain), wizard integration, GHA template, E2E, fixtures from live test account, tag `v0.2.0-rc1`.
>
> **Status:** 📝 Planned · **Properties:** code-heavy + release-cadence · **Predecessor:** [Sprint 21](sprint-21-cpanel-adapter-prep.md) (cPanel adapter read-only + public launch prep).

## Kontekst

Sprint 21 wylądował read-only stack (UAPI client + SSH fallback + `webox doctor cpanel`) plus przygotowania do public launch (README EN, asciinema cast). Sprint 22 zamyka kolejny adapter pełnym scope'em: tworzenie projektu na cPanel z TUI, restart aplikacji, dodanie SSL, GHA workflow template, end-to-end testy, i tagowanie `v0.2.0-rc1`.

**Gating dependency: TASK-21.7 (cPanel test account onboarding) musi zostać domknięte przez maintainera przed startem Sprintu 22.** Bez live test account'u nie da się:
1. Wymienić research-derived fixtures w `providers/cpanel/uapi/testdata/` na live captures.
2. Wygenerować `verified` audit trail dla `cpanel-generic` preset.
3. Zaszyć fixtures dla mutating ops (createProject odpowiedź, restartApp success/failure shape, itd.).

Jeśli account nie pojawi się do 2026-06-23, Sprint 22 zaczyna się od TASK-22.0 (account procurement spike, 1 dzień) zanim ruszą mutating taski.

## Cel sprintu

Po Sprincie 22:

1. **`providers/cpanel/cpanel.go`** implementuje pełny `providers.HostingProvider` interface: `CreateProject`, `RemoveProject`, `StartApp`, `StopApp`, `RestartApp`, `ListDomains` (carry HTTPS+SSH), `AddSSLDomain`, `RemoveSSLDomain`. Mutating ops idą przez `uapi.MutatingClient` (ErrSprintScopeNotMutable znika).
2. **TUI wizard** rozumie `cpanel-generic` (i `cpanel-cloudlinux-selector`) preset, prowadzi przez Node.js / SSH / GHA flow tak samo jak `smallhost-devil`.
3. **GHA workflow template** dla cPanel — analogiczny do `assets/workflows/smallhost-devil.yml` ale używa cPanel UAPI dla deploy + SSH dla restart.
4. **E2E test** w `internal/e2e/cpanel_test.go` używa `testing/sshmock` + `httptest` żeby drive'ować pełen flow: wizard → createProject → addSSLDomain → restart → status feed.
5. **`v0.2.0-rc1` tag** + release notes (`docs/release-notes/v0.2.0-rc1.md`) + CHANGELOG bump.

Czego **nie** umiemy w Sprincie 22: DirectAdmin / CyberPanel (Sprint 23+), Webox-managed reverse-DNS na cPanel (out-of-scope w v0.2), automatyczna migracja smallhost → cPanel projektu (v0.3+).

## Taski

### TASK-22.0 (CONDITIONAL) — Live cPanel test account procurement

- **Estymata:** S (< 4h) lub blokuje sprint
- **Zależności:** TASK-21.7 z poprzedniego sprintu.
- **Acceptance Criteria:**
  - [ ] Test cPanel account secured (vendor purchase lub H88 partnership).
  - [ ] Credentials stored in `webox` keyring under alias `cpanel-test-account`.
  - [ ] `webox doctor cpanel --host=$TEST_HOST --user=$TEST_USER --token=$TOKEN` returns OK or DEGRADED verdict.
  - [ ] All 7 fixtures in `providers/cpanel/uapi/testdata/` replaced with live-captured equivalents.
  - [ ] `cpanel-generic` preset's `verified` block populated with fixture_dir, last_verified_at, verified_by.

### TASK-22.1 — Mutating UAPI client + `MutatingClient` interface

- **Estymata:** L (1.5-2 dni)
- **Zależności:** TASK-21.1 + (jeśli account ready) TASK-22.0.
- **Acceptance Criteria:**
  - [ ] `providers/cpanel/uapi/mutating.go` implements `MutatingClient.Call` (replaces stub from Sprint 21).
  - [ ] Modules covered: `PassengerApps.{create_application, edit_application, restart_application}`, `Mysql.{create_database, create_user, grant_privileges}`, `SSL.{install_ssl}`, `DomainInfo.{add_addon_domain, add_subdomain}`.
  - [ ] Env-var guard: `WEBOX_CPANEL_MUTATIONS=1` required to enable; otherwise `MutatingClient.Call` returns `ErrSprintScopeNotMutable` (defence in depth — operator opts in explicitly).
  - [ ] Per-method tests with golden response fixtures (live + research).

### TASK-22.2 — `providers/cpanel/cpanel.go` adapter implementation

- **Estymata:** L (1.5-2 dni)
- **Zależności:** TASK-22.1.
- **Acceptance Criteria:**
  - [ ] `Cpanel` struct satisfies `providers.HostingProvider` interface.
  - [ ] `CreateProject(ctx, spec) → ProjectArtifacts` orchestrates: addon domain → MySQL DB + user → PassengerApp + path → SSL install (if requested).
  - [ ] Atomic rollback on partial failure: if SSL install fails after PassengerApp create, the PassengerApp gets cleaned up (LIFO undo log, same shape as `smallhost`).
  - [ ] `Remove*` ops are idempotent (missing resource = nil error).
  - [ ] No hardcoded provider name in business logic — every cPanel-specific path goes through `providers.HostingProvider` interface (AGENTS.md §1 guardrail).

### TASK-22.3 — TUI wizard integration with cPanel preset

- **Estymata:** M (1 dzień)
- **Zależności:** TASK-22.2.
- **Acceptance Criteria:**
  - [ ] `webox` cockpit detects `cpanel-generic` preset from config.json and routes the wizard through cPanel flow (Node.js version select, addon domain pick, SSL toggle).
  - [ ] Wizard step "Pick hosting" lists every `verified` + `candidate` preset from the registry (filtered by `provider_type=cpanel`).
  - [ ] E2E test in `internal/e2e/cpanel_wizard_test.go` drives the full happy path via teatest + sshmock + httptest.

### TASK-22.4 — GHA workflow template for cPanel

- **Estymata:** M (0.5-1 dzień)
- **Zależności:** TASK-22.2.
- **Acceptance Criteria:**
  - [ ] `assets/workflows/cpanel-uapi.yml` ships via `embed.FS` (same pattern as `smallhost-devil.yml`).
  - [ ] Template uses cPanel UAPI tokens (NOT password) for deploy; SSH for restart.
  - [ ] `webox provider new cpanel --preset=cpanel-uapi` (generator from Sprint 15) wires the template by default.
  - [ ] Workflow uses pinned 40-char SHAs for every `uses:` (AGENTS.md §1 guardrail).
  - [ ] Documented in `docs/contributing/PROVIDER.md` and `docs/providers/preconfiguration-vision.md`.

### TASK-22.5 — E2E test suite for cPanel adapter

- **Estymata:** M (1 dzień)
- **Zależności:** TASK-22.2 + TASK-22.3.
- **Acceptance Criteria:**
  - [ ] `internal/e2e/cpanel_test.go` covers: wizard → createProject → addSSLDomain → restart → status feed.
  - [ ] Uses `testing/sshmock` for SSH + `httptest.NewTLSServer` for UAPI.
  - [ ] Negative paths: createProject with name collision, SSL install fails after PassengerApp create (rollback verified), token expired mid-flow.
  - [ ] Bench (informational, not gated): full createProject < 5s wall-clock under mock.

### TASK-22.6 — `v0.2.0-rc1` release prep

- **Estymata:** S (< 2h)
- **Zależności:** All above.
- **Acceptance Criteria:**
  - [ ] Tag `v0.2.0-rc1` created on `main` after merge.
  - [ ] Release notes in `docs/release-notes/v0.2.0-rc1.md` enumerate every cPanel-related change since `v0.1.0-rc2`.
  - [ ] CHANGELOG `[Unreleased]` section moved to `[0.2.0-rc1]` with release date.
  - [ ] Binary artifacts built via GoReleaser for linux/macOS/windows (carry-over from Sprint 14).
  - [ ] Release published as **pre-release** on GitHub (not "latest").

### TASK-22.7 — Sprint review + retro + carry-over decision

- **Estymata:** S (< 2h)
- **Zależności:** All above.
- **Acceptance Criteria:**
  - [ ] Retro in `docs/retros/YYYY-MM-DD-sprint-22.md`.
  - [ ] Sprint outcome filled in.
  - [ ] Decision point: Sprint 23 = DirectAdmin? CyberPanel? Public Launch (Sprint 16) redux? Documented + new sprint plan drafted.

## Risk watch

| Ryzyko | Mitygacja |
|---|---|
| TASK-22.0 blocked > 1 dzień | Sprint pauza do retro; pivot to Sprint 23 (DirectAdmin) bez cPanel mutations. |
| cPanel UAPI deprecates a Sprint-22 endpoint mid-sprint | Pin to documented v1 surface; SSH fallback covers gaps. Document in retro. |
| Rollback path bugs slip through (createProject partial failure) | Mandatory negative tests in TASK-22.5; LIFO undo log replays in test harness with fault injection. |
| `v0.2.0-rc1` ships with broken `cpanel-cloudlinux-selector` preset (not part of MVP scope) | Mark preset as `candidate` not `verified` until Sprint 23+ live validation. |
| Token leakage via stack trace in mutating ops | Redactor extended (Sprint 21 retro action item) to scrub `cpanel <user>:<token>` from any error message before logging. |

## Outcome (wypełnij po sprincie)

- 📌 Path selected: <fill at sprint start>
- ✅ Done: <fill as tasks close>
- ⏭️ Carry-over: <task → Sprint 23 + reason>
- 📌 Decyzje: <ADR jeśli powstał>
- 🧠 Surprises: <co się okazało inne niż w docs>
- 📊 Metrics: cPanel adapter coverage, mutating-ops latency, rollback test pass-rate.
