# Retro: pre-implementation audit & environment bootstrap

**Date:** 2026-05-22
**Scope:**
Complete pre-implementation audit of Webox documentation, fix all P0/P1
findings, and bootstrap the AI-agent implementation environment
(`AGENTS.md`, `.cursor/rules`, `.cursor/skills`, `.cursor/hooks`,
`Makefile`, `.editorconfig`, `CHANGELOG.md`).

**Commits (chronological):**

1. `c7c5dff docs: restructure documentation and initialize project structure`
2. `a41afd7 docs(audit): publish pre-implementation audit with 39 findings`
3. `f810fd5 docs(audit): apply P0 fixes from AUDIT.md (A1-A8) and add second-pass findings`
4. `c79cc0f docs(scope): apply P0+P1 fixes from AUDIT and second-pass findings`
5. `73dd53e chore(tooling): add Makefile, .editorconfig, CHANGELOG.md and harden .gitignore`
6. `2f21692 docs(agents): add AGENTS.md handbook and contextual .cursor/rules`
7. `adf3e7c docs(agents): add eight workflow-oriented .cursor/skills`
8. `63f7510 chore(agents): add .cursor/hooks for secrets, gofmt, commits and scope`

---

## What worked well

- **Audit-first sequencing.** Producing `docs/AUDIT.md` with 39 findings
  *before* touching any file gave every later commit a numbered anchor
  (e.g. "Refs: AUDIT A1"). PR review next sprint can verify each line of
  code against the finding it closes.
- **Two-stream audit (`AUDIT.md` + second-pass findings).** Splitting the
  original 39 findings from 19 follow-ups kept the first pass focused on
  consistency/scope while the second pass tackled deeper crypto/security
  details (AES-GCM nonce, `WEBOX_MASTER_PASSWORD` risk, host-key
  resolution UX).
- **Context7 for crypto & keyring verification.** Confirming the exact
  semantics of `go-keyring` sentinel errors and `argon2.IDKey`
  parameters via Context7 caught the broken keyring-detection logic
  (AUDIT A1) that no amount of reading the library README would have
  surfaced.
- **Skills > Rules.** Splitting policy into eight task-shaped Skills
  beats one mega `rules.md`. Each skill ships only when its description
  matches the active task, keeping the base context lean and the policy
  proximate to the action.
- **Hooks tested before re-enabling `failClosed`.** A synthetic-secret
  test triggered my own hook and almost locked me out. Lesson learned:
  ship hooks `failClosed: false` first, smoke-test them, then
  selectively tighten.

## What didn't work

- **Initial `Write` of `docs/AUDIT.md` silently dropped the file.** Had
  to re-run before `git add` succeeded. Suspect a transient FS sync
  issue; mitigation: always `ls`-verify after `Write` of large files.
- **First commit-validator regex was too strict on subject length 73.**
  A few of my commit drafts were exactly 73 chars; trimmed them
  manually. Acceptable, but I should consider documenting the
  *exact* 72-char limit prominently in `commit-policy` skill.
- **`StrReplace` mismatch on `docs/DESIGN.md §15.3` header.** Trailing
  whitespace difference. Required a `Grep`-first verification dance.
  Lesson: for tricky surgical edits, always `Grep` the literal line
  first, then `StrReplace` with the verified string.
- **Chicken-and-egg with `failClosed: true` shell hook before scripts
  were executable.** Lost two minutes restoring access. Mitigation
  applied: hooks ship `failClosed: false` initially.

## Surprises / unknowns discovered

- **`teatest` lives in `x/exp/teatest`** — it's still experimental.
  This needs careful version pinning in `go.mod` (full commit SHA,
  not a tag) per the audit. Documented in TESTING §5.1.
- **`Ctrl+S` collides with XON/XOFF flow control** in many terminals,
  making it a silent UX trap. Replaced with `Alt+M` in UX §12.2.
