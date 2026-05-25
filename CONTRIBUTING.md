# Contributing to Webox

Thanks for stopping by — Webox grows because operators and developers like you turn frustration into adapters, fixtures, and docs.

This file is the **5-minute on-ramp**. The deep-dive workflow (PL, with retrospectives, audit links, sprint cadence) lives at [`docs/CONTRIBUTING.md`](./docs/CONTRIBUTING.md); both stay in sync.

> By contributing you agree to the [Apache License 2.0](./LICENSE) and our [Code of Conduct](./CODE_OF_CONDUCT.md).

---

## 1. Setup (under 5 minutes)

```bash
git clone https://github.com/dilitS/webox.git
cd webox
make bootstrap   # installs dev tools + git hooks
make ci          # full local CI bundle (mirrors GitHub Actions 1:1)
```

If `make ci` is green you are ready. Most contributors live in two extra commands:

```bash
make dev PKG=./providers/...   # TDD watch loop
make mock                      # offline cockpit smoke test (no SSH/HTTP/GitHub)
```

Requirements: **Go 1.24+**, `git`, a POSIX shell, and a panel account if you plan to capture real fixtures. Everything else is pinned in `tools/go.mod` and auto-fetched.

---

## 2. Branching + commits

- **Branch name:** `feat/s<NN>-<task>-<slug>` (e.g. `feat/s15-04-provider-new-generator`). Use the matching sprint short-id from [`docs/sprints/`](./docs/sprints/) so reviewers find context in one click.
- **Commit messages:** **Conventional Commits 1.0.0** without gitmoji. Subject ≤ 72 chars, lowercase first letter, no trailing period. Body explains the *why* and links the spec.
- **Types we use:** `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`, `security`.

```text
feat(providers): parse devil www add output for smallhost

Add parser handling success, exists, and invalid-node-version cases
with strict ANSI stripping and size validation. Fixtures: www_add_ok,
www_add_exists, www_add_invalid_node, www_add_malicious_ansi.

Refs: docs/providers/smallhost.md §2.1
```

A pre-commit hook (`make setup-hooks`) auto-runs `gofumpt`, `goimports`, fast lint, and a secret-tripwire; the commit-msg hook validates Conventional Commits.

---

## 3. Pull-request checklist

Open a **draft PR early** — visibility beats perfection. Then before flipping to *Ready for review*, run through [`.github/pull_request_template.md`](./.github/pull_request_template.md). The fast version:

- [ ] `make ci` is green locally (`lint`, `vet`, `test -race`, `vulncheck`, `cover-check`, `build`).
- [ ] If you touched `tui/bento`, `make bench-check` is green too (perf gate).
- [ ] Coverage ≥ 80 % for any package you touched (70 % global floor).
- [ ] No `TODO` / `FIXME` without a linked issue.
- [ ] No `//nolint` without a one-line justification.
- [ ] [`CHANGELOG.md`](./CHANGELOG.md) `[Unreleased]` has a `1-2-sentence why` entry under the right category.
- [ ] Docs updated if the change alters a contract (PRD, DESIGN, SECURITY, ADR).

---

## 4. Three ways to contribute

### 4.1 Add a hosting-panel adapter ✦ the highest-leverage path

Webox's value scales with the number of panels it speaks. The fastest way in:

```bash
make build
./bin/webox provider new <name> --preset cpanel-uapi   # or directadmin / cyberpanel / blank
```

The command scaffolds `providers/<name>/` (skeleton, parsers, tests, fixture README) and patches the production blank-import block. Then follow the **4-hour walkthrough**: [`docs/contributing/PROVIDER.md`](./docs/contributing/PROVIDER.md). It covers preset vs. adapter decisions, fixture sourcing + sanitisation, TDD parsers, `sshmock` integration, the capability probe, and the PR template with a pair-review request line.

Difficulty hint: 🟢 cPanel UAPI (mainstream, well-documented) · 🟡 DirectAdmin (mixed API) · 🔴 CyberPanel (root concerns; expect a security review).

**Pair-review available** — open an issue or DM the maintainer before you start; you will not be left alone with the first PR.

### 4.2 Add a translation

Webox ships with `i18n/<view>.<lang>.json` snapshots. Picking up a language:

1. Copy `i18n/<view>.en.json` to `i18n/<view>.<lang>.json`.
2. Translate values only; **never** rename keys.
3. Run `make i18n-check` — the script fails if your file has missing or surplus keys.
4. PR with `docs(i18n)` scope.

Migration plan + key catalog: [`docs/adr/0006-jezyk-interfejsu-en-domyslny.md`](./docs/adr/0006-jezyk-interfejsu-en-domyslny.md) (PL) and [`docs/conventions.md` §i18n](./docs/conventions.md).

### 4.3 Bug fix or small feature

Browse the [good-first-issue list](https://github.com/dilitS/webox/issues?q=is%3Aopen+label%3Agood-first-issue). Every entry has a difficulty badge, an estimated time, and a maintainer who will pair on the first PR. If your change is bigger than a typo, drop a comment first — we will help scope it so review is fast.

---

## 5. What we will NOT merge

Same rules apply to everyone, including the maintainer:

- **Secrets in `config.json`, logs, error messages, stack traces.** Use the keyring or `secrets.enc`.
- **AES-GCM nonce from anything but `crypto/rand.Read(12 bytes)`** — no `time.Now()`, no counters.
- **Auto-accepting an SSH host key mismatch** — strict block + explicit operator confirmation.
- **Telemetry / phone-home of any kind.** `--debug-trace` writes locally to `~/.cache/webox/trace.jsonl` and that is the entire surface.
- **Hardcoded provider names in business logic.** Everything routes through `providers.HostingProvider`.
- **`Update()` / `View()` doing I/O.** All side effects flow through `tea.Cmd`.
- **GitHub Actions pinned by tag (`@v4`).** Always full 40-char SHA.

Full guardrail list: [`.cursor/rules/00-charter.mdc`](./.cursor/rules/00-charter.mdc) and [`AGENTS.md`](./AGENTS.md) §1.

---

## 6. Maintainer SLA

We are solo-maintained today — be patient and we will be responsive:

| Action | Target | Notes |
|---|---|---|
| Acknowledge new issue | ≤ 3 business days | Label + scope question if needed. |
| First review on draft PR | ≤ 5 business days | Pair-review available on request. |
| Re-review after changes | ≤ 3 business days | Faster when the PR is small. |
| Security report (`SECURITY.md`) | ≤ 24 hours | Coordinated disclosure, see policy. |

If we miss a window, ping the PR/issue thread — it is almost always a notification miss, not a refusal.

---

## 7. Pointers

| Question | Doc |
|---|---|
| Coding conventions, error handling, generics, logging | [`docs/conventions.md`](./docs/conventions.md) |
| Top 15 anti-patterns to avoid | [`docs/gotchas.md`](./docs/gotchas.md) |
| Library catalog + supply-chain policy | [`docs/dependencies.md`](./docs/dependencies.md) |
| Architecture decisions (ADRs) | [`docs/adr/`](./docs/adr/) |
| Roadmap (v0.1 → v0.2 → v1.0) | [`docs/ROADMAP.md`](./docs/ROADMAP.md) |
| Sprint plans | [`docs/sprints/`](./docs/sprints/) |
| Provider walkthrough (4-hour) | [`docs/contributing/PROVIDER.md`](./docs/contributing/PROVIDER.md) |
| Detailed PL contributor handbook | [`docs/CONTRIBUTING.md`](./docs/CONTRIBUTING.md) |

Welcome aboard.
