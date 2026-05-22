---
name: task-start
description: Begin work on the next open sprint task — pick the task, create the feature branch, load the authoritative docs into context, and start the TDD watch loop. Use when the user says "let's start the next task", "pick up where we left off", "rozpocznij task", "kolejne zadanie", "zaczynamy implementację", or any phrasing that implies kicking off a new piece of sprint work.
---

# Skill — Task Start

## When to use

Trigger this skill whenever the user signals "let's begin coding" without specifying exactly which task. The skill handles the boilerplate: branch creation, doc loading, TDD watch start, so the user only sees the meaningful checkpoints.

## Trigger phrases (PL/EN)

- "rozpocznij task" / "kolejne zadanie" / "co dalej?"
- "let's start" / "next task" / "pick up"
- "begin sprint 01 task 3"
- "I'm ready to code"

## The flow

```
0. Bootstrap check  — verify git hooks installed and tools present
1. Resolve task     — read current sprint, find next open task
2. Read spec        — load authoritative docs referenced by the task
3. Branch           — feat/s<NN>-<TT>-<slug>
4. Plan             — write 3-5 sentence plan in chat
5. Watch            — start `make dev PKG=<package>` in background
6. TDD loop         — delegate to skill `tdd-loop`
```

## Step-by-step

### Step 0 — Bootstrap check

Verify automation is wired in:

```bash
test -f .git/hooks/pre-commit -o "$(git config core.hooksPath)" = ".githooks" \
  || make setup-hooks
```

If anything is missing, **stop and run `make bootstrap` first**.

### Step 1 — Resolve task

```bash
make next-task            # prints e.g. TASK-01.3
make next-task-verbose    # full task block
```

If the user named an explicit task (`TASK-01.3`), pass it to step 3 directly.

### Step 2 — Read spec

For each link in the task's **Docs:** field, read the relevant section. Do not skip — drift between spec and implementation is the #1 cause of rework.

If no doc reference exists, **stop** and either:
- Ask the user for the authoritative source, or
- Open the matching ADR (skill `adr-create`).

### Step 3 — Create branch

```bash
make sprint-start TASK=<TASK-XX.Y>
```

This wraps `scripts/start-sprint.sh`: clean checkout from default branch, fresh feature branch, optional editor open.

### Step 4 — Write the plan

Output the plan as chat text before any code:

```markdown
## Plan: <task name>

What    — one sentence about the smallest meaningful step.
Why     — link to the authoritative section.
Tests   — what's the first failing test we'll write.
Files   — exhaustive list of files we'll touch (no surprises later).
Risks   — anything from RISKS.md that's relevant.
```

### Step 5 — Start the watcher

In a side terminal (background):

```bash
make dev PKG=./<package>/...
```

If the package doesn't exist yet, fall back to `PKG=./...` until the first file lands.

### Step 6 — Hand off to TDD loop

Delegate to skill `tdd-loop` for Red → Green → Refactor → Commit.

## Anti-patterns

- **Starting coding without reading the spec** — drift guaranteed.
- **Branching from a dirty tree** — `scripts/start-sprint.sh` refuses, do not work around.
- **Skipping the plan step** — even one sentence beats "let me figure it out as I go."
- **Multiple tasks per branch** — one task = one PR = one focused review.

## Done criteria

This skill is complete when:

- [ ] Working on a feature branch named per convention.
- [ ] Task's authoritative docs are quoted/cited in the plan.
- [ ] Plan is written and visible in the chat transcript.
- [ ] `make dev` is running.
- [ ] `tdd-loop` skill is active (or being applied).

## Linked skills

- `tdd-loop` — the actual code work.
- `commit-policy` — when the first commit lands.
- `audit-trace` — verify the task links back to AUDIT.md / DESIGN.md correctly.
- `retro` — once the task is done.
