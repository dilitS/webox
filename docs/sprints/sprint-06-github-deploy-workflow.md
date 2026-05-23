# Sprint 06 — GitHub Repo + Deploy Workflow + First Deploy

> **Daty:** TBD → TBD (planowane 2-3 tygodnie solo) · **Czas:** ~50-70h skupienia
>
> **Cel:** zamknąć MVP path "od kliknięcia `n` do działającej aplikacji w internecie": po Sprintcie 05 wizard tworzy domenę + SSL + opcjonalną bazę, ale projekt jest pusty. Sprint 06 dodaje **GitHub repo creation + deploy keys + Actions workflow + first deploy via SSH/rsync** oraz **resume on launch** dla `pending_cleanups.json`. Po sprincie 06 Webox potrafi zaprowadzić nową aplikację Node/React/static do stanu "200 OK on https://app.demo.smallhost.pl" bez ręcznego kroku poza wpisaniem PAT-a / `gh auth login`.

---

## TL;DR

Po sprincie 06:

- Wizard projektu ma dodatkowe kroki: **Repo** (wybór nazwy / public/private), **Secrets** (PAT-detect + Actions secrets push), **Deploy preview** (pokazany rendered `deploy.yml` przed commitem).
- `services/github/` ma minimalny klient (`gh` CLI wrapper + REST fallback) dla: create repo, set deploy key, set Actions secret, dispatch first workflow run.
- `assets/workflows/` osadza templates dla `vite-react`, `node-express`, `static` przez `embed.FS`; wszystkie `uses:` są pinowane do pełnego 40-char SHA (AGENTS.md §2.1, §7.1).
- `wizard.Execute` rozszerzony o kroki `CleanupKindGitHubRepo`, `CleanupKindDeployKey`, `CleanupKindActionsSecret`, `CleanupKindWorkflowFile`. LIFO rollback odwraca w odwrotnej kolejności (workflow file → secret → deploy key → repo).
- Dashboard `last_deploy` przestaje być placeholderem — odczytywany z GitHub Actions REST i cache'owany przez `status/`.
- Startup pipeline w `cmd/webox`: jeśli `pending_cleanups.json` istnieje, TUI otwiera **Resume Wizard** modal zamiast Dashboard.
- `webox doctor github` (read-only) pokazuje status PAT / `gh` / repo / deploy key.

**Nie robimy w tym sprincie:**

- Multi-provider deploys (smallhost only).
- OAuth Device Flow zamiast PAT/`gh`: post-MVP RFC.
- Live log stream (`tail -f` przez SSH): STRETCH v0.2+.
- DAG rollback / topological sort: v0.3+; rozszerzamy istniejący LIFO o nowe kindy.
- Env Merger / `.env` interactive diff: STRETCH v0.2+.
- DB migracje (Drizzle / Prisma push) w workflow: post-MVP polish.

---

## Pre-flight Checklist

- [ ] Sprint 05 zamknięty z retro i `Outcome`.
- [ ] Re-read `PRD.md §6 F4/F5/F6/F7/F8`.
- [ ] Re-read `DESIGN.md §10` (DAG ADR — nie wdrażamy, ale referencja), `§11`, `§14`, `§15`.
- [ ] Re-read `UX.md §4.4`, `§4.5`, `§7` (Resume modal), `§11.3`.
- [ ] Re-read `SECURITY.md §3`, `§5.4`, `§5.6`, `§9`, `§10`.
- [ ] Re-read `docs/providers/smallhost.md §6` (deploy / rsync excludes).
- [ ] Verify `gh` CLI ≥ 2.50 available on `make doctor`; document fallback ścieżkę REST + PAT.
- [ ] Confirm `make ci` green on `main` after Sprint 05 merge.

---

## Taski

### TASK-06.1 — Resume-on-launch for `pending_cleanups.json`

- **Estymata:** M
- **Zależności:** Sprint 05 done
- **Acceptance Criteria:**
  - [ ] `cmd/webox` startup scans `DefaultPendingPath()`; if a snapshot exists, TUI opens `StateResumeWizard` instead of Dashboard.
  - [ ] Resume modal shows: wizard run ID, started_at, provider profile, remaining cleanup steps; offers "Roll back now / Keep and exit / Discard snapshot".
  - [ ] "Discard" requires confirmation phrase typed back (anti-fat-finger).
  - [ ] `wizard.LoadPending` already handles `ErrCorruptedSnapshot` / `ErrSchemaMismatch`; TUI surfaces both as actionable errors.
  - [ ] Tests: corrupted snapshot path, schema mismatch path, happy resume into rollback, "keep and exit" leaves the file alone.
- **Docs:** `UX.md §7`, `DESIGN.md §10.0`, `AUDIT.md §8 IMP-1`.

