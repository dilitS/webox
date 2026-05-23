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
