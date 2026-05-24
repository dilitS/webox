# Changelog

All notable changes to **Webox** are documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).

Until `v1.0.0` (GA), **MINOR** bumps may contain breaking changes per [ROADMAP §2.1](./docs/ROADMAP.md#21-semver).
After `v1.0.0`, breaking changes are reserved for MAJOR bumps only.

Each entry is short and links to the PR / issue that introduced the change.
For the *why* behind larger architectural shifts, read the corresponding [ADR](./docs/adr/).

---

## [Unreleased]

### Added
- **UI/UX refresh (2026-05-24) — Bento Ultra cockpit polish + offline mock mode.**
  - `tui/components/statusbar.go` introduces a new full-width cockpit status bar (`WEBOX vX.Y.Z [LIVE]` badge on the left, pipe-delimited `clock · profile · uptime · load · RAM · ping` stream on the right). Tone (success/warning/info/error) drives the `LIVE`/`STALE`/`PENDING`/`OFFLINE` pill colour. Pure renderer — no I/O, no time calls, fully unit-tested in `statusbar_test.go`.
  - `tui/bento/tiles.go` rebrands every tile to match the reference cockpit: `[Active Projects]` with dot-suffixed rows (`●` Success/Warning/Error/Muted) and rounded selection pill; `[SERVER: <project>]` with iconified key-value rows (Profile / Stack / Node.js / Status / HTTP / SSL / Repo / Last Deploy) and status-tinted dots; `[CI/CD PIPELINE: Main Branch]` with `Build #N: STATUS` badge and `[1] step ✓` rows; `[Live Server Logs]` with timestamped `INFO/WARN/ERROR/DEBUG` colour-coded lines; cyan `[Topology]` placeholder tile. Each tile picks its own `TileAccent` (Primary/Cyan/Warning/Error) which paints the rounded border so the operator can identify panes by colour alone.
  - `tui/bento/engine.go` rewires the Ultra grid into a two-column header row (Projects ↔ Server/CI/CD stack) over a full-width Live Logs row — matching the reference image exactly. The engine accepts an optional pre-rendered status bar via `WithStatusBar` and reserves vertical space for it before computing tile heights.
  - `tui/theme/theme.go` adds the `Accent` token (`#38BDF8` cyan default / `#0277C2` light) so the CI/CD tile renders with a distinct cool border without polluting the primary magenta palette.
  - `cmd/webox/run.go` ships a `--mock` flag (and `WEBOX_MOCK=1` env) that boots the cockpit with `tui.MockOptions(configPath)`. No SSH, no HTTP probes, no GitHub API calls: every fetcher returns deterministic seed data (six demo projects mirroring the reference image — ShopEase-Web, API-Gateway, Auth-Service, Dashboard, Dashboard-Admin, Payment-UI — with a fixed `2026-05-24 14:32:01 UTC` clock so screenshots stay reproducible).
  - `tui/mockdata.go` exports `MockOptions`, `MockConfig`, `MockProjectStatuses`, `MockLiveLogLines`, `MockGitHubLogsFetcher`, and a `mockWizardRunner` that satisfies `tui.WizardRunner` with non-mutating in-memory responses. `tui/Options.PreloadedConfig` short-circuits the on-disk config loader so `Init()` does not race against a missing `~/.config/webox/config.json` in mock mode.
  - `internal/version/format.go` exposes `version.Short()` (compact `vX.Y.Z` for the new status bar; full `Format()` line untouched).
  - `tui/components/modal.go` extends `ModalTone` with `ToneSuccess` (re-used by the status bar's green `LIVE` pill) and adds a `Tone = ModalTone` alias for callers that want the shorter name.
- **Sprint 10 — Live CI/CD Pipeline Panel + F8 Workflow Logs Modal.**
  - `services/github.Transport` extended with `GetWorkflowSteps(ctx, repo, runID) ([]Step, error)` and `GetWorkflowLogs(ctx, repo, runID, maxLines) ([]WorkflowLogLine, error)` plus the matching `Client` facades. CLI primary path proxies through `gh api /repos/.../actions/runs/<id>/jobs` and shells out to `gh run view <runID> --log`; REST fallback hits the same jobs endpoint and returns a typed `ErrPATScopeInsufficient` for the log endpoint (zip stream we deliberately do not unpack in-process).
  - New sentinel errors `ErrRunNotFound` (treated as recoverable "no run yet") and `ErrStepsParseError` (gh schema skew worth investigating).
  - `services/github.Step` and `WorkflowLogLine` projections + `WorkflowRunSummary.IsTerminal` so the tile can switch between static badge and live elapsed-time rendering without touching the transport.
  - `services/github/logs.go::parseGHLogLines` redacts every log line through `internal/log.Redact` **before** it leaves the transport boundary, then optionally caps to the last `maxLines` (Sprint 10 plan TASK-10.3 hard cap = 50).
  - `services/github.WorkflowRun` gained the missing `RunNumber` field (`run_number`) so the tile can render `Build #N`.
  - `tui/bento.NewCICDPipelineTile` ships with a `CICDPipelineSnapshot` (alias / workflow / run number / status / duration / steps / `RateLimited` / `RateLimitHint` / `ErrorMessage`). Steps render as numbered list with UX-§3.1 badges (`✓ ✗ ⏳ … ⊘ ⊗ ?`). Header indicator switches between `[LIVE]` / `[STALE]` / `[LIMITED]` and the footer hints `[F8] View logs · [Enter] Open run`.
  - `tui/cicd.go` adds the polling pipeline: 10-second `tea.Tick` (`status.GitHubStepsTTL`), `status.GetOrFetchMeta` SWR cache (`gh:steps:<owner>/<repo>:<workflow>`), per-project snapshot map, and graceful rate-limit handling (cached steps preserved, hint extracted from `reset=<RFC3339>` markers when present).
  - F8 logs modal: `cicdLogsModalForm` + double-border `components.RenderModal`, red border for `FAILED ✗` runs, `↑/↓` scroll, `Esc/F8` to dismiss. Lines arrive already redacted from the transport so the modal cannot leak PATs.
  - `tui/update.go::onDashboardSelectionChanged` invalidates the active project's CI/CD cache entry and triggers an immediate refetch when the operator moves the selection cursor, satisfying TASK-10.4.
  - `cmd/webox/run.go` wires `pipelineFetcherFor` and `logsFetcherFor` against the shared `ghsvc.Client` so all three GitHub call paths (last-deploy / pipeline / logs) reuse the same auth state.
  - `status` package: new `PrefixGitHubSteps = "gh:steps:"` and `GitHubStepsTTL = 10s`; `EventDeploy` invalidation list now includes `gh:steps:` so the post-deploy refresh shows fresh pipeline data immediately.
- **Sprint 09 — Live Log Stream foundations + Header Bar Server Metrics.**
  - `services/sshtail/` — context-cancellable `tail -f` streamer with a
    1-method `Executor` seam (production wires it to `ssh.Pool`; tests
    inject canned byte streams without booting a mock SSH server).
    Pre-buffer redaction via `internal/log.Redact` is the
    non-negotiable security contract: every emitted `Line.Raw` is
    already sanitised, `Redacted` flags whether a regex matched.
    Sentinels: `ErrLogPathInvalid`, `ErrSessionClosed`,
    `ErrReconnectFailed`, `ErrStreamerClosed`. Exponential backoff
    (2s/4s/8s) and `shellEscape` + `validateLogPath` for log-path
    sanitisation (rejects `..`, NULs, newlines).
  - `services/sshmetrics/` — `Poller.Poll` with parsers for `uptime`
    (Linux days+H:M, Linux days+min, Linux H:M, FreeBSD, macOS
    `up D+H:M`) and `free -m`. `Metrics` projection (Uptime / Memory /
    RTT) cached via `status.Cache` SWR (TTL 5s, key
    `ssh:metrics:<alias>`). Graceful degradation when `free` is
    missing (FreeBSD): zeroed RAM rather than failing the whole poll.
    `FormatUptime`/`FormatRAM`/`FormatLoadAvg`/`FormatRTT` helpers.
  - `tui/components/` — generic thread-safe `RingBuffer[T]` (Push /
    Snapshot / Tail / Len / Cap, circular overwrite, default capacity
    1000, snapshot returns independent copy). `ANSIStrip` (SGR + OSC +
    residual) and `ParseLogLevel` with ordered fall-through
    (ANSI colour → bracketed prefix → `level:` / `level=` → JSON
    `"level":"…"` → word-boundary scan → `LevelInfo`). Benchmarks:
    `RingBuffer.Push` ≈ 6 ns/op, Redact 200-char PAT line ≈ 18 µs/op
    (both well under Sprint 09's perf budget).
  - `tui/bento/` — two new live tiles backed by snapshots so the
    layout engine stays free of `services/` imports:
    `NewHeaderMetricsTile` (`HeaderMetricsSnapshot` →
    `[LIVE]`/`[STALE]` badge + Uptime/Load/RAM/Ping row) and
    `NewMicroLogsTile` (`MicroLogLine` → marker-per-level micro tail
    with `(redacted)` annotation). Placeholders kept as the
    "no data yet" fallback for both slots.
  - `tui/` — `TabLogs` promoted to MVP (`Enabled()` returns true);
    `enterLiveLogsTab` lazily allocates the ring buffer per project,
    `updateLiveLogsKey` honours `f` (toggle auto-scroll), `c` (clear
    buffer), `↑/↓` (pause auto-scroll + scroll), `Esc/1/←` (back to
    Overview). New view `tui/views/live_logs.go` renders the tab with
    `Active File · Stream · Connected · Buffer N/N` strip,
    level-coloured rows, and the Sprint 09 keybinding hints.
  - `internal/log/redact.go` — three new patterns: JWT (anchored on
    `eyJ…` header), generic `key=value`/`key: value` for `password`,
    `passwd`, `token`, `secret`, `api[_-]?key`, `access[_-]?key` in
    CLI args / env / JSON, and `mysql/mysqldump/psql -p<password>`
    (anchored on the binary name to avoid touching unrelated tools).
    Corpus expanded to 13 secret families with a 200-sample property
    test (0% leakage, well under the 5% acceptance margin).
  - `tui/cockpit_snapshot_test.go` — new `TestSprint09Snapshots`
    produces `docs/screenshots/sprint-09-live-logs-120x35.txt`
    (opt-in via `WEBOX_SNAPSHOT=1`) so reviewers can diff the
    live-log tab visually without an SSH session.

### Security
- **CI/CD pipeline log redaction at the transport boundary.** Every
  line returned by `services/github.GetWorkflowLogs` passes through
  `internal/log.Redact` *before* it is buffered, scrolled, or rendered
  by the F8 modal. Tests prove the modal cannot leak `ghp_…` PATs even
  when the workflow output prints them verbatim
  (`TestCLITransport_GetWorkflowLogs_TailAndRedact`,
  `TestParseGHLogLines_RedactsSecrets`).
- **CI/CD cache key never carries credentials.** The status-cache key
  `gh:steps:<owner>/<repo>:<workflow>` deliberately omits PAT/auth
  state — gh CLI's cached auth handles the request, not the cache
  layer (SECURITY §10.4).
- **Rate-limit graceful degradation.** The CI/CD tile preserves the
  last successful pipeline snapshot and renders a `[LIMITED]` badge +
  reset hint instead of clearing the data, so a primary/secondary
  GitHub rate-limit response cannot hide an in-flight failed deploy
  from the operator (TASK-10.5).
- **Goroutine leak coverage for the SSH tail pipeline.**
  `services/sshtail/leak_test.go` runs `goleak.VerifyNone` on the
  cancel-to-shutdown happy path *and* the exhausted-reconnect failure
  path. Both must clean up within 500 ms (CI jitter buffer over the
  100 ms perf-budget cap).
- **Redactor corpus uplift.** Sprint 09 added test coverage for
  GitHub PATs (`ghp_`/`github_pat_`/`ghs_`), OpenAI keys (`sk-`),
  AWS access keys, JWTs, OpenSSH/RSA private key blocks, MySQL/Postgres
  URIs with embedded credentials, generic `key=value` secrets, and
  MySQL/PSQL `-p<password>` CLI flags. Recall validated against a 200-
  sample randomised property test.

### Added
- **Sprint 08 — Bento Ultra Layout Engine + premium components.**
  - `tui/bento/` adaptive layout engine with `BentoTile` interface,
    `Slot` enum, `Registry`, and a stateless `Engine` that renders four
    tiers (`Tiny` ≤70×22 fallback, `Standard` 100×30, `Ultra` 120×35,
    `UltraPlus` 160×45). Mode detection is pure (`bento.DetectMode`);
    `bento.Resolve` layers in the `WEBOX_LAYOUT` env override
    (`tiny`/`standard`/`ultra`/`ultraplus`/`auto`).
  - Six tile implementations: `ProjectsTile`, `OverviewTile`, plus four
    placeholder tiles for `Header Metrics`, `CI/CD Pipeline`,
    `Live Micro-Logs`, and `Topology` — each advertises the sprint
    (09/10/11) that will wire its live data, so the Ultra silhouette is
    visible end-to-end even before the next sprints land.
  - `tui/theme/` palette extended with a `Light()` variant (eleven
    OKLCH-anchored roles), premium `StatusBadge` (filled background +
    bold for `ONLINE`/`BUILDING`/`OFFLINE`/`STALE`/`DEGRADED`), and a
    `Gradient()` utility (sRGB interpolation, multi-byte rune safe).
  - `tui/components/` package — `RenderHeaderBar` (gradient title +
    pill badge), `LogoArt`, `FormatModeBadge`, `RenderModal`
    (double-border with `Info`/`Warning`/`Error` tones and a
    drop-shadow strip), `SpinnerStyle`/`NewAdaptiveSpinner` (`Dot`
    for Standard, `Pulse` for Ultra/UltraPlus).
  - `Model.BentoMode()` plus `Options.LayoutOverride` for tests; the
    spinner frame set is recomputed on `tea.WindowSizeMsg` when the
    resolved mode changes.
  - Opt-in cockpit snapshot generator (`WEBOX_SNAPSHOT=1 go test ./tui
    -run TestCockpitSnapshots`) that writes ANSI-stripped renders to
    `docs/screenshots/sprint-08-*.txt` for visual review.

### Changed
- **MVP scope (v0.1) significantly expanded by [ADR-0007](./docs/adr/0007-bento-ultra-eskalacja-mvp.md):** Bento Ultra adaptive layout (`100×30` / `120×35` / `160×45`), Live Log Stream (`tail -f` via SSH with ring buffer + ANSI level coloring + pre-render redaction), Live CI/CD Pipeline Panel (live GitHub Actions workflow steps + click-through logs), and Live Service Topology Map are now in v0.1 — previously they were 🔶 STRETCH (v0.2+). Roadmap re-baselined from P50 22 → 27 weeks, P70 32 → 35 weeks. Four new sprints added: 08 (Bento Ultra Layout Engine + OKLCH theme + sprint-leak cleanup), 09 (Live Log Stream + Header Bar Metrics), 10 (CI/CD Panel), 11 (Topology Map). Rationale: brand promise of "Terminal Cockpit klasy premium" from PRD §3 and UX TL;DR requires premium visual layer in MVP, not v0.2+ — early-adopter perception of v0.1 matters more than +5-week delay. v0.2 reshuffled to focus on second provider, Env Merger, Sound Engine, fast-chord bindings, and multi-provider dashboard aggregator (instead of catching up on visual layer).

### Added
- ADR-0007 — explicit override of the [AGENTS.md §2.4](./AGENTS.md#24-scope-discipline) scope-discipline guardrail to escalate Bento Ultra, Live Log Stream, GHA live panel, and Topology Map from STRETCH (v0.2+) to MVP (v0.1). Cross-linked from PRD §6 (F14/F15 P1→P0), ROADMAP §3.0/§3.1/§3.3/§3.5/§4.2, AGENTS §3.1/§3.2, UX TL;DR/§3.4/§4.2/§4.3 Tab [4]. Sprint plans `sprint-08-bento-ultra.md`, `sprint-09-live-log-stream.md`, `sprint-10-cicd-panel.md`, `sprint-11-topology-map.md` created with full task breakdown, AC, risk watch, and outcome templates.
- TUI project actions (Sprint 07 push toward production): `[r] Restart`,
  `[s] SSL Renew`, and `[v] Tail Logs` on the project-detail screen,
  wired through a new `WizardRunner.{RestartApp,RenewSSL,TailLog,
  ListProviderSubdomains}` seam. Restart and renew invalidate the
  matching `status.Cache` prefix on success; tail logs renders the
  last 200 lines inside a scoped panel. `providers.HostingProvider`
  gained `TailLog(ctx, domain, lines)` with line-count clamping
  (`defaultTailLines=200`, `maxTailLines=10000`), and the small.pl
  adapter ships an implementation that tails `node.log` + `error.log`
  while treating "missing file" exit codes as soft errors.
- `webox doctor github [--json]` — read-only GitHub integration
  diagnostics in `services/doctor/github.go`. Checks the `gh` CLI
  presence on PATH, parses `gh auth status` (with PAT redaction via
  `internal/log.Redact`), probes `GET /rate_limit` through the gh
  transport, and reports keyring slot presence for the default PAT
  account. CLI argument parser now treats `github` as a subcommand
  for `webox doctor`, and `--json` is forwarded regardless of
  position.
- Dashboard `last_deploy` integration: `tui.FetchProjectStatusesWithGitHub`
  resolves the most recent workflow run per project through a SWR
  cache (`gh:lastDeploy:<owner>/<repo>:<workflow>`, 60s TTL) and
  formats it as `2m ago · success`. The production wiring lives in
  `cmd/webox`; nil fetchers degrade gracefully to a `—` placeholder
  so the dashboard never blocks on GitHub.
- Read-only import preview (PRD F9): pressing `i` on the dashboard
  scans every configured profile for subdomains via
  `WizardRunner.ListProviderSubdomains`, joins them with
  `config.Projects`, and shows a managed/new diff. Accepting with
  `a` writes stub `config.Project` entries for the unmanaged rows
  with `ImportedAt` set; no server resource is mutated. The new
  `StateImportPreview` route lives alongside the existing wizard
  states.
- `services/github/` — minimal GitHub integration for Sprint 06 with
  `gh` CLI as the primary transport, REST+PAT fallback, repository
  creation, deploy keys, Actions secrets via sealed-box encryption,
  workflow dispatch, latest-run polling, workflow-file commits, and
  metadata-only cleanup methods for LIFO rollback.
- `assets/workflows/` and `wizard/workflow_validate.go` — embedded
  deploy workflow templates for `vite-react`, `node-express`, and
  `static`; all GitHub Actions `uses:` references are pinned to full
  40-character SHAs and rendered workflow fields reject GitHub
  expression / shell injection.
- Resume-on-launch for `pending_cleanups.json`: the TUI now opens a
  Resume Wizard when an interrupted LIFO snapshot exists, supports
  rollback from the loaded stack, keep-and-exit, and phrase-confirmed
  discard.
- `wizard.ExecuteGitHubProvision` — GitHub-side wizard sequencing for
  repo creation, deploy key, Actions secrets, workflow file commit, and
  workflow dispatch, with cleanup steps persisted after every successful
  external mutation.
- TUI regression coverage for Sprint 06: keymap matrix tests for wizard
  text-vs-picker behavior, Resume Wizard tests, and committed golden
  view fixtures for init/project wizard review states at 80×24 and
  100×30.
- `docs/sprints/sprint-06-github-deploy-workflow.md` —
  rolling-wave plan for Sprint 06 closing the MVP path: resume on
  launch for `pending_cleanups.json`, `services/github` minimal
  client, embedded workflow templates pinned to full 40-char SHAs,
  wizard extension for repo/secrets/workflow/deploy, post-deploy
  SSH verification, and a TUI keymap-matrix test follow-up.
- `docs/retros/2026-05-23-sprint-05.md` — Sprint 05 retrospective
  capturing the secret-shape guard pattern in `wizard.Stack.Push`,
  the Vim-key-eats-input regression and its picker/text-step gate,
  the `wizardStackSlot` pointer-on-Model decision, and the
  promotion of preflight failures to sentinel errors.
- `wizard/` package — first writable flow in Webox. Five files split
  by responsibility: `types.go` (CleanupStep, ProvisionPlan,
  ProvisionReport, DatabaseCredentials, ProvisionStatus),
  `plan.go` (supported stacks `vite-react`/`node-express`/`static`,
  supported DB kinds `mysql`/`postgres`, ValidatePlan that wraps
  `ErrInvalidPlan`), `rollback.go` (LIFO `Stack` with secret-shape
  guard on every Push and `MakeStepRunner` dispatcher over
  `providers.HostingProvider`), `pending_cleanups.go` (atomic
  persistence of the stack into `pending_cleanups.json` with
  schema_version pinning, `ErrCorruptedSnapshot` /
  `ErrSchemaMismatch` sentinels, and a `FilePersister` that uses
  `os.O_EXCL` tmpfile + rename in the same directory), and
  `execute.go` (Preflight, CheckSubdomainAvailable, Execute,
  IsRecoverable; pushes cleanups in reverse order of provisioning
  so SSL is removed before subdomain).
- `wizard/errors.go` — explicit sentinels for the wizard package
  (`ErrInvalidStep`, `ErrSecretInCleanup`, `ErrUnsupportedKind`,
  `ErrInvalidPlan`, `ErrCorruptedSnapshot`, `ErrSchemaMismatch`,
  `ErrPreflightSSHDisconnected`, `ErrPreflightNilStatus`). Lets the
  TUI branch via `errors.Is` instead of string matching, and keeps
  `err113` lint green.
- `tui/wizard.go`, `tui/wizard_runner.go`,
  `tui/views/project_wizard.go`, and `tui/views/init_wizard.go`
  (rewrite) — interactive init wizard (Welcome → Alias → Host →
  Port → User → Review) and full project wizard (Profile → Stack →
  DB choice → DB kind → DB name → Domain → Review → Execute →
  Failure → Rolling back). The runner seam keeps `Update` pure: it
  builds the `HostingProvider` on demand inside a `tea.Cmd` so
  side-effecting I/O never happens during message dispatch. Vim-style
  `j`/`k` navigation is gated to picker steps only so text inputs
  consume every rune.
- `tui/commands.go` wizard tea.Cmds: `saveProfileCmd`,
  `wizardPreflightCmd`, `wizardDomainCheckCmd`, `wizardExecuteCmd`,
  `wizardRollbackCmd`. The execute command generates a UUID per
  wizard run, threads the same `*wizard.Stack` through the model via
  a small mutex-guarded `wizardStackSlot`, persists progress after
  every success, and clears `pending_cleanups.json` on commit.
- Wizard test corpus: `wizard/plan_test.go`,
  `wizard/rollback_test.go`, `wizard/pending_cleanups_test.go`,
  `wizard/execute_test.go`, `wizard/fake_provider_test.go`, and
  `tui/wizard_test.go`. Scenarios cover happy path, domain
  collision (recoverable, stays in wizard), SSL failure with
  rollback, DB failure with rollback, persistence corruption, schema
  drift, context cancellation, and the dashboard→wizard re-entry
  with cache invalidation. `go.mod` adds `github.com/google/uuid` as
  the only new direct dependency.
- `docs/sprints/sprint-04-tui-shell.md` — rolling-wave plan for
  Sprint 04 (Bubble Tea / Lipgloss bootstrap, MVU shell, read-only
  dashboard with SWR refresh, Project Detail Overview tab). Dependency
  sign-off section enumerates the four direct deps the sprint adds.

### Changed
- `tui/model.go`, `tui/states.go`, `tui/messages.go`, `tui/update.go`
  and `tui/view.go` extended with wizard sub-states
  (`StateProjectWizard`, `InitWizardStep`, `ProjectWizardStep`) and
  matching `tea.Msg` types (`ProfileSavedMsg`,
  `ProfileSaveFailedMsg`, `ProjectWizardPreflightMsg`,
  `ProjectWizardDomainCheckedMsg`, `ProjectWizardExecutedMsg`,
  `ProjectWizardRolledBackMsg`). All wizard state lives inside
  `Model`; no new globals.
- `tui/commands.go` `loadConfigCmd` now returns
  `ConfigLoadedMsg{Missing: true}` whenever `cfg.Profiles` is empty,
  not only when the file is absent. This lets the TUI route any
  zero-profile install (including hand-edited configs) through the
  init wizard instead of dropping the user on an empty dashboard.
- `wizard/plan.go` no longer imports `providers/smallhost` directly.
  `ValidatePlan` now accepts a `providers.PlanValidators` set
  resolved from the new validator registry, so the wizard layer can
  drive any registered provider without a compile-time dependency on
  a concrete adapter. `providers/validators.go` exposes
  `RegisterPlanValidators` / `PlanValidatorsFor` with sentinel
  `ErrUnknownValidator` / `ErrInvalidValidatorSet` errors, and
  `wizard.Execute` resolves validators from the provider's name.
- `smallhost.Provider` carries an injectable `now func() time.Time`
  clock instead of the package-level `nowFn`. The shared mutable
  global is gone, which removes a `t.Parallel()` race vector and
  lets tests assert latency calculations deterministically via
  `SetClock`.
- `cmd/webox/run.go` factors the TUI bootstrap into
  `runTUIWith(configPathResolver, teaProgramFactory, teaRunner)`
  seams so the CLI's failure paths (config lookup, program build,
  program run) are unit-testable without a real terminal.

### Fixed
- `services/doctor/github.go` `parseGHAccount` now strips the leading
  `✓` (or `x`) status glyph that `gh` 2.40+ prepends to the
  "Logged in to ..." line. Without this, `webox doctor github`
  reported WARN ("no active account was parsed") for properly
  authenticated users on modern `gh` releases.

### Security
- GitHub PAT and Actions secret handling now has explicit redaction and
  non-leak tests: CLI/REST errors are filtered through the project
  redactor, Actions secret plaintext is passed only through stdin or
  sealed-box ciphertext, and GitHub rollback snapshots carry only
  metadata, never token or key material.
- Every `wizard.Stack.Push` rejects `CleanupStep.Params` values that
  match the project-wide secret regex corpus (same source as
  `internal/log/redact.go`). Tests in `wizard/rollback_test.go` and
  `wizard/pending_cleanups_test.go` assert that a
  `REDACTED-NEVER-A-REAL-SECRET-...` token in any param surfaces
  `ErrSecretInCleanup` and is never written to disk.
- `ProvisionReport.Credentials` is populated only in memory while
  the wizard runs; on success the TUI never re-renders the
  plaintext password and `RemovePending` truncates the snapshot
  file before the wizard transitions back to the dashboard.
- `docs/retros/2026-05-23-sprint-03.md` — Sprint 03 retrospective
  capturing the executor-seam pattern, the tripwire-prefixed fixture
  passwords, and the `commit-msg` hook learnings.
- `providers/smallhost/methods.go` + `executor.go` (TASK-03.6) —
  HostingProvider method skeletons wire the Devil parsers to a
  narrow `Executor` seam. Production wiring uses `NewSSHExecutor`
  over `ssh.Pool`; tests inject a recording fake. Every command
  builder uses pre-validated tokens (`ValidateDomain`,
  `ValidateDBName`, `ValidateNodeVersion`) before concatenation so
  shell injection is impossible at the boundary. Methods map
  parser sentinels onto the HostingProvider contract (idempotent
  Remove*, ErrSubdomainExists, ErrAppNotFound, ErrAppNotNode,
  ErrDNSNotResolving, ErrRateLimitLetsEncrypt, ErrCLINotFound). The
  fail-closed branch — methods invoked before SetExecutor — returns
  `providers.ErrUnknownOutputFormat` wrapped with an "executor not
  configured" sentinel so wiring bugs surface in tests instead of
  silent no-ops.
- `parseVhostList`, `parseSSLAdd`, `parseSSLDelete`, `parseDBAdd`,
  `parseDBDelete` in `providers/smallhost/parsers.go` (TASK-03.5) —
  cover the SSL provisioning round-trip (account IP lookup → cert
  install → cert delete) and the MySQL/PostgreSQL provisioning
  round-trip (create with panel-generated credentials → delete).
  `parseSSLAdd` maps DNS-not-resolving and Let's Encrypt rate-limit
  outputs onto `ErrDNSNotResolving` / `ErrRateLimitLetsEncrypt`.
  `parseDBAdd` extracts username + password via named regex groups
  and the test corpus asserts the password never leaks back into
  error strings. `parseSSLDelete` / `parseDBDelete` treat "no cert" /
  "not found" as nil so LIFO rollback can replay safely. Fixtures
  use `REDACTED-NEVER-A-REAL-SECRET-` as a tripwire prefix the
  redactor will catch even if a real password ever slips in.
- `providers/smallhost/parsers.go` + `testing/fixtures/devil/`
  (TASK-03.4) — defensive parsers for `devil www add`, `devil www
  restart`, and `devil www list`. `stripAndNormalize` caps each
  command output at 1 MiB, strips ANSI escapes, normalises CRLF/CR
  to LF, and rejects non-printable bytes via
  `providers.ErrUnknownOutputFormat`. Maps the well-known panel
  responses onto sentinels (`ErrSubdomainExists`,
  `ErrNodeVersionUnsupported`, `ErrAppNotFound`, `ErrAppNotNode`)
  using named regex groups; unknown shapes fail closed without
  echoing raw output into operator logs. Fixtures ship with
  `.fixture.md` provenance notes (`captured: inferred` until live
  capture replaces them), a CRLF variant, an empty-list rendering,
  and an adversarial fixture mixing ANSI colour, NUL/BEL bytes, and
  `$(rm -rf /)` to verify the parser never lets the substring into
  the returned error.
- `providers/smallhost/paths.go` (TASK-03.3) — pure path helpers
  (`GetDeployPath`, `GetLogPath`, `EnvPath`, `StoragePath`) plus
  `ValidateDomain` / `ValidateUser` with the `ErrInvalidDomain` /
  `ErrInvalidUser` sentinels. The validators reject leading/trailing
  dashes, uppercase, NUL/CR/LF/space, `..`, `/`, `\` and any label
  longer than 63 characters before the value reaches a path or
  command string. Helpers fail closed by returning "" for invalid
  domain or user so the rsync target never collapses to `/`.
- `providers/smallhost/config.go` + `methods.go` (TASK-03.2) — adapter
  constructor and typed [`Properties`] bag for small.pl / Devil. The
  factory rejects unsupported `restart_method`, parses `ssh_pool_max`
  (range `[1,16]`, default 3), and `ssh_algorithms_legacy_compat`
  (default false). Registration happens in `init()` via the new
  registry. Method stubs implementing `HostingProvider` return a
  `providers.ErrUnknownOutputFormat`-wrapped sentinel until TASK-03.6
  replaces them; this keeps the interface contract testable now
  without leaking half-finished SSH wiring into later tasks.
- `providers/provider.go`, `providers/errors.go`, `providers/registry.go`
  (TASK-03.1) — canonical `HostingProvider` contract, sentinel errors
  (`ErrInvalidProviderConfig`, `ErrUnknownProvider`,
  `ErrProviderAlreadyRegistered`, `ErrUnknownOutputFormat`,
  `ErrOutputTooLarge`, `ErrSubdomainExists`,
  `ErrNodeVersionUnsupported`, `ErrAppNotFound`, `ErrAppNotNode`,
  `ErrDNSNotResolving`, `ErrRateLimitLetsEncrypt`, `ErrDBNameTaken`,
  `ErrCLINotFound`), and a sync-guarded factory registry with
  `Register` / `Unregister` / `Names` / `New`. `New` normalises Port
  to 22 and Properties to non-nil before invoking the factory, runs
  registry lookup before validation so a typo in `type` surfaces as
  `ErrUnknownProvider` instead of being masked by validation noise,
  and propagates factory errors via `%w` while keeping the provider
  name in the message. Coverage: 100%.
- `docs/sprints/sprint-03-provider-smallhost.md` — rolling-wave plan for
  Sprint 03 (provider contracts, `smallhost` constructor, path helpers,
  Devil parser fixtures, and smallhost method skeleton over `ssh.Exec`).
- `docs/retros/2026-05-23-sprint-02.md` — Sprint 02 retrospective with
  the `x/crypto/ssh` security upgrade, pool race fix, and process change
  to run lint after each task commit.
- `services/httpcheck/` (TASK-02.7) — dashboard probes for HTTP status
  and TLS certificate expiry. `ProbeHTTP` returns status code, class
  (`2xx`/`3xx`/`4xx`/`5xx`) and latency with a default 1 s timeout;
  `ProbeTLS` performs a TLS handshake and returns leaf `NotAfter` plus
  `DaysLeft`, also with injectable 1 s timeout / clock seams. Tests use
  `httptest.NewServer` and `httptest.NewTLSServer`.
- `status/ttl.go` + invalidation metadata (TASK-02.6) — ADR-0005 TTL
  constants and deterministic prefixes (`http:`, `ssh:node:`, `ssl:`,
  `gh:lastDeploy:`), event-to-prefix invalidation for Restart / Deploy /
  SSL / Node changes, `Cache.Invalidate(prefix)`,
  `Cache.InvalidateEvent(event)`, and `GetOrFetchMeta[T]` returning
  `Metadata{IsStale, Age, FetchedAt, ExpiresAt}` for dashboard buffered
  badges.
- `status/cache.go` (TASK-02.5) — generic package-level
  `GetOrFetch[T]` implementing the in-memory SWR contract from
  DESIGN §8 / ADR-0005: fresh hit returns immediately, stale hit returns
  buffered data while refreshing in the background, cold miss blocks on
  fetch, and `singleflight` dedupes concurrent misses per key. Adds
  direct dependency `golang.org/x/sync v0.20.0` after the Sprint 02
  SSH security update raised the main module to Go 1.25.
- `ssh/exec.go` + `ssh/keepalive.go` (TASK-02.4) — pooled `Exec`
  helper returning `ExecResult{Stdout, Stderr, ExitCode, Duration}`,
  per-client `keepalive@openssh.com` global request loop (default
  15 s), and reconnect classification via `RetryPolicy` with default
  `3s/6s/12s` backoff. `Exec` intentionally does not replay commands
  after transport failure; providers must verify remote state first.
- `ssh/pool.go` + `ssh/dialer.go` (TASK-02.3) — concurrency-safe SSH
  connection pool keyed by `Target.Key()` with default `max=3` per host,
  5 s acquire timeout, 60 s idle timeout, `Acquire`/`Release`/`Close`,
  lazy + background idle cleanup, double-release no-op behavior, and a
  `NetDialer` that upgrades `net.Dialer` TCP connections through
  `golang.org/x/crypto/ssh.NewClientConn`.
- `testing/sshmock/` (TASK-02.2) — deterministic in-process SSH server
  for integration tests without real hosting accounts or shelling out to
  system `ssh`. It binds localhost on a random port, generates ephemeral
  ed25519 host/client keys per test, enforces public-key-only auth, maps
  command strings to stdout/stderr/exit status, and injects disconnect /
  delay failures for pool and reconnect tests.
- `ssh/errors.go`, `ssh/types.go`, `ssh/client_config.go` (TASK-02.1) —
  foundation for the Sprint 02 connection pool. Ships five sentinel
  errors (`ErrPoolBusy`, `ErrHostKeyUnknown`, `ErrHostKeyMismatch`,
  `ErrReconnectExhausted`, `ErrHostKeyDBRequired`), the `Target` /
  `ExecResult` / `Clock` / `Dialer` / `HostKeyDB` contracts, and a
  `BuildClientConfig` builder that declares the algorithm whitelist
  from `docs/SECURITY.md §5.5` (ed25519, rsa-sha2-512, rsa-sha2-256,
  ecdsa-sha2-nistp256; `ssh-rsa` only when
  `LegacyAlgorithmCompat=true`; `ssh-dss` never) and wraps a
  `HostKeyCallback` that maps `knownhosts.KeyError` outcomes onto
  distinguishable unknown / mismatch sentinels. Coverage: 100%.
- `cmd/webox` now launches the Bubble Tea TUI shell, with read-only
  dashboard navigation, Project Detail Overview, SWR-backed status refresh
  commands, and Sprint 04 teatest smoke coverage.
- `docs/sprints/sprint-05-wizard-project.md` — rolling-wave plan for the
  project wizard, first-run profile setup, provider-side provisioning, and
  LIFO rollback with `pending_cleanups.json`.
- `docs/retros/2026-05-23-sprint-04.md` — Sprint 04 retrospective covering
  the `config.Load` first-run mismatch, teatest output capture, Charm v1/v2
  import-path decision, and golden snapshot gaps.

### Security
- Main module toolchain floor is now `go 1.25.0` so Webox can use
  `golang.org/x/crypto v0.52.0`, the first `x/crypto/ssh` release that
  fixes all `govulncheck` findings triggered by the new SSH client /
  server code paths in Sprint 02. Keeping Go 1.24 would leave reachable
  SSH vulnerabilities in `ssh.NetDialer`, `ssh.Exec`, keepalive, and
  `testing/sshmock`.
- `ssh.BuildClientConfig` refuses to construct a `ClientConfig` without
  a `HostKeyDB`, returning the typed `ErrHostKeyDBRequired` sentinel
  instead of falling back to `cryptossh.InsecureIgnoreHostKey`. This
  enforces the "strict block on host-key mismatch" guardrail from
  `AGENTS.md §2.1` end-to-end — there is no code path that produces a
  working `ssh.Client` without an explicit known_hosts implementation.

### Security
- `secrets.FallbackBackend` (TASK-01.7) now stores credentials in an
  AES-GCM-256 encrypted file keyed by Argon2id (`time=3, memory=64MB,
  parallelism=2, keyLen=32`), so headless Linux / Docker / WSL /
  FreeBSD environments without an OS keyring can still run Webox
  without ever writing plaintext secrets to disk. Per
  `docs/SECURITY.md §4.2.1` and `AUDIT §8 IMP-2`, every write generates
  a fresh 96-bit nonce via `crypto/rand.Read` and any CSPRNG failure
  panics rather than degrading silently (test
  `TestFallbackBackend_CSPRNGFailurePanics`). In-memory keys live
  in `memguard.LockedBuffer` and are zeroed by `Close()` /
  `RotatePassword`. File format is the
  `version(1B) | salt(16B) | nonce(12B) | ciphertext+tag` blob
  documented in the sprint plan.
- `secrets.ReadMasterPassword` (TASK-01.7) reads the fallback master
  password through `golang.org/x/term.ReadPassword` (no echo) and
  honours `WEBOX_MASTER_PASSWORD` for ephemeral CI runners. Per
  `docs/SECURITY.md §4.2.2` and `AUDIT §8 IMP-3`, presence of the env
  var on a workstation host (CI markers absent, `DISPLAY`/`SSH_CLIENT`/
  `XDG_SESSION_TYPE` present) emits a single warning to STDERR so
  contributors notice the `/proc/<pid>/environ` exposure surface.

### Added
- `services/doctor/` + `cmd/webox/doctor.go` — MVP `webox doctor`
  subcommand with local diagnostics for Go runtime info, config-dir
  writability, secrets backend classification (`os` / `fallback` /
  `none`), `secrets.enc` permission checks, `WEBOX_MASTER_PASSWORD`
  workstation warning, and `SSH_AUTH_SOCK` probing. Reports render both
  as colored human output and `--json` machine output with stable exit
  codes `0/1/2`.
- `i18n/i18n.go` + tests — Sprint-01 translation skeleton with a tiny
  in-memory `Catalog`, `T(key, args...)`, EN/PL tables, five doctor
  strings, and fail-soft fallback to English / the raw key.
- `internal/telemetry/telemetry.go` + tests — explicit local-only
  telemetry seam (`Sink`, `Event`, `Disabled`) so future instrumentation
  can depend on a stable no-op interface without violating the
  zero-remote-telemetry policy.
- `docs/sprints/sprint-02-ssh-cache.md` — full rolling-wave plan for the
  next sprint (SSH connection pool, `testing/sshmock`, SWR cache, HTTP /
  TLS probes).
- `secrets/fallback.go`, `secrets/fallback_crypto.go`,
  `secrets/fallback_io.go` — full `FallbackBackend` with `NewFallback`,
  `Get`/`Set`/`Delete`/`Close`/`RotatePassword`, atomic write through
  `<path>.tmp.<pid>.<rand>` + `fsync` + `rename` + parent-dir
  `fsync`, intra-process `sync.Mutex` and cross-process `flock(2)` on
  `<path>.lock`. Zero-value backend returns the new
  `ErrFallbackLocked` for every operation so callers must construct
  through `NewFallback`.
- `secrets/password.go` + `secrets/password_test.go` — master-password
  resolution helper covered by table-driven CI-vs-workstation
  heuristic tests, env-var path, and a non-terminal stdin pipe path.
- `secrets/lock_unix.go` + `secrets/lock_windows.go` — `flock(2)`
  helper with exponential backoff, deadline-aware retries, context
  cancellation, and an `ErrSecretsLocked` sentinel. Windows is a
  compile stub awaiting `LockFileEx` (release-blocked v0.2+, mirroring
  `config/lock_windows.go`).
- `secrets/fallback_test.go` + `secrets/fallback_branches_test.go` —
  TDD suite exercising round-trip, persistence across re-open, wrong
  password (`ErrAuthFailed`), corrupted/forged file branches
  (`ErrCorruptedSecrets`), 1000-write nonce uniqueness, password
  rotation with persist-failure rollback, file permissions, 16-way
  concurrent `Set`, `RotatePassword` on locked / too-short input, and
  the CSPRNG panic seam.
- `secrets.MasterPasswordMinLen` (12, per ADR-0004) and the
  `MasterPasswordEnv` constant exported for downstream `cmd/webox`
  consumers.
- Dependencies: `github.com/awnumar/memguard v0.23.0` (Apache-2.0;
  locked buffers, pure Go), `golang.org/x/crypto v0.41.0` (Argon2id),
  `golang.org/x/term v0.34.0` (no-echo terminal read). All three are
  the libraries declared in `AGENTS.md §1.2`; main module stays on
  `go 1.24` (downstream crypto is the last release that doesn't push
  the toolchain past 1.24).

### Changed
- `cmd/webox` now routes `webox doctor` and `webox doctor --json` through
  the same lightweight manual parser used for `--version` / `--help`,
  keeping ADR-0001's "small CLI surface" intact without introducing
  Cobra or other command frameworks.
- `docs/sprints/sprint-01-foundations.md`, `docs/sprints/README.md`, and
  `docs/retros/2026-05-23-sprint-01.md` close Sprint 01 at 10/10 tasks,
  57/57 AC and link the ready-to-start Sprint 02 plan.
- `secrets/backend.go` no longer ships a placeholder `FallbackBackend`
  — the type and its methods now live in `secrets/fallback.go`. The
  interface itself is unchanged.
- `secrets.Detect()` returns `nil, ErrKeyringUnavailable` on any of
  the previously-fallback paths (unsupported platform, probe Set
  failure, probe Get failure other than `ErrNotFound`). The TASK-01.6
  behaviour of returning an unusable locked `&FallbackBackend{}`
  placeholder was a transitional shim; the real wiring asks the
  caller to construct `NewFallback` with a resolved password.
- `secrets/errors.go` retires `ErrFallbackUnavailable` in favour of
  `ErrFallbackLocked` (zero-value backend), `ErrAuthFailed`,
  `ErrCorruptedSecrets`, `ErrMasterPasswordTooShort`, and
  `ErrKeyringUnavailable` (Detect path).

### Fixed
- `tools/go.mod` now pins dev tooling to Go 1.26.3 instead of 1.26.2 so
  `govulncheck` no longer runs on vulnerable `net@go1.26.2`
  (`GO-2026-4971`) in CI.
- `config.Validate()` now enforces two non-negotiable guardrails that the
  first Sprint 01 pass missed during review:
  - rejects secret-shaped strings anywhere in `config.json`
    (`ghp_`, `ghs_`, `github_pat_`, `sk-`, PEM private-key headers) via
    `ErrSecretInConfig`, matching `AGENTS.md §2.1`;
  - rejects dangling `projects[].profile_alias` references via
    `ErrDanglingProfileAlias`, aligning runtime validation with
    `docs/DESIGN.md §6.1` FK semantics.
- `config.Load()` maps both review-fix sentinels to `ErrSchemaMismatch`
  instead of accepting malformed-but-well-shaped configs.
- `docs/DESIGN.md §6.1` now points at the real fixture path
  `testdata/config/valid_v1.json` instead of the stale pre-bootstrap path.

### Added
- `secrets/backend.go` — `Backend` interface for secret storage
  (`Get`, `Set`, `Delete`) plus a TASK-01.7 placeholder `FallbackBackend`
  returning `ErrFallbackUnavailable`.
- `secrets/keyring.go` — OS keyring detection through write/read/delete probe
  using `github.com/zalando/go-keyring`. `Detect()` now distinguishes
  `ErrUnsupportedPlatform` (fallback) from `ErrNotFound` after a successful
  probe write (`ErrBrokenKeyring`, with doctor hint) and cleans the probe key
  after successful writes.
- `secrets/keyring_test.go` and `secrets/keyring_mock_test.go` — mock-driven
  TDD suite for happy path, unsupported platform fallback, broken keyring
  detection, cleanup, wrapper behavior, and the `go-keyring` mock backend.
- Dependency: `github.com/zalando/go-keyring` v0.2.8. This is the keyring
  library already selected in `AGENTS.md §1.2`; the PR documents the dependency
  rationale and keeps usage isolated behind `secrets.Backend`.
- `internal/log/redact.go` — pure `Redact(input string) string` for local
  diagnostic output. It redacts SSH private key blocks, GitHub classic and
  fine-grained tokens, AWS access-key-shaped values, `Authorization: Bearer`
  headers, password-bearing URLs, sensitive `.env` assignments, JSON
  password/token/secret fields, OpenAI-style `sk-...` tokens, and long
  `ssh-rsa` key material.
- `internal/log/redact_test.go` and `testdata/redact/*.txt` — table-driven
  malicious-input corpus without storing complete secret-shaped literals on
  disk. Tests assert that redacted output never contains the original generated
  secret and that safe substrings remain intact.
- `internal/log/redact_bench_test.go` — `BenchmarkRedact100KB`, currently
  ~4.64ms/op locally on Apple M4, satisfying the Sprint 01 <5ms target.
- `config/migrate.go` — real migration framework for on-disk config data:
  `type Migration func(in []byte) (out []byte, newVersion int, err error)`,
  `var migrations = map[int]Migration{0: migrateV0toV1}`, and public
  `Migrate(data []byte)` that iterates forward to `config.Current`, rejects
  non-forward migrators, validates current-version input, and logs each
  transition through `slog` (`migrationFrom`, `migrationTo`).
- `config/migrate_v0_to_v1.go` — placeholder v0→v1 migrator for the
  pre-schema draft shape (`profile` singular, no `schema_version`) into the
  canonical v1 shape (`schema_version: 1`, `profiles[]`, `projects[]`,
  optional `settings`).
- `config.Load()` now migrates stale configs before v1 schema validation,
  writes a backup of the original bytes as
  `<path>.bak.v<old>.<timestamp>`, then persists migrated v1 through
  atomic `Save()`.
- `testdata/config/v0.json` and `testdata/config/v0_migrated_to_v1.json`
  drive the migration golden test and `Load` backup+save integration test.
- `config/save.go` — atomic `Save(ctx, path, cfg)` for `config.json`:
  parent-dir create (`0700`), exclusive `<path>.lock` `flock(2)` with
  timeout/backoff, JSON marshal+validate, write to
  `<path>.tmp.<pid>.<rand>`, `fsync(file)`, atomic `rename`, and
  `fsync(parent dir)` for durability on Unix filesystems.
- `config/lock_unix.go` — Unix lock helper around `syscall.Flock`
  (`LOCK_EX|LOCK_NB`) with exponential backoff and `ErrConfigLocked`
  sentinel on timeout; `config/lock_windows.go` added as compile stub so
  future `LockFileEx` work has a stable seam.
- `config/save_test.go` — TDD suite for the write path:
  happy path + perms, concurrent saves, invalid cfg no-write, pre-rename
  abort leaves original intact, cancelled context, parent-dir creation,
  lock timeout, and helper branch tests (`writeTempFile`, `syncDirectory`,
  `marshalConfig`).
- `config/internal_branches_test.go` — white-box tests covering otherwise
  hard-to-hit branches: broken embedded schema parse/compile, lowered
  generic `summarise()` path, `tempPath()` randomness failure, and
  non-object `validateProfileAliasIntegrity`.
- `config/load.go` — `config.Load(ctx, path) (*Config, error)` reads, schema-validates,
  decodes, and forward-migrates `config.json`. Distinguishable error sentinels:
  `ErrCorruptedConfig` (I/O / malformed JSON), `ErrSchemaMismatch` (schema
  violation or future-version downgrade), `ErrMigrationFailed` (legacy
  `schema_version` cannot be advanced — wired up fully in TASK-01.4). Missing
  files return `DefaultConfig()` without any disk side effect.
- `config.DefaultConfig()` — exported factory for the in-memory defaults
  (`SchemaVersion: 1`, `Language: "en"`, allocated empty Profile/Project slices)
  Load returns when `config.json` is absent.
- Tests:
  - `config/load_test.go` — table-driven `TestLoad_TableDriven` (corrupt JSON,
    two schema-violation fixtures, future schema_version), plus dedicated tests
    for happy path, missing-file no-side-effect invariant, cancelled context,
    and unreadable file (chmod 000, skipped under root).
  - `config/migrate_internal_test.go` — golden v0→v1 migration, idempotence,
    current-version no-op, missing/non-forward/failing migrator paths, `slog`
    transition fields, and `Load` backup+save integration.
- `config/types.go` — strongly-typed `Config`, `Profile`, `Project`,
  `SecretMeta`, `Settings` structs implementing `docs/DESIGN.md §6.1`. No
  field uses `any`/`interface{}` (enforced by reflection-driven test).
  `SecretSource` enum constants (`managed`/`server_only`/`external`)
  mirror `docs/SECURITY.md §10.6`.
- `config/schema.json` + `config/schema.go` — embedded JSON Schema
  (Draft 2020-12) describing the on-disk format, lazily compiled with
  format assertion enabled (`uuid`, `date-time`). New
  `config.Validate(raw []byte) error` returns `errors.Is(_, ErrInvalidJSON)`
  for malformed JSON and `errors.Is(_, ErrSchemaViolation)` for
  schema-level errors with a flattened, lower-cased message digest.
- `config/errors.go` — sentinel errors `ErrInvalidJSON`,
  `ErrSchemaViolation` (additional `Err*` will land with TASK-01.2 Load).
- `testdata/config/valid_v1.json` — canonical golden fixture
  exercising every documented optional field (language, port,
  properties, repo, imported_at, secrets_meta, settings).
- `testdata/config/invalid_*.json` — four negative fixtures driving the
  schema test table: missing schema_version, missing profile.type,
  uppercase alias regex violation, non-UUID project id.

### Changed
- `go.mod` — added `github.com/santhosh-tekuri/jsonschema/v6` v6.0.2
  (Apache 2.0; no-network compiler, format assertion via
  `c.AssertFormat()`).
- `docs/sprints/sprint-01-foundations.md` — TASK-01.1 acceptance criteria
  marked done; field list aligned with DESIGN §6.1
  (`Profiles/Projects/Settings`, dropping the speculative
  `Defaults/LastSync` from the original draft).
- `docs/sprints/sprint-00-bootstrap.md` — marked all 46 acceptance-criteria
  checkboxes and 4 pre-flight items as done; populated `## Outcome` with
  decisions (module path, `tools/go.mod` over `tools.go`, signs placeholder,
  versioned `.githooks/`), surprises, and shipped metrics. Fixes the
  `make next-task` drift: the script now correctly advances to TASK-01.1.

### Added
- `docs/retros/2026-05-22-sprint-00.md` — Sprint 00 (Bootstrap)
  retrospective: 10/10 tasks closed, 23 commits to `main`, coverage 96.4%,
  with explicit lessons for AC-discipline and `goreleaser` schema gotchas.
- `scripts/` — full dev-loop automation: `dev-watch.sh` (TDD with
  auto-detected gow / fswatch+entr / inotifywait / polling fallback),
  `sprint-status.sh`, `next-task.sh`, `new-task.sh`, `start-sprint.sh`,
  `retro-new.sh`, `pr-create.sh`, `commit-msg-suggest.sh`,
  `changelog-add.sh`, `install-git-hooks.sh`, `bootstrap.sh`. All scripts
  share `lib.sh` (colors, sprint discovery, repo helpers).
- `.githooks/` — versioned, opt-in git hooks wired by `make setup-hooks`:
  `pre-commit` (gofumpt/goimports auto-fix, fast lint, secret tripwire),
  `commit-msg` (Conventional Commits 1.0.0 validation), `pre-push`
  (`make test-short`, override `WEBOX_PREPUSH=full`), `prepare-commit-msg`
  (auto-suggest CC from staged diff).
- `Makefile` — new dev-flow targets (`dev`, `bootstrap`, `setup-hooks`,
  `sprint-status`, `next-task`, `next-task-verbose`, `sprint-start`,
  `new-task`, `retro`, `pr`, `commit-suggest`, `changelog`, `ci-fast`).
- `.github/labeler.yml` + `.github/workflows/labeler.yml` — automatic
  path-based PR labels (area/docs, area/security, area/config, …).
- `.github/workflows/dependabot-auto-merge.yml` — auto-merge patch + minor
  (non-prod) Dependabot bumps after CI green; majors require human review.
- `.vscode/settings.json` + `.vscode/extensions.json` — project-scoped
  format-on-save, gopls with gofumpt, golangci-lint on save, recommended
  Cursor/VS Code extensions for new contributors.
- `.cursor/skills/task-start/SKILL.md` — agent picks next sprint task,
  reads spec, branches, starts watch loop, hands off to `tdd-loop`.
- `.cursor/skills/auto-changelog/SKILL.md` — agent maintains
  `CHANGELOG.md` `[Unreleased]` as part of every behavior change.
- `docs/sprints/README.md` §6.0 — automation reference for the whole
  workflow (Makefile / hooks / skills / CI).
- `docs/sprints/` — rolling-wave sprint planning system:
  - `README.md` — methodology (DoR, DoD, cadence, anti-patterns, capacity rules).
  - `sprint-00-bootstrap.md` — full task breakdown (10 tasks) for repo
    bootstrap, CI pipeline, `goreleaser` dry-run, and GitHub policy files.
  - `sprint-01-foundations.md` — full task breakdown (8+2 tasks) for
    `config/` (atomic write + flock + migrations), `secrets/` (keyring
    probe detection, AES-GCM fallback with `memguard`), `redactor`, and
    `webox doctor` minimum with explicit TDD targets and coverage gates.
- `docs/RISKS.md` — risk register with 13 enumerated risks, likelihood ×
  impact scoring, mitigation strategies, and concrete contingency
  (plan B) paths. Active monitoring threshold ≥ 9, escalation ≥ 16.
- `SECURITY.md` (repo root) — GitHub-visible security policy with private
  reporting channel and link to `docs/SECURITY.md` threat model.
- `.github/pull_request_template.md` — DoD checklist with sprint/task
  linkage, security checklist for crypto/SSH changes, and 7-day cooldown
  reminder for handmade crypto code (per `RISKS.md` R-003).
- `.github/ISSUE_TEMPLATE/{bug,feature,config}.yml` — structured issue
  forms with pre-submit redaction reminders and roadmap awareness.
- `.github/CODEOWNERS` — protect critical surface (`secrets/`, `docs/adr/`,
  `.github/workflows/`, sprint planning) behind owner review.
- `.github/dependabot.yml` — weekly Go module + GitHub Actions updates,
  Conventional Commits prefixes.
- `docs/AUDIT.md` — comprehensive pre-implementation audit with 39 findings
  (P0–P3) and 5 open decisions blocking the start of `v0.1` implementation.
- `AGENTS.md` — operator handbook for AI coding agents (stack, guardrails,
  TDD workflow, scope policy, conventional commits, retrospective cadence).
- `.cursor/skills/` — workflow skills (TDD, add-provider, ADR, audit-trace,
  secret-flow, retro, commit-policy, release-check) auto-triggered on
  matching tasks.
- `.cursor/rules/` — contextual rules wired to file patterns
  (`alwaysApply: false` + glob-based `description`).
- `.cursor/hooks/` + `.cursor/hooks.json` — `beforeSubmitPrompt`,
  `beforeShellExecution` and `afterFileEdit` guards:
  `secret-scan-prompt.sh` (ask on token in prompt),
  `secret-scan-shell.sh` (deny on secret in shell argv),
  `secret-scan-file.sh` (post-write context warning),
  `gofmt.sh` (auto `goimports` / `gofmt -s -w` on Go files),
  `commit-validator.sh` (Conventional Commits 1.0.0 enforcement),
  `scope-guard.sh` (STRETCH-path tripwires per AUDIT A6).
- `docs/retros/` — institutionalised retrospective notes. First entry:
  `2026-05-22-pre-implementation-audit.md` documenting the full audit +
  environment bootstrap effort.
- `Makefile` — canonical task interface (`make build`, `make test`,
  `make lint`, `make vulncheck`, `make doctor`, etc.).
- `.editorconfig` — repo-wide formatting baseline (LF, UTF-8, gofmt
  tab style, 2-space YAML/MD).
- `.gitignore` — broadened to cover Go build artifacts, runtime state
  (`webox.log`, `pending_cleanups.json`, `secrets.enc`), and editor noise.
- `go.mod` (`module github.com/dilitS/webox`, `go 1.24`) and the canonical package layout per docs/DESIGN.md §2.1: `cmd/webox`, `tui`/`tui/views`, `providers`/`smallhost`/`mock`, `ssh`, `services`, `config`, `secrets`, `status`, `wizard`, `env` (STRETCH stub), `i18n`, `assets`, `testing`, `internal/log`, `internal/version` — each with a godoc-style `doc.go` (TASK-00.1 + TASK-00.6).
- `internal/version` exports `String()`/`Format(v, c, d)` — pure helper plus ldflags-fed package vars (`Version`/`Commit`/`Date`). 8 table-driven cases (TASK-00.5).
- `cmd/webox` parses `--version`, `--help`/`-h`, `--debug` per ADR-0001 with manual `os.Args` parsing; `Run([]string, stdout, stderr) int` is the testable seam (`main` is a thin wrapper). Unknown args exit 2 with a hint to `--help`. Coverage 100% on `Run`/`parseArgs` (TASK-00.5).
- `tools/go.mod` — isolated modfile pinning dev tools via Go 1.24 `tool` directive: `golangci-lint` v2.12.2, `govulncheck` v1.3.0, `gofumpt` v0.10.0, `goimports`, `goreleaser` v2.15.4. Main module stays on `go 1.24`; tools live in the pinned tools Go version with `GOTOOLCHAIN` derived from the modfile and pinned in `Makefile` so every contributor and CI runner uses bit-identical tool builds (TASK-00.2).
- `.golangci.yml` — golangci-lint v2 config enforcing the linter set declared in `CONTRIBUTING.md §2.1` and `AGENTS.md §2.2`: correctness (`bodyclose`, `errcheck`, `errorlint`, `govet`, `ineffassign`, `staticcheck`, `unused`), security (`gosec`), style (`dupl`, `gocritic`, `misspell`, `revive`, `whitespace`), maintainability (`gocyclo` ≤ 20 per AUDIT IMP-19, `prealloc`, `unconvert`, `unparam`), error discipline (`err113`), observability (`loggercheck`, `mnd`); test files relax `dupl`/`err113`/`gocyclo`/`gosec`/`mnd`/`unparam`. `gofumpt`+`goimports` run as v2 formatters with `local-prefixes: github.com/dilitS/webox`. `make lint` exits 0 against the current tree (TASK-00.3).
- `.github/workflows/ci.yml` — first green CI pipeline. Five jobs (`lint`, `test`, `vulncheck`, `build`, `ci-summary`) gated by a single fan-in summary check that branch protection can pin against. Triggered on every branch `push` plus `pull_request` to `main`; PR reruns auto-cancel via `concurrency`, while `push` runs finish. `lint` runs `golangci-lint v2` plus `go vet`; `test` is a Linux/macOS matrix with coverage artifact upload (14-day retention); `vulncheck` is ubuntu-only; `build` cross-compiles `linux/darwin × amd64/arm64` with `CGO_ENABLED=0` and native smoke-tests the CLI where the runner can execute the target binary. Top-level `permissions: contents: read`; Go telemetry disabled via `GOTELEMETRY=off`. All actions pinned to full 40-char commit SHA with version comment for auditability and Dependabot-friendly bumps (TASK-00.4).
- `.goreleaser.yml` — initial GoReleaser v2 config for Sprint 00 dry-runs: `builds` matrix `linux/darwin × amd64/arm64` with `CGO_ENABLED=0`, `archives` as `tar.gz`, `checksum.algorithm: sha256`, and a clearly marked signing placeholder that preserves the future `cosign sign-blob --bundle=...` shape without requiring real signing material yet. `goreleaser check` and `make release-dry-run` both exit 0 locally (TASK-00.8).

### Changed
- `cmd/webox/run.go` — declared `exitOK`/`exitMisuse` constants and named `parseArgs` returns to satisfy `mnd` and `gocritic.unnamedResult`; behaviour unchanged (TASK-00.3).
- `docs/CONTRIBUTING.md §1.1` — split the requirements table: end-user prerequisites stay in the table, dev tools are now documented as **pinned via `tools/go.mod`** with their exact versions and the `go tool -modfile` workflow (TASK-00.2 follow-up).
- `.github/workflows/labeler.yml` and `.github/workflows/dependabot-auto-merge.yml` — pre-existing workflows now SHA-pinned: `actions/labeler@f27b608878404679385c85cfa523b85ccb86e213 # v6.1.0`, `dependabot/fetch-metadata@25dd0e34f4fe68f24cc83900b1fe3fe149efef98 # v3.1.0`. Removes the "TODO: pin in Sprint 00" placeholders (TASK-00.4).
- `README.md` — added the CI status badge linking to `actions/workflows/ci.yml`, satisfying Sprint 00 bootstrap visibility requirements for `v0.0.0-bootstrap` readiness (TASK-00.4 / TASK-00.10 dependency).
- `Makefile` — local `make ci` now includes `build`, so the canonical local bundle better matches the GitHub Actions gate instead of skipping the binary smoke build altogether (TASK-00.4).
- `go.mod`, `Makefile`, `.goreleaser.yml`, `.github/workflows/ci.yml`, README links, Go imports/tests, and internal agent docs/rules/skills — renamed the module path from the bootstrap placeholder `github.com/webox/webox` to `github.com/dilitS/webox` so imports, ldflags, pkg.go.dev links, CI metadata, and release references all align with the actual GitHub origin before tagging `v0.0.0-bootstrap` (TASK-00.10).
- `docs/ROADMAP.md` — replaced single-line estimate with P50/P70/P90 table
  (solo: ~22 weeks P50, ~32 weeks P90), added sprint → release mapping
  table, and a re-baseline checkpoint after Sprint 03.
- `docs/DESIGN.md` §10 — clarified MVP uses **LIFO stack** with
  `pending_cleanups.json`; DAG is `v0.3+` stretch (IMP-1).
- `docs/AUDIT.md` §8 — folded the 19 second-pass `IMP-*` findings into the
  durable audit record, then removed the temporary improvement plan file.
- `README.md` — replaced inline data-URI hero with a committed SVG asset,
  added pre-MVP installation/status section, removed dead placeholder links,
  and clarified MVP vs STRETCH package boundaries.
- `docs/adr/0001`, `PRD.md`, `AGENTS.md` — clarified that the CLI ban applies
  to **operator commands**, while startup/debug/diagnostic flags remain allowed.
- `docs/adr/0005` — corrected cold-cache dashboard math: 20 SSH-heavy project
  fetches are pool-limited and warm progressively instead of completing in ~3 s.
- `docs/adr/0004` — replaced stale `zerocopy.Wipe` language with `memguard`
  and documented Go memory-safety limits.
- `docs/DESIGN.md` §6 — replaced racey PID-based lockfile with
  `flock(2)` / `LockFileEx` via `github.com/gofrs/flock` (AUDIT A8).
- `docs/DESIGN.md` §8 — replaced 60-line generic Go snippet with
  functional contract description and TTL/invalidation table
  (Go does not support generic methods — AUDIT A3).
- `docs/DESIGN.md` §3 + §4 — unified `ProviderConfig` struct and
  `Factory` signature across DESIGN / CONTRIBUTING / smallhost docs
  (AUDIT A2). Fixed `CPINalled` typo → `CLIInstalled` (AUDIT A4).
- `docs/DESIGN.md` — added missing subsections referenced from other
  docs: §2.1 (repo layout), §2.2 (data flow), §2.3 (MVU rules),
  §3.1–§3.4 (contract, properties bag, parsing), §5.1–§5.4 (SSH pool),
  §6.1–§6.4 (config schema/perms/save/migrations), §13.5 (workflow
  template), §15.1–§15.3 (doctor categories/redactor/JSON schema)
  (AUDIT A5, A7).
- `docs/DESIGN.md`, `docs/UX.md` — marked scope-crept sections as
  `🔶 STRETCH (v0.2+)`: Env Merger, Border Pulsing, Sound Engine,
  Live Service Topology Map, Bento Ultra, fast-chord bindings, tabs
  `EnvDiff` / `Database` / `Logs` (AUDIT A6).
- `docs/SECURITY.md` §4.2 — rewrote keyring detection to distinguish
  `keyring.ErrUnsupportedPlatform` from `keyring.ErrNotFound` via
  probe write/read/delete sentinel (AUDIT A1).
- `docs/SECURITY.md` §4.2.1 — explicit AES-GCM nonce spec via
  `crypto/rand.Read`; banned time-based and counter-based nonces
  (IMP-2).
- `docs/SECURITY.md` §4.2.2 — added warning that `WEBOX_MASTER_PASSWORD`
  is readable through `/proc/<pid>/environ` and CI-only (IMP-3).
- `docs/SECURITY.md` §4.3 — replaced invented `zerocopy.Wipe` with
  `awnumar/memguard.LockedBuffer`; documented Go GC limitations
  honestly (AUDIT C4, IMP-9).
- `docs/SECURITY.md` §5.4 — v0.1 host-key-mismatch resolution via
  in-TUI two-step phrase confirmation; CLI command deferred to v0.2+
  (IMP-4).
- `docs/SECURITY.md` §6.1 — split GitHub token scope into default
  (no `Administration` scope) vs opt-in (auto-create repo with
  warning) (AUDIT B7).
- `docs/SECURITY.md` §9.3 — removed false promise of clipboard
  auto-clearing; ostrzeżenie + manualne czyszczenie (IMP-8).
- `docs/SECURITY.md` §10.4 — added `cyberpanel` web-root entry (D7).
- `docs/UX.md` §12.2 — replaced `Ctrl+S` mute shortcut with
  `Alt+M`/`Ctrl+M` (Ctrl+S triggers XON/XOFF in many terminals — D6).
- `docs/TESTING.md` §5.3 — removed `Reveal .env` test from MVP
  (whole `/env` tab is post-MVP — B1).
- `docs/TESTING.md` §5.1 — added stability note about `teatest`
  living in `x/exp/` (experimental import path — C2).
- `docs/TESTING.md` §6.1 — updated linter list to `golangci-lint v2`
  names (B3).
- `docs/CONTRIBUTING.md` §1.1 — bumped `golangci-lint` to `v2.x+`,
  clarified Go `1.24+` target and `CGO_ENABLED=0` for release (B4, D5).
- `docs/CONTRIBUTING.md` §2.1 — full v1→v2 lint name mapping table,
  `gocyclo` max bumped to 20 with required `//nolint` justification
  (B3, IMP-19).
- `docs/providers/smallhost.md` §5.4 — split SSL flow into smallhost
  subdomain (instant DNS) vs custom domain (deferred SSL with
  background retry up to 48 h — IMP-15).
- `docs/providers/smallhost.md` §6 — workflow template now uses
  `rsync --exclude` for persistent dirs and `.env`, caches `~/.npm`,
  and post-deploy SSH-checks that `.env` is `0600` before declaring
  success (C6, IMP-10, IMP-17).
- `README.md` — rewrote to badge-driven layout with mermaid diagrams
  (architecture, provider pattern, project creation flow, security
  model, roadmap timeline). Added comparison table vs alternatives
  and collapsible FAQ.
- `Makefile` — `make lint|fmt|vulncheck|snapshot|release-dry-run` now invoke `go tool -modfile=tools/go.mod` (no more `@latest`); new `make tools-install` installs binaries to `$GOBIN` for direct CLI use; dropped redundant `staticcheck` target (covered by golangci-lint).

### Removed
- Temporary improvement-plan staging file; all still-relevant findings now live
  in `docs/AUDIT.md §8` and the target documents they affected.
- Premature claim that `webox` ships with `Ctrl+S` mute shortcut.
- Auto-clipboard-clearing promise (replaced with user-facing warning).
- DAG-based engine as MVP scope (deferred to v0.3+; LIFO is MVP).

### Security
- Documented strict separation of GitHub token scopes by scenario.
- Hardened AES-GCM nonce policy to mandate `crypto/rand`.
- Locked SSH `known_hosts` workflow to **strict block** on mismatch
  with a user-confirmed phrase before any TOFU-style override.

---

## How this changelog gets updated

- **Every behavior-affecting PR adds an entry to `[Unreleased]`** in the
  appropriate section (`Added` / `Changed` / `Deprecated` / `Removed` /
  `Fixed` / `Security`).
- The maintainer cuts a release by:
  1. Renaming `[Unreleased]` to `[v0.X.Y] — YYYY-MM-DD`.
  2. Creating a new empty `[Unreleased]` section above it.
  3. Tagging `v0.X.Y` in git and pushing — GoReleaser publishes
     binaries and updates this file in the release notes.
- Internal-only refactors (no behavior change) can skip the entry.
  Docs-only PRs (typos, formatting) can skip too. When in doubt, add
  an entry — better noisy than missing context months later.

[Unreleased]: https://github.com/dilitS/webox/compare/v0.0.0-bootstrap...HEAD