- **`golangci-lint v2` renamed many linters** (`gas` → `gosec`,
  `gomnd` → `mnd`, `goerr113` → `err113`, `logrlint` → `loggercheck`).
  CONTRIBUTING.md now carries the migration table.
- **GitHub Action SHA pinning** is non-negotiable per supply-chain
  best practices — `actions/checkout@v4` is a moving target. Added
  to AGENTS §2.1.
- **MVP-vs-STRETCH ambiguity was widespread.** `DESIGN.md` happily
  described DAG-based rollback, Bento Ultra, sound engine, topology
  map, env merger as if they were in scope. They aren't (per
  `PRD.md` / `ROADMAP.md`). Fixed via explicit `🔶 STRETCH (v0.X+)`
  banners and audit IMP-1.

## Changes to apply going forward

- **AGENTS.md §7 — top gotchas:** keep this table current; every
  retro that surfaces a new gotcha appends a row.
- **`.cursor/skills/commit-policy`:** add a line about the 72-char
  hard limit being *exclusive*, not inclusive.
- **`.cursor/hooks/secret-scan-shell.sh`:** flip `failClosed: true`
  once we have one week of green operation (currently `false`).
  Track as Issue.
- **`docs/retros/`:** institutionalise — every non-trivial task gets
  a retro file. Format mirrors `.cursor/skills/retro/SKILL.md`.
- **`make audit-status`:** wire up to actually parse `docs/AUDIT.md`
  status table and exit non-zero if P0 / P1 remain. Currently a stub
  in `Makefile`; should be filled before v0.1 release per release-check
  skill item 12.
- **Context7 usage:** during implementation, re-verify each crypto
  primitive's API surface (`crypto/cipher.NewGCM`, `argon2.IDKey`,
  `memguard.NewBufferFromBytes`) at the moment of writing the code,
  not from memory.

## Open questions

- Should we add a `sessionStart` hook injecting a brief "what to read
  first" message linking to `AGENTS.md` + `docs/AUDIT.md`? Trade-off:
  helps onboarding, adds noise for repeat sessions. **Decision: defer**
  — revisit after one week of agent operation.
- Should the `commit-validator.sh` enforce the `Refs: <ID>` footer for
  commits touching `providers/`, `secrets/`, `wizard/`? **Decision:
  defer** — pre-commit hook + manual review at PR time is enough for
  now. Reconsider if we see scope drift.
- Should the `scope-guard.sh` *block* (`emit_deny`) instead of just
  injecting context for known STRETCH paths? **Decision: defer** —
  current behaviour preserves agent flexibility; promotion to block
  requires more evidence of false positives first.
- How aggressive should `secret-scan-file.sh` be on Markdown files
  that legitimately *describe* secrets (e.g. `docs/SECURITY.md`
  with `BEGIN PRIVATE KEY` in a code fence)? Currently allowlisted
  by path. **Decision: keep allowlist** — refine if false negatives
  appear during real use.

## Numbers

| Metric | Value |
|---|---|
| AUDIT findings | 39 (P0: 8, P1: 11, P2: 14, P3: 6) |
| Folded second-pass findings (`IMP-*`) | 19 |
| Open decisions | 5 |
| Commits in this audit batch | 8 |
| New files (cursor env + tooling) | 27 |
| Docs lines added/changed | ~3 000 |
| Coverage threshold mandated | 70% MVP / 80% v0.2 |
| Time spent (rough) | one extended session |

## Next-step gate

Before any production code lands:

- [ ] Maintainer reviews and accepts `docs/AUDIT.md`, including §8 folded `IMP-*` findings.
- [ ] Five open decisions in AUDIT closed (or explicitly deferred).
- [ ] First `tdd-loop`-driven parser (devil `www add`) lands as the
      canary commit — proves the agent environment end-to-end.
- [ ] Second retro after the canary commit, capturing first-real-use
      friction with `.cursor/hooks` and `.cursor/skills`.

The implementation environment is now ready. Any commit that touches
production code starts with the `tdd-loop` skill and traces back to
exactly one doc anchor and (if applicable) one AUDIT/IMP finding.
