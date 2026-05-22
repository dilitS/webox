<!--
Thanks for the contribution! Please fill out the sections below.
For trivial doc-only PRs, you can shorten the checklist (note "docs-only" in title).
-->

## Summary

<!-- 2-4 sentences: what does this PR do and why? -->

## Scope linkage

- **Sprint:** <!-- e.g., sprint-01-foundations -->
- **Task:** <!-- e.g., TASK-01.3 -->
- **Authoritative doc:** <!-- e.g., docs/DESIGN.md §6.2 / docs/SECURITY.md §4.2 / docs/AUDIT.md §8 IMP-X -->
- **Closes:** <!-- #issue (if any) -->

## Type of change

- [ ] `feat` — new feature
- [ ] `fix` — bug fix
- [ ] `refactor` — internal rework, no behavior change
- [ ] `docs` — documentation only
- [ ] `test` — adds or updates tests
- [ ] `build` / `ci` / `chore` — tooling, CI, deps
- [ ] `security` — affects the security posture (requires extra review)

## Definition of Done checklist

- [ ] `make ci` passes locally (`lint`, `test`, `vulncheck`, `build`).
- [ ] Tests added/updated; **TDD** order respected for critical logic (parser, crypto, state machine, cache).
- [ ] Coverage for changed package **≥ target** (config 85 %, secrets 85 %, redactor 95 %).
- [ ] No `TODO`/`FIXME` without a linked issue.
- [ ] No `//nolint` without justification comment.
- [ ] CHANGELOG `Unreleased` entry added (user-visible or security change).
- [ ] Documentation updated where the contract changed (skill: `audit-trace`).
- [ ] Conventional Commits in commit subjects (skill: `commit-policy`).
- [ ] No secrets, tokens, fixtures with real credentials in the diff.

## Security checklist (mandatory for `security`/crypto/SSH/secret changes)

- [ ] Reviewed against `docs/SECURITY.md` relevant section(s).
- [ ] `gosec ./...` clean (or warnings explicitly justified).
- [ ] Secrets handled through `memguard.LockedBuffer` if held in memory.
- [ ] CSPRNG operations use `crypto/rand` with **panic-on-error**.
- [ ] Redactor coverage extended if introducing a new secret type.
- [ ] 7-day cooldown self-review for handmade crypto code (see `RISKS.md` R-003).

## Screenshots / asciinema (TUI changes)

<!--
For TUI / UX changes attach asciinema cast or PNG of the affected screen.
-->

## Out-of-scope / follow-ups

<!-- List things explicitly NOT done in this PR with rationale -->

## Reviewer notes

<!-- Anything reviewers should focus on or be wary of -->
