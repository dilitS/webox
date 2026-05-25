<p align="center">
  <img alt="Webox — terminal cockpit for shared hosting" src="https://raw.githubusercontent.com/dilitS/webox/main/docs/assets/webox-readme-hero.svg" width="860">
</p>

<p align="center">
  <strong>Your shared hosting, operated from one terminal.</strong>
</p>

<p align="center">
  <a href="https://github.com/dilitS/webox/actions/workflows/ci.yml"><img src="https://github.com/dilitS/webox/actions/workflows/ci.yml/badge.svg" alt="CI status"></a>
  <a href="https://github.com/dilitS/webox/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-Apache--2.0-7d56f4" alt="License: Apache-2.0"></a>
  <a href="https://github.com/dilitS/webox/blob/main/docs/ROADMAP.md"><img src="https://img.shields.io/badge/v0.1-RC-d846ef" alt="v0.1 release candidate"></a>
  <a href="https://pkg.go.dev/github.com/dilitS/webox"><img src="https://img.shields.io/badge/go-1.25%2B-00ADD8?logo=go" alt="Go 1.25+"></a>
  <a href="https://github.com/dilitS/webox/blob/main/CONTRIBUTING.md"><img src="https://img.shields.io/badge/PRs-welcome-04b575" alt="PRs welcome"></a>
</p>

<p align="center">
  <em>
    A 45-second scripted <code>--mock</code> cockpit tour ships with v0.1.0:<br/>
    <a href="https://github.com/dilitS/webox/blob/main/assets/demo/README.md"><code>assets/demo/</code></a> · <a href="https://github.com/dilitS/webox/blob/main/scripts/record-demo.sh"><code>scripts/record-demo.sh</code></a> · <a href="https://github.com/dilitS/webox/blob/main/assets/screenshots/README.md"><code>assets/screenshots/</code></a>
  </em>
</p>

---

## Why Webox

If you ship Node.js projects on **shared hosting** — small.pl today, **cPanel** and **DirectAdmin** next — you already know the loop: SSH in, run a panel CLI command, click around the web panel, copy a deploy key into GitHub, hand-write a workflow, hope the SSL renews on time. Webox folds every one of those steps into a single **terminal cockpit** with strict guardrails, transactional rollback, and zero remote telemetry. You stay on the hosting you already pay for; you stop tab-juggling between five surfaces; and when an incident hits you triage from a dashboard, not from a tail of grep'd logs.

This is the tool for the operator who is *one* person responsible for 5–30 small projects, not for the team that already left shared hosting behind.

---

## Try it in 30 seconds

```bash
git clone https://github.com/dilitS/webox.git
cd webox
make build
./bin/webox --mock
```

No SSH, no GitHub token, no config — `--mock` boots the full Bento Ultra cockpit with deterministic synthetic data (`shop-ease.io`, fake commits, fake build numbers) so you can poke around before connecting a real profile. Toggle the same mode with `WEBOX_MOCK=1 ./bin/webox`.

> **Requirements:** Go 1.25+, a POSIX shell. Tested on macOS (arm64/amd64) and Linux (amd64).

---

## What you can do today (v0.1)

One verified provider (**small.pl / Devil**), more coming. Everything below is implemented and covered by tests.

- **Project wizard** — 5 steps from empty subdomain to deployed app (subdomain → database → SSL → GitHub repo + workflow → first deploy). LIFO rollback cleans up on any failure.
- **Cockpit dashboard** — Bento Ultra layout (`120×35`) with live tiles: project list, detail panel, CI/CD pipeline, server topology, live log stream, header metrics.
- **One-key operations** — restart Node app, renew SSL, tail logs, open last GitHub Actions run, all from the dashboard.
- **Import existing projects** — read-only preview detects subdomain, SSL, Node version, deploy paths, and flags drift between what the panel says and what `webox` expects.
- **Stale detection** — surfaces projects whose config disagrees with reality on the server (subdomain removed, app stopped, SSL expired).
- **Secrets done right** — system keyring (Keychain / Secret Service / Credential Manager) with AES-256-GCM + Argon2id fallback for headless boxes. Zero plaintext secrets in `config.json`. Ever.
- **Defensive parsing** — every `devil`/`uapi` output is strict-regex parsed; no `eval`, no blind shell, no host key auto-accept.
- **`webox doctor`** — self-diagnostics + GitHub integration check, JSON output for scripting.

---

## Add your hosting in 4 hours

The most useful PR you can send is **a new provider adapter**. Webox is built around a single `HostingProvider` interface — every panel is a swappable implementation. No business logic, no TUI code, no security code needs to change.

```bash
./bin/webox provider new my_panel --preset cpanel-uapi
# scaffolds providers/my_panel/ — skeleton, parsers, tests, fixture README
# patches cmd/webox/providers.go blank-import block (sorted, idempotent)
# prints next-step walkthrough
```

