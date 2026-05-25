# Sprint 21 — cPanel Adapter (part 1) + Public Launch Prep

> **Daty:** 2026-06-08 → 2026-06-22 (2 tygodnie) · **Cel:** Pierwsza warstwa cPanel adaptera (UAPI client + read-only ops + `webox doctor cpanel`) i jednoczesne przygotowanie public launch — repo polish, asciinema, cPanel test account.
>
> **Status:** 🚧 In progress (kicked off 2026-05-25) · **Properties:** code + ops mix · **Path selected:** **A** (full parallel — code + ops tracks both active).

## Kontekst

Po Sprintach 19 (preset registry) i 20 (TUI polish + Provider Catalog) projekt ma **infrastrukturę** dla wielu providerów (preset registry) **i** dopracowane TUI. Brakuje drugiego provider adaptera, żeby udowodnić generic-design.

Ten sprint zastępuje pierwotnie planowany **Sprint 17 — cPanel Adapter MVP part 1** (zachowując task ID TASK-17.x → TASK-21.x). Decyzję uzasadnia [ADR-0009 — Sprint renumbering after Sprint 19 out-of-order completion](../adr/0009-sprint-renumber-post-19.md) (do utworzenia w Sprint 21 jeśli nie istnieje).

Równolegle **Public Launch** (pierwotny Sprint 16) jest operator-time work (writing, posting, partnerships) — nie blokuje code-heavy adaptera, więc składamy go do tego samego sprintu jako parallel track.

## Cel sprintu

Po Sprincie 21:

1. `webox doctor cpanel --host=… --user=…` zwraca werdykt o providerze (UAPI accessible? SSH fallback works? PassengerApps installed?) + listę domen i baz read-only.
2. `webox doctor preset --id=cpanel-generic --probe` (Sprint 19 carry-over) realnie probe'uje cPanel na danym hoście i raportuje confidence.
3. Operator może pokazać projekt w nieswoim hosting-environment'ie (cPanel test account) — bez wizard'a / mutating ops, ale z pełnym status feed.
4. README EN opublikowany; landing page PL → EN translation done; asciinema cast attached do release notes.

Czego **nie** umiemy w Sprincie 21: tworzenie nowych projektów na cPanel (Sprint 22), SSL renewal na cPanel (Sprint 22), GHA deploy template dla cPanel (Sprint 22).

## Taski

### Code track (Webox cPanel adapter foundation)

#### TASK-21.1 — UAPI client (read-only ops only)

- **Estymata:** L (1.5-2 dni)
- **Zależności:** —
- **Acceptance Criteria:**
  - [ ] `providers/cpanel/uapi/client.go` exposes `Client.Call(module, function, args)` returning typed responses for the 4 read-only modules: `DomainInfo`, `PassengerApps`, `Mysql.list_databases`, `SSL.list_keys`.
  - [ ] HTTPS only; basic auth (login + token); rate-limit aware (429 → exponential backoff).
  - [ ] All requests carry `User-Agent: webox/<version> +https://github.com/dilitS/webox` (transparency).
  - [ ] No mutating endpoints accessible from this client; the type system prevents misuse (e.g., separate `MutatingClient` interface returns `ErrSprintScopeNotMutable` for v0.2 scope).
  - [ ] Table-driven tests with golden response fixtures from cPanel test account.
- **Pliki:**
  - `providers/cpanel/uapi/client.go` (new)
  - `providers/cpanel/uapi/client_test.go` (new)
  - `providers/cpanel/uapi/types.go` (new — typed response shapes)
  - `providers/cpanel/uapi/testdata/*.json` (new — golden fixtures)
- **Docs:** [DESIGN.md §5 (Provider abstraction)](../DESIGN.md), [docs/contributing/PROVIDER.md](../contributing/PROVIDER.md).

#### TASK-21.2 — SSH fallback layer

- **Estymata:** M (1 dzień)
- **Zależności:** TASK-21.1.
- **Acceptance Criteria:**
  - [ ] When UAPI returns 401/403/429 OR is disabled by the host, `providers/cpanel.HostingProvider` falls back to `ssh.Run(profile, "uapi <module> <function>")`.
  - [ ] Fallback is opt-in per hosting provider (`PreferUAPI: true|false` in preset).
  - [ ] Tests with `testing/sshmock` simulate UAPI unavailable + SSH fallback succeeds.

