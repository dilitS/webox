# Webox Git Hooks

Versioned, opt-in client-side hooks. Activated via:

```bash
make setup-hooks
```

This sets `git config core.hooksPath .githooks` for **this clone only**. No system-wide changes.

## Hooks

| Hook | When | What | Bypass |
|------|------|------|--------|
| `pre-commit` | before snapshot | Auto-format Go files (gofumpt/goimports), fast lint on staged Go files | `git commit --no-verify` |
| `commit-msg` | before commit recorded | Conventional Commits 1.0.0 validation | `git commit --no-verify` |
| `prepare-commit-msg` | before editor opens | Pre-fills CC template if message is empty | edit anyway |
| `pre-push` | before push | `make test-short` quick sanity | `git push --no-verify` |

## Philosophy

- **Fast**: each hook must finish in seconds — slow hooks get bypassed.
- **Auto-fix > block**: if we can format/repair, we do; we block only on hard errors.
- **Bypassable for emergencies**: every hook supports `--no-verify`. We log a warning when bypassed.
- **CI is the source of truth**: hooks are convenience, not safety.

## Disable temporarily

```bash
git -c core.hooksPath= commit -m "..."
```

## Uninstall

```bash
git config --unset core.hooksPath
```
