> **Estimated time:** 2-3 h · **Difficulty:** 🔴 root-level concerns · **Labels:** `help wanted` `provider` `research`
>
> This is a **research-only** issue. No code changes are expected from the deliverable PR — only a documentation update with concrete findings.

## What we want

A short, evidence-backed write-up appended to [`docs/providers/cyberpanel.md`](https://github.com/dilitS/webox/blob/main/docs/providers/cyberpanel.md) that resolves the four open `TO BE VERIFIED` questions below. The output is a documentation PR; the goal is to determine **whether and how** CyberPanel can ship as a Webox adapter.

CyberPanel is unusual in the Webox provider matrix because:

1. It runs **OpenLiteSpeed / LiteSpeed** instead of Apache/Nginx, so subdomain + SSL behaviour differs from cPanel/DA.
2. It is **root-installed on a VPS** — not really a shared-hosting panel the way small.pl or cPanel is. Most users self-host CyberPanel, which raises a different security posture than "regulated commercial shared hosting".
3. Its **public API surface is smaller and less documented** than UAPI/DA. Many capabilities are CLI-only via `cli.py` running as root.

Before we spend Sprint 20+ implementing an adapter, we need this research.

## Open questions (the deliverable)

For each question, the PR must add a concrete answer with a source URL and (where possible) a captured fixture or CLI output snippet.

1. **Authentication model.** Does CyberPanel expose an HTTP API token, or is everything `cli.py` over SSH as root? If HTTP: how is the token issued, what is the rate limit, what is the scope model?
2. **Subdomain creation capability.** Can a non-root user create a subdomain via the API? Or does it require `cli.py createWebsite`? If root-only, **stop here** — Webox cannot ship a CyberPanel adapter that requires shared root access to operator machines. Document the dealbreaker and close the issue.
3. **SSL + database.** Does CyberPanel expose Let's Encrypt issuance and MySQL/MariaDB DB creation via the same auth surface as (1)? Capture the relevant `cli.py` invocation or HTTP endpoint.
4. **Multi-tenant isolation.** When a single CyberPanel install hosts multiple websites under different users, can Webox safely target one site without touching siblings? What is the isolation guarantee?

## Acceptance criteria

- [ ] PR modifies `docs/providers/cyberpanel.md` with a new `## Phase 1 capability assessment (2026-05-NN)` section.
- [ ] Each of the four questions has a numbered answer with:
  - A direct quote or paraphrase of the upstream documentation (URL included).
  - Either a captured `cli.py --help` output snippet, an HTTP endpoint name, **or** an explicit "documented but untested" disclaimer.
- [ ] A final **Verdict** subsection: one of `green-light` (ready for Sprint 20+ adapter PR), `yellow-light` (proceed with named caveats), or `red-light` (do not ship — explain why and recommend deprecating the docs page).
- [ ] No code changes outside `docs/providers/cyberpanel.md` and (optionally) `CHANGELOG.md` `[Unreleased] / Added`.
- [ ] If you spin up a CyberPanel install for this research, document it in the `.local/notes/` folder of *your* fork only — do not commit fixtures or `cli.py` traces from a live system.

## What we will NOT accept

- Speculative answers without a source URL.
- "We can probably figure it out later" verdicts — the whole point of this issue is to make a deterministic ship/skip decision.
- Any fixture with a real CyberPanel admin password, SSH key, or LiteSpeed license key.

## Useful references

- [CyberPanel documentation](https://community.cyberpanel.net/docs)
- [CyberPanel `cli.py` source on GitHub](https://github.com/usmannasir/cyberpanel)
- [OpenLiteSpeed virtual-host model](https://openlitespeed.org/kb/virtual-hosts/)
- [`docs/providers/cyberpanel.md`](https://github.com/dilitS/webox/blob/main/docs/providers/cyberpanel.md) — the current research notes (mostly `TO BE VERIFIED`)
- [`docs/SECURITY.md` §6 — supply chain](https://github.com/dilitS/webox/blob/main/docs/SECURITY.md) — relevant for the root-user posture question