Four presets ship today: `blank`, `cpanel-uapi`, `directadmin`, `cyberpanel`. Each preset seeds the generated package with vendor-specific scaffolding and a working `go build` straight out of the box. Then walk through [`docs/contributing/PROVIDER.md`](https://github.com/dilitS/webox/blob/main/docs/contributing/PROVIDER.md) — a 4-hour guide covering preset vs. adapter trade-offs, fixture sourcing + sanitisation, TDD parsers, `sshmock` integration, the capability probe, and the PR template.

> **Pair-review available.** Open an issue with the [`provider request` template](https://github.com/dilitS/webox/issues/new?template=provider_request.yml) or DM the maintainer before you start. You will not be left alone with the first PR.

---

## Architecture highlights

- **Provider Pattern.** Every hosting panel hides behind one interface (`providers.HostingProvider`). Adding cPanel, DirectAdmin, or CyberPanel means an adapter, not a refactor. See [ADR-0003](https://github.com/dilitS/webox/blob/main/docs/adr/0003-provider-pattern.md).
- **MVU TUI via Bubble Tea.** Pure `Update()` and `View()` functions, all I/O routed through `tea.Cmd`. Testable via `teatest` golden files. See [DESIGN §12](https://github.com/dilitS/webox/blob/main/docs/DESIGN.md).
- **SSH pool + SWR cache.** Multiplexed SSH connections with an inflight limiter; status data uses stale-while-revalidate semantics so the dashboard stays responsive under flaky networks.
- **Keyring-first secrets + AES-GCM fallback.** Cross-platform system keyring, AES-256-GCM (`crypto/rand` nonce, never `time.Now()`) + Argon2id KDF for headless boxes. Strict SSH host-key block on mismatch — no auto-accept. See [`docs/SECURITY.md`](https://github.com/dilitS/webox/blob/main/docs/SECURITY.md).
- **Atomic config + JSON Schema + ≥ 80 % coverage.** `config.json` writes go through `flock(2)` → tmp write → `fsync` → `rename` → `fsync(dir)`. Coverage gate is 80 % per package (70 % global floor) and enforced in CI alongside `golangci-lint v2`, `govulncheck`, and a 5 ms `bento` render perf budget.

Full architecture: [`docs/DESIGN.md`](https://github.com/dilitS/webox/blob/main/docs/DESIGN.md). All architecture decisions: [`docs/adr/`](https://github.com/dilitS/webox/tree/main/docs/adr).

---

## Status & roadmap

| Milestone | Target | Headline deliverable |
|---|---|---|
| **v0.1** | Q2/Q3 2026 | small.pl / Devil — one verified provider end-to-end. |
| **v0.2** | Q3/Q4 2026 | **cPanel adapter** + live log stream + GitHub Actions deploy monitor + full Command Palette. |
| **v0.3** | Q1/Q2 2027 | **DirectAdmin adapter** + non-interactive CLI flags + in-app updater. |
| **v1.0 GA** | 2027 | 3+ months stable, community-shipped provider, ≥ 80 % coverage, signed releases (cosign + SLSA). |

Live roadmap, scope rules, and version gates: [`docs/ROADMAP.md`](https://github.com/dilitS/webox/blob/main/docs/ROADMAP.md). Active sprint board: [`docs/sprints/`](https://github.com/dilitS/webox/tree/main/docs/sprints).

---

## Contributing

Welcome — pair-review is available and the first PR is the hardest. Three on-ramps:

- **5-minute setup, branching, PR checklist:** [`CONTRIBUTING.md`](https://github.com/dilitS/webox/blob/main/CONTRIBUTING.md).
- **Add a hosting-panel adapter (highest leverage):** [`docs/contributing/PROVIDER.md`](https://github.com/dilitS/webox/blob/main/docs/contributing/PROVIDER.md) + [`webox provider new`](https://github.com/dilitS/webox/blob/main/cmd/webox/provider_new.go).
- **Bug fix / small feature:** browse the [`good-first-issue` list](https://github.com/dilitS/webox/issues?q=is%3Aopen+label%3Agood-first-issue) — every entry has a difficulty badge, an estimated time, and a maintainer who will pair on the first PR.

What we will **not** merge: secrets in plaintext config / logs / errors, AES-GCM nonce from `time.Now()`, SSH host-key auto-accept, any form of telemetry / phone-home, hardcoded provider names in business logic, GitHub Actions pinned by tag. Full guardrail list: [`.cursor/rules/00-charter.mdc`](https://github.com/dilitS/webox/blob/main/.cursor/rules/00-charter.mdc) and [`AGENTS.md`](https://github.com/dilitS/webox/blob/main/AGENTS.md) §1.

Code of Conduct: [`CODE_OF_CONDUCT.md`](https://github.com/dilitS/webox/blob/main/CODE_OF_CONDUCT.md). Security disclosure: [`SECURITY.md`](https://github.com/dilitS/webox/blob/main/SECURITY.md) (private GitHub Security Advisories — never a public issue).

---

## License & credits

[**Apache License 2.0**](https://github.com/dilitS/webox/blob/main/LICENSE) — open source from day one, with an explicit patent grant so adapters for commercial hosting panels (cPanel LLC, DirectAdmin Inc., CyberPanel/OpenLiteSpeed) ship without legal ambiguity.

Built on top of work by:

- [**Charmbracelet**](https://charm.sh/) — [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lipgloss](https://github.com/charmbracelet/lipgloss), [Bubbles](https://github.com/charmbracelet/bubbles), and the entire Charm ecosystem that made a modern TUI possible.
- **The small.pl / Devil team** — for shipping a hosting platform that has a real CLI in 2026 and for being a generous launch partner for v0.1.
- **The Go SSH and keyring maintainers** — [`golang.org/x/crypto/ssh`](https://pkg.go.dev/golang.org/x/crypto/ssh), [`github.com/zalando/go-keyring`](https://github.com/zalando/go-keyring), and the long tail of well-licensed Go libraries cataloged in [`docs/dependencies.md`](https://github.com/dilitS/webox/blob/main/docs/dependencies.md).

---

<p align="center">
  <sub>
    If you have ever written a private shell script because your hosting workflow was too clumsy —<br/>
    <strong>this project is speaking directly to you.</strong>
  </sub>
</p>
