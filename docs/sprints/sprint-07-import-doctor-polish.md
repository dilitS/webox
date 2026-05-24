# Sprint 07 — Import, Doctor GitHub, Deploy Polish

> **Daty:** TBD → TBD · **Czas:** ~35-50h skupienia
>
> **Cel:** domknąć luki po Sprint 06 bez scope creepu: połączyć gotowy
> `services/github` i workflow renderer z realnym TUI flow, dodać
> read-only `webox doctor github`, dopiąć import istniejących projektów
> oraz poprawić ostatnie UX/polish elementy potrzebne do v0.1 MVP.

---

## TL;DR

Po sprincie 07:

- Project wizard ma realne ekrany Repo / Secrets / Workflow Preview / Deploy.
- Dashboard pokazuje `last_deploy` z GitHub Actions przez `status.Cache`
  (TTL 60s, key bez PAT).
- `webox doctor github` i `webox doctor github --json` raportują `gh`,
  PAT fallback, repo URL, deploy key metadata i ostatni workflow run.
- Import istniejących projektów działa read-only i oznacza braki jako
  actionable warnings, bez auto-naprawiania.
- Post-deploy probe sprawdza `.env` perms i `curl https://domain`, ale
  failure renderuje warning, nie cofa udanego deployu.

**Nie robimy w tym sprincie:**

- OAuth Device Flow (`webox auth login github`) — post-MVP.
- Env Merger, live log stream, `/db`, `/env`, topology map — dalej v0.2+.
- DAG rollback — dalej v0.3+.
- Nowy provider — MVP zostaje `smallhost`.

---

## Pre-flight Checklist

- [ ] Re-read `docs/sprints/sprint-06-github-deploy-workflow.md` Outcome.
- [ ] Re-read `PRD.md §6 F4-F10` i `§7` import existing projects.
- [ ] Re-read `DESIGN.md §8`, `§10.0`, `§13`, `§15.1-15.3`.
- [ ] Re-read `UX.md §4.4`, `§7`, `§11.3`.
- [ ] Re-read `SECURITY.md §9`, `§10.3`, `§10.4`.
- [ ] Decide with maintainer whether to add `gopkg.in/yaml.v3`; if yes,
  update ADR/PR rationale and replace conservative workflow text validation
  with AST validation.
- [ ] Confirm `go test -race ./services/github ./wizard ./tui/... ./cmd/webox`
  is green from Sprint 06 branch head.

---

## Taski

### TASK-07.1 — Wire Repo / Secrets / Workflow / Deploy Screens

- **Estymata:** L
- **Acceptance Criteria:**
  - [ ] `ProjectStepRepo`, `ProjectStepSecrets`, `ProjectStepWorkflowPreview`,
    `ProjectStepDeploy` are reachable from the TUI project wizard.
  - [ ] Repo name, visibility, branch and workflow preview are validated before
    any GitHub mutation.
  - [ ] `wizard.ExecuteGitHubProvision` is called from `wizardExecuteCmd`
    after provider-side subdomain / SSL / DB success.
  - [ ] Failure UI exposes rollback all, rollback GitHub-only, keep and exit.
  - [ ] Tests cover success path, repo conflict, dispatch failure, keep-and-exit.

### TASK-07.2 — Dashboard `last_deploy` Through Status Cache

- **Estymata:** M
- **Acceptance Criteria:**
  - [ ] `FetchProjectStatuses` can use an injected GitHub run fetcher.
  - [ ] Cache key is `gh:run:<owner>/<repo>` and never includes PAT.
  - [ ] TTL is `status.GitHubLastDeployTTL` (60s).
  - [ ] Dashboard and detail view render success/failure/in-progress states.
  - [ ] Tests cover stale cache, network timeout, failure conclusion.

### TASK-07.3 — `webox doctor github`

- **Estymata:** M
- **Acceptance Criteria:**
  - [ ] CLI accepts `webox doctor github` and `webox doctor github --json`.
  - [ ] Checks are read-only: `gh auth status`, `gh api /rate_limit`, optional
    configured repo metadata.
  - [ ] JSON schema remains versioned and stable.
  - [ ] Text output gives install/auth hints without printing token values.
  - [ ] Golden JSON fixture covers a warn-state report.

### TASK-07.4 — Post-deploy Probe

- **Estymata:** M
- **Acceptance Criteria:**
  - [ ] After successful workflow run, Webox verifies `.env` is `0600` and
    outside web root.
  - [ ] Webox probes `https://domain` and accepts 2xx/3xx.
  - [ ] Probe failure is warning-only and shown in wizard done screen.
  - [ ] Tests cover insecure perms and HTTP failure warnings.

### TASK-07.5 — Import Existing Projects

- **Estymata:** L
- **Acceptance Criteria:**
  - [ ] Import is read-only and provider-agnostic over `HostingProvider`.
  - [ ] Existing subdomains become `config.Project` entries with `ImportedAt`.
  - [ ] Missing repo / workflow / SSL / Node metadata surface as warnings.
  - [ ] No server mutation during import.
  - [ ] Tests cover clean import, duplicate config entry, unmanaged stack.

### TASK-07.6 — Sprint 06 Hardening Follow-ups

- **Estymata:** S
- **Acceptance Criteria:**
  - [ ] Race tests for touched packages are green.
  - [ ] `make lint` is green.
  - [ ] Workflow validation decision is documented: no YAML dep vs approved
    `gopkg.in/yaml.v3`.
  - [ ] CHANGELOG entries consolidated and Sprint 07 retro created.

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| TUI wizard becomes too large and brittle | H | Keep GitHub screen state in focused form structs; I/O only in `tea.Cmd`. |
| Doctor accidentally becomes mutating | H | `doctor github` only uses GET/CLI status commands; no `secret set`, no repo writes. |
| PAT leaks through status errors | H | Every GitHub-facing error is redacted; tests scan for token prefixes. |
| Import creates false confidence | M | Imported projects are marked `ImportedAt` and warnings stay visible until reconciled. |
| YAML dependency decision blocks workflow validator | M | Maintain conservative validator until explicit dependency sign-off. |

