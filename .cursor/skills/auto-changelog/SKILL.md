---
name: auto-changelog
description: Maintain CHANGELOG.md `[Unreleased]` section automatically as part of every user-visible or security-relevant change. Use when the agent has just produced code or doc changes that fall under Keep-a-Changelog categories (Added, Changed, Deprecated, Removed, Fixed, Security) and the user has not yet updated the CHANGELOG, or when the user asks "did we update the changelog?", "wpisz do changeloga", or similar.
---

# Skill — Auto Changelog

## When to use

Trigger this skill **before staging a commit** when the diff contains any of:

- New public-facing behavior (`feat`).
- Bug fix (`fix`).
- Crypto / SSH / secret handling change (`security`).
- Public API or contract change (`refactor!`, `feat!`).
- Removed or deprecated feature.

**Do NOT** trigger for:

- Pure internal refactors with no behavior change.
- Style-only commits (`style`, `chore` for whitespace).
- Tests-only commits, unless they backfill coverage on a previous unreleased change (then add to *Changed*).

## The flow

```
1. Detect    — classify diff into Keep-a-Changelog sections
2. Draft     — write a one-line, user-visible entry
3. Apply     — use scripts/changelog-add.sh
4. Verify    — read back the section, confirm placement
```

## Step-by-step

### Step 1 — Detect category

| Code/Doc change | Category |
|------|----------|
| New CLI command, new feature, new provider operation | **Added** |
| Behavior change (default switched, semantics altered) | **Changed** |
| Public API marked for removal next release | **Deprecated** |
| API/command removed | **Removed** |
| Bug fix (any severity) | **Fixed** |
| Crypto, SSH host key, secret handling, GitHub token, log redaction | **Security** |

If unsure, prefer **Changed** for behavior shifts and **Security** for anything touching `secrets/`, `internal/log/redact*`, SSH host keys, or GitHub token scopes.

### Step 2 — Draft the entry

**Format rules** (Keep a Changelog 1.1.0):

- One line per change.
- Past tense or imperative — be consistent with surrounding entries.
- Start with the **module/package** in code-spans when applicable: `` `config.Save` now ... ``.
- Reference the audit / IMP finding when relevant: `` ... (fixes AUDIT §8 IMP-2). ``
- No PR numbers yet (pre-release; add at release-cut time).

**Good examples:**

```markdown
- `config.Save` now performs an atomic rename + fsync under `flock(2)`.
- `secrets.Fallback` panics on `crypto/rand.Read` failure rather than
  silently producing a zero nonce (AUDIT §8 IMP-2).
- Redactor coverage extended to GitHub fine-grained PATs (`github_pat_…`).
```

**Avoid:**

- "Various improvements" — be specific.
- "Refactored package X" — refactor without user impact does not belong here.
- Internal implementation details that don't affect users.

### Step 3 — Apply via script

```bash
make changelog KIND=security MSG="\`secrets.Fallback\` panics on CSPRNG failure (AUDIT §8 IMP-2)"
```

The script (`scripts/changelog-add.sh`) inserts the entry under the right header in `[Unreleased]`, creating the header if missing. Idempotent.

### Step 4 — Verify

```bash
head -40 CHANGELOG.md
```

Confirm:

- [ ] Entry is under correct section.
- [ ] No accidental duplicate (script is idempotent but human edits can collide).
- [ ] Spacing is clean (blank line between sections preserved).
- [ ] No leaked secret text in the entry itself.

## Anti-patterns

- **"I'll add it at the end of the sprint"** — entries written stale lose context. Do it in the same PR.
- **Bundling unrelated changes** — each entry is one logical change.
- **Crypto change without `Security` section** — security review is gated on this.
- **Copy-pasting commit subject** — the changelog entry is for users, the commit is for developers; tone may differ.

## Done criteria

- [ ] Entry exists in `CHANGELOG.md` `[Unreleased]` under the correct section.
- [ ] Entry stands on its own (a user reading the changelog understands it without the diff).
- [ ] No secrets, tokens, or hostnames in the entry text.
- [ ] If `Security` — also referenced in `RISKS.md` or `docs/SECURITY.md` if introducing new threat surface.

## Linked skills / hooks

- `commit-policy` — the actual commit happens after this.
- `audit-trace` — verify the entry references the right audit/finding when applicable.
- `.cursor/hooks/secret-scan-file.sh` — fires after CHANGELOG edits to catch accidental secrets.
