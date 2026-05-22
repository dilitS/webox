# Webox

> Shared hosting, operated like a real developer tool.

Webox is a terminal-first operator tool for developers who run projects on shared hosting and are tired of bouncing between a hosting panel, SSH, GitHub, SSL settings, log files, and handwritten shell scripts.

The project starts with `small.pl` / Devil as the MVP target, but the architecture is intentionally designed for future providers such as cPanel, DirectAdmin, and CyberPanel.

## Short English Summary

Webox is a Go-based TUI for operating shared-hosting projects from one place. It aims to compress the full workflow of creating, deploying, restarting, inspecting, and maintaining apps on shared hosting into a single terminal UI, with GitHub Actions for code deployment and provider adapters for different hosting panels.

## Status

| Area | Current state |
|---|---|
| Product stage | `Pre-MVP` / docs-first design phase |
| Primary MVP provider | `small.pl` / Devil |
| UI model | TUI (`Bubble Tea` + `Lipgloss`) |
| Language strategy | English-first UI, Polish optional |
| Deployment model | GitHub Actions for code deploys |
| Security posture | Keyring-first secrets, SSH host-key verification, zero remote telemetry |
| Repository maturity | Architecture and documentation defined before implementation |

## What Problem Webox Solves

Shared hosting is still a very rational choice for small commercial websites, client projects, landing pages, and lightweight Node apps. It is cheap, good enough, and already paid for.

What is terrible is the developer experience.

A typical workflow for a new project on shared hosting often means:

- opening the hosting panel to create a subdomain,
- opening SSH to inspect directories or restart the app,
- generating SSL in another place,
- wiring GitHub Actions manually,
- copying secrets into `.env`,
- checking logs through another command or another tab,
- repeating the same choreography for every new project.

Webox exists to replace that fragmented workflow with one coherent operator cockpit inside the terminal.

## What Webox Is

- A terminal operator cockpit for shared-hosting projects.
- A productivity layer over hosting panels, SSH, and GitHub.
- A docs-driven Go application designed around a provider abstraction.
- A serious answer to "I already wrote shell scripts for this, but now I need something maintainable."

## What Webox Is Not

- Not a replacement for your hosting provider's billing/support panel.
- Not a replacement for GitHub.
- Not a VPS, Docker, or Kubernetes platform.
- Not a general-purpose SSH client.
- Not a website builder.
- Not a remote telemetry product.

## Why This Project Is Interesting

There are many tools for servers you fully control.
There are very few good tools for environments you do not fully control but still use every day.

That is exactly why Webox has a sharp niche:

- developers with many small projects,
- freelancers and tiny agencies,
- shared-hosting environments where Docker is unavailable or unnecessary,
- workflows where speed, repeatability, and safety matter more than infrastructure theater.

This is not "another panel." It is a missing developer tool for a neglected but very real segment.

## MVP Scope

`v0.1` is intentionally narrow.

The MVP focuses on one provider and one real painkiller workflow:

- `small.pl` / Devil only,
- first-run setup,
- provider profile management,
- project creation wizard,
- dashboard with status overview,
- restart, logs, SSL actions,
- import of existing projects,
- rollback of partially failed setup flows,
- local config + secure secret handling.

The MVP does **not** try to win every hosting platform at once.
It tries to be genuinely useful for one real one first.

## Core Product Principles

1. **Narrow MVP, wide architecture.**  
   The product surface stays small in `v0.1`, but the internal design is ready for more providers from day one.

2. **Operator speed beats feature count.**  
   One polished restart/import/dashboard flow is worth more than ten half-designed menus.

3. **Terminal-native, not terminal-themed.**  
   This is meant to feel like a proper TUI tool, not a web app squeezed into a terminal.

4. **Security is part of the product, not an appendix.**  
   Host-key verification, secret storage, token scopes, and logging discipline are first-class concerns.

5. **Docs before code chaos.**  
   The repository is being shaped as a serious product and architecture effort, not a pile of improvised scripts.

## Example User Journey

The intended "happy path" looks like this:

1. Configure one hosting profile in `webox`.
2. Create a new project through a wizard.
3. Let Webox set up hosting resources and GitHub-side deployment wiring.
4. Land on a dashboard that shows project health, SSL state, Node version, and last deploy state.
5. Use the same interface later for restart, logs, import, and maintenance.

The important detail is not one specific command.  
The important detail is removing context switching.

## TUI Preview

This is an illustrative direction, not a finished screenshot:

```text
╭─ Webox ──────────────────────────────────────────────────────────╮
│ [Profile: main · small.pl] [/]                                  │
│                                                                 │
│  ┌─ Projects ───────────────┐ ┌─ sui.biuromody.smallhost.pl ──┐ │
│  │  ▶ sui.biuromody     🟢  │ │  Status  │ ONLINE              │ │
│  │    makspomoc         🟢  │ │  Node    │ v24.15.0            │ │
│  │    legacy          STALE │ │  SSL     │ 27 days left        │ │
│  │  [ + New project ]       │ │  Deploy  │ 2h ago ✓            │ │
│  └──────────────────────────┘ │  [r] Restart  [v] Logs         │ │
│                               └─────────────────────────────────┘ │
│                                                                 │
│  q:quit  ↑↓:navigate  →:details  n:new  /:command               │
╰─────────────────────────────────────────────────────────────────╯
```

