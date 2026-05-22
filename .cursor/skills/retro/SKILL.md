---
name: retro
description: Run a short retrospective after completing a task, sprint, or PR — capture what worked, what didn't, and what changes to apply going forward. Use after completing any non-trivial unit of work (parser, provider method, security fix, doc rewrite) or at the end of a working session.
---

# Retrospective — Webox

## Cadence

| Trigger | When to run |
|---|---|
| **Task retro** | After every non-trivial task completion (parser, security fix, full provider method). |
| **Sprint retro** | After every group of 5–10 related commits or every PR merge. |
| **Audit retro** | After completing all P0/P1 items from AUDIT or a second-pass audit sweep. |
| **Release retro** | After every release (v0.1.0, v0.1.1, etc.). |

## Template

Copy this into a markdown comment, a PR description, or a working notes file:

```markdown
## Retro: <task / sprint / release name>

**Date:** YYYY-MM-DD
**Scope:** <what was done — link to commits / PR / issues>

### What worked well

- ...

### What didn't work / friction points

- ...

### Surprises / unknowns discovered

- ...

### Changes to apply going forward

- **AGENTS.md:** ...
- **`.cursor/rules/*`:** ...
- **`.cursor/skills/*`:** ...
- **`.cursor/hooks/*`:** ...
- **Process:** ...
- **Tooling:** ...

### Open questions to resolve

- ...
```

## Examples

### Good retro entry

```markdown
## Retro: smallhost parser implementation

**Date:** 2026-05-25
**Scope:** Implemented parse_www_add, parse_mysql_add, parse_ssl_add with TDD.
        Commits: a1b2c3d..d4e5f6g (3 PRs).

### What worked well

- TDD loop was fast: ~12 minutes per parser from fixture to green.
- Sanitized fixtures (`testuser`, `203.0.113.10`) caught one accidental
  leak during PR review.
- `make ci` locally matched CI exactly — no surprises on push.

### What didn't work

- I initially mocked `devil` output via stub responses instead of
  fixturing real bytes. Caught by review: stubs missed `\r\n` line
  endings that real `devil` returns.
- Wrote `parse_*` and `parse_*_test.go` in the same commit — should
  have been Red commit first, Green commit second for cleaner history.

### Surprises

- `devil mysql add` output includes a blank line between "Database
  created" and "Username:" — original regex didn't tolerate that.
- `golangci-lint v2` flagged `mnd` (magic number detector) on `12` for
  GCM nonce size — added `//nolint:mnd // GCM nonce per RFC 5116`.

### Changes to apply

- **AGENTS.md §7**: add row about `\r\n` in panel output parsing.
- **`.cursor/skills/tdd-loop`**: separate "Red" and "Green" into distinct
  commits when feasible (currently bundled).
- **Process**: always capture **real** bytes for fixtures, never stub.

### Open questions

- Does `devil ssl www add` ever return mixed `\n` and `\r\n` in the same
  output? Need to capture more samples on retry attempts.
```

### Bad retro entry (avoid)

```markdown
## Retro

It went well. Nothing to add.
```

(No specifics, no actionable changes, no surprises.)

## Where retros live

- **Task retros**: in PR description under `## Retrospective` section.
- **Sprint retros**: in `docs/retros/sprint-<NN>.md` (create if missing).
- **Audit retros**: appended to `docs/AUDIT.md` under `Status changes` section.
- **Release retros**: in GH Release notes + `docs/retros/release-<vX.Y.Z>.md`.

## What to do with the output

Each retro must produce **at least one actionable change**. If a retro generates no changes, you weren't honest about friction or you're not running it often enough.

| Actionable type | Where to apply |
|---|---|
| Workflow change | Update `AGENTS.md §4` or relevant skill in `.cursor/skills/`. |
| Coding convention | Add row to `.cursor/rules/10-go-style.mdc` or open new rule. |
| New pitfall discovered | Add to `AGENTS.md §7.1` (top gotchas). |
| Doc gap discovered | Open issue or PR fixing the doc. |
| Tooling improvement | New Make target, hook, or CI gate. |

## Anti-patterns

- **Retro as performance review**: focus on *what*, not *who*.
- **Vague retros**: "communication was poor" — what specifically? With whom? When?
- **No follow-through**: every retro must trigger at least one change.
- **Skipping retros when things go well**: success retros reveal what to keep doing.

## Done criteria

- [ ] Template filled with specifics (no vague entries).
- [ ] At least one actionable change identified.
- [ ] Action applied to the right artifact (AGENTS.md / skill / rule / hook / doc).
- [ ] Retro committed/linked from the PR or sprint marker.
