<p align="center">
  <img alt="Webox — terminal cockpit for shared hosting" src="https://raw.githubusercontent.com/dilitS/webox/main/docs/assets/webox-readme-hero.svg" width="860">
</p>

# Webox

**Your shared hosting, operated from one terminal.** Webox folds SSH + panel CLI + SSL + GitHub + CI/CD into a single cockpit with transactional rollback, strict guardrails, and zero remote telemetry — for the operator running 5–30 small projects on shared hosting (small.pl today, cPanel and DirectAdmin next), not for teams that already left it behind.

<p align="center">
  <a href="https://github.com/dilitS/webox/actions/workflows/ci.yml"><img src="https://github.com/dilitS/webox/actions/workflows/ci.yml/badge.svg" alt="CI status"></a>
  <a href="https://github.com/dilitS/webox/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-Apache--2.0-7d56f4" alt="License: Apache-2.0"></a>
  <a href="https://github.com/dilitS/webox/releases/tag/v0.1.0-rc2"><img src="https://img.shields.io/badge/v0.1-RC2-d846ef" alt="v0.1.0-rc2"></a>
  <a href="https://pkg.go.dev/github.com/dilitS/webox"><img src="https://img.shields.io/badge/go-1.25%2B-00ADD8?logo=go" alt="Go 1.25+"></a>
</p>

## Try it in 30 seconds

```bash
git clone https://github.com/dilitS/webox.git && cd webox && make build
./bin/webox --mock   # synthetic data; no SSH, no token, no config
```
> Requirements: **Go 1.25+**, POSIX shell. Tested on macOS arm64/amd64 and Linux amd64. Static gallery: [`docs/screenshots/sprint-20/`](./docs/screenshots/sprint-20). 45-second demo: [`docs/screenshots/sprint-21/demo.cast`](./docs/screenshots/sprint-21/demo.cast) (asciinema 3.x) — rendered GIF below; recorded via [`scripts/record-demo.sh`](./scripts/record-demo.sh).

<p align="center"><img alt="Webox v0.1 — 45-second mock cockpit tour" src="https://raw.githubusercontent.com/dilitS/webox/main/docs/screenshots/sprint-21/demo.gif" width="860"></p>

## What works today (v0.1)

One verified provider (**small.pl / Devil**), every line test-covered:

- **Project wizard** — subdomain → database → SSL → GitHub repo + workflow → first deploy, with LIFO rollback.
- **Cockpit dashboard** — Bento Ultra (`120×35`) live tiles, Provider Catalog (`p`), Help overlay (`?`), Project Detail tabs (Overview / Env Diff / Database / Logs), layout-aware mouse drill / back.
- **One-key ops + import preview** — restart, SSL renew, tail logs, open last GHA run; import surfaces drift between `config.json` and the live panel.
- **Secrets done right** — system keyring + AES-256-GCM with `crypto/rand` nonce (never `time.Now()`) and Argon2id KDF for headless boxes. Strict SSH host-key block, never auto-accept.
- **`webox doctor`** — self-diagnostics + GitHub check + embedded provider catalog browser, JSON for scripting.

## Add a hosting panel in 4 hours

The highest-leverage PR is **a new adapter** behind `providers.HostingProvider`. No business logic, TUI, or security code changes.

```bash
./bin/webox provider new my_panel --preset=cpanel-uapi   # scaffolds skeleton + fixtures + tests
```

Presets: `blank`, `cpanel-uapi`, `directadmin`, `cyberpanel`. Guide: [`docs/contributing/PROVIDER.md`](./docs/contributing/PROVIDER.md). Pair-review available — open a [`provider request` issue](https://github.com/dilitS/webox/issues/new?template=provider_request.yml).

## Roadmap

| Milestone | Target | Headline |
|---|---|---|
| **v0.1** | Q2/Q3 2026 | small.pl / Devil — one verified provider end-to-end. |
| **v0.2** | Q3/Q4 2026 | **cPanel adapter** + live log stream + GHA deploy monitor + Command Palette. |
| **v0.3** | Q1/Q2 2027 | **DirectAdmin adapter** + non-interactive CLI + in-app updater. |
| **v1.0 GA** | 2027 | 3+ months stable, community-shipped provider, ≥ 80 % coverage, cosign + SLSA. |
> Plan: [`docs/ROADMAP.md`](./docs/ROADMAP.md) · architecture: [`docs/DESIGN.md`](./docs/DESIGN.md) · ADRs: [`docs/adr/`](./docs/adr) · security: [`docs/SECURITY.md`](./docs/SECURITY.md).

## Contributing, security, license

Pair-review available; first PR is the hardest. Setup, branching, PR checklist: [`CONTRIBUTING.md`](./CONTRIBUTING.md). Guardrails (**never** merged): plaintext secrets, AES-GCM nonce from `time.Now()`, SSH host-key auto-accept, telemetry, hardcoded provider names in business logic, Actions pinned by tag — full list in [`.cursor/rules/00-charter.mdc`](./.cursor/rules/00-charter.mdc) and [`AGENTS.md`](./AGENTS.md) §1. Security disclosure: private GitHub Security Advisories per [`SECURITY.md`](./SECURITY.md), never a public issue.

[**Apache 2.0**](./LICENSE) — explicit patent grant so adapters for commercial panels (cPanel LLC, DirectAdmin Inc., CyberPanel / OpenLiteSpeed) ship without legal ambiguity. Built on [Charmbracelet's](https://charm.sh/) Bubble Tea + Lipgloss, [`golang.org/x/crypto/ssh`](https://pkg.go.dev/golang.org/x/crypto/ssh), [`zalando/go-keyring`](https://github.com/zalando/go-keyring), and the small.pl / Devil team as a generous launch partner.
