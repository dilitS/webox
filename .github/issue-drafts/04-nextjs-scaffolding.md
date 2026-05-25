> **Estimated time:** 1-2 h · **Difficulty:** 🟢 starter · **Labels:** `good-first-issue` `help wanted` `documentation`
>
> **Maintainer pair-review available** — comment below before you start.

## What we want

Add a **Next.js deploy workflow template** to Webox's project-scaffolding library so that a new project created via the cockpit wizard with `Stack: Next.js` produces a working `.github/workflows/deploy.yml` on first build.

Three templates already ship today: [`assets/workflows/vite-react/deploy.yml`](https://github.com/dilitS/webox/blob/main/assets/workflows/vite-react/deploy.yml), [`assets/workflows/node-express/deploy.yml`](https://github.com/dilitS/webox/blob/main/assets/workflows/node-express/deploy.yml), and [`assets/workflows/static/deploy.yml`](https://github.com/dilitS/webox/blob/main/assets/workflows/static/deploy.yml). Adding `nextjs` follows the same pattern — this is a great first PR if you want to learn the Webox codebase.

## How to start (15 minutes)

1. Read the Vite-React template:

   ```
   assets/workflows/vite-react/deploy.yml
   ```

   It uses `[[ … ]]` Go template delimiters (not `{{ … }}` — `.yml` already uses braces). The fields are documented in [`assets/workflows.go`](https://github.com/dilitS/webox/blob/main/assets/workflows.go) as `WorkflowData`.

2. Copy the directory:

   ```bash
   cp -r assets/workflows/vite-react assets/workflows/nextjs
   ```

3. Edit `assets/workflows/nextjs/deploy.yml` to match Next.js conventions:
   - `npm ci && npm run build` (default).
   - The `out/` directory is the rsync source if `next.config.js` has `output: 'export'`, otherwise `.next/` for SSR — **pick `output: 'export'` for v0.2 scope** (shared hosting can't run a Node.js SSR server reliably; SSR is v0.3+ scope).
   - Adjust `DistDir` default + restart hook (static export doesn't need a restart — the rsync is the deploy).

4. Add the stack to the wizard list in [`wizard/stacks.go`](https://github.com/dilitS/webox/blob/main/wizard/stacks.go) (look for the `vite-react` entry and clone it).

5. Add a parser table-driven test case in [`assets/workflows_test.go`](https://github.com/dilitS/webox/blob/main/assets/workflows_test.go) — render the new template with sample `WorkflowData` and assert it contains the expected stanzas.

## Acceptance criteria

- [ ] `assets/workflows/nextjs/deploy.yml` exists and is valid YAML (`yamllint` clean).
- [ ] `RenderDeployWorkflow("nextjs", data)` returns valid GitHub Actions YAML.
- [ ] `wizard/stacks.go` lists `nextjs` with the `output: 'export'` build flag clearly documented in a comment.
- [ ] At least one table-driven test case in `assets/workflows_test.go` covers the new stack.
- [ ] `make lint && make test` green.
- [ ] CHANGELOG `[Unreleased] / Added` entry.
- [ ] If you change the wizard step copy, update the relevant golden file under `tui/testdata/golden/`.

## What we will NOT accept

- A Next.js SSR template that assumes a running Node.js server — that is out of scope for shared hosting in v0.2.
- Editing files outside `assets/`, `wizard/`, and `tui/testdata/golden/` without explanation.
- Workflows pinned by tag (`uses: actions/checkout@v4`) — Webox pins all actions by full 40-char SHA. See [`AGENTS.md` §1](https://github.com/dilitS/webox/blob/main/AGENTS.md).

## Useful references

- [Next.js static export docs](https://nextjs.org/docs/pages/guides/static-exports)
- [`assets/workflows/vite-react/deploy.yml`](https://github.com/dilitS/webox/blob/main/assets/workflows/vite-react/deploy.yml) — closest analogue
- [`assets/workflows.go`](https://github.com/dilitS/webox/blob/main/assets/workflows.go) — `WorkflowData` field documentation
- [`docs/providers/smallhost.md` §6](https://github.com/dilitS/webox/blob/main/docs/providers/smallhost.md) — template variable reference
