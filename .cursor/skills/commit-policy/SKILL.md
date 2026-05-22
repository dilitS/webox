---
name: commit-policy
description: Compose conventional-commit messages and stage commits for Webox, including CHANGELOG entries. Use when the user asks to commit, when wrapping up a task, or when reviewing pre-commit state.
---

# Commit Policy — Webox

## Format

```
<type>(<scope>): <imperative summary ≤72 chars>

<body — why, not what, hard-wrapped at 80 cols>

<footer — Refs: <issue/audit-id>, BREAKING CHANGE: ...>
```

## Types

| Type | When |
|---|---|
| `feat` | New end-user-visible behavior. |
| `fix` | Bug fix (user observable). |
| `refactor` | Internal restructure, no behavior change. |
| `perf` | Optimization. |
| `test` | Test addition/edit only. |
| `docs` | Only `docs/` or `README` / `AGENTS.md` / `CHANGELOG.md`. |
| `chore` | Tooling, deps, config (`Makefile`, `.golangci.yml`). |
| `ci` | CI workflow changes. |
| `build` | Build system (`GoReleaser`, embed configs). |
| `revert` | Revert a prior commit. |

## Scopes

Top-level packages: `providers`, `tui`, `ssh`, `config`, `secrets`, `status`, `wizard`, `services`, `i18n`, `assets`, `cmd`.

Feature areas: `audit`, `scope`, `release`.

Use the **most specific** scope. `feat(providers/smallhost): ...` is better than `feat(providers): ...` when the change is provider-specific.

## Composition workflow

```
- [ ] 1. Run `make ci` locally → must pass.
- [ ] 2. Stage exactly the files for this logical change (no junk).
- [ ] 3. Draft summary (≤72 chars, imperative: "add", "fix", "remove").
- [ ] 4. Draft body explaining "why" (link AUDIT finding or issue when relevant).
- [ ] 5. Update CHANGELOG.md [Unreleased] if user-visible.
- [ ] 6. Stage CHANGELOG.md alongside the change.
- [ ] 7. Commit with HEREDOC for clean formatting.
- [ ] 8. Verify with `git log -1 --stat`.
```

### HEREDOC commit (recommended)

```bash
git commit -m "$(cat <<'EOF'
feat(providers/smallhost): parse devil www add output

Implements parser for `devil www add <domain> nodejs <version>` covering
three known outputs: success, exists, invalid node version. Strips ANSI
escapes, validates output size ≤ 1 MB, uses named regex groups, returns
sentinel errors from providers/smallhost/errors.go. Fixtures sanitized
for testuser@s1.small.pl on 2026-04-12.

Refs: docs/providers/smallhost.md §2.1, AUDIT P0
EOF
)"
```

## Good examples

```
feat(providers/smallhost): parse devil www add output

Implements parser handling success, exists, invalid_node cases with
strict ANSI stripping and size validation. 4 fixtures, table-driven
tests, golden file regen via make test-tui.

Refs: docs/providers/smallhost.md §2.1
```

```
fix(secrets): use crypto/rand for AES-GCM nonce

Per AUDIT §8 IMP-2. Replaces time-based nonce with
crypto/rand.Read(12 bytes). Adds panic on CSPRNG failure and unit
test verifying nonce uniqueness across two consecutive writes.

Refs: IMP-2, SECURITY §4.2.1
```

```
docs(scope): mark sound engine and topology map as STRETCH v0.2+

Aligns DESIGN §17, §18 and UX §3.4, §12 with ROADMAP §3.3 which
excludes both from MVP. Adds explicit STRETCH banners and cross-refs
to AUDIT A6.

Refs: AUDIT A6
```

```
chore(ci): pin golangci-lint to v2.x and update lint name mapping

Replaces v1 lint names (gas, gomnd, goerr113, logrlint) with v2
equivalents (gosec, mnd, err113, loggercheck). Bumps gocyclo max
from 15 to 20 per IMP-19 (provider methods naturally hit 16-20 with
explicit error handling).

Refs: AUDIT B3, IMP-19
```

## Bad examples (rewrite before committing)

```
❌ "wip"
❌ "fix bug"
❌ ":sparkles: feature add"
❌ "update files"
❌ "merge branch main"
❌ "make tests pass"
❌ "address review comments"  # what review? what comments? link them.
❌ "thinking..."
```

## CHANGELOG entry format

Each behavior-affecting commit adds **one entry** to `CHANGELOG.md [Unreleased]`:

```markdown
### Added
- `providers/smallhost/parse_www_add.go` — parser for `devil www add`
  with strict ANSI stripping and 4 golden fixtures. Refs:
  `docs/providers/smallhost.md §2.1`.

### Changed
- `secrets/aesgcm.go` — generate AES-GCM nonce via `crypto/rand.Read`;
  panic on CSPRNG failure (IMP-2).

### Security
- Hardened AES-GCM nonce policy: deterministic sources (time, counter,
  hash) explicitly banned. New unit test enforces nonce uniqueness.
```

## When to amend vs new commit

| Situation | Action |
|---|---|
| Pre-commit hook auto-modifies files (gofmt) | `git commit --amend --no-edit` (single auto-fix only) |
| You forgot to add CHANGELOG entry | `git commit --amend` to include it |
| User adds new requirement after commit | **New commit** — preserves intent boundary |
| Commit failed CI | **New commit** with fix (amend implies "fix and try again" but loses CI history) |
| You already pushed to remote | **Never amend** without explicit user consent |

## Squash policy at PR merge

| Strategy | When |
|---|---|
| **Squash and merge** | Default for feature branches (1 logical change → 1 commit on main). |
| **Rebase and merge** | When 2–3 commits are individually meaningful and clean. |
| **Merge commit** | Never on main. |

Each commit on `main` after merge should pass `make ci`. No `wip` or `fix typo` commits on main — squash them.

## Pre-commit checklist

```
- [ ] make ci passes
- [ ] Conventional Commits format
- [ ] Subject ≤72 chars, imperative mood
- [ ] Body explains why
- [ ] CHANGELOG [Unreleased] updated (if behavior change)
- [ ] AUDIT/IMP finding referenced (if applicable)
- [ ] No secrets in any committed file (auto-checked by hook)
- [ ] No scope violations (run audit-trace skill if uncertain)
```
