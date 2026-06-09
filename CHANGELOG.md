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

- **Sprint 23 closed — full retro + outcome (2026-05-27).** Sprint 23 retro ([`docs/retros/2026-05-27-sprint-23.md`](./docs/retros/2026-05-27-sprint-23.md)) captures: what worked (decision-matrix-as-code, mirror discipline from Sprint 21/22 that made TASK-23.1 + 23.2 fast, `ErrAPIDisabled` as the right escape hatch for DA's two-API reality, SSH fallback solving a *different* problem class than cpanel, `splitOnLastNewline` + curl `--write-out` trick reusable for future adapters, `make ci` green first try), friction (gocyclo budget needed 2 helper extractions, `app_root_template` rejected by preset schema → pushed to adapter Properties layer, `t.Parallel`+`t.Setenv` incompatibility flagged second time = `docs/gotchas.md` follow-up), surprises (3 wire shapes for DA Live API responses, MySQL user 8-char prefix cap, "Live API" terminology drift), changes to apply (`internal/sshcmd` extraction in Sprint 24 once 3rd provider tips the value calculation, `Reader.Transport()` method instead of N-case type switch, DA test account procurement as Sprint 24 critical path). Sprint 24 draft drafted in retro §Sprint 24 draft. Plan ([`docs/sprints/sprint-23-second-provider-or-launch.md`](./docs/sprints/sprint-23-second-provider-or-launch.md)) Outcome section filled: 6/6 tasks done, carry-overs explicit, metrics documented (85.2% package coverage, 79.3% project coverage, `make ci` zielony pierwsze podejście).

- **Sprint 23 — DirectAdmin adapter foundation (TASK-23.1 / 23.2 / 23.3 / 23.4 / 23.5, 2026-05-27).** Second hosting-provider adapter lands as the read-only / diagnostic-only foundation. Path A selected from the Sprint 23 decision matrix because Path C (Public Launch) is blocked on `v0.2.0-rc1` (operator-side cpanel credential rotation) and Path B (CyberPanel) requires root + threat-model RFC. Five tasks closed in one session, code-only, no operator dependencies.
  1. **TASK-23.1 — Live API read-only client + transport** ([`providers/directadmin/api/transport.go`](./providers/directadmin/api/transport.go), [`providers/directadmin/api/client.go`](./providers/directadmin/api/client.go), [`providers/directadmin/api/errors.go`](./providers/directadmin/api/errors.go), [`providers/directadmin/api/types.go`](./providers/directadmin/api/types.go)). HTTPS-only transport, `Authorization: Basic <user:loginkey>`, retry policy (500ms × 2ⁿ × 3 attempts), 4 MiB body cap, 30s default timeout. Typed `Reader` interface with `Whoami` / `ListDomains` / `ListSubdomains` / `ListDatabases` / `ListSSLCertificates`; generic shape-tolerant `decodeList[T]` accepts 3 wire shapes (modern wrapper key, `{"data":[...]}`, bare top-level array). 9 typed error sentinels: `ErrInvalidEndpoint`, `ErrMissingCredentials`, `ErrAuthenticationFailed`, `ErrRateLimited`, `ErrServerError`, `ErrMalformedResponse`, `ErrTransportUnavailable`, `ErrAPIDisabled` (DA-specific — 404 on `/api/*` or 503 with canonical "Live API disabled" body), `ErrUnexpectedHTTPStatus`. 7 research-derived golden fixtures (constructed from public DA Swagger spec). Coverage: 88.7% of statements.
  2. **TASK-23.2 — SSH fallback + Composite layer** ([`providers/directadmin/api/ssh.go`](./providers/directadmin/api/ssh.go), [`providers/directadmin/api/sshpool.go`](./providers/directadmin/api/sshpool.go), [`providers/directadmin/api/composite.go`](./providers/directadmin/api/composite.go)). Unlike cpanel (which has a first-class `uapi` CLI), DirectAdmin's SSH integration shells out to `curl -sk --user <user>:<key> https://localhost:<port>/api/...` from inside the box. Useful when the operator's machine can SSH but cannot reach :2222 directly (restrictive firewalls, NAT, IP allow-lists). The `--write-out '\n%{http_code}'` curl trick lets the fallback classify HTTP status codes through the same `classifyHTTPStatus` helper the HTTPS transport uses. Composite prefers HTTPS, falls over to SSH only on `ErrTransportUnavailable` (auth / rate-limit / API-disabled surface verbatim because SSH would hit the same wall). Defence-in-depth: `shellQuote` with `'\''` escape protects against shell metacharacters even though Sprint 23 only sources args from typed credentials.
  3. **TASK-23.3 — `webox doctor directadmin` CLI** ([`cmd/webox/directadmin.go`](./cmd/webox/directadmin.go), [`cmd/webox/directadmin_runner.go`](./cmd/webox/directadmin_runner.go)). New diagnostic subcommand mirrors `doctor cpanel` for consistency. Invocation: `webox doctor directadmin --host=HOST --user=USER [--loginkey=KEY] [--api-port=2222] [--ssh-port=22] [--timeout=30s] [--no-ssh] [--no-api] [--json]`. Five sections: Whoami / Domains / Subdomains / Databases / SSLCertificates. Status taxonomy: `OK`, `DISABLED` (`ErrAPIDisabled` — Live API off, counts as success in rollup), `AUTH_FAILED`, `UNREACHABLE`, `FAILED`. Exit codes mirror cpanel: 0 (OK/DEGRADED), 1 (BLOCKED), 2 (flag misuse). Parser extended ([`cmd/webox/run.go`](./cmd/webox/run.go)) with new `directadmin` doctor target, `--loginkey` (DA's bearer credential name), and `--no-api`. Shared flags (`--host` / `--user` / `--timeout` / `--api-port` / `--ssh-port` / `--no-ssh`) now accept directadmin context too. New `applyAPIPortFlag` / `applySSHPortFlag` helpers keep parser functions under the gocyclo budget after the directadmin path was added. Two intentional `//nolint:dupl` blocks document the duplication between `writeCpanelText`/`writeDirectadminText` and the two native SSH runners (per-provider typed sentinels prevent a clean shared helper; revisit when a third adapter lands).
  4. **TASK-23.4 — `directadmin-generic` preset graduate research → candidate** ([`assets/provider-presets/directadmin-generic.json`](./assets/provider-presets/directadmin-generic.json)). Status flip, display_name clarified ("Live API + SSH loopback fallback"), `node_versions_path` corrected to CloudLinux Selector's canonical `/opt/alt/alt-nodejs`, probes extended with `test -x /usr/local/directadmin/directadmin` to detect the panel binary, known_risks reframed against the actual adapter's behaviour (ErrAPIDisabled surfaces when /api/* is off, DB user 8-char prefix limit as a naming-policy constraint). `verified` block remains empty until Sprint 24 live capture replaces research-derived fixtures (mirror of cpanel-generic status post-Sprint-21).
  5. **TASK-23.5 — `docs/providers/directadmin.md` status update**. Document flipped from "RESEARCH / PLANNED POST-MVP (v0.3)" to "READ-ONLY CLIENT SHIPPED (Sprint 23, v0.2 path)". Added implementation status table, updated §9 test plan to reflect the Sprint 23 → Sprint 24 split (research fixtures shipped now, live capture deferred), new §10 "Implementation notes" documenting what landed, what didn't, what's open for Sprint 24 (legacy `/CMD_API_*` adapter decision, mutating client, DB user prefix enforcement, Nginx Unit vs Passenger detection).

  **Tests**: 33 unit tests across 4 test files (`client_test.go`, `ssh_test.go`, `composite_test.go`, `directadmin_test.go`) plus 6 parser tests (`directadmin_parser_test.go`). Package coverage: 85.2% (above the 80% bar). Lint clean. All existing cpanel/preset tests still pass.

- **Sprint 22 + Sprint 23 plans drafted (2026-05-25).** Sprint 22 ([`docs/sprints/sprint-22-cpanel-adapter-mutations.md`](./docs/sprints/sprint-22-cpanel-adapter-mutations.md)) scopes the second half of the cPanel adapter: mutating UAPI client (`MutatingClient.Call` replaces the Sprint 21 stub behind an env-var guard `WEBOX_CPANEL_MUTATIONS=1`), full `providers.HostingProvider` implementation with LIFO rollback on partial `CreateProject` failure, TUI wizard integration with `cpanel-generic` preset, GHA workflow template (`assets/workflows/cpanel-uapi.yml`), E2E suite (`internal/e2e/cpanel_test.go`), live-account fixture replacement (carry-over of Sprint 21 TASK-21.7), and `v0.2.0-rc1` tag as pre-release. Sprint 23 ([`docs/sprints/sprint-23-second-provider-or-launch.md`](./docs/sprints/sprint-23-second-provider-or-launch.md)) is a decision-doc: three pre-scoped paths (A = DirectAdmin adapter, B = CyberPanel adapter MVP, C = Public Launch redux), decision matrix to be filled at Sprint 22 retro; the chosen path then expands into the full sprint plan. Sprint table in [`docs/sprints/README.md`](./docs/sprints/README.md) refreshed with both entries.
- **Sprint 21 closed — full retro + outcome (2026-05-25).** Sprint 21 retro ([`docs/retros/2026-05-25-sprint-21.md`](./docs/retros/2026-05-25-sprint-21.md)) captures: what worked (Reader interface pattern + generic decoders + shellQuote escape table + composite generics + smoke test catching the `provider new cpanel <X>` parser collision before tag), friction points (envelope JSON-tag conflict took 2 hours to root-cause, `err113`/`gocritic` lint surprised three times in a row), surprises (cPanel legacy map shape exists in the wild, `httptest` TLS server quirks with connection-refused tests, asciinema 3.x non-determinism), changes to apply going forward (redactor extension for `cpanel <user>:<token>`, default to generic-driven decoders earlier, snapshot the parser context-aware state machine), open questions (mutating ops env-var guard naming, fixture-vs-live-account workflow for `verified` graduation). Sprint plan ([`docs/sprints/sprint-21-cpanel-adapter-prep.md`](./docs/sprints/sprint-21-cpanel-adapter-prep.md)) marked `✅ Closed 2026-05-25` with Outcome section completed: Path A (full parallel) selected, 6/8 tasks done, TASK-21.7 (test account) → carry-over to Sprint 22 TASK-22.0, TASK-21.8 = this entry.
- **Sprint 21 TASK-21.3 — `webox doctor cpanel` CLI (2026-05-25).** New diagnostic subcommand exercises every read-only module shipped by TASK-21.1 + TASK-21.2 in a single invocation: `webox doctor cpanel --host=panel.example.com --user=operator [--token=...] [--api-port=2083] [--ssh-port=22] [--timeout=30s] [--no-ssh] [--no-uapi] [--json]`. Architecture: pure validation + rollup logic ([`cmd/webox/cpanel.go`](./cmd/webox/cpanel.go)) plus a thin SSH runner ([`cmd/webox/cpanel_runner.go`](./cmd/webox/cpanel_runner.go)) that shells out to the operator's native `ssh` binary with the same `BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10` profile as `doctor preset --probe`. The composite layer (`uapi.Composite`) prefers HTTPS when `--token` is supplied, falls over to SSH on `errors.Is(err, uapi.ErrTransportUnavailable)`, and surfaces auth / rate-limit / module-disabled errors verbatim because SSH would hit the same wall. Per-section status taxonomy: `OK` (call succeeded), `DISABLED` (`uapi.ErrModuleFunctionDenied` — feature not enabled on the account; counts as success in the rollup because it's not a Webox failure), `AUTH_FAILED` (`uapi.ErrAuthenticationFailed` — rotate the token), `UNREACHABLE` (`uapi.ErrTransportUnavailable` — DNS / refused / TLS / timeout; this also catches context-deadline-exceeded thanks to a fix in `transport.sleepWithCtx` that wraps the context error with `ErrTransportUnavailable`), `FAILED` (any other terminal error). Rollup verdict: `OK` (all sections OK or DISABLED), `BLOCKED` (every section failed), `DEGRADED` (mixed). Exit codes: `0` (OK or DEGRADED — partial success still renders the report), `1` (BLOCKED), `2` (flag misuse). Text report layout: per-section block with status pill + transport + count + capped 6-entry sample list, summary line, optional notes (e.g. "`--token absent`: HTTPS UAPI disabled, falling back to SSH"). JSON schema: `host`, `user`, `api_port`, `ssh_port`, `verdict`, `sections[]{name, transport, status, count, sample?, error?}`, `notes[]`. **23 unit tests** in [`cmd/webox/cpanel_test.go`](./cmd/webox/cpanel_test.go) cover: option validation matrix (4 cases), default-application (apiPort=2083, sshPort=22, timeout=30s), rollup-verdict 7-case table (all-OK / all-disabled / mixed / one-failure / all-failed / all-auth-failed / all-unreachable), happy-path with all 4 sections OK, partial failure produces DEGRADED, all-fail produces BLOCKED, text report renders sections, JSON schema stable, BLOCKED exits non-zero, builder wires HTTPS-only / SSH-only / both, missing-host rejected, end-to-end faked SSH, CLI parse table for `--host` / `--user` / `--token` / `--api-port` / `--ssh-port` / `--timeout` / `--no-ssh` / `--no-uapi` / `--json` / context-aware rejection of `--token` outside `doctor cpanel`, `--no-ssh` without `--token` rejected, `--no-uapi` + `--no-ssh` rejected, truncate / capSample helpers. CLI parser updated ([`cmd/webox/run.go`](./cmd/webox/run.go)): new `cpanel` doctor target, new prefixed flags `--token`, `--api-port`, `--ssh-port`, new boolean toggles `--no-ssh`, `--no-uapi`, `--host` / `--user` / `--timeout` generalised to accept either `preset` or `cpanel` context, `--port` stays preset-only with a helpful error hint pointing at `--ssh-port` / `--api-port` for the cpanel context. The parser now resolves the `provider new cpanel <X>` vs `doctor cpanel` ambiguity by making `simpleFlagHandled` and `applyContextualToken` state-aware: when `provider new` is pending its name slot, the `github` / `preset` / `cpanel` tokens drop through to the provider-name branch so `webox provider new cpanel` continues to work unchanged. Linter: 0 issues across the new files. **Transport fix**: `transport.sleepWithCtx` now wraps `ctx.Err()` with `ErrTransportUnavailable` so context-cancelled retries surface as UNREACHABLE rather than FAILED; one test path in `client_test.go` validates the wrapping via the `TestClient_ContextCancelInterrupts` scenario.
- **Sprint 21 TASK-21.1 + TASK-21.2 — cPanel UAPI read-only client + SSH fallback (2026-05-25).** First half of the cPanel adapter foundation lands as the standalone, importable transport layer the future `providers/cpanel` adapter will compose. **HTTPS path** ([`providers/cpanel/uapi/transport.go`](./providers/cpanel/uapi/transport.go), [`providers/cpanel/uapi/client.go`](./providers/cpanel/uapi/client.go)): typed `Client.{ListDomains,ListPassengerApps,ListMysqlDatabases,ListSSLKeys}` over `https://host:2083/execute/<Module>/<Function>`, HTTPS-only (constructor rejects `http://` schemes), `Authorization: cpanel <user>:<token>` header, configurable `*http.Client`, exponential backoff on 429/5xx (500 ms × 2ⁿ, capped at 3 retries), 30 s overall timeout default, 4 MiB body cap so a runaway HTML page never balloons memory, every error mapped to a typed sentinel (`ErrInvalidEndpoint`, `ErrMissingCredentials`, `ErrAuthenticationFailed`, `ErrRateLimited`, `ErrServerError`, `ErrMalformedResponse`, `ErrModuleFunctionDenied`, `ErrAPIResultFailure`, `ErrUnexpectedHTTPStatus`, `ErrTransportUnavailable`). **SSH path** ([`providers/cpanel/uapi/ssh.go`](./providers/cpanel/uapi/ssh.go), [`providers/cpanel/uapi/sshpool.go`](./providers/cpanel/uapi/sshpool.go)): typed `SSHFallback` shelling out to `uapi --user=<user> --output=jsonpretty <Module> <function>` over the project's `ssh.Pool`. Every shell parameter is wrapped via the `'\''`-escaping `shellQuote` (defence in depth — Sprint 21 only invokes typed constants, but the quoting will protect future user-typed arg maps too). The runner abstraction (`SSHRunner` interface) keeps unit tests dependency-free; production wiring lives in `SSHPoolRunner` (separate file) and adapts `ssh.Exec` while distinguishing exit-with-non-zero (returned via exitCode, err=nil) from transport failure (wrapped in `ErrTransportUnavailable`). **Composite layer** ([`providers/cpanel/uapi/composite.go`](./providers/cpanel/uapi/composite.go)): closed `Reader` interface plus generic `Composite{Primary, Secondary}` that prefers HTTPS, fails over to SSH on `errors.Is(err, ErrTransportUnavailable)`, surfaces every other error verbatim (auth / rate-limit / malformed body / module disabled don't fail over because SSH would hit the same wall). Generics-driven dispatcher (`tryComposite[T any]`) keeps each `List*` method a one-line forward without runtime type assertions. **Decoders** ([`providers/cpanel/uapi/decoders.go`](./providers/cpanel/uapi/decoders.go)): generic shape-tolerant `decodeListResponse[T]` accepts modern object-wrapper, top-level array, **and** legacy map-keyed shapes (cPanel < 88 returned PassengerApps + Mysql.list_databases as map keyed by row name); the legacy key is promoted to the row's `Name` field via a `nameFromKey` callback. Deterministic stable-sort by name keeps test fixtures and TUI rows identical across cPanel versions. **Fixtures** under [`providers/cpanel/uapi/testdata/`](./providers/cpanel/uapi/testdata/): 7 golden JSON files (`list_domains_ok`, `list_passenger_apps_ok`, `list_passenger_apps_legacy`, `list_mysql_databases_ok`, `list_ssl_keys_ok`, `error_module_denied`, `error_invalid_envelope`) — research-derived from public api.docs.cpanel.net until TASK-21.7 onboards the live test account, then replaced one-for-one. **Mutating layer** ([`uapi.MutatingClient`](./providers/cpanel/uapi/client.go)) returns `ErrSprintScopeNotMutable` until Sprint 22 wires the real implementation — the type system enforces the "no destructive ops in v0.2-rc" guardrail. Coverage: **77.9%** of statements across **24 unit tests** (constructor validation, 4 happy paths × 2 transports, legacy map shape, transient + terminal error matrix, retry budget assertion, context cancellation, shell-quote 10-case table including injection attempts, composite fall-over / fall-through, transport-unavailable mapping on RFC 5737 unreachable IP).
- **Sprint 21 TASK-21.4 — `webox doctor preset --probe` over real SSH (2026-05-25).** The Sprint-19 stub is now a live execution path. New invocation: `webox doctor preset --id=<id> --probe --host=<host> --user=<user> [--port=N] [--timeout=30s] [--json]`. Architecture: pure-logic summarizer (`summarizeProbe`, `formatProbeText`, `formatProbeJSON` — 11 unit tests covering empty/all-OK/mixed/error cases, truncation, JSON schema stability) lives in [`cmd/webox/probe.go`](./cmd/webox/probe.go); production runner shells out to the operator's native `ssh` binary with `BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10` so Webox owns zero new auth surface (auth, host-key validation, tunnelling delegate to `~/.ssh/config` + `ssh-agent`). Probe commands ship as a single argv element to `ssh`, no local shell expansion; only the remote shell parses. Output: per-probe block (command / status / exit / time / stdout-or-stderr preview capped at 240 chars) + summary line + confidence score 0-100 (integer share of OK probes, rounded down so the score is never inflated). JSON schema: `preset_id`, `preset_name`, `host`, `user`, `confidence`, `ok_count`, `mismatch_count`, `failed_count`, `results[]`. Exit codes: 0 (all OK), 1 (≥1 FAILED — dial / network error), 2 (≥1 MISMATCH — panel disagrees with preset). Per-probe timeout configurable via `--timeout=30s` (defaults to 30 s). New CLI flags `--host`, `--user`, `--port`, `--timeout` are scoped to `doctor preset --probe` and surface focused error messages outside that context. Falls back to a declarative-metadata dump when only `--probe` is set (no `--host` / `--user`) so the documentation surface stays useful on operator laptops without target hosts. Help text in [`cmd/webox/run.go`](./cmd/webox/run.go) documents every flag with examples.
- **Sprint 21 TASK-21.6 — asciinema 3.x demo cast + GIF (2026-05-25).** New artefacts at [`docs/screenshots/sprint-21/demo.cast`](./docs/screenshots/sprint-21/demo.cast) (13.6 KB asciicast-v3 JSON, pinned `120×35`) and [`docs/screenshots/sprint-21/demo.gif`](./docs/screenshots/sprint-21/demo.gif) (1171×806 px, 98 KB). Recorded deterministically via [`scripts/record-demo.sh`](./scripts/record-demo.sh) using `asciinema rec --window-size 120x35 --command "expect -f …"`: spawn → 5× Tab tile cycle → Enter project drill → F8 CI/CD → 4× `j` scroll → Esc → 4× Tab Live Log Stream → Esc → `q` quit (target 45-60 s wall-clock). GIF rendered via [`agg`](https://github.com/asciinema/agg) (idle-time-limit 1.5 s). README EN embeds the GIF inline via `raw.githubusercontent.com/.../demo.gif`. The recording script now soft-warns (instead of dying) when the parent terminal is smaller than `120×35` — `asciinema rec --window-size` ensures the cast is still pinned at the right framing — and renders the GIF inline when `agg` is on PATH. Replaces the ASCII fallback in the README that was added by TASK-21.5.
- **Sprint 21 TASK-21.5 — README EN final at 58 lines (2026-05-25).** Compacted the root [`README.md`](./README.md) from 137 to 58 lines (cap: 60) while preserving every entry point a launch reader needs: single H1 (`# Webox`), value proposition in the first paragraph, install snippet that fits one terminal, "What works today" (5 bullets with Sprint 20 keybindings — Provider Catalog `p`, Help overlay `?`, Project Detail tabs, mouse drill/back), provider-adapter `webox provider new` invitation, 4-row roadmap table, contributing + security + license merged into the closing block. Replaced the broken `docs/assets/webox-cockpit-30s.gif` reference with a link to the existing [`docs/screenshots/sprint-20/`](./docs/screenshots/sprint-20) static gallery; asciinema cast lands with TASK-21.6. Bumped the release-badge to point at `v0.1.0-rc2`. All 13 internal links verified to resolve. PL community keeps the long-form workflow at [`docs/CONTRIBUTING.md`](./docs/CONTRIBUTING.md) (legacy detailed PL guide; banner already in place since Sprint 15).
- **Sprint 20 follow-up — CI integration for `make smoke-test` (2026-05-25).** New advisory job `smoke-tui` in [`.github/workflows/ci.yml`](./.github/workflows/ci.yml) runs the tuistory PTY harness on every push / PR (Ubuntu, Node 24, pinned `actions/setup-node@48b55a0` SHA per supply-chain rule). The job is `continue-on-error: true` and excluded from `ci-summary.needs` — we want a few weeks of green runs before promoting it into the mandatory gate. On every run (success or failure) it uploads `docs/screenshots/sprint-20/manual/` as the `smoke-tui-<sha>` artifact (17 text snapshots + `REPORT.md`, 14-day retention) so PR reviewers can diff the actual rendered terminal output without re-running the suite locally. The job builds the binary (`make build`), installs tuistory without a lock file (Sprint 20 retro decision: keep harness self-contained), and runs `make smoke-test`. No new required checks were added to `ci-summary`.
- **Sprint 20 — `make smoke-test` (tuistory-driven manual smoke tests) (2026-05-25).** Closes the Sprint 20 retro open question about operator validation never happening. New `scripts/manual-test/` (Node 24+, TypeScript, isolated `node_modules/`) drives `./bin/webox --mock` through every Sprint 20 user-facing change via [tuistory](https://github.com/remorses/tuistory) (Playwright-for-terminals): 5 scenarios × 34 assertions × ~83s wall-clock end-to-end. Coverage: (1) bento mode flipping on resize 160→120→100→60→120, (2) help overlay (`?`) open/dismiss/strict-block routing (verifies `n` press is silently swallowed while overlay is up), (3) Provider Catalog (`p`) cursor / detail / clipboard-hint, (4) Project Detail tab navigation (1/2/3/4 + `Tab` back-nav alias), (5) layout-aware mouse left-click (status-bar no-op, Projects tile drill, Project Detail body click-to-back). Artefacts: 17 text snapshots + `REPORT.md` under [`docs/screenshots/sprint-20/manual/`](./docs/screenshots/sprint-20/manual/) — diff-friendly so regressions surface inline in PR review. Runner exits 1 with a stderr `✗ scenario: assertion` line on any failure. Make target gates on Node ≥24 and pre-installed `node_modules/`.
- **Sprint 20 TASK-20.2 — Provider Catalog screen (2026-05-25).** New cockpit surface bound to the dashboard `[p]` shortcut. Read-only browser over the Sprint-19 embedded preset registry: rows are grouped by region (`🇵🇱 Poland`, `🇪🇺 Europe`, `🌍 Global`, `🛠 Advanced`) with status pills (`VERIFIED`/`CANDIDATE`/`RESEARCH`/`DEPRECATED`/`COMMUNITY`) and capability badges (`SSH · API · Node · SSL · DB · Logs · Safe Restart · Fixtures`). `↑/↓` walks the cursor (wraps at top/bottom), `Enter`/`→` toggles a deep-dive detail strip with markets, panel API, restart method, paths, probes (documented, never executed), known risks, sources, and verification audit trail. `c` copies a stable plain-text "Webox Provider Briefing" to the OS clipboard via per-OS helper (`pbcopy` / `xsel` / `xclip` / `wl-copy` / `clip.exe`); on headless servers the failure surfaces an inline remediation hint instead of crashing. `Esc` returns to the dashboard. Files: [`tui/surface/catalog/catalog.go`](./tui/surface/catalog/catalog.go), [`tui/views/provider_catalog.go`](./tui/views/provider_catalog.go), [`tui/provider_catalog.go`](./tui/provider_catalog.go), [`tui/clipboard.go`](./tui/clipboard.go). Tests: 7 unit tests for the model handlers (key routing, copy success/failure, briefing composition), 5 view-layer renderer tests (empty/groups/detail/copy-hint/load-errors), 2 surface adapter tests.
- **Sprint 20 TASK-20.5 — Help overlay (`?`) (2026-05-25).** Operator presses `?` from any surface; the cockpit replaces the body with a centered modal listing the surface-specific keys (parsed live from the active `surface.Footer().Text`) plus a static "Global keys" block (`[?]`/`[Esc]`/`[Enter]`/`[q]`/`[Ctrl+C]`). The overlay is strict-block: while open only the dismissal trio (`?`/`Esc`/`Enter`) and the global quit shortcuts reach the model, so a stray paste cannot accidentally mutate underlying state. Implementation refactored `updateKey` to extract `handleOverlayKey`, dropping cyclomatic complexity below the lint gate. Files: [`tui/help_overlay.go`](./tui/help_overlay.go), [`tui/update.go`](./tui/update.go), [`tui/view.go`](./tui/view.go). Tests: 6 e2e-style unit tests covering toggle, dismiss-on-Esc, key-routing isolation, dashboard binding extraction, project-detail binding extraction, force-quit safety.
- **Sprint 20 — Screenshot probe binary `cmd/screenshot/` (2026-05-25).** Replaces the ad-hoc `probe.go` scripts that previous sprints used to regenerate cockpit screenshots. `go run ./cmd/screenshot --mode {catalog|catalog-detail|help-dashboard|help-detail|standard} --width 120 --height 35` drives `tui.Model` through the same fixture used by tests, then writes `View()` to stdout. Used to generate the four new Sprint 20 captures (`11-provider-catalog-120x35.txt`, `12-provider-catalog-detail-140x45.txt`, `13-help-overlay-dashboard-120x35.txt`, `14-help-overlay-detail-120x35.txt`).
- **Sprint 20 — TUI mouse left-click semantics + sprint plan (wave 1, 2026-05-25).** First wave of the [Sprint 20 — TUI Polish & Provider Catalog](./docs/sprints/sprint-20-tui-polish-and-catalog.md) plan addresses the post-Sprint-19 operator feedback ("nawigacja, skalowanie, scrolling, klikanie wszystko niedopracowane"):
  1. **Mouse left-click drives the cockpit** ([`tui/update.go`](./tui/update.go), [`tui/tile_focus_test.go`](./tui/tile_focus_test.go)). Until Sprint 20 the click was a no-op (only wheel-up/down were wired). The new contract is a coarse "click-to-drill / click-to-back": clicking on the dashboard opens the currently-selected project's detail surface (mirrors `Right`/`Enter`); clicking on the project detail returns to the dashboard (mirrors `Esc`/`Left`). While a tile is Tab-focused the click is a no-op so the operator does not lose scroll context. Per-tile / per-row hit testing (proper "click on this tile = focus this tile") is deferred to Sprint 20 TASK-20.1 because it requires the bento engine to publish a layout map. Three new unit tests cover dashboard-drill, project-detail-back, and the focused-tile guard.
  2. **`Tab` becomes a back-nav alias on the project detail surface.** The legacy `Esc`/`Left` shortcuts still work; `Tab` joins them so the operator's muscle-memory "switch panes" gesture does not silently fail. Test in [`tui/tile_focus_test.go`](./tui/tile_focus_test.go).
- **Sprint 20 — TUI Polish & Provider Catalog plan ([docs/sprints/sprint-20-tui-polish-and-catalog.md](./docs/sprints/sprint-20-tui-polish-and-catalog.md), 2026-05-25).** Six tasks scoped: (20.1) layout-aware mouse hit testing (proper click-to-focus), (20.2) Provider Catalog screen (carry-over of TASK-19.4), (20.3) Standard mode redesign with mini-bento, (20.4) Project Detail tabs 2/3 unstub (Env Diff + Database read-only views), (20.5) `?` help overhaul, (20.6) CHANGELOG/retro/screenshots. Risk watch covers layout abstraction leaking, golden file churn, clipboard headless fallback.
- **Sprint 21 — cPanel Adapter (part 1) + Public Launch Prep plan ([docs/sprints/sprint-21-cpanel-adapter-prep.md](./docs/sprints/sprint-21-cpanel-adapter-prep.md), 2026-05-25).** Replaces the originally-planned Sprint 17 (renumbered after Sprint 19 out-of-order completion). Eight tasks across two parallel tracks: code (UAPI client, SSH fallback, `webox doctor cpanel`, `webox doctor preset --probe` carry-over) and ops (README EN final, asciinema cast, cPanel test account onboarding). Sprint includes an explicit Path-A/B/C decision gate so the maintainer can collapse to code-only or ops-only depending on test-account availability and launch timing.
- **Sprint 20 — Manual smoke-test screenshot suite ([`docs/screenshots/sprint-20/`](./docs/screenshots/sprint-20/), 2026-05-25).** Eight `.txt` captures of `webox --mock` rendered View() at every Bento tier (Tiny `60×18`, Standard `100×30`, Ultra `120×35`, UltraPlus `160×45`) plus tab-focus modes and Project Detail Overview / Logs surfaces. Generated headlessly via direct `tui.Model.Update` + `View` so the captures stay in sync with code; regression baseline for Sprint 20 TUI polish work.
- **Sprint 19 — Preset Registry foundation (TASK-19.1 / 19.2 / 19.3 / 19.5 / 19.6 / 19.7 / 19.8, 2026-05-25).** Webox now ships an embedded provider-preset catalog so the operator can answer the "Webox knows your hosting, not just your panel" promise from [`docs/providers/preconfiguration-vision.md`](./docs/providers/preconfiguration-vision.md). Six artefacts land together:
  1. **JSON Schema (Draft 2020-12)** at [`assets/provider-presets/schema.json`](./assets/provider-presets/schema.json) — defines preset shape, enum-locks `status` (`research / candidate / verified / deprecated / community`) and `panel.api`, regex-locks `id` / `provider_type` / `node_runtime` / `restart_method` / `ssl_provider`, and **forbids shell metacharacters** (`;`, `|`, `&`, `$`, `` ` ``, `>`, `<`) inside `probes[]` so a malicious PR cannot smuggle command injection into the catalog.
  2. **Validator + secret tripwire** ([`presets/validator.go`](./presets/validator.go)) — recycles the `santhosh-tekuri/jsonschema/v6` infrastructure already used by `config/schema.go`. Rejects payloads that contain GitHub classic / fine-grained tokens (`gh[ps]_…`, `github_pat_…`), openai-style keys (`sk-…`), AWS access keys (`AKIA…`), PEM private-key blocks, or `ssh-{rsa,ed25519,dss,ecdsa} …` public-key blobs. `errors.Is(err, presets.ErrSecretInPreset)` lets CI gates branch on the failure.
  3. **Loader + registry** ([`presets/loader.go`](./presets/loader.go), [`presets/registry.go`](./presets/registry.go)) — `embed.FS`-only ingestion via [`assets.ProviderPresetsFS()`](./assets/provider_presets.go), per-file errors collected without aborting the catalog, deterministic id-sorted listings, region grouping (`Poland → Europe → Global → Advanced`), provider-type filtering with verified-first ordering, thread-safe via `sync.RWMutex`, and singleton lazy init via `sync.Once` (`presets.Default()` / `MustDefault()`). Filename must match preset id (`<id>.json`); duplicate ids are rejected as defensive dead code (unreachable in `embed.FS` but kept so a future filesystem path stays safe).
  4. **Six initial presets** in [`assets/provider-presets/`](./assets/provider-presets/): `smallhost-devil` (verified, fixtures from Sprint 03), `cpanel-generic` + `cpanel-cloudlinux-selector` (research, public docs), `directadmin-generic` (research, Live API + Legacy fallback), `cyberpanel-generic` (research, ⚠ root-required), `mock` (verified, deterministic seed for `--mock` UI). Every preset declares `markets`, `panel`, `capabilities`, `paths`, `probes`, `known_risks`, `sources`, and `verified` — and zero secrets.
  5. **`webox doctor preset` CLI** ([`cmd/webox/preset.go`](./cmd/webox/preset.go)). Three modes: `webox doctor preset` lists everything grouped by region with capability badges (`SSH · API · Node · SSL · DB · Logs · Safe Restart · Fixtures`) and surfaces `LoadErrors` for schema drift; `webox doctor preset --id=<id>` shows one preset's full surface (panel, capabilities, paths, probes, known risks, sources, verification audit trail); `--json` flips both modes to machine-readable output (separate `presetListJSON` / `presets.Preset` shapes so `--json` consumers stay stable across schema bumps); `--probe` is an explicit stub with stderr message — live capability execution lands with the cPanel adapter (Sprint 17/18). Help text + `--id=` / `--probe` flag parsing extended in [`cmd/webox/run.go`](./cmd/webox/run.go).
  6. **[ADR-0008](./docs/adr/0008-preset-registry.md)** — locks in the embedded-only / schema-validated / singleton-with-DI-seam architecture; rejects filesystem-loadable presets and URL registries as v0.2 baseline (re-evaluable post-v1.0); rejects plugin marketplace per `AGENTS.md §4`. ADR documents threat model: shell-injection probe vector mitigated by schema regex; future probe execution must use `exec.Command` with separate args (never `sh -c`); path-traversal in `paths.*_template` mitigated by reusing `wizard.ValidateWorkflowField` at adapter integration time.
  - **Tests:** 36 new test cases across `presets/preset_test.go`, `presets/validator_test.go`, `presets/loader_test.go`, `presets/registry_test.go`, `cmd/webox/preset_test.go`. Coverage: types (Status enum, Region grouping, CapabilityBadges determinism, AllPanelAPIs closed set), validator (malformed JSON, schema violations across 11 mutations, secret tripwire across 6 payloads), loader (skip-on-error, filename mismatch, two-unrelated-presets, missing dir, FormatLoadErrors determinism, embedded catalog smoke), registry (id-sorted listing, ErrPresetNotFound, by-provider-type ordering by status, by-region grouping, Regions display order, LoadErrors copy-out, 32-goroutine concurrency safety, Default singleton identity), CLI (text + JSON list, text + JSON show, not-found→exitMisuse, probe stub message, schema-drift surfaced in text, parser dispatch + cross-flag validation `--probe requires --id` + `--id requires preset target`).
- **Sprint 19 — Contributor preset walkthrough ([`docs/contributing/PRESET.md`](./docs/contributing/PRESET.md), 2026-05-25).** 1-hour, no-Go-knowledge-required guide for adding a preset for an arbitrary cPanel / DirectAdmin / CyberPanel hoster. Steps: browse catalog → SSH probe → copy-edit JSON → `go test ./presets/...` validates schema + secret tripwire → optional fixture capture for `verified` → PR with hoster + plan + sources. Includes the "what NOT to put in a preset" guardrails (no tokens / passwords / SSH keys / real domains / authenticated URLs), the 5-item manual checklist for `verified` graduation, and an FAQ that links back to `PROVIDER.md` (adapter walkthrough) for the case "panel not yet supported".

### Changed

- **Sprint 20 — Per-surface footer hints replace one-size-fits-all default (2026-05-25).** Closes the Sprint 14 TASK-14.1 TODO. Until Sprint 20 every surface advertised the same string `[q] quit · [?] help · [/] command palette · [Tab] cycle panels`, which was misleading on three counts: `[/] command palette` does not exist in v0.1 (or v0.2); `[Tab] cycle panels` is dashboard-only; the wizard / project-detail / resume / import surfaces never used those keys. Each surface now publishes a state-relevant footer that the View layer pins:
  - **Dashboard:** `[q] quit · [?] help · [Tab] cycle panels · [Right/Enter] open · [n] new · [i] import` ([`tui/surface_adapters.go`](./tui/surface_adapters.go)).
  - **Project Detail:** `[q] quit · [?] help · [Esc/Tab] back · [1] overview · [4] logs · [r] restart · [s] ssl · [v] tail` ([`tui/surface/projectdetail/projectdetail.go`](./tui/surface/projectdetail/projectdetail.go)).
  - **Project Wizard:** `[q] quit · [?] help · [Tab/Enter] next · [Shift+Tab] back · [Esc] cancel` ([`tui/surface/projectwizard/projectwizard.go`](./tui/surface/projectwizard/projectwizard.go)).
  - **Init Wizard:** `[q] quit · [?] help · [Tab/Enter] next · [Esc] back` ([`tui/surface/initwizard/initwizard.go`](./tui/surface/initwizard/initwizard.go)).
  - **Resume Wizard:** `[q] quit · [?] help · [r] rollback · [d] discard · [k] keep snapshot` ([`tui/surface/resumewizard/resumewizard.go`](./tui/surface/resumewizard/resumewizard.go)).
  - **Import Preview:** `[q] quit · [?] help · [a] import all · [Esc] back` ([`tui/surface/importpreview/importpreview.go`](./tui/surface/importpreview/importpreview.go)).
  - When a tile is focused the chrome collapses to the absolute-minimum legend (`[q] quit · [?] help`) plus the focus suffix (`focus: <slot> · [PgUp/PgDn] scroll panel · [Esc] release`), so the chrome stays readable on `120` column terminals where the longer dashboard hint would truncate the focus annotation. Per-surface tests updated to assert the new keys and explicitly forbid `command palette` references; new e2e `TestCockpit_TileFocusReleasesOnEsc` needles on `[Right/Enter] open` (footer changes between focused / released frames so Bubble Tea's diff renderer guarantees a fresh emission post-Esc).
- **Sprint 20 — Project Detail v0.2 tabs no longer raise redundant alert (2026-05-25).** Pressing `2` (Env Diff) / `3` (Database) / `h` / `l` on the project detail surface used to raise `tab available in v0.2` on every press. The label itself already renders `[2] Env Diff - unlocked in v0.2`; the alert was redundant noise that new operators reported as a routing bug. The keys are now silent no-ops; the active tab is preserved. Tests `TestProjectDetail_PressingDimmedTabIsSilent`, `TestUpdateProjectDetailDisabledTabsAreSilent`, and `TestProjectDetailKeyHandlerSilentlyIgnoresDimmedTabs` enforce the contract.
- **Sprint 20 — Bento UltraPlus deep-dive strip removed (2026-05-25).** The `≥160×45` UltraPlus tier used to render a `[Deep-dive strip] Reserved for service timelines (Sprint 11+)` placeholder line below the live-log row. Sprint 11 has long since shipped (live log stream + topology map both live in the main grid since 2026-05-23 / Sprint 11), so the placeholder carried no information and stole two precious viewport rows. The strip + the `bentoDeepDiveLines` reservation are gone; the freed rows now flow into the live-log target via [`bentoLogsTargetUltraPlus`](./tui/bento/engine.go) (UltraPlus now shows 14 log lines vs. 12 previously). If a future sprint wants to reintroduce a deep-dive strip it must land behind a feature flag plus a fresh ADR; the previous incarnation was UX dead weight.
- **Sprint 20 — Tiny mode resize hint corrected (2026-05-25).** The `<70×22` Tiny fallback used to instruct the operator to "Resize the window, then press [r] to redraw." `[r]` was never wired to a redraw command at the global level — Bubble Tea auto-emits a `tea.WindowSizeMsg` on every resize, so no manual key press is needed. The new copy tells the truth: "Resize the window — the cockpit re-renders automatically. Press [q] or [Ctrl+C] to quit." Snapshot in [`docs/screenshots/sprint-08-tiny-fallback-60x18.txt`](./docs/screenshots/sprint-08-tiny-fallback-60x18.txt) regenerated.
- **Sprint 20 — Sprint plan README updated (2026-05-25).** [`docs/sprints/README.md`](./docs/sprints/README.md) Sprint table refreshed: Sprint 20 (TUI Polish & Provider Catalog) and Sprint 21 (cPanel Adapter part 1 + Public Launch Prep) added; Sprint 17/18 numbering retired in favour of 21/22+ to keep cPanel-adapter logical sequence intact after the Sprint 19 out-of-order completion. Project stages section updated to reflect the new ordering.

### Added

- **`Reader.Transport() string` lands on both cpanel and directadmin API interfaces (2026-05-27, post-Sprint-23 polish).** The Sprint 23 retro noted that `directadminTransportLabel` (and Sprint 21's `transportLabel` for cpanel) used a runtime type-switch against the known Reader implementations — a fragile pattern because a new implementation silently falls through the default arm and surfaces as `?` instead of breaking the build. The polish lifts the transport label onto the Reader interfaces themselves: `(*Client).Transport()` returns `"HTTPS"`, `(*SSHFallback).Transport()` returns `"SSH"`, `(*Composite).Transport()` delegates to its non-nil legs (`Primary.Transport() + "+" + Secondary.Transport()`, or whichever leg is wired, or `"?"` for the construction-error case). Caller sites collapse to one-liners (`r.Transport()`); a hypothetical third transport type (CLI plugin? legacy CMD_API?) now MUST declare its label at compile time. Tests added: `TestComposite_TransportDelegatesToLegs` (4 cases each side), `TestClient_TransportReturnsHTTPS`, `TestSSHFallback_TransportReturnsSSH`. Files: [`providers/cpanel/uapi/composite.go`](./providers/cpanel/uapi/composite.go), [`providers/cpanel/uapi/client.go`](./providers/cpanel/uapi/client.go), [`providers/cpanel/uapi/ssh.go`](./providers/cpanel/uapi/ssh.go), [`providers/cpanel/uapi/transport_label_test.go`](./providers/cpanel/uapi/transport_label_test.go), [`providers/directadmin/api/{composite,client,ssh}.go`](./providers/directadmin/api/), [`providers/directadmin/api/composite_test.go`](./providers/directadmin/api/composite_test.go), [`cmd/webox/cpanel.go`](./cmd/webox/cpanel.go), [`cmd/webox/directadmin.go`](./cmd/webox/directadmin.go). Pulled forward from Sprint 24 backlog because the post-Sprint-23 polish window was the right time (interface budget is now 6/6; no further changes without an ADR).
- **Redactor patterns extended for cpanel + directadmin credentials (2026-05-27).** [`internal/log/redact.go`](./internal/log/redact.go) gains three new rules: (1) `Authorization: cpanel <user>:<token>` — preserves the literal `cpanel ` prefix and username, redacts the token only (post-incident triage benefits from seeing the username; the token must never leave the box); (2) `Authorization: Basic <base64>` — redacts the whole base64 envelope because DirectAdmin embeds both username and login key in it, so partial redaction would still leak via `base64 -d`; (3) `login[_-]?key` joins the generic key=value alternation arm so `--loginkey=…` CLI flags and `DA_LOGIN_KEY=…` env lines round-trip through the existing redactor. Corpus test ([`internal/log/redact_corpus_test.go`](./internal/log/redact_corpus_test.go)) gains four new cases (Authorization cpanel-user-token, Authorization Basic, directadmin loginkey CLI flag, directadmin DA_LOGIN_KEY env line) using deterministic literal secrets so the assertions actually prove the bytes are scrubbed. Pulled forward from Sprint 21 retro action item (`internal/log/redact.go` extension for `cpanel <user>:<token>`).
- **`docs/gotchas.md` gains three new sections (2026-05-27).** Sections 14a-14c capture recurring patterns that bit Sprint 22 / 23 retros: (14a) `t.Parallel()` + `t.Setenv()` incompatibility — picks one of two fixes (seam injection or sequential test), with examples from `cmd/webox` and the cpanel E2E suite; (14b) contextual-token parser without state-aware logic — explains the `webox doctor cpanel` vs `webox provider new cpanel <X>` ambiguity and mandates negative tests for every prefixed flag; (14c) `Pool.Close ≠ "server-side counter frozen"` — documents the cryptossh server-side drain race that flaked `TestKeepaliveLoopStopsOnPoolClose` on PR-17 and PR-18, with a pointer to the `waitForStableCount` helper as the template. File: [`docs/gotchas.md`](./docs/gotchas.md). Pulled forward from Sprint 23 retro action items.

### Fixed

- **DirectAdmin endpoint constants unified on `%s` verbs + composite nil-leg guard (2026-06-09, post-Sprint-23 polish).** `EndpointListDatabases` / `EndpointListSSLCertificates` in [`providers/directadmin/api/types.go`](./providers/directadmin/api/types.go) shipped with literal `{user}` placeholders while [`client.go`](./providers/directadmin/api/client.go) quietly shadowed them with local `%s`-formatted constants — so the package-level constants were dead, and any future refactor that wired them through `endpointURL` would have produced a malformed `/api/users/{user}/databases` request. The constants now carry the same `%s` verb the transport formats, `client.go` consumes them directly (local duplicates removed), and `TestEndpointURL_AppliesAPIPrefix` exercises both so the trap can't reopen. Separately, `tryComposite` ([`providers/directadmin/api/composite.go`](./providers/directadmin/api/composite.go)) is brought to parity with cpanel: it now guards both legs against `nil` (the exported `Primary`/`Secondary` fields allow a constructor bypass past `NewComposite`'s checks) and returns the zero value alongside terminal errors instead of leaking a partial Primary result on fail-over. Tests: `TestComposite_FailoverWithNilSecondaryDoesNotPanic`, `TestComposite_BothLegsNilReturnsTransportUnavailable`.
- **`TestKeepaliveLoopStopsOnPoolClose` server-side drain race eliminated (2026-05-27).** PR #17 macOS run flaked with `keepalive count changed after pool close: just-after=1 final=2`, even though the Sprint-02 `Pool.Close → wg.Wait` contract guarantees every keepalive goroutine has exited before Close returns. Root cause: a different layer of asynchrony on the *server* side. `cryptossh`'s internal server goroutine reads SSH packets from the wire and delivers each global request to a `requests` chan; `sshmock.handleGlobalRequests` then picks one up under mutex and `globalRequests[reqType]++` *before* replying. If `pool.Close` torn down the client connection while one already-parsed keepalive request was still queued in that chan, the server-side increment landed after `pool.Close` had returned — so the snapshot taken immediately after Close missed it, and the snapshot taken 50 ms later caught it. The fix replaces the immediate-snapshot with a polling `waitForStableCount` helper ([`ssh/exec_test.go`](./ssh/exec_test.go)) that waits for two consecutive samples 50 ms apart to agree (timeout 2 s) before sampling, then keeps the existing 50 ms post-stability assertion that asserts the count stays frozen across many keepalive intervals. The production `Pool.Close` contract is unchanged; only the test's assumption that "Close return = server count frozen" gets corrected to "Close return = server count *converging*". Stress run: `go test -race -count=50 -run TestKeepaliveLoopStopsOnPoolClose ./ssh` is green (~8 s).
- **`ssh.Pool.Close` now blocks until every background goroutine has exited (2026-05-26).** Closes a latent race in the Sprint-02 connection pool that surfaced as a flaky CI run on `0c752cf` (`TestKeepaliveLoopStopsOnPoolClose` failed with `before=3 after=4`). `Pool.Close` previously only signalled `done` and then returned, leaving the keepalive goroutines free to complete an in-flight `client.SendRequest(keepalive@openssh.com, ...)` after Close had nominally finished — a stray packet would reach the server, the operator's session count would tick up by one, and any caller relying on "Close = quiescent pool" had its assumption silently broken. The fix wires a `sync.WaitGroup` through every `cleanupLoop` and `keepaliveLoop` start site, tears down the underlying `*ssh.Client` between `close(done)` and `wg.Wait()` so in-flight `SendRequest` calls fail fast instead of completing on the wire, and adds a re-check of `done` inside `keepaliveLoop` right after a ticker tick to narrow the racey window even further. The test itself was rewritten to capture the keepalive counter twice **after** Close (one snapshot immediately, one after a 50 ms sleep) instead of sampling once before / once after Close, removing the residual race in the assertion itself. Stress run: `go test -race -count=200 -run TestKeepaliveLoopStopsOnPoolClose ./ssh` is green (~15 s). Files: [`ssh/pool.go`](./ssh/pool.go), [`ssh/keepalive.go`](./ssh/keepalive.go), [`ssh/exec_test.go`](./ssh/exec_test.go).
- **CI smoke-tui mis-diagnosis cleaned up + Node 22 LTS pin made consistent (2026-05-26).** A previous commit (`a5ba582`) downgraded the local Makefile guard from Node 24+ to Node 22+ with a claim that GitHub Actions runners "lack Node 24, causing 'Illegal instruction' crashes". That diagnosis is wrong on two counts: (1) `actions/setup-node` installs whatever version is requested, including Node 24, and (2) the actual crash is a SIGILL inside `zigpty.linux-x64.node` (the Zig-built PTY native addon used by `tuistory ≥ 0.8`) — an environmental incompatibility between the prebuilt CPU baseline and the GHA runner kernel/CPU pair that no Node version bump can resolve. The earlier commit also left `.github/workflows/ci.yml` still pinning `node-version: '24'`, so even the symptomatic effect was nil. This commit (a) keeps the relaxed Node 22+ local guard (Node 22.6 is where `--experimental-strip-types` stabilised, so the relax is semantically correct), (b) updates the CI workflow to Node 22 LTS so developer machines, the Makefile guard, and CI all agree on one runtime, (c) rewrites the workflow comment to document the actual `zigpty` SIGILL with a pointer to the prebuilt-binary mismatch theory, and (d) reverts an unrelated stylistic regression in [`scripts/manual-test/smoke.ts`](./scripts/manual-test/smoke.ts) where Copilot had inserted unnecessary `\`` escapes inside a plain double-quoted string. The smoke-tui job stays `continue-on-error: true` (advisory) until tuistory exposes a `node-pty` fallback or `zigpty` ships a more portable build.
- **Sprint 20 — Chrome footer no longer advertises non-existent `[/] command palette` (2026-05-25).** Six places hardcoded the dishonest hint string (each surface package + `dashboardSurface` adapter + `view.go` renderChromeBottom). All gone. Tests positively assert each surface publishes its own keys AND negatively assert `command palette` is absent (regression guard). [Most-reported "feels unfinished" cue from operator feedback after Sprint 19.](./docs/sprints/sprint-20-tui-polish-and-catalog.md#kontekst)

---

## [v0.1.0-rc1] — 2026-05-25

> **Release candidate** for the first public release. CI green, coverage 80.4 %, lint clean, `govulncheck` clean, bench within 5 ms budget. Operator-only validation pending: real-account integration tests, fresh-install on Linux, asciinema recording, cosign signature pipeline. Promote to `v0.1.0` GA from the **same commit** once `.cursor/skills/release-check/SKILL.md` checklist is fully ticked — no code changes expected between RC1 and GA.

### Fixed

- **CLI help text disambiguates `--preset` form (post-verification fix, 2026-05-25).** `webox --help` and the `provider new` error messages now consistently show `--preset=PRESET` (the equals-form, which is what the parser actually accepts). Discovered during the Sprint 15 pre-release smoke test: the previous help text rendered `[--preset P]` which most users would type as `--preset cpanel-uapi` (space-separated) and hit a `unknown argument "--preset"` error. The parser is unchanged — all existing tests still pass — only the docstrings + error messages were tightened. Space-form support is a v0.2 candidate; not blocking v0.1.0.

### Changed

- **Sprint 15 — Sprint outcome + Sprint 16 pre-planning refresh (TASK-15.8, 2026-05-25).** Sprint 15 zamknięte z 8/8 tasków done (15.1-15.7 + 15.9; 15.2 / 15.5 carry-overy są **operator-only** artefakty: asciinema recording + landing body native-speaker pass). [`docs/sprints/sprint-15-launch-readiness.md`](./docs/sprints/sprint-15-launch-readiness.md) Outcome section wypełniona — explicit decyzja: **generator zostaje w mainline `webox` binary** (nie `webox-dev`), bo to operator/contributor-facing komenda, nie debug. [`docs/sprints/README.md`](./docs/sprints/README.md) Sprint 15 status → ✅ Done z retro linkiem. [`docs/sprints/sprint-16-public-launch.md`](./docs/sprints/sprint-16-public-launch.md) Pre-flight Checklist zaktualizowany: tasks z Sprint 15 oznaczone jako gotowe, 3 nowe carry-over checkboxes (cast recording, screenshot capture, landing EN body), 1 nowa decyzja-do-Sprint-16-retro (community vs. maintainer-led cPanel skeleton). [`docs/ROADMAP.md`](./docs/ROADMAP.md) Sprint 15 row zaktualizowany. Nowy retro plik [`docs/retros/2026-05-25-sprint-15.md`](./docs/retros/2026-05-25-sprint-15.md) — 4 sekcje (what worked, what didn't, surprises, changes to apply going forward) + 3 open questions kierujące Sprint 16 planning.

- **Sprint 15 — 5 launch-day `good-first-issue` body drafts (TASK-15.6, 2026-05-25).** `.github/issue-drafts/` now holds five maintainer-approved issue bodies ready for `gh issue create --body-file …`:
  1. **cPanel UAPI skeleton** (🟢 mainstream · 4-8 h · `good-first-issue` `help wanted` `provider`).
  2. **DirectAdmin skeleton** (🟡 mixed API · 4-6 h · `good-first-issue` `help wanted` `provider`).
  3. **CyberPanel API research** (🔴 root concerns · 2-3 h · `help wanted` `provider` `research`).
  4. **Next.js scaffolding template** (🟢 starter · 1-2 h · `good-first-issue` `help wanted` `documentation`).
  5. **German (DE) translation** (🟢 no-code · 1 h · `good-first-issue` `help wanted` `documentation`).

  Every body has a difficulty badge, estimated time, "maintainer pair-review available" promise, 30-second start command, explicit acceptance criteria, a "what we will NOT accept" guardrail block, and useful references. `create-all.sh` ships all five sequentially (gh label-attach is rate-limited under concurrency), skips already-open issues with the same title, supports `--dry-run` and `--continue-from <N>`, and prints the required label-create commands. The five issues are the **public scoreboard for the v0.1.0 launch** — closed issues will be the visible social proof in Sprint 16.

- **Sprint 15 — README.md rewritten as v0.1 launch landing (TASK-15.1, 2026-05-25).** The repo's front page now follows the conversion structure from `docs/sprints/sprint-15-launch-readiness.md` TASK-15.1: Hero (badges + demo links) → **Why Webox** (one paragraph, magnet words `cPanel` / `DirectAdmin` / `shared hosting` / `Node.js` / `terminal`) → **Try it in 30 seconds** (`git clone && make build && ./bin/webox --mock`, no SSH, no config) → **What you can do today** (one verified provider, 8 implemented capabilities) → **Add your hosting in 4 hours** (`webox provider new` demo + 4-preset matrix + 4-hour walkthrough link + explicit pair-review promise) → **Architecture highlights** (Provider Pattern, MVU Bubble Tea, SSH pool + SWR, keyring + AES-GCM, atomic config + 80 % coverage) → **Status & roadmap** (small.pl v0.1 → cPanel v0.2 → DirectAdmin v0.3) → **Contributing** (3 paths + guardrail summary) → **License & credits** (Apache-2.0 + Charmbracelet + small.pl + Go ecosystem ack). All 22 links absolute so the README works in forks. 136 lines vs. previous 506 — strict adherence to "no testimonials, no `go install`, no >10 feature list" from the AC. Demo + screenshot embeds reference the recorder-script READMEs so the page remains clean before the operator records the canonical `demo.cast` and `dashboard.png` for v0.1.0 GA.

### Added

- **Sprint 15 — Landing-page license-regression guard (TASK-15.5 partial, 2026-05-25).**
  - `scripts/lint-landing-license.sh` — committable artefact for the otherwise-gitignored `landing/` folder. Uses `ripgrep` with `\bMIT\b` word-boundary matching across `landing/**/*.{html,htm,json,md,txt,js,mjs,ts,svg,xml}` (skipping `node_modules`, `dist`, `vendor`). Exit `0` clean / `1` if `landing/` missing / `2` if any obsolete-license reference appears. Supports `--json` for CI pipelines. The maintainer runs it before every landing deploy (`landing/` is decoupled from the main repo, so primary CI cannot enforce this — the deploy pipeline must).
  - **Landing EN scaffold (local-only, gitignored):** `landing/en/index.html` bootstrapped from the PL version with the full **head/meta layer translated** — `<html lang="en">`, EN `<title>`, EN meta description (cPanel coming soon mention), canonical → `/en/`, `og:locale="en_US"` + alternate `pl_PL`, EN OG/Twitter titles + descriptions. The body content (1100+ lines of marketing copy and ASCII mockups) is intentionally left as PL with a prominent HTML comment marking the work-in-progress translation — a native-speaker review pass is in the acceptance criteria and is deferred to the operator's deployment cycle before publishing `https://webox.dev/en/`. `landing/index.html` PL canonical updated so `x-default` hreflang now points to the EN page (international SEO default).
  - **Why landing isn't committed:** `landing/` is gitignored on purpose — deployment is decoupled (Cloudflare Pages / Vercel pulls from a separate location). The lint script is the bridge that lets the main repo keep enforcing license consistency on the gitignored assets.

- **Sprint 15 — Asciinema + static-PNG demo capture scaffolding (TASK-15.2, 2026-05-25).**
  - `scripts/record-demo.sh` — deterministic 45-60 s `expect`-driven asciinema recording of the `--mock` cockpit. Enforces exactly **120×35 terminal** (Bento Ultra framing); auto-builds `./bin/webox` if missing; validates `asciinema` + `expect` dependencies upfront; emits both `assets/demo/demo.cast` and `assets/demo/demo.sh.log` (companion keystroke script for reviewers diffing timing). The scripted scenario follows the 7 beats in `docs/sprints/sprint-15-launch-readiness.md` TASK-15.2: boot → 5-tile Tab tour → shop-ease detail → CI/CD pipeline modal (F8) + step scroll → Live Log Stream → Esc to Topology Map → quit. Re-running the script always produces the same cast — no ad-libbing in canonical artefacts.
  - `scripts/capture-screenshot.sh` — renders `assets/screenshots/dashboard.png` (cockpit-at-rest frame @ t=8s) using [`agg`](https://github.com/asciinema/agg) + `ffmpeg`; prints fallback manual-capture instructions when `agg` is missing. Documents how to capture `assets/screenshots/wizard.png` (manual screenshot of new-project wizard step 3).
  - `assets/demo/` and `assets/screenshots/` directories scaffolded with `.gitkeep` markers + per-folder `README.md` explaining the render commands, the < 100 kB / Git LFS threshold, and the "never commit ad-libbed casts" policy.
  - Actual `.cast` + `.png` files are intentionally *not* committed yet — recording requires an interactive operator with a 120×35 terminal. The scaffolding lets any contributor produce them with a single command before v0.1.0 GA.

- **Sprint 15 — Repo hygiene & public-readiness audit (TASK-15.7, 2026-05-25).**
  - `.github/ISSUE_TEMPLATE/provider_request.yml` — new form-based template for community-driven panel adapter requests / volunteers. Fields: panel name, vendor URL, public API/CLI docs URL, SSH availability dropdown, Node.js runtime dropdown, target-market paragraph, commitment-level dropdown (suggesting / want to implement / want pair-review / want to co-maintain), test-account availability, free-form notes. Pre-submit checklist links the 4-hour `docs/contributing/PROVIDER.md` walkthrough. Labels: `provider`, `needs-triage`.
  - `.github/FUNDING.yml` — placeholder funding configuration (GitHub Sponsors / Ko-fi / custom). All keys are empty so GitHub renders no Sponsor button until real handles are wired up before v0.1.0 — see Sprint 16.
  - **License audit:** all `MIT` references across `README.md`, `docs/PRD.md §12.1`, `.cursor/skills/release-check/SKILL.md`, `landing/index.html` (Schema.org JSON-LD + badge + footer link) and `landing/landing-page-plan.md` rewritten to Apache-2.0 with cross-reference to the 2026-05-25 license change rationale. `docs/dependencies.md` keeps `MIT` in the *allowed upstream license catalog* — the only legitimate remaining occurrence.
  - **Hardcoded-credential audit:** `docs/CONTRIBUTING.md §1.4` now uses `<your-test-host.example.com>` / `<your-test-login>` placeholders instead of the maintainer's personal small.pl host name. `docs/UX.md` preamble gains a "konwencja mockupów" note declaring all account names in ASCII screenshots (`s1.small.pl`, `biuromody`, `mysql://biuro_local:secPassword@…`) as fictional dogfooding values. `docs/MIGRATION_NOTES.md`, `docs/retros/*`, and `docs/AUDIT.md` re-scanned: no `/Users/seba/...` absolute paths, no exploit walkthroughs, no personal frustration content.

- **Sprint 15 — Root `CONTRIBUTING.md` EN on-ramp (TASK-15.3, 2026-05-25).** 143-line EN entry point at repo root, separate from the existing 350-line PL deep-dive at [`docs/CONTRIBUTING.md`](./docs/CONTRIBUTING.md) (now marked as "legacy detailed PL guide" with a pointer to the root file). Sections in the order an external contributor wants them: 5-minute Setup → Branching + commits → PR checklist → **Three ways to contribute** (add a hosting-panel adapter ✦ highest leverage, add a translation, bug fix / small feature) → What we will NOT merge (guardrail summary) → Maintainer SLA → Pointers. The "add an adapter" section is the magnet: links the new `webox provider new <name>` generator + the 4-hour [`docs/contributing/PROVIDER.md`](./docs/contributing/PROVIDER.md) walkthrough + difficulty badges + explicit "pair-review available — open an issue first" promise. All 15 cross-links verified to resolve. By contributing the author agrees to Apache-2.0 + [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md).

- **Sprint 15 — `webox provider new <name>` adapter scaffolding generator (TASK-15.4, 2026-05-25).** New CLI subcommand that scaffolds a complete `providers/<name>/` adapter package + matching fixtures + production blank import, dropping the cost of adding a hosting panel from "read 9 method signatures and copy-paste" to one command. Highlights:
  - **Generated files (all `gofmt`-clean and AST-parseable on every run):** `providers/<name>/{doc,provider,provider_test,parsers,parsers_test}.go` + `testing/fixtures/<name>/README.md`.
  - **Embedded templates** under `assets/provider-template/*.tmpl` (Go `text/template`), loaded via `//go:embed` so the generator works in any clone without external paths.
  - **`--preset PRESET`** (one of `blank`, `cpanel-uapi`, `directadmin`, `cyberpanel`) seeds the doc comments with the chosen vendor's display name; vendor-specific transport scaffolding lands in Sprint 17/18 work.
  - **`--dry-run`** prints the would-write plan and patched-imports note without touching disk — operators see exactly what the generator will do before committing.
  - **Idempotent blank-import patch:** rewrites `cmd/webox/providers.go` in canonical sorted order; re-running on an already-registered name is a no-op (no spurious diff).
  - **Strict name validation:** lowercase ASCII letter start + `[a-z0-9_]{2,31}` body + reserved-name guard (blocks `smallhost`, `mock`, `main`, `init`, `test`, `testing`).
  - **CLI hygiene:** `parseArgs` was refactored into focused helpers (`applySimpleFlag`, `applyPrefixedFlag`, `postParseValidation`) so the new subcommand keeps the cyclomatic-complexity gate green; stdout stays empty for clean pipe usage; walkthrough message lands on stderr.
  - **Tests:** 22 new test cases covering name regex, reserved list, preset matrix, dry-run safety, idempotency, AST validity of the generated Go, parsed-imports round-trip, `--preset` / `--dry-run` parse error matrix. End-to-end smoke verified by running `./bin/webox provider new sandboxtest && go test ./providers/sandboxtest/...` (green; output cleaned post-verification).
  - **Why now:** Sprint 15 TASK-15.4 calls this out as "the single strongest contributor magnet, more important than README marketing." `docs/contributing/PROVIDER.md` (already merged in the Sprint 15 docs scaffold) references the generator as Step 1 of the 4-hour walkthrough.

- **Launch readiness scaffolding — Sprint 15-20+ plans + `.local/` ops room (2026-05-25).** Post-Sprint-14 strategic planning iteration that captures the decision to push `v0.1` GA into a **public OSS launch** rather than staying in technical-only mode. Six new sprint plans now own the post-MVP path:
  - `docs/sprints/sprint-15-launch-readiness.md` — README EN, asciinema z `--mock`, `webox provider new` generator, `docs/contributing/PROVIDER.md` walkthrough, AGENTS.md slim (≤150 linii), Apache-2.0 consistency, repo public-readiness audit. Głównie nie-kod.
  - `docs/sprints/sprint-16-public-launch.md` — Tydzień 1 PL soft launch + Tydzień 2 Show HN + r/golang + r/selfhosted (środa rano EST). Partnership outreach H88 (small.pl/lh.pl) + cPanel test account purchase + `docs/providers/cpanel.md` real-world expansion.
  - `docs/sprints/sprint-17-cpanel-adapter.md` — `providers/cpanel/` skeleton, UAPI client (token auth `:2083`), SSH `uapi` fallback, read-only ops (ListProjects, GetStatus, GetLogs, Restart), `webox doctor cpanel`.
  - `docs/sprints/sprint-18-cpanel-polish.md` — Mutating ops (CreateSubdomain via Application Manager, CreateDatabase, IssueSSL z AutoSSL), idempotentne `Remove*`, wizard integration, E2E, v0.2.0-rc1 cut.
  - `docs/sprints/sprint-19-preset-registry.md` — `assets/provider-presets/*.json` JSON schema, `presets/loader.go` + `embed.FS`, Provider Catalog TUI (positioning: „Webox zna Twój hosting, nie tylko Twój panel"), `webox doctor preset`, ADR-0008.
  - `docs/sprints/sprint-20-plus-options.md` — Decision matrix po Sprint 19 retro: A) DirectAdmin adapter (community-driven), B) OAuth Device Flow + Quality Polish (default), C) Repositioning + content marketing.
  - `docs/sprints/README.md` i `docs/ROADMAP.md` zaktualizowane z nową tabelą sprintów 15-20+.
- **AGENTS.md docs refactor (Sprint 15 TASK-15.9 prep, 2026-05-25).** Trzy nowe wydzielone dokumenty + skrót AGENTS.md:
  - `docs/dependencies.md` — autoritatywny katalog Go zależności + toolchain + tool/`go.mod` pinning + supply chain policy (Apache/MIT/BSD only).
  - `docs/conventions.md` — pełne konwencje kodu Go (naming, error handling, context discipline, generics, logging, testing, Conventional Commits, PR structure).
  - `docs/gotchas.md` — Top 15 anty-pattern-ów (keyring detection, AES-GCM nonce, PID lockfiles, hardcoded provider name, secrets w log, `t.Parallel()` z global stubs, raw goroutines w Bubble Tea, etc.).
  - `docs/contributing/PROVIDER.md` — 4-godzinny walkthrough EN dla zewnętrznych kontrybutorów dodających adapter providera (preset vs adapter decision, scaffold generator, fixtures, TDD parsery, integration sshmock, capability probe, PR template z pair-review request).
  - `AGENTS.md` skrócony z 619 → ~150 linii: TL;DR, guardrails skrót, documentation map (pytanie → doc), workflow TDD, scope discipline, decision policy, skills reference. **Wszystkie guardraile nadal w pełni egzekwowane** przez `.cursor/rules/00-charter.mdc`.
- **`.local/` private operations room scaffolding (2026-05-25).** Gitignorowany katalog (dodany do `.gitignore`) z drafts dla launch + partnerships + portfolio + metrics:
  - `.local/strategy/` — go-to-market timeline + Reddit/HN/Twitter/dev.to drafts.
  - `.local/partnerships/` — outreach drafts dla H88 (small.pl/lh.pl), PL/EU/US hosters list, cPanel vendor partner path.
  - `.local/portfolio/` — one-pager + elevator pitches (30s/60s/180s).
  - `.local/metrics/analytics.md` — KPI tracking + weekly snapshot template + funnel mental model.
  - `.local/notes/2026-05-25-initial-launch-strategy.md` — pełny zapis decyzji strategicznych z dzisiejszej sesji planowania (8 lock-in decisions, hidden assumptions, ścieżki ryzyka).

### Changed

- **License: MIT → Apache-2.0 (2026-05-25).** Wymiana wymuszona przez explicit patent grant ważny przy adapterach paneli komercyjnych (cPanel LLC, DirectAdmin Inc., CyberPanel/OpenLiteSpeed). `docs/CONTRIBUTING.md` TL;DR zaktualizowane. Full audit closed in **TASK-15.7** (2026-05-25): `README.md` shield + footer, `docs/PRD.md §12.1`, `.cursor/skills/release-check/SKILL.md` and the local `landing/index.html` (Schema.org JSON-LD `license` field + visible badge + footer link) + `landing/landing-page-plan.md` now all reference Apache-2.0. `docs/dependencies.md` keeps `MIT` in the allowed-upstream-license list — that is the only legitimate remaining occurrence.
- **`docs/ROADMAP.md` (2026-05-25).** Sekcja v0.2 całkowicie przepisana z nową roadmap'ą post-MVP: Sprint 15 (Launch Readiness) → Sprint 16 (Public Launch + cPanel Research) → Sprint 17-18 (cPanel Adapter delivery + v0.2.0-rc1) → Sprint 19 (Preset Registry → marketing differentiator) → Sprint 20+ (Decision Matrix). Poprzedni `sprint-15-v02-foundation-plan` (z TASK-14.8) zostaje jako notion historical — Sprint 15 jest teraz Launch Readiness, nie v0.2 foundation. Konsekwencje tej zmiany ujęte w `docs/sprints/sprint-14-architecture-hardening.md` TASK-14.8 retro section.

### Added (Sprint 14 - prior entries continue below)

- **Sprint 14 — Surface migration completed (TASK-14.1, 2026-05-25).** Every production state in the cockpit now has a dedicated `surface.Surface` adapter under `tui/surface/<state>/`:
  - `tui/surface/initwizard/` — initial setup wizard (Crumb: "Init Wizard", AcceptsScroll: false).
  - `tui/surface/projectdetail/` — project detail (overview + live logs tabs, AcceptsScroll: true).
  - `tui/surface/projectwizard/` — new project wizard.
  - `tui/surface/resumewizard/` — resume wizard for paused project creations.
  - `tui/surface/importpreview/` — import preview list (AcceptsScroll: true).
  - The big switch in `tui/view.go::renderRootBody` is now empty for production states; the only remaining branch is a defensive default that surfaces a placeholder string for unmigrated `State` values, replacing what used to be a silent empty-body bug class. `TestSurfaceFor_AllProductionStatesMigrated` pins this contract.
- **Sprint 14 — Per-tile scroll + focus rotation (TASK-14.2, 2026-05-25).** `Tab` and `Shift+Tab` now cycle keyboard focus between scrollable tiles in the dashboard (CI/CD pipeline, Live Server Logs). While a tile is focused, `PgUp` / `PgDn` / `Home` / `End` and the mouse wheel scroll *that* tile's offset; the global body viewport stays inert. `Esc` releases focus without leaving the dashboard.
  - New `bento.ScrollableTile` interface (`Scroll(delta) ScrollableTile` + `ScrollOffset() int`) is implemented by `microLogsTile` and `cicdPipelineTile`. Non-scrollable tiles deliberately skip the interface so the focus rotation never lands on them.
  - State lifted to `tui.Model` (`focusedTile *bento.Slot`, `tileScrollOffsets map[bento.Slot]int`) so the offsets survive the value-typed Update returns.
  - Footer hint dynamically swaps from the global "PgUp/PgDn (offset/max)" form to "focus: <name> · [PgUp/PgDn] scroll panel · [Esc] release" so the operator always knows which scope the next scroll key will affect.
  - Legacy `Tab → Project Detail` mapping moved to `Right` / `Enter`; the existing e2e scenario was updated.
  - Tests: 6 new unit scenarios (`tui/tile_focus_test.go`) + 2 new e2e scenarios (`internal/e2e/cockpit_test.go::TestCockpit_FocusedTileScrollsIndependentOfViewport`, `TestCockpit_TileFocusReleasesOnEsc`, `TestCockpit_TabBackwardsLandsOnLogsCycle`).
- **Sprint 14 — `ssh.InflightLimiter` + `ExecWithRetryLimited` (TASK-14.3, 2026-05-25).** Global SSH concurrency cap built on `golang.org/x/sync/semaphore.Weighted`, sized via `max(8, profiles/2)` per the Sprint 14 AC. Where the per-host `Pool.MaxPerHost` cap protects an individual remote from a thundering herd, the in-flight limiter protects the operator's machine and the upstream provider from cumulative pressure — a 50-project cockpit refresh used to fan out into ~150 simultaneous SSH dials, the limiter clamps that at ~25.
  - `ssh/inflight.go` — `NewInflightLimiter(profiles)`, `Acquire(ctx)`, `TryAcquire()`, `Release()`. Nil-safe (callers can disable the cap by passing nil).
  - `ssh/retry.go` — new `ExecWithRetryLimited(ctx, pool, limiter, ...)` entry point; legacy `ExecWithRetry` delegates with `limiter == nil` to keep test seams stable.
  - Default retry policy realigned to the AC: 3 attempts, 200 ms base, 1.2 s cap (was 4 / 100 ms / 1 s).
  - Race test (`ssh/inflight_test.go::TestInflightLimiter_GoroutineCapHonoured`) spawns 100 simulated profiles × 200 ticks, asserts peak in-flight stays ≤ budget × 3 (overshoot factor accounts for cooperative scheduling between Acquire and the body increment).
- **Sprint 14 — Nightly E2E workflow (`.github/workflows/nightly.yml`, TASK-14.5, 2026-05-25).** New scheduled workflow runs `go test -race -count=1 -v ./internal/e2e/...` daily at 03:13 UTC and uploads the verbatim output as an artefact (7-day retention). The `internal/e2e/` package now ships 12 multi-tick teatest scenarios (5 from Sprint 13 + 7 added during Sprint 14: host-key modal, debug trace event, viewport scroll, focused tile scroll, focus release on Esc, Shift+Tab backwards rotation, package load smoke check). Total wall-clock budget per nightly run: well under the 10 s AC ceiling.
- **Sprint 14 — `bento.TileBlock` + `BlockRenderer` (TASK-14.7, 2026-05-25).** Structured replacement for the string-level `clipTileBlock` heuristic. `TileBlock{TopBorder, Header, Body, BottomBorder, AccentRGB}` decomposes a rendered tile into typed lanes; `clipBlock(block, maxLines)` operates on the structure instead of counting magic-numbered border rows. The legacy `clipTileBlock(rendered, maxLines)` wrapper now parses into a TileBlock via `parseTileBlock`, defers to `clipBlock`, then renders back — the only surviving call site for the old algorithm. Magic constants `borderRows = 2` and `bordersAndHeader = 3` removed from `engine.go`. Bench gate (`make bench-check`) green: 117 / 182 / 195 µs/op (Apple M4), 25× under the 5 ms budget.
- **Sprint 14 plan — `docs/sprints/sprint-15-v02-foundation-plan.md` (TASK-14.8, 2026-05-25).** New 4-task sprint plan freezing the v0.2 foundation backlog: cPanel adapter (Provider Pattern validation), OAuth Device Flow PoC behind `WEBOX_EXPERIMENTAL=1`, `config.json` schema v3 migration with optional DB fields, ADR-0010 i18n migration plan. `docs/ROADMAP.md` Sprint table updated with Sprint 15 entry.
- **Sprint 14 — Mock cockpit acknowledges Sprint-14 subsystems (2026-05-25).** `tui/mockdata.go` `MockLiveLogLines()` now seeds two additional log lines that surface the new telemetry sink and SSH pool metrics in the offline demo, so `webox --mock` is self-documenting for operators discovering the Sprint-14 features for the first time. The lines are synthetic ("cockpit: telemetry.Sink = Disabled", "ssh.pool: MaxPerHost=3, ExecMetrics{…}") and contain zero secret-shaped content.

### Changed
- **Sprint 14 — golangci-lint v2 hygiene pass (2026-05-25).** The new Sprint-14 code (host-key modal, retry layer, telemetry file sink, `--debug-trace` CLI wiring) ships with a clean `make lint` run:
  - Replaced `errors.New("telemetry: empty trace path")` with the typed sentinel `telemetry.ErrEmptyTracePath` so callers can branch with `errors.Is` and the `err113` rule stays green.
  - Extracted `defaultRetryAttempts` / `defaultRetryBaseBackoff` / `defaultRetryMaxBackoff` constants in `ssh/retry.go` (was triggering `mnd` magic-number flags).
  - Named return values on `openTraceSink` (gocritic `unnamedResult`) and renamed the inner variable to avoid shadow.
  - `WriteString(fmt.Sprintf(…))` → `fmt.Fprintf` in `tui/host_key_modal.go` (`staticcheck` QF1012).
  - Documented `nolint:gosec` on the `os.OpenFile(resolved, …, 0600)` call — path is operator-supplied via the audited `--debug-trace=PATH` flag, file mode is locked at 0600.
  - Removed two dead-code units (`runMockTUI`, `Model.dismissHostKeyModal`) that were superseded by the trace-aware `runMockTUIWithTrace` and the inline Esc handler in `Update`.
  - `gofumpt -w` applied to every touched file. `make ci` exits clean with 81 % coverage; `make bench-check` reports 117 / 186 / 202 µs/op (Apple M4), 25× under the 5 ms budget.

### Added
- **Sprint 14 — E2E expansion: host-key modal, --debug-trace event, viewport scroll (TASK-14.5, 2026-05-25).** Three new multi-tick scenarios in `internal/e2e/cockpit_test.go` raise the operator-visible coverage from the Sprint 13 baseline of 5 to **9 scenarios** (sub-second total wall clock):
  - `TestCockpit_HostKeyModalRendersAtRuntime` — boots the mock cockpit, injects `StatusRefreshFailedMsg{Err: ssh.ErrHostKeyMismatch}`, asserts the strict-block modal painted "Host key mismatch", "ssh-keygen -R", "OUT OF BAND", "SECURITY" inside the composed frame (chrome + tile + overlay). Scope is intentionally render-side; the dismiss-on-Esc keyboard contract stays at the cheaper unit tier.
  - `TestCockpit_DebugTraceEmitsHostKeyEvent` — wires a recording `telemetry.Sink` into `tui.MockOptions`, replays the same failure, then verifies the trace contains both `status.refresh_failed` with `err_class=host_key_mismatch` AND `modal.hostkey_open` with `kind=mismatch`. This guards the emit-call-sites at the e2e tier so a future Update refactor that swallows the message cannot silently break the trace contract.
  - `TestCockpit_PgDownScrollsViewportInOverflow` — opens the cockpit at 120×22 (forces Bento Ultra overflow), sends `PgDown`/`Home`, asserts the chrome footer's `↕ scroll: PgUp/PgDn` indicator persists across the keyboard flow. Catches regressions in the viewport scroll routing introduced when the chrome contract was extracted in Sprint 13.
  - Duplicated `recordingSink` lives in the `internal/e2e` package by design — `tui/trace_emit_test.go` keeps its own copy so the e2e package depends on `tui` only through the public surface (matches the package-boundary convention from `internal/e2e/doc.go`).

- **Sprint 14 — `--debug-trace=PATH` local JSONL trace (TASK-14.6, 2026-05-25).** New CLI flag (and `telemetry.FileSink`) record structured cockpit events to a local file for offline debugging. Strict guarantees:
  - **Local-only.** No network, no auto-upload, no fallback transport. The package `internal/telemetry/file_sink.go` is grep-clean — there is literally no `net/http` import.
  - **Mode 0600** on the file + `O_APPEND|O_CREATE` so multiple runs accumulate without widening access. Parent dir created with `0700` if missing.
  - **Redactor on every line.** The encode → redact → write pipeline runs the canonical JSON envelope through `internal/log.Redact` before disk hits, catching `ghp_*` / `github_pat_*` / Authorization headers / `mysql://user:pw@…` / `"password":"…"` (`TestFileSink_RedactsSecretsBeforeWrite` pins this on a real-world payload).
  - **Drop on backpressure**, not block. Bounded channel + background drain goroutine; full queue drops the event and bumps an atomic counter (`DroppedEvents()`) instead of stalling the cockpit's Update hot path (`TestFileSink_DropOnFullQueue` with 1k producers).
  - **Coarse error labels in trace.** `classifyErrForTrace` maps the SSH error chain to short categories (`host_key_mismatch`, `host_key_unknown`, `pool_busy`, `reconnect_exhausted`, `other`) so the trace never embeds raw error messages that could leak paths or hostnames.
  - **First emit-call-sites in the cockpit.** `tui/update.go` emits `status.refresh_failed` (with `err_class`) and `modal.hostkey_open` (with `kind`); further call sites land in TASK-14.6 follow-up batches. `Options.Trace` defaults to `telemetry.Disabled` so production runs without the flag are bit-for-bit identical.
  - **CLI surface.** `webox --debug-trace=/tmp/webox.jsonl` (also composes with `--mock`, `doctor`, `--json`). Empty path is a CLI misuse error. Tests: `TestParseArgs_DebugTracePathRoundtrip`, `TestParseArgs_DebugTraceEmptyPathIsMisuse`, `TestParseArgs_DebugTraceAlongsideDoctor`.
  - **Compile-time guards.** `var _ Sink = (*FileSink)(nil)` and `var _ io.Closer = (*FileSink)(nil)` so the contract drift is caught at `go build`, not at runtime.

- **Sprint 14 — `ssh.ExecWithRetry` + `ExecMetrics` (TASK-14.3, 2026-05-25).** `ssh/retry.go` adds a thin retry layer on top of `ssh.Exec` that distinguishes "pool exhausted, back off and try again" from "terminal error, surface it now". Behaviour highlights:
  - Retries **only** on `ssh.ErrPoolBusy`; terminal sentinels (`ErrHostKeyMismatch`, `ErrHostKeyUnknown`, command exit codes, auth failures) bypass the loop so the host-key modal / wizard parser see the original error in one tick.
  - Backoff is exponential (`BaseBackoff << attempt`) clamped at `MaxBackoff`, with ±20 % jitter to prevent the thundering-herd pattern when the cockpit's periodic status refresh wakes every project goroutine simultaneously.
  - Defaults (`DefaultRetryableExecPolicy`): 4 attempts, 100 ms base, 1 s cap → ~2.3 s worst-case wall clock, comfortably inside the 5 s SWR freshness budget (DESIGN §8).
  - `ExecMetrics` exposes atomic counters (`Acquires`, `PoolBusyHits`, `Retries`, `TerminalErrors`) and a JSON-stable `ExecMetricsSnapshot` — the data feed for the upcoming `--debug-trace` JSONL stream (TASK-14.6).
  - Idempotency contract documented in the godoc + sprint plan: only read-only / safely-replayable commands MAY use `ExecWithRetry`. State-mutating ops keep using bare `Exec` so the provider parser can inspect the remote side before deciding whether to replay (DESIGN §9).
  - Tests: jitter bounds (`-20%/0/+20%`), exponential clamp at `MaxBackoff`, busy → retry → success path, terminal-error bypass, budget exhaustion, context cancellation, nil-safe `Snapshot()`. Per AGENTS.md §7.1 the tests that swap the `execFunc` package seam run sequentially (no `t.Parallel()`).

### Security
- **Sprint 14 — Host-key mismatch / unknown-key modal (TASK-14.4, 2026-05-25).** When any SSH operation surfaces `ssh.ErrHostKeyMismatch` or `ssh.ErrHostKeyUnknown`, the cockpit now opens a strict-block modal (`tui/host_key_modal.go`) instead of swallowing the failure into a dismissible alert toast. The modal:
  - **Never** renders the offered key, its fingerprint, SHA-256, MD5, or any cryptographic material — that policy is locked behind `TestRenderHostKeyModal_NeverLeaksKeyMaterial`, which asserts the absence of `AAAAB3`, `ssh-ed25519`, `ssh-rsa`, `ecdsa-sha2`, `SHA256:`, `MD5:` substrings.
  - Surfaces the exact recovery command (`ssh-keygen -R <host> -f <known_hosts>`), the literal `known_hosts` path, and a brief MITM-aware warning citing `SECURITY §5`.
  - Blocks all keyboard input except `Esc`/`Enter` (close) and `q`/`Ctrl+C` (quit) — `TestUpdate_HostKeyModal_BlocksKeysAndDismissesOnEsc` verifies cursor / Tab / Right cannot move the selection while the modal is open, so the operator cannot accidentally re-trigger an SSH command on top of a refused connection.
  - Distinguishes `Kind=mismatch` (red border, "potential man-in-the-middle attack" copy) from `Kind=unknown` (warning border, "first connection — verify out-of-band before accepting" copy).
  - Does **not** continue the connection on its own; closing returns control without retrying. The next user-triggered SSH op picks up cleanly once the operator runs `ssh-keygen -R`.
  - Wired into `Update` via `tryRaiseHostKeyModal(err)` from `StatusRefreshFailedMsg`; additional call sites (`ProjectActionCompletedMsg`, `ImportScanCompletedMsg`, wizard preflight) will be hooked in TASK-14.4 follow-up. The legacy alert-toast path keeps working for non-host-key errors (`TestUpdate_StatusRefreshFailed_NonHostKeyKeepsLegacyAlert`).

### Changed
- **Project rules + roadmap sync (2026-05-25).** Charter (`.cursor/rules/00-charter.mdc`) updated to reflect [ADR-0007](./docs/adr/0007-bento-ultra-eskalacja-mvp.md) — Bento Ultra, Live Log Stream, Live Service Topology, CI/CD Live Panel and header metrics are **in MVP**, not STRETCH. Added explicit no-telemetry / no-plugin-marketplace clauses, perf gate guardrail (`make bench-check` with 5 ms budget), e2e scenario requirement, and host-key UX policy (modal fallback in Sprint 14 → full `webox doctor security --update-host-key` in v0.2+). `.cursor/rules/20-bubbletea-mvu.mdc` gained the Sprint 13 chrome contract + mouse API + surface contract sections; `.cursor/rules/50-tests.mdc` documents the `internal/e2e/` layer and `make bench-check`. `AGENTS.md` repo layout reflects the new `tui/surface/`, `tui/bento/`, `internal/e2e/` packages. `docs/ROADMAP.md` adds Sprint 13 and Sprint 14 rows.

### Added
- **Sprint 14 plan — Architecture hardening (`docs/sprints/sprint-14-architecture-hardening.md`) (2026-05-25).** 8 tasks tied directly to the post-RC code-review critique: full migration of remaining states to `surface.Surface`, per-tile scroll + focus rotation, SSH in-flight semaphore + retry, host-key mismatch modal, `internal/e2e/` expansion + nightly CI, local `--debug-trace`, structured `TileBlock` refactor, v0.2 backlog freeze. Explicitly **no telemetry**, no plugin marketplace, no AI features — Sprint 14 is technical hardening only.

- **Sprint 13 — Per-frame benchmark + CI perf gate (2026-05-24).**
  - `tui/bento/engine_bench_test.go` — `BenchmarkRenderMode/{standard-100x30,ultra-120x35,ultraplus-160x45}` measures the cockpit's per-frame composition cost (`ns/op` + `B/op`) using a representative 5-tile stub (Projects + Server + Topology + CI/CD + Logs). Current baseline on Apple M4: 138 / 183 / 192 µs/op respectively — comfortably inside the 60 fps budget (~16 ms).
  - `Makefile` — new `make bench` target runs the suite verbatim; new `make bench-check` parses the output and fails when any single `ns/op` exceeds `BENCH_MAX_NS` (default 5 000 000 ns / 5 ms — 26× headroom over current baseline). A new `make ci-full` target chains `make ci` + `make bench-check` for release-candidate hardening.
  - Rationale: lipgloss is a string builder where subtle changes (extra `Padding` call, alternate border style) can grow allocations 5–10×; without a CI gate we would only learn about regressions when an operator notices terminal lag.

- **Sprint 13 — E2E test layer (`internal/e2e/`) (2026-05-24).**
  - `internal/e2e/doc.go` documents the package boundary: per-surface unit tests stay in `tui/views/`, per-message Update tests in `tui/`, single-frame snapshots in `tui/cockpit_snapshot_test.go`, **multi-tick keyboard flows** here.
  - `internal/e2e/cockpit_test.go` — 5 deterministic scenarios driven by `teatest`:
    - `TestCockpit_MockBootShowsAllSurfaces` — mock-mode boot pins every Bento Ultra slot (Active Projects / Server / Topology / CI/CD / Live Logs) so a regression in `View()` composition surfaces immediately.
    - `TestCockpit_TabIntoProjectDetailAndBack` — `Tab` from dashboard → Project Detail body, `Esc` returns; the most travelled keyboard path in production.
    - `TestCockpit_OpenLiveLogsTab` — `4` opens the Sprint 09 live-log surface; guards the ring buffer + redactor wiring.
    - `TestCockpit_TinyFallbackShowsResizeWarning` — `60×18` viewport must surface "Terminal too small" instead of silently truncating.
    - `TestCockpit_ScrollHintAppearsOnOverflow` — `120×22` forces a Bento Ultra overflow and asserts the bottom chrome renders `↕ scroll: PgUp/PgDn`.
  - Whole package budget: < 1 s wall clock today (5 scenarios in ~0.5 s); CI gate cap is 10 s per `internal/e2e` package per `doc.go`.

- **Sprint 13 — Surface contract foundation (2026-05-24).**
  - `tui/surface/` — new leaf package with the [`Surface`](./tui/surface/surface.go) interface (`Body` / `Crumb` / `Footer` / `AcceptsScroll`), a read-only [`Context`](./tui/surface/surface.go) snapshot, a structured [`FooterHint`](./tui/surface/surface.go), and a thread-safe [`Registry`](./tui/surface/registry.go) for tests / future per-Model lookups. Six unit tests pin the contract (`registry register/lookup/reset`, `FooterHint.Empty()`, compile-time contract guard).
  - `tui/surface_adapters.go` — `Model.surfaceFor()` returns a fresh [`dashboardSurface`](./tui/surface_adapters.go) per render so the value-typed Model semantics stay intact (no stale pointer captured between ticks). The dashboard adapter is the first migrated state; remaining surfaces (init wizard, project detail, wizards, import preview) keep flowing through `renderRootBody` until Sprint 14.
  - `tui/view.go::renderRootBody` now prefers `m.surfaceFor()` and falls back to the legacy switch — one seam for the gradual decomposition of the `tui/` god-package.
  - Regression suite: `TestDashboardSurface_BodyMatchesLegacyRenderer` proves the adapter is byte-identical to the legacy path across `100×30 / 120×35 / 160×45`; `TestSurfaceFor_UnmigratedStatesReturnNil` guards the fallback path and fails loudly the day a new state is migrated without updating expectations.

- **Sprint 13 — Chrome contract + Bento height budgets (2026-05-24).**
  - `tui/view.go::View` now composes every surface in three pinned slots: a one-line top chrome (cockpit status bar — dashboard reuses the bento engine's bar via `WithStatusBar`; every other state gets a pinned bar from `renderChromeTop`), a scrollable body (sliced by `renderViewport`), and a one-line bottom chrome (key-hints + `↕ scroll: PgUp/PgDn · Home/End · Mouse · (offset/max)` indicator). Tiny mode (`< 70×22`) intentionally skips the chrome so the "Terminal too small" warning stays self-contained.
  - `tui/update.go::updateMouse` — mouse-wheel scrolling for the body slot using the post-1.3 Bubble Tea API (`MouseActionPress` × `MouseButtonWheelUp / WheelDown`). Long-press / drag does not loop scrolls (we only react to press). Step is 3 lines so a trackpad flick feels precise but a real wheel does not stall.
  - `tui/bento/engine.go::planRowBudgets` + `clipTileBlock` + `framedIndicatorLine` — height-aware Ultra/Ultra+ grid. Each row gets an explicit budget (`status bar → top row (Projects+Server) → second row (Topology+CI/CD) → logs → optional UltraPlus strip`). Tiles that exceed their budget keep the top border + header + last visible body line and replace the overflow with a `┃ … +N more lines · scroll inside tab/modal ┃` row that reuses the tile's exact pixel width + accent colour so the cockpit frame never breaks geometrically. `equalizeBlockHeights` then pins siblings to the same row height, eliminating the empty whitespace under the shorter tile (Topology now matches CI/CD; Server matches Active Projects).
  - `tui/components/asciigraph/asciigraph.go` — topology nodes downgraded to `lipgloss.NormalBorder()` (`┌─┐└─┘`) so the visual hierarchy reads as *grid > tile > nodes* instead of three competing frame weights. Edge renderer collapsed from 3 lines (label / filler / arrow) to 2 lines (label-on-glyph / arrow), saving 3 rows across both edges.
  - `tui/views/dashboard.go` — Standard Cockpit (`100×30 ≤ width < 120×35`) restyled to share the Bento Ultra visual grammar: bracketed emoji headers (`📂 [Active Projects]`, `🖥 [SERVER: …]`), rounded selection pills painted in the primary accent, thick-bordered panels, and a centralised cockpit status bar pinned by `tui.View`.
  - `docs/UX.md` §4.2 + §6.2 — documented the chrome contract, the height budget eliminating empty whitespace, the `┃ … +N more lines ┃` indicator, mouse-wheel scroll semantics, and the inverted border hierarchy in the topology tile.
  - `docs/DESIGN.md` §2.4 — new "Chrome contract: status bar / body / footer" section formalising the three-slot composition + bento height budgeting algorithm.

- **Sprint 11 — Live Service Topology Map (2026-05-24).**
  - `tui/components/asciigraph/` — new pure-function renderer for the cockpit's service-topology tile. Exposes `Graph`, `Node`, `Edge`, `EdgeGlyphs(state, pulse)` and `Render(g, width)`. Heavy box-drawing nodes (`┏━━┓`) connect via state-aware glyph pairs (`──────────` + `✓` for ONLINE, `╌╌ ╌╌ ╌╌ ╌` + `▶` for BUILDING, `━━━━━━━━━━` + `⚠` for DEGRADED, `⚡ ⚡ ⚡ ⚡ ⚡` + `✗` for OFFLINE). 12 unit tests pin the glyph contract, online/offline/building/DB-leaf paths, label truncation, and determinism.
  - `tui/bento/topology.go` — `NewTopologyTile(TopologySnapshot)` exposes the renderer via the `BentoTile` interface; the snapshot carries `Graph`, `Pulse`, and a `HelpHint` line ("All systems nominal" / "Deploy in flight" / etc.).
  - `tui/topology.go::buildTopologySnapshot` — pure builder that folds `config.Project`, `ProjectStatus`, and `cicdSnapshotEntry` into an `asciigraph.Graph`. Edge states mirror the underlying signals (CI status → repo→server edge; HTTP/SSL → server→subdomain edge; SSL<14d demotes to DEGRADED without flipping to OFFLINE). 5 unit tests cover healthy / SSL-degraded / offline-cascade / building / missing-status paths.
  - `tui/bento/engine.go` — Ultra (`120×35`) **and** Ultra+ (`160×45`) now render the topology tile under the logs row (TASK-11.* explicitly promoted topology to MVP per the new cockpit reference image; previously Ultra+ only).
  - Pulse animation driven by `m.nowFn().Second()%2` so BUILDING/OFFLINE edges shimmer on the existing refresh tick — no extra timer, no goroutine, no leak risk.
- **UI/UX refresh round 2 (2026-05-24).**
  - All bento tiles now render with `lipgloss.ThickBorder()` (`┏━━━┓`) instead of `lipgloss.RoundedBorder()`. Focused tiles upgrade to `lipgloss.DoubleBorder()` (`╔═══╗`) so the active panel always reads as the brightest frame.
  - `theme.Styles.Panel` / `Styles.ActivePanel` rebuilt around the same thick/double border pair so wizard and detail screens share the cockpit's frame language end-to-end.
  - Tile headers gained tone-on-tone emoji prefixes: `📂 [Active Projects]`, `🖥 [SERVER: …]`, `🚀 [CI/CD PIPELINE: Main Branch]`, `📜 [Live Server Logs]`, `🌐 [Live Service Topology]`, `📊 [Header Metrics]`. Emoji live only in headers (where they sit on their own line); data rows keep 1-cell geometric glyphs (▣ ◆ ◉ ✓ ↔ ⚿ ⎇ ⏲) so column alignment stays intact.
  - `tui/views/init_wizard.go` — new ASCII WEBOX banner painted above step 1 of the init flow:
    ```
    ╦ ╦╔═╗╔╗ ╔═╗═╗ ╦
    ║║║║╣ ╠╩╗║ ║╔╩╦╝
    ╚╩╝╚═╝╚═╝╚═╝╩ ╚═  ·  v0.1 cockpit
    ```
  - `tui/view.go::chromeWrap` — every non-dashboard surface (Init Wizard, Project Detail, Live Logs, Project Wizard, Resume Wizard, Import Preview) now renders the global status bar + footer hints around its body so the cockpit feels coherent across screens. Surfaces below the Standard threshold (`100×30`) keep the legacy split-pane silhouette.
- **`cmd/webox/run.go` — `tea.WithAltScreen()` + `tea.WithMouseCellMotion()`.**
  - The TUI is now a true full-screen app (like vim / htop / lazygit): screen swaps replace the current frame instead of scrolling host terminal history. Alternate screen buffer is released on quit so the operator returns to a clean prompt.
  - Mouse cell motion is enabled at program level so future click-through surfaces (CI/CD step click → open run, log scroll) can opt in without bumping program options.
- **`docs/sprints/sprint-12-polish-release.md`** — full plan for the v0.1 RC1 release sprint (Standard Cockpit topology fallback, chrome consistency audit, asciinema demos, performance budget enforcement, release tooling smoke-test, CHANGELOG release notes + tag).
- **`docs/sprints/sprint-13-v01-ga-and-post-mvp-foundation.md`** — full plan for v0.1 GA + post-MVP foundation (GA tag, provider research, OAuth Device Flow PoC behind `WEBOX_EXPERIMENTAL`, `config.json` schema v3 with optional DB fields, ADR-0010 for generic DAG layout deferral, bug bash round 2).

### Changed
- **Sprint 12 — Responsive cockpit polish (2026-05-24).**
  - `tui/view.go` + `tui/update.go` — full-frame viewport slicing now keeps overflowing renders inside the TUI instead of dumping extra lines into terminal history. When a frame is taller than the current window, the operator can move through it with `PgUp`, `PgDn`, `Home`, `End`; existing `↑/↓` flows (dashboard selection, live logs buffer, CI/CD modal) remain unchanged.
  - `tui/bento/engine.go` — Ultra layout moved from `Projects | Server+CI/CD` + full-width logs + full-width topology to a true responsive `2×2 + logs` grid: `Projects | Server` on the first row, `Topology | CI/CD` on the second row, logs full-width below. Column widths now react to viewport bands instead of a single hard-coded ratio.
  - `tui/topology.go` + `tui/views/dashboard.go` — Standard Cockpit (`100×30`) now renders a textual `Connections:` fallback inside `Overview`, built from the same topology snapshot as the Ultra tile so both layouts describe the same system state.
  - `tui/views/*.go` — non-dashboard screens now share the cockpit's visual grammar more consistently: bracketed emoji titles (`🪄 [Init Wizard]`, `🧱 [New Project Wizard]`, `♻ [Resume Wizard]`, `📥 [Import Existing Projects]`, `🖥 [Project Detail: …]`, `📜 [Live Logs: …]`) plus the previously introduced shared chrome.
  - `docs/UX.md` and `docs/sprints/sprint-12-polish-release.md` now describe the shipped responsive layout/overflow behavior rather than the old RC1-centric Sprint 12 plan.
- `docs/sprints/sprint-11-topology-map.md` — closed out with full `Outcome` section: 12 + 5 unit tests, coverage metrics, decisions (asciigraph stays a leaf renderer, topology first-class in Ultra, thick borders adopted cockpit-wide), surprises (emoji column width, alt-screen mode fix), security validation (zero new network calls).
- `tui/bento/tiles.go::renderTilePanel` — focus state now upgrades the border style from thick to double instead of merely brightening the colour. The accent colour stays consistent so role-grouping (magenta column / cyan column) reads at a glance.
- `tui/view.go` overview lines reverted to 1-cell geometric glyphs (▣ ◆ ◉ ✓ ↔ ⚿ ⎇ ⏲) after the emoji set introduced subtle column-shift glitches in the first polish round.

### Added (previous entries continue)
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

[Unreleased]: https://github.com/dilitS/webox/compare/v0.1.0-rc1...HEAD
[v0.1.0-rc1]: https://github.com/dilitS/webox/releases/tag/v0.1.0-rc1