### TASK-06.2 — GitHub client (`services/github/`)

- **Estymata:** L
- **Zależności:** Sprint 05 done
- **Acceptance Criteria:**
  - [ ] `services/github/client.go` exposes `CreateRepo`, `AddDeployKey`, `SetActionsSecret`, `DispatchWorkflow`, `GetLatestRun`.
  - [ ] Default transport is `gh` CLI (auth handled by user); REST fallback uses PAT from keyring (`secrets.GetGitHubPAT`).
  - [ ] All methods take `context.Context`; HTTP retries use exponential backoff with jitter.
  - [ ] PAT redaction tests: every error message and every log line scanned for `ghp_` / `github_pat_` / `ghs_` regex.
  - [ ] `services/github/ghmock/` provides recorded cassettes for tests (no live API in CI).
  - [ ] Sentinel errors: `ErrRepoExists`, `ErrPATInvalid`, `ErrPATScopeInsufficient`, `ErrRateLimited`, `ErrWorkflowDispatchFailed`.
- **Docs:** `PRD.md F4/F5`, `SECURITY.md §3.3`, `§10.4`, `DESIGN.md §11`.

### TASK-06.3 — Workflow templates (`assets/workflows/`)

- **Estymata:** M
- **Zależności:** TASK-06.2
- **Acceptance Criteria:**
  - [ ] `assets/workflows/{vite-react,node-express,static}/deploy.yml` osadzone przez `//go:embed`.
  - [ ] Każdy `uses:` step pinowany do pełnego 40-char SHA + komentarz z tagiem dla audit.
  - [ ] Templates render through `text/template` z polami: domain, ssh user, deploy path, rsync excludes, restart command.
  - [ ] Validator: `wizard/workflow_validate.go` używa `gopkg.in/yaml.v3` do unmarshalu przed zapisem; odrzuca wstrzyknięcie `${{}}` w polach, które nie powinny być expression-evaluated.
  - [ ] Tests: render każdej template z fixture-em planem, parse z powrotem przez YAML, snapshot rendered output.
  - [ ] Brak dynamicznego pobierania workflow z sieci.
- **Docs:** `AGENTS.md §2.1, §7.1`, `DESIGN.md §11`, `SECURITY.md §5.6`.

### TASK-06.4 — Wizard rozszerzony o Repo / Secrets / Workflow / Deploy

- **Estymata:** L
- **Zależności:** TASK-06.2, TASK-06.3
- **Acceptance Criteria:**
  - [ ] Project wizard ma dodatkowe kroki: `ProjectStepRepo`, `ProjectStepSecrets`, `ProjectStepWorkflowPreview`, `ProjectStepDeploy`.
  - [ ] Każdy z `CreateRepo`, `AddDeployKey`, `SetActionsSecret`, commit workflow do repo pushuje cleanup step (`CleanupKindGitHubRepo`, `CleanupKindDeployKey`, `CleanupKindActionsSecret`, `CleanupKindWorkflowFile`).
  - [ ] `MakeStepRunner` rozszerzony o nowe kindy; smallhost adapter pozostaje provider-agnostic wobec GitHub steps (dispatch przez `services/github`).
  - [ ] `wizardExecuteCmd` po Sprint 05 sekwencji wykonuje: create repo → add deploy key → set Actions secrets → commit workflow → dispatch first run; każdy success persistowany do `pending_cleanups.json`.
  - [ ] Failure UI ma trzy ścieżki: rollback all, rollback GitHub-only (keep provider-side), keep and exit.
  - [ ] Tests: success path, repo-already-exists conflict (recoverable), workflow dispatch 422 (`ErrWorkflowDispatchFailed`), rollback runs in correct LIFO order.
- **Docs:** `PRD.md F4-F8`, `DESIGN.md §10`, `§11`, `UX.md §4.4`.

### TASK-06.5 — First deploy + `last_deploy` integration

- **Estymata:** M
- **Zależności:** TASK-06.4
- **Acceptance Criteria:**
  - [ ] After `DispatchWorkflow`, wizard polls `GetLatestRun` until `completed`; surfaces in-progress spinner with link to GitHub Actions URL.
  - [ ] `last_deploy` w Dashboard / Project Detail czyta z `services/github.GetLatestRun` przez `status/` cache (TTL = 60s, key `gh:run:<repo>`).
  - [ ] Failure stanu (run conclusion ≠ `success`) renderuje "Deploy failed: <reason>" + skrót logu z `gh run view` ostatnich 50 linii.
  - [ ] No plaintext PAT w cache key ani w `status.Cache` storage.
  - [ ] Tests: happy poll loop (mock 3x in_progress → 1x completed), failure conclusion, network timeout, PAT-redaction smoke.
- **Docs:** `PRD.md F6/F8`, `DESIGN.md §8`, `§11`.