#### TASK-21.3 — `webox doctor cpanel` CLI

- **Estymata:** M (1 dzień)
- **Zależności:** TASK-21.1, TASK-21.2.
- **Acceptance Criteria:**
  - [ ] New CLI subcommand `webox doctor cpanel --host=<h> --user=<u>` (mirror of `webox doctor github`).
  - [ ] Output sections: Auth (UAPI vs SSH path), Domains, Databases, SSL keys, Passenger apps, Verdict (`OK` / `DEGRADED` / `BLOCKED`).
  - [ ] `--json` flag for machine-readable verdict.
  - [ ] Help text references [docs/contributing/PRESET.md](../contributing/PRESET.md) for adding cPanel-specific tweaks.

#### TASK-21.4 — `webox doctor preset --probe` (Sprint 19 carry-over)

- **Estymata:** M (1 dzień)
- **Zależności:** TASK-21.1.
- **Acceptance Criteria:**
  - [ ] `webox doctor preset --probe --id=<preset> --host=<h> --user=<u>` runs the `Probes.NodeVersion`, `Probes.PHPVersion`, `Probes.NginxVersion` commands (from preset YAML) over SSH.
  - [ ] Output reports per-probe: command, raw output, parsed value, match-status (matches preset's expected version pattern? `OK` / `MISMATCH`).
  - [ ] Confidence score (0-100) summarising probe agreement with preset metadata.

### Ops track (Public Launch prep, parallel to code)

#### TASK-21.5 — README EN final (Sprint 16 carry-over)

- **Estymata:** S (< 2h)
- **Zależności:** —
- **Acceptance Criteria:**
  - [ ] `README.md` (root) is EN-first, ≤ 60 lines, single H1 + value proposition, install + 30-second cockpit screenshot.
  - [ ] PL translation lands in `landing/pl/index.html` (already exists from Sprint 15).
  - [ ] All references to legacy `docs/CONTRIBUTING.md` (PL) replaced with `CONTRIBUTING.md` (EN entry) + linkbacks.

#### TASK-21.6 — asciinema demo cast

- **Estymata:** M (0.5 dnia)
- **Zależności:** Sprint 20 TUI polish (so the demo doesn't show buggy chrome).
- **Acceptance Criteria:**
  - [ ] `docs/screenshots/sprint-21/demo.cast` (asciinema format) captures: launch → mock dashboard → tab focus → project drill → tab logs → quit.
  - [ ] `docs/screenshots/sprint-21/demo.gif` rendered version (LFS or `git-lfs`).
  - [ ] README EN embeds the gif inline.

#### TASK-21.7 — cPanel test account onboarding

- **Estymata:** M (0.5-1 dnia)
- **Zależności:** —
- **Acceptance Criteria:**
  - [ ] Test cPanel account secured (sponsor: maintainer's purchase, partnership outreach to H88 in flight).
  - [ ] Credentials stored in keyring under alias `cpanel-test-account`.
  - [ ] Smoke test: `webox doctor cpanel --host=$TEST_HOST --user=$TEST_USER` returns clean verdict.
  - [ ] Test fixtures captured for `providers/cpanel/uapi/testdata/`.

#### TASK-21.8 — CHANGELOG, retro, sprint review

- **Estymata:** S (< 2h)
- **Zależności:** All of the above.
- **Acceptance Criteria:**
  - [ ] CHANGELOG entries for cPanel adapter (Added: read-only UAPI client, SSH fallback, `webox doctor cpanel`, `webox doctor preset --probe`).
  - [ ] Retro in `docs/retros/2026-06-22-sprint-21.md`.
  - [ ] Sprint outcome filled in.

## Path selection (decision gate at sprint start)

Before kickoff, the maintainer chooses one of:

- **Path A — execute as planned** (cPanel + Public Launch parallel). Code track + ops track both run; sprint is full but deliverable.
- **Path B — code-only** (drop ops track to Sprint 22). If the maintainer doesn't have a cPanel test account yet, code track stays but ops track defers. Sprint shrinks to 4 code tasks.
- **Path C — ops-only "soft launch"** (drop code track to Sprint 22). If launch timing matters more than v0.2 speed, focus the sprint on README EN + asciinema + Show HN post + outreach. Sprint 22 picks up cPanel work.

The choice is captured at sprint start in this document under "Outcome".

## Risk watch

| Ryzyko | Mitygacja |
|---|---|
| cPanel test account onboarding takes > 1 day (vendor delay) | Path B fallback: code with mocked UAPI fixtures. Real fixtures backfilled in Sprint 22. |
| UAPI rate-limits during dev (429) | Sprint cache responses to disk during dev (`PROVIDERS_DEV_CACHE=1` env override). |
| Public launch posts attract negative attention before cPanel adapter ships | Path B/C: launch only when v0.2.0-rc1 is tagged. Soft launch first to small communities (PL r/programowanie). |
| README EN drift vs PL landing | Single source of truth: PL landing references README EN; CI lint catches divergence (post-Sprint 22). |

## Outcome (wypełnij po sprincie)

- 📌 Path selected: **A** — full parallel (code + ops tracks both active). Maintainer chose "B and C" which functionally maps to A (the two alternatives in the path-selection menu, when both picked, equal the full-parallel scope). Recorded at sprint start 2026-05-25.
- ✅ Done:
  - **TASK-21.5 (2026-05-25)** — README EN final at 60 lines (≤ 60-line cap), single H1 (`# Webox`), value proposition + install + 30-second tour pointer + GIF embed (TASK-21.6) and static gallery link. RC2 badge wired to `https://github.com/dilitS/webox/releases/tag/v0.1.0-rc2`. All 13 internal links resolve. `docs/CONTRIBUTING.md` (PL legacy) banner + linkbacks already in place since Sprint 15.
  - **TASK-21.4 (2026-05-25)** — `webox doctor preset --probe --id=<id> --host=<h> --user=<u>` now executes the preset's probe commands over a real SSH session. Architecture: `cmd/webox/probe.go` ships a 95 %-tested `ProbeRunner` interface (pure summarization logic in `summarizeProbe` / `formatProbeText` / `formatProbeJSON` covers 11 unit tests) and a production `sshExecRunner` that shells out to the operator's native `ssh` binary with `BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10`. Auth, host-key validation, and tunnelling all delegate to the operator's `~/.ssh/config` and `ssh-agent` — Webox owns no new auth surface. The probe command is passed as a single argv element so no local shell expansion runs (only the remote shell on the panel parses it). Output formats: text (per-probe block + summary line + confidence 0-100) and JSON (`--json`, stable schema: `preset_id`, `preset_name`, `host`, `user`, `confidence`, `ok_count`, `mismatch_count`, `failed_count`, `results[]`). Exit codes: 0 (all OK), 1 (≥1 FAILED — dial / network), 2 (≥1 MISMATCH — panel disagrees). Per-probe timeout configurable via `--timeout=30s`, defaults to 30 s. New CLI flags: `--host`, `--user`, `--port`, `--timeout`; rejected outside `doctor preset --probe` context with focused error messages. Falls back to a declarative-metadata dump when only `--probe` is set (no `--host` / `--user`) so the docs surface stays useful even on operators' laptops without target hosts.
  - **TASK-21.6 (2026-05-25)** — asciinema 3.x cast recorded via `scripts/record-demo.sh`, pinned to `120×35` via `asciinema rec --window-size 120x35` so the cockpit Bento Ultra layout renders correctly regardless of the operator's parent terminal. Deterministic keystroke flow via `expect`: spawn → 5× Tab (cycle tiles) → Enter (project detail) → F8 (CI/CD) → 4× `j` (scroll steps) → Esc → 4× Tab (Live Log Stream) → Esc → `q` (quit). Wall-clock target 45-60 s; actual cast 13.6 KB JSON. GIF rendered via `agg` (1171×806 px, 98 KB, idle-time-limit 1.5 s) and committed under `docs/screenshots/sprint-21/demo.gif`. README EN embeds the GIF inline as `raw.githubusercontent.com/.../demo.gif`. The recording script now warns (instead of dying) when the parent terminal is smaller than `120×35` and renders the GIF inline when `agg` is on PATH.
- ⏭️ Carry-over: <task → Sprint 22 + reason>
- 📌 Decyzje: <ADR jeśli powstał>
- 🧠 Surprises: <co się okazało inne niż w docs>
- 📊 Metrics: cPanel UAPI test pass-rate, coverage delta, asciinema cast wall-clock.
