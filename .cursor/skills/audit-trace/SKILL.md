---
name: audit-trace
description: Trace any code change back to the authoritative docs (PRD, DESIGN, UX, SECURITY, ROADMAP, ADR, AUDIT) and flag drift before commit. Use when reviewing diffs, before merging large PRs, or when uncertain whether a change is in MVP scope.
---

# Audit Trace — Webox

## Purpose

Every line of code in Webox must trace back to a documented intent. This skill enforces that traceability by walking the agent through a structured check before commit.

## When to run

- Before every commit that touches `providers/`, `secrets/`, `wizard/`, `config/`, or `tui/states.go`.
- Before opening a PR.
- When reviewing someone else's PR.
- When in doubt whether a feature is MVP or stretch.

## Traceability matrix

For each changed file, fill in:

| File | Doc reference | Marker | Status |
|---|---|---|---|
| `providers/smallhost/parse_www_add.go` | `docs/providers/smallhost.md §2.1` | 🔵 MVP | ✅ in scope |
| `tui/views/topology.go` | `docs/UX.md §3.4` | 🔶 STRETCH | ❌ **scope violation** |
| `wizard/dag.go` | `docs/DESIGN.md §10.1` | 🔶 STRETCH v0.3+ | ❌ **scope violation** |
| `secrets/aesgcm.go` | `docs/SECURITY.md §4.2.1` | 🔵 MVP | ✅ in scope |

Any row with "scope violation" must be either:

1. Removed from the diff.
2. Justified by an updated ADR (which itself opens an MVP scope expansion).

## Checklist

```
- [ ] 1. Identify every file changed in this commit/PR.
- [ ] 2. For each, find the doc that authorizes the change.
- [ ] 3. Check the doc's scope marker (🔵 MVP, 🔶 STRETCH, etc.).
- [ ] 4. Verify scope matches the current milestone (v0.1 → only 🔵).
- [ ] 5. If a referenced anchor doesn't exist, fix it now.
- [ ] 6. Check AUDIT.md (including §8 folded IMP-* findings) for relevant findings.
- [ ] 7. Confirm CHANGELOG entry references the AUDIT/IMP item.
```

## Common scope violations

| Violation | What was added | Why it's wrong | Fix |
|---|---|---|---|
| Live log stream | `tui/views/logs_live.go` | Marked `STRETCH v0.2+` in `UX §4.3 Tab [4]` | Remove from MVP PR; add to v0.2 milestone. |
| DAG transactional engine | `wizard/dag.go` with topological sort | Marked `STRETCH v0.3+` per AUDIT §8 IMP-1 | MVP uses LIFO stack only. |
| Sound engine | `sound/player.go` | Marked `STRETCH (osobny RFC)` in `UX §12` + AUDIT C1 | Defer to separate RFC. |
| Env merger | `env/merger.go` | Marked `STRETCH v0.2+` in `UX §9.3` + `DESIGN §11.1` | Out of MVP. |
| Fast-chord bindings | `tui/keys/chord.go` | Marked `STRETCH v0.2+` in `UX §6.1` | Single keys only in MVP. |
| Topology map | `tui/views/topology.go` | Marked `STRETCH v0.2+` in `UX §3.4` + `DESIGN §18` | No topology in MVP dashboard. |
| Sinusoidal border pulsing | `tui/animation/pulse.go` | Marked `STRETCH v0.2+` in `DESIGN §16.1` | Latency-aware spinner (§16.2) is MVP, pulsing is not. |

## How to flag violation in PR

If you find a scope violation **during** writing code:

```
🛑 Found scope violation:
  File: tui/views/topology.go
  Doc: docs/UX.md §3.4 (🔶 STRETCH v0.2+)
  Action: removing from this PR. Will reopen as separate v0.2 PR.
```

If you find it in someone else's PR during review:

```
**Scope check — please address before merge:**

`tui/views/topology.go` implements the Live Service Topology Map, which is
marked as `🔶 STRETCH (v0.2+)` in [`docs/UX.md §3.4`](docs/UX.md#34-...) and
`docs/DESIGN.md §18`. Either:

1. Move this work to a v0.2 milestone PR, or
2. Open an ADR proposing to promote this feature to MVP, with reviewer
   sign-off, before re-submitting.
```

## Anchor sanity check

If your changed file references doc anchors, verify they exist:

```bash
# Quick check: grep for the anchor in the target doc
rg "## 4\.3" docs/UX.md
rg "### 11\.1" docs/DESIGN.md
```

If anchor doesn't exist, **fix the doc first** (add the section) before the code references it.

## AUDIT-derived finding tracking

When a commit addresses an AUDIT finding:

```
fix(security): correct AES-GCM nonce generation to use crypto/rand

Per AUDIT §8 IMP-2. Adds explicit panic-on-CSPRNG-failure and
deduplication test ensuring two consecutive writes produce different
nonces.

Refs: IMP-2
```

In CHANGELOG:

```markdown
- `secrets/aesgcm.go` — generate AES-GCM nonce via `crypto/rand.Read`,
  panic on CSPRNG failure (IMP-2).
```

## Done criteria

- [ ] Every changed file mapped to a doc reference.
- [ ] No scope violations remain.
- [ ] All referenced doc anchors exist.
- [ ] CHANGELOG references relevant AUDIT/IMP findings.
- [ ] Commit message links to docs/AUDIT.md entry by ID.