---

## Outcome (2026-05-23 — partial)

- ✅ Done:
  - TASK-07.2 — Dashboard `last_deploy` SWR-cached via injected
    `GitHubLastDeployFetcher`; cache key `gh:lastDeploy:<owner>/<repo>:<workflow>`
    (no PAT in key), TTL `status.GitHubLastDeployTTL` (60s). Production wiring
    in `cmd/webox` uses the `services/github` CLI transport. Tests cover hit,
    miss, failure-degrades, nil-fetcher, and cache reuse across refreshes.
  - TASK-07.3 — `webox doctor github [--json]` implemented as a read-only
    sub-runner in `services/doctor/github.go`. Checks `gh` CLI presence,
    `gh auth status` (parsed with PAT redaction via `internal/log.Redact`),
    `gh api /rate_limit` with a 10% warn threshold, and keyring slot
    presence (no token value). CLI parser routes `github` as a doctor
    subcommand, `--json` accepted in any position.
  - TASK-07.5 — Read-only import preview reachable from Dashboard via
    `i`. Calls `WizardRunner.ListProviderSubdomains` per profile, joins
    with `config.Projects`, presents a managed/new diff modal, and on
    `a` writes stub `config.Project` entries with `ImportedAt`. No
    server resource is touched.
  - TUI project actions (carry-over from Sprint 06 polish backlog):
    `r/s/v` on project detail dispatch restart / SSL renew / log tail
    through the new `WizardRunner` seam. Restart/renew invalidate the
    matching `status.Cache` prefix; log tail renders inside a scoped
    panel with truncation hint. `providers.HostingProvider.TailLog`
    added with line clamping and adapter implementation for small.pl.
- ⏭️ Carry-over (next sprint or dedicated PR):
  - TASK-07.1 — TUI Repo / Secrets / Workflow Preview / Deploy screens.
    The underlying `wizard.ExecuteGitHubProvision` is ready and tested;
    the missing piece is the wizard form (steps, PAT prompt, preview)
    plus the runner method that calls `ExecuteGitHubProvision` after a
    successful provider-side execution. Scope is significant enough to
    warrant a dedicated focused PR.
  - TASK-07.4 — Post-deploy probe (`.env` perms, `https://domain`
    verify). Plumbing exists at the provider layer (`TailLog`, SSH
    pool); needs a dedicated probe runner + wizard-done warning UI.
  - TASK-07.6 — Workflow YAML validation decision (no extra dep
    confirmed for MVP; stays as conservative text-level validator).
- 📌 Decyzje:
  - Doctor GitHub stays strictly read-only. The PAT slot probe returns
    only a boolean and is wrapped in `secrets.ErrSecretNotFound` /
    `secrets.ErrKeyringUnavailable` checks so the doctor never opens a
    cleartext token. `gh auth status` output is passed through
    `internal/log.Redact` before any rendering branch.
  - Import preview is intentionally not editable. Stub projects keep
    `ImportedAt != nil` so the dashboard renders them with the `STALE`
    badge until the operator runs the wizard for them. The provider is
    never asked to mutate anything during import.
  - `last_deploy` cache key is `gh:lastDeploy:<owner>/<repo>:<workflow>`
    rather than per-run-id, to keep the cache small and avoid TTL
    fragmentation across pushes. Failure renders as `unavailable` while
    keeping the cache intact (SWR falls back to the previous value on
    a subsequent successful probe).
- 🧠 Surprises:
  - The MVU `Update` clears `m.alert` unconditionally on every
    incoming message, so any async `tea.Cmd` that set an alert would
    have it wiped by the next `StatusRefreshedMsg`. Fixed by gating
    the clear on `tea.KeyMsg`. Added a regression test in
    `tui/actions_test.go`.
  - golangci-lint v2 (mnd, err113) flags many magic numbers and any
    inline `errors.New` introduced for fast iteration. Every new
    integer constant in this sprint is named (`hoursPerDay`,
    `daysPerMonth`, `repoRefParts`, `importColDomainWidth`, etc.) and
    `services/doctor/github.go` exports
    `ErrRateLimitFetcherMissing` so the rate-limit default uses a
    sentinel instead of a dynamic message.
- 📊 Metryki:
  - `make ci` (full bundle): green — lint 0 issues, govulncheck clean,
    coverage 70.3% (threshold 70%), race tests green.
  - Coverage hot spots:
    - `services/doctor` 72.6% (new `github.go` covered by table
      tests for all four checks + JSON leak guard).
    - `tui` 65.1% (project actions + import + last-deploy fetcher).
    - `tui/views` 21.5% (renderer smoke tests for import preview
      added; remaining gap is the styled wizard renderers that have
      golden snapshots instead).
- 🔒 Security validation:
  - [x] No PAT/token/key in logs, errors, status cache, config, or
    pending snapshots. `services/doctor/github.go`
    `TestGitHubDoctor_JSONIsStableAndDoesNotLeakSecrets` greps the
    rendered JSON for `ghp_`, `ghs_`, `github_pat_`, `PRIVATE KEY`.
  - [x] `doctor github` is read-only: every dep is a GET / status
    probe; no `secret set`, no repo writes.
  - [ ] Post-deploy `.env` probe: deferred with TASK-07.4.

---

## Retro Link

`docs/retros/YYYY-MM-DD-sprint-07.md`
