---
name: tdd-loop
description: Run a strict TDD loop (Read → Plan → Red → Green → Refactor → Commit) for Webox features that mandate test-first development (parsers, validators, redactor, cache, state machine, keyring detection, config migrations). Use when the user asks to implement, fix, or refactor any of these subsystems, or when starting a new behavior-affecting unit of work.
---

# TDD Loop — Webox

## When to use

Trigger this skill **proactively** when the agent is about to write code in:

- `providers/*/parse*.go` (output parsers).
- `*/validate*.go` or `*_validator.go` (input validators).
- `internal/log/redact.go` (secret redactor).
- `status/cache*.go` (SWR cache).
- `tui/update.go` and `tui/states.go` (state machine transitions).
- `secrets/keyring*.go` (keyring detection).
- `config/migrate*.go` (config migrations).
- Any new pure function with non-trivial behavior.

## The loop

```
Task progress:
- [ ] 1. Read   — find the relevant spec
- [ ] 2. Plan   — describe the smallest meaningful step
- [ ] 3. Red    — write a failing test
- [ ] 4. Green  — write the minimum code
- [ ] 5. Refactor — clean up, lint, docs
- [ ] 6. Commit — conventional commit + CHANGELOG entry
```

### Step 1 — Read

Locate the authoritative spec **before** writing anything:

| Domain | Source of truth |
|---|---|
| Provider output format | `docs/providers/<name>.md` §2 + golden fixture |
| Crypto / secrets | `docs/SECURITY.md` §4 + §9 |
| TUI state machine | `docs/DESIGN.md` §12 + `docs/UX.md` §11 |
| Status cache | `docs/DESIGN.md` §8 + `docs/adr/0005` |
| Config schema | `docs/DESIGN.md` §6.1 + migrations §6.4 |

If no doc exists for what you're about to build, **stop and write the spec first** (or escalate to maintainer).

### Step 2 — Plan

Write 3–5 sentences:

```markdown
## Plan: parseDevilWwwAdd

I will add a parser for `devil www add` output handling three known cases:
success (`Added domain X with nodejs Y`), exists (`exists: ...`), and
invalid node (`invalid node version`). The parser strips ANSI escapes,
validates output size ≤ 1 MB, and uses named regex groups. Unknown output
shape returns `ErrUnknownOutputFormat`. Golden fixtures live in
`testing/fixtures/devil/www_add_*.txt`.

Affected guardrails: AGENTS.md §2.3 (test-first), §7 row "Sekret w error".
```

### Step 3 — Red

Write the failing test **first**. The error message must be readable:

```go
func TestParseDevilWwwAdd_Success(t *testing.T) {
    input := loadFixture(t, "devil/www_add_ok.txt")
    got, err := parseDevilWwwAdd(input)
    if err != nil {
        t.Fatalf("parseDevilWwwAdd success case failed: %v", err)
    }
    want := &AddResult{Domain: "test.user.smallhost.pl", NodeVersion: "24"}
    if !reflect.DeepEqual(got, want) {
        t.Errorf("got = %+v, want = %+v", got, want)
    }
}
```

Run `go test ./...` and **confirm the test fails** (not just compile error).

### Step 4 — Green

Write the smallest possible implementation that makes the test pass. **No** premature abstraction, **no** "refactor while I'm here". If you find yourself writing more than 30 lines, you're either solving too much or missing a test.

### Step 5 — Refactor

Once green:

```bash
make fmt
make lint
make test
```

If lint flags something, fix it. If lint demands `//nolint`, write a justified comment.

Update doc comments if behavior is non-obvious. Add a CHANGELOG entry under `[Unreleased] / Added` or `[Unreleased] / Fixed`.

### Step 6 — Commit

Conventional Commits 1.0.0 without gitmoji:

```
feat(providers): parse devil www add output for smallhost

Add parser handling success, exists, and invalid-node-version cases
with strict ANSI stripping and size validation. Fixtures: www_add_ok,
www_add_exists, www_add_invalid_node, www_add_malicious_ansi.

Refs: docs/providers/smallhost.md §2.1
```

## Anti-patterns to avoid

- **Implementing first, testing after**: drift from spec, missed edge cases, false sense of completion.
- **Mocking what you should fixture**: `devil` output is captured as **fixture** (real bytes), not mocked.
- **Single happy-path test**: every parser needs at least 3 fixtures (success, known-error, malicious).
- **Comparing `err.Error() == "..."`**: use `errors.Is(err, ErrFoo)`.
- **Refactoring during Green**: do it during Refactor phase, otherwise tests become unreliable.

## Tools reference

| Task | Command |
|---|---|
| Single package tests | `go test -race ./providers/smallhost/...` |
| Verbose single test | `go test -race -v -run TestParseDevilWwwAdd ./providers/smallhost/...` |
| Coverage for package | `go test -coverprofile=/tmp/cov.out ./providers/smallhost/... && go tool cover -html=/tmp/cov.out` |
| Linter on staged | `golangci-lint run ./providers/smallhost/...` |

## Done criteria

- [ ] Test file added with at least 3 cases (success, known error, malicious).
- [ ] Implementation passes `make test` (`-race`).
- [ ] `make lint` clean.
- [ ] CHANGELOG `[Unreleased]` entry added.
- [ ] Commit message follows Conventional Commits.
- [ ] No secret patterns introduced (run `secret-scanner` hook auto-fires).
