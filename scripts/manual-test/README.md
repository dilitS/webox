# scripts/manual-test/ — Webox TUI smoke tests (tuistory)

This directory hosts the Sprint 20 manual-but-automated smoke-test
runner. It uses [tuistory](https://github.com/remorses/tuistory)
(Playwright-for-terminals) to drive `./bin/webox --mock` in a
headless PTY, exercising every Sprint 20 user-facing change:

| Scenario              | Sprint 20 task    | What it asserts                                                                    |
| --------------------- | ----------------- | ---------------------------------------------------------------------------------- |
| `bento-modes`         | TASK-20.3 + chrome| Resize 160×45 → 120×35 → 100×30 → 60×18 → 120×35 flips Bento tiers, no leaks       |
| `help-overlay`        | TASK-20.5         | `?` opens overlay, `n` is strict-blocked, `Esc` returns to underlying state intact |
| `provider-catalog`    | TASK-20.2         | `[p]` opens catalog, `↓` walks cursor, `Enter` toggles detail, `c` surfaces hint   |
| `project-detail`      | TASK-20.4         | Tabs 2/3 unstubbed (no v0.2 placeholder), `Tab` is back-nav alias                  |
| `mouse-click`         | TASK-20.1         | Click status bar = no-op; click Projects tile = drill; click detail = back        |

All 34 assertions must pass for a green smoke run (~80s wall-clock).

## Prerequisites

- Node.js **24+** (uses `--experimental-strip-types` for inline TypeScript)
- `tuistory` and its deps (installed via `npm install`)
- `./bin/webox` built (`make build`)
- macOS / Linux / Windows host with a usable PTY backend (`tuistory`
  ships its own through `ghostty-opentui`)

## One-time setup

```sh
npm install
```

This installs `tuistory@^0.8.0` and its dependency tree into
`node_modules/`. The directory is `.gitignore`-d so the Webox Go repo
stays Go-only at the top level.

Heads-up: `npm install` reports ~23 transitive vulnerabilities (low /
moderate) inside the tuistory dependency tree. The runner is
**developer-only tooling** that never ships to the production binary,
runs only against the local mock fixture, and binds to nothing — so
the policy in `AGENTS.md §1` (no new `go.mod` deps without sign-off)
is preserved. If a vuln becomes critical, pin via `overrides` in
`package.json`.

## Running

From the repo root (preferred — the Makefile target also checks Node
version):

```sh
make smoke-test
```

Or directly:

```sh
cd scripts/manual-test
node --experimental-strip-types --no-warnings smoke.ts
```

Output:

- `docs/screenshots/sprint-20/manual/{NN}-{name}.txt` — one
  deterministic text snapshot per interesting frame (17 in total).
- `docs/screenshots/sprint-20/manual/REPORT.md` — green/red table
  with per-scenario summaries; the PR diff includes this so reviewers
  see at a glance what changed.

Exit code: `0` on full green, `1` on any failed assertion. The runner
prints failures to stderr with a `✗ scenario: message` prefix.

## Adding a scenario

1. Create `scenarios/<name>.ts` exporting `async function run()`.
2. Use `openWebox(opts)` from `lib.ts` to spin up a PTY.
3. Drive the TUI with `press` / `type` / `clickAt` / `resize`.
4. Snapshot with `snapshot(session, "NN-name")`.
5. Assert with `expect(condition, scenario, message)`.
6. Register the scenario in `smoke.ts` under `SCENARIOS`.

Keep each scenario focused on **one user-visible change**. A scenario
that exercises ten orthogonal things is hard to triage when only one
asserts goes red.

## Watching a live run

While the runner sleeps inside `waitIdle`, attach in a second
terminal:

```sh
tuistory attach -s <webox-bin>-<webox-binary-hash>--mock
```

(See `tuistory sessions` for the active name.) Press `Ctrl+C` twice
to detach without killing the test.

## Why text and not PNG?

`tuistory`'s JS API exposes `session.text()`, not `session.screenshot()`
(PNG capture is CLI-only). Text snapshots:

- diff cleanly in `git diff` so the PR reviewer sees the exact byte
  that changed,
- compress trivially (smallest is 662 B, largest 8.9 kB),
- never require an image-diff tool to triage a regression.

PNG captures live under `docs/screenshots/sprint-20/` (committed by
the `cmd/screenshot` Go probe) and demo cards at
`assets/screenshots/dashboard.png` (asciinema → ffmpeg). Neither is
in the smoke-test loop.

## Troubleshooting

| Symptom                                                       | Likely cause                                                                |
| ------------------------------------------------------------- | --------------------------------------------------------------------------- |
| `webox binary not found at .../bin/webox`                     | `make build` not run, or running outside repo root                          |
| `node:internal/modules/esm/loader: Cannot find package`       | `npm install` not run in `scripts/manual-test/`                             |
| `Error: napi_number_expected` from `session.resize`           | Wrong API shape (must be `{ cols, rows }`, not positional)                  |
| Scenario hangs at "WEBOX" wait                                | `./bin/webox --mock` failed to start; run it directly to see stderr        |
| Different snapshot bytes between two runs                      | Clock or random seed leaked into a fixture; check mock data deterministic   |
| `bento-modes` resize step fails after Tiny                    | Bento engine may not have re-painted; bump `waitIdle` timeout to 5s        |