For the detailed UX specification, see [`docs/UX.md`](./docs/UX.md).

## Architecture At A Glance

Webox is designed as a small Go monolith with clear boundaries:

- `tui/` for the Bubble Tea state machine and rendering,
- `providers/` for hosting-panel adapters behind one contract,
- `ssh/` for controlled SSH/SFTP operations,
- `services/` for GitHub and network integrations,
- `config/` and `secrets/` for local state and secret management.

The core architectural decision is simple:

- **product scope** stays narrow,
- **provider abstraction** stays clean,
- **unsafe operational shortcuts** are rejected unless they are explicitly modeled.

The design docs go much deeper here:

- [`docs/DESIGN.md`](./docs/DESIGN.md)
- [`docs/adr/`](./docs/adr/)

## Security Posture

Webox touches real infrastructure and real credentials, so the security model is explicit:

- system keyring as the default secret backend,
- encrypted-file fallback for headless environments,
- dedicated `known_hosts` file for Webox-managed SSH trust,
- strict SSH host-key mismatch handling,
- fine-grained GitHub token expectations,
- no remote telemetry by default,
- defensive parsing of hosting-panel output,
- release supply-chain hardening planned around signed artifacts.

Read the full security model in [`docs/SECURITY.md`](./docs/SECURITY.md).

## Documentation Map

Deep product and architecture docs are currently written in Polish. The public README stays accessible on purpose, while the detailed internal spec lives in `docs/`.

The repository is documentation-heavy on purpose. Start here depending on what you need:

- [`docs/PRD.md`](./docs/PRD.md)  
  Product scope, personas, priorities, non-goals, and success criteria.

- [`docs/DESIGN.md`](./docs/DESIGN.md)  
  Architecture, contracts, config model, rollback, state machine, GitHub integration.

- [`docs/UX.md`](./docs/UX.md)  
  Design system, layouts, key bindings, command palette, user flows.

- [`docs/SECURITY.md`](./docs/SECURITY.md)  
  Threat model, secrets, SSH trust, token handling, `.env` strategy.

- [`docs/TESTING.md`](./docs/TESTING.md)  
  Unit/integration/TUI strategy, fixtures, CI, release checklist.

- [`docs/ROADMAP.md`](./docs/ROADMAP.md)  
  MVP boundary, later phases, GA criteria.

- [`docs/CONTRIBUTING.md`](./docs/CONTRIBUTING.md)  
  Contributor workflow, provider authoring, translation process, review rules.

- [`docs/providers/smallhost.md`](./docs/providers/smallhost.md)  
  The reference provider spec for the MVP target.

- [`docs/README.md`](./docs/README.md)  
  Internal documentation entry point and reading paths.

## Roadmap Snapshot

### `v0.1`

- One production-quality provider target: `small.pl`
- Operator dashboard
- Project creation flow
- Import existing projects
- Rollback-safe setup logic
- Secure local secret handling

### `v0.2`

- Second provider
- Fuller Command Palette
- Better deployment visibility
- More complete environment-management flows

### `v0.3+`

- More providers
- More automation surfaces
- More polished update/distribution story

The detailed versioned plan lives in [`docs/ROADMAP.md`](./docs/ROADMAP.md).

## Repository Philosophy

This repository is intentionally not pretending the code is already finished.

Instead, it does something rarer and better:

- it defines the product honestly,
- narrows the MVP responsibly,
- documents trade-offs,
- records architectural decisions,
- and leaves less room for fake certainty.

That makes the future implementation slower at the start, but much faster once it begins in earnest.

## Contributing

Contributions are welcome, but the bar should stay high.

Especially valuable contributions will be:

- provider research backed by real documentation and fixture captures,
- UX review from people who actually live in the terminal,
- security review of secret handling and SSH trust flows,
- implementation PRs that preserve the current design discipline,
- careful criticism that reduces scope creep instead of adding noise.

Start with [`docs/CONTRIBUTING.md`](./docs/CONTRIBUTING.md).

## FAQ

### Why a TUI instead of a regular CLI?

Because the core value is not a single command.  
It is a persistent dashboard and low-friction switching between operational actions.

### Why shared hosting at all?

Because many real projects do not need a VPS, Kubernetes, or platform engineering overhead.  
They need a faster workflow on top of infrastructure that already exists.

### Why GitHub Actions for deployment?

Because it creates a repeatable deployment path and avoids turning each developer laptop into a snowflake deploy machine.

### Why start with only one provider?

Because multi-provider support is where many good ideas go to die.  
One real adapter done properly is more valuable than four speculative ones.

## Current Limitation

There is no production-ready binary yet.  
At this stage, the repository is primarily a high-quality design and documentation base preparing the implementation.

That is deliberate.

## License

[`MIT`](./LICENSE)

## Final Note

Webox is not trying to look bigger than it is.
It is trying to be sharper than most early-stage tools are.

If you have ever written a private shell script because the official hosting workflow was too clumsy, this project is speaking directly to you.