### TASK-06.6 — Post-deploy SSH verification

- **Estymata:** M
- **Zależności:** TASK-06.5
- **Acceptance Criteria:**
  - [ ] After workflow `success`, wizard runs SSH probe: `.env` ma `0600`, leży poza web root, `curl https://domain` zwraca 2xx/3xx.
  - [ ] Probe failure renderuje warning ale nie failuje wizardu (deploy ZADZIAŁAŁ; probe to safety net).
  - [ ] `webox doctor github` (read-only) pokazuje per-project: repo URL, deploy key SHA256, last run status, latency.
  - [ ] Doctor JSON output (`webox doctor github --json`) ma stały schema z `services/doctor` (versioned).
  - [ ] Tests: env perm violation surfaced jako warning; doctor JSON schema golden file.
- **Docs:** `SECURITY.md §10`, `DESIGN.md §13`, `providers/smallhost.md §6`.

### TASK-06.7 — Keymap matrix test + golden views

- **Estymata:** S
- **Zależności:** TASK-06.4
- **Acceptance Criteria:**
  - [ ] `tui/keymap_test.go` — table-driven test: for every step × every relevant key, asserts the expected `Update` transition (catches Vim-key-eating-input regressions before they ship; retro callout from Sprint 05).
  - [ ] Golden snapshots for `tui/views/project_wizard.go` and `tui/views/init_wizard.go` (80×24, 100×30) committed under `tui/views/testdata/`.
  - [ ] `make cover-check` raises `tui/views/` coverage above 60%.
- **Docs:** `docs/retros/2026-05-23-sprint-05.md` (open-question follow-up).

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| PAT wycieknie do logów / error messages | H | `services/github` rejected-substring guard + `internal/log/redact.go` regex; full PAT redaction test suite obowiązkowy. |
| Workflow templates outdated `uses:` SHA | M | Każdy SHA + tag w komentarzu; doctor sprawdza `gh release` ostatnich akcji vs zapisane SHA i ostrzega bez auto-update. |
| `gh` CLI nieobecny u usera | M | REST fallback przez PAT; `webox doctor` wykrywa, surface'uje jasny komunikat z linkiem do instalacji. |
| GitHub API rate limit (60/h dla anon, 5000/h dla PAT) | M | Exponential backoff + jitter, dashboard `last_deploy` cache TTL = 60s. |
| LIFO order źle odwraca workflow-after-deploy state | H | Rollback `CleanupKindWorkflowFile` to git commit/PR removal, nie reset SSH state; smallhost cleanups Sprint 05 nadal działają niezależnie. |
| Resume modal nie pokazuje się userowi, który zamknął wizard `Ctrl-C` | H | Snapshot zostaje na dysku; TASK-06.1 detect at startup; teatest scenariusz pokrywa. |
| First deploy zawiedzie z powodu missing Node version on small.pl | M | Wizard już sprawdza preflight `devil www options`; sprint 06 dodaje weryfikację po deployu. |
| Workflow file zostanie wepchnięty do złej gałęzi | M | Wizard wymaga explicit branch name (default `main`); validator odrzuca `*` / glob; smoke test commit do `main`. |

---

## Dependencies signoff

TASK-06.2 / TASK-06.3 mogą wymagać dodania:

1. `github.com/google/go-github/v68` (REST API, opcjonalne, jeśli `gh` CLI fallback okaże się za cienki). Decyzja w pre-flight: jeśli `gh` CLI obsługuje 95% scenariuszy, **nie dodajemy** zależności i polegamy na CLI + minimalnym handcrafted REST client. Każde dodanie wymaga ADR + maintainer sign-off zgodnie z AGENTS.md §1.2.

`gopkg.in/yaml.v3` jest już w `go.mod` (TASK-06.3 reuse).

---

## Outcome (wypełnij po sprincie)

- ✅ Done: TASK-06.X, ...
- ⏭️ Carry-over: ...
- 📌 Decyzje: ...
- 🧠 Surprises: ...
- 📊 Metryki:
  - Coverage `services/github/`: %
  - Coverage wizard-related new code: %
  - Workflow template render scenarios: %
  - End-to-end happy path duration (s):
- 🔒 Security validation:
  - [ ] No PAT in logs, error messages, status cache, or pending snapshot.
  - [ ] Workflow `uses:` pinned to full 40-char SHA, no tag-only references.
  - [ ] `.env` post-deploy: `0600` perms, outside web root, secrets not echoed in any operator log.
  - [ ] `go test -race ./services/github ./wizard ./tui/... ./cmd/webox` green.
- ➡️ Następny sprint: `sprint-07-import-doctor-polish.md`

---

## Retro Link

`docs/retros/YYYY-MM-DD-sprint-06.md`
