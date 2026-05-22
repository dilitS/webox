---
name: adr-create
description: Create an Architecture Decision Record (ADR) in docs/adr/ for Webox. Use when the user proposes or has just made a non-trivial architectural choice (new dependency, schema change, security model shift, rollback strategy, etc.) that future contributors need to understand.
---

# Create an ADR — Webox

## When ADR is required

Create an ADR for any of these:

- Adding/removing a **major** dependency (`go.mod` line).
- Changing the **public API** (`HostingProvider`, `ProviderConfig`).
- Switching a **security mechanism** (keyring backend, host key trust, secret storage).
- Changing a **rollback strategy** (LIFO ↔ DAG, atomic save mechanism).
- Modifying the **config schema** (`schema_version` bump).
- Changing the **i18n strategy** (file format, fallback chain).
- Adopting a **new architectural pattern** (e.g. introducing a new package layer).

**Do not** ADR for: linter config tweaks, minor refactors, individual test additions, doc fixes.

## File naming

```
docs/adr/000X-<kebab-case-decision>.md
```

X = next sequential number after the highest existing ADR. Current as of 2026-05-22:

```
0001-tui-zamiast-cli.md
0002-deploy-tylko-przez-github-actions.md
0003-provider-pattern.md
0004-przechowywanie-sekretow-keyring.md
0005-cache-statusow-projektow.md
0006-jezyk-interfejsu-en-domyslny.md
```

Next free: `0007-...`.

## Structure

```markdown
# ADR-000X: <Decision Title>

> Status: Proposed | Accepted | Rejected | Superseded by ADR-000Y · Data: YYYY-MM-DD · Autor: @username · Reviewers: @maintainer (+ others)

## Context

<2-4 zdania opisujące problem, ograniczenia, presje. Bez TL;DR — całość ma być TL;DR.>

## Decision

<Jednoznaczne stwierdzenie: "Webox będzie używać X." Nigdy "rozważamy".>

## Rationale

<Lista bullet'ów: dlaczego ta decyzja, a nie alternatywne.>

- Powód 1: ...
- Powód 2: ...
- Powód 3: ...

## Consequences

### Positive

- ...

### Negative / Trade-offs

- ...

### Mitigations

- Dla każdego negatywnego trade-off'a: jak go adresujemy.

## Alternatives considered

### Option A: <name>

Pros: ... · Cons: ... · Why rejected: ...

### Option B: <name>

Pros: ... · Cons: ... · Why rejected: ...

## References

- [PRD §X](../PRD.md#x), [DESIGN §Y](../DESIGN.md#y), …
- External: [link to spec / library / RFC].
- Related ADRs: ADR-000Z (if any).

## Implementation notes

<Optional. Krótki opis konkretnych kroków implementacyjnych, jeśli ADR nie jest oczywisty do wykonania.>
```

## Workflow

```
- [ ] 1. Number the ADR (next sequential).
- [ ] 2. Draft sections in this order: Context → Alternatives → Decision → Rationale → Consequences.
- [ ] 3. Set status = Proposed.
- [ ] 4. Open PR with the ADR.
- [ ] 5. Discussion in PR; revise as needed.
- [ ] 6. On merge: status = Accepted + reviewers list updated.
- [ ] 7. Link the ADR from relevant docs (DESIGN.md, SECURITY.md, etc.).
- [ ] 8. CHANGELOG entry under [Unreleased] / Added.
```

## Anti-patterns

- **Writing decision before alternatives**: leads to confirmation bias. Brainstorm 2–3 alternatives first.
- **"We chose X because it's better"**: not a rationale. Be specific: "X has Y feature that Z requires".
- **No trade-offs**: every architectural decision has cost. If you can't name one, you haven't thought deeply enough.
- **Vague status**: always one of `Proposed | Accepted | Rejected | Superseded by ADR-000Y`. No "Draft", "WIP".
- **Skipping references**: ADR should be a hub linking to PRD/DESIGN/SECURITY/external specs.

## Example: ADR-0004 (already in repo)

`docs/adr/0004-przechowywanie-sekretow-keyring.md` is the canonical template. Open it before drafting a new ADR.

## After acceptance

- Link from `docs/DESIGN.md` (architecture impact).
- Link from `docs/SECURITY.md` if security-relevant.
- Link from `AGENTS.md §1.2` if introducing a key library.
- Update [PRD.md](../../docs/PRD.md) "Open decisions" section if this ADR closes an open decision.

## Done criteria

- [ ] File numbered correctly.
- [ ] All required sections filled.
- [ ] Status = Accepted (post-merge).
- [ ] Cross-linked from at least one other doc.
- [ ] CHANGELOG entry added.
