# `.github/issue-drafts/` — ready-to-create good-first-issue bodies

These five Markdown files are the **launch-day** community on-ramp. Each one
is a fully-written GitHub issue body ready for `gh issue create --body-file …`.
The maintainer ships all five **on the day the repository goes public** so the
issue tracker has visible momentum from minute one.

> Why five and not thirty: focus converts. Five well-scoped good-first-issues
> with a difficulty badge, an estimated time, and a maintainer who pairs on
> the first PR — that is a public scoreboard. Thirty open issues with no
> context is noise.

## Quick reference

| File | Title | Labels | Difficulty | Estimated time |
|---|---|---|---|---|
| [`01-cpanel-skeleton.md`](./01-cpanel-skeleton.md) | Skeleton: cPanel UAPI provider adapter | `good-first-issue` `help wanted` `provider` | 🟢 mainstream | 4-8 h |
| [`02-directadmin-skeleton.md`](./02-directadmin-skeleton.md) | Skeleton: DirectAdmin provider adapter | `good-first-issue` `help wanted` `provider` | 🟡 mixed API | 4-6 h |
| [`03-cyberpanel-research.md`](./03-cyberpanel-research.md) | Research: CyberPanel API for Phase 1 capabilities | `help wanted` `provider` `research` | 🔴 root-level concerns | 2-3 h |
| [`04-nextjs-scaffolding.md`](./04-nextjs-scaffolding.md) | Add scaffolding template: Next.js | `good-first-issue` `help wanted` `documentation` | 🟢 starter | 1-2 h |
| [`05-de-translation.md`](./05-de-translation.md) | Translate cockpit dialog labels to German (DE) | `good-first-issue` `help wanted` `documentation` | 🟢 translation | 1 h |

## Required labels (create once before shipping issues)

```bash
gh label create good-first-issue --color 0e8a16 --description "Pair-review available — perfect first PR"
gh label create "help wanted"    --color 7057ff --description "Maintainer would like community help"
gh label create provider         --color d93f0b --description "Adding or improving a hosting-panel adapter"
gh label create research         --color fbca04 --description "Investigation needed before implementation"
gh label create documentation    --color 0075ca --description "Docs, translations, examples"
```

## Ship all five in one go

```bash
# Run from repo root after `gh auth login`.
bash .github/issue-drafts/create-all.sh
```

The script creates issues sequentially (not in parallel — GitHub label-attach is
rate-limited) and prints the resulting issue URLs. Re-running it after one of
the issues lands a PR is safe: the script asks for `--continue-from` to skip
already-shipped indices.
