# Contribute a Provider Preset in 1 Hour

> **Audience:** developers and power users who use a hosting panel and want Webox to "know" their host out of the box. **No Go knowledge required.** You will edit one JSON file and submit a PR.
>
> Companion guides: [PROVIDER.md](./PROVIDER.md) (writing a full hosting-panel adapter — needs Go), [docs/providers/preconfiguration-vision.md](../providers/preconfiguration-vision.md) (architecture), [ADR-0008](../adr/0008-preset-registry.md) (why we ship presets embedded).

---

## What is a preset?

A **preset** is a JSON file that tells Webox how _your specific hosting_ is configured: where files live, how Node.js is started, how restarts work, where logs are, what risks operators should know about.

Webox ships **adapters** (Go code talking to a panel: cPanel UAPI, DirectAdmin API, Devil CLI, …) and **presets** (data describing one host). Adding a preset is **drastically cheaper** than writing an adapter — and often more useful, because the same panel can have very different runtime stacks at different hosters.

Examples of preset IDs we already ship:

- `smallhost-devil` — small.pl with the Devil panel (status: verified)
- `cpanel-generic` — vanilla cPanel + Application Manager + Passenger (status: research)
- `cpanel-cloudlinux-selector` — cPanel + CloudLinux Node.js Selector (status: research)
- `directadmin-generic` — DirectAdmin Live API + Legacy fallback (status: research)
- `cyberpanel-generic` — CyberPanel + OpenLiteSpeed (status: research, requires_root)

Your contribution will probably be `<panel>-<your-hoster>`, e.g. `cpanel-hostarmada` or `directadmin-hitme-pl`.

---

## Before you start (5 minutes)

You will need:

1. **Webox cloned locally.** `git clone https://github.com/dilitS/webox && cd webox`
2. **Go 1.25+** for `go build ./...` (downloads happen once via `make tidy`).
3. **A real account on the hoster you want to add.** Free trial / cheap plan is fine — you only need to inspect paths and run a few read-only commands.
4. **SSH access.** Most hosters expose SSH on a non-22 port; check your panel.

You **do not** need:

- A keyring / GitHub token / cPanel API token (presets are static data; secrets stay out of the file).
- Any prior contribution to Webox.
- Existing Go experience.

---

## Step 1 — Browse the existing catalog (5 min)

```bash
make build
./bin/webox doctor preset
```

You will see something like:

```
Poland
  smallhost-devil                  verified   [SSH · Node · SSL · DB · Logs · Safe Restart · Fixtures]

Global
  cpanel-cloudlinux-selector       research   [SSH · API · Node · SSL · DB · Logs · Safe Restart]
  cpanel-generic                   research   [SSH · API · Node · SSL · DB · Logs · Safe Restart]
  cyberpanel-generic               research   [SSH · API · SSL · DB]
  directadmin-generic              research   [SSH · API · Node · SSL · DB · Logs · Safe Restart]
  mock                             verified   [Logs · Safe Restart]

5 preset(s) loaded. Use `webox doctor preset --id <id>` for details.
```

To see one preset in full:

```bash
./bin/webox doctor preset --id=cpanel-generic
./bin/webox doctor preset --id=cpanel-generic --json   # machine-readable, for scripts
```

Pick the closest existing preset as a starting point. If your hoster is generic cPanel + Application Manager → start from `cpanel-generic`. If they use CloudLinux Node.js Selector → start from `cpanel-cloudlinux-selector`. If they are CyberPanel-based → start from `cyberpanel-generic` (and read the *known_risks* section before promising "verified").

---

## Step 2 — Capture host facts (15 min)

Open SSH to the hoster. Run the **probes** for that adapter family:

For cPanel:

```bash
uapi --output=json Version get_version
uapi --output=json PassengerApps list_applications
command -v passenger-config
command -v cloudlinux-selector || command -v selectorctl || command -v nodejs_selector
```

For DirectAdmin:

```bash
command -v nodejs_selector
command -v cloudlinux-selector
command -v unitc
ls /etc/directadmin/conf
```

For CyberPanel:

```bash
command -v cyberpanel
cyberpanel --help
command -v pm2
test -w /usr/local/lsws/conf || sudo -n true
```

Save the **shape** of each output (1–2 lines is fine). Note:

- Node.js runtime present? (Passenger, CloudLinux Selector, Nginx Unit, PM2, none?)
- Restart command that works without affecting other apps?
- Where deploy lives by default? (e.g. `/home/<user>/<app_root>/public`)
- Where logs live? (e.g. `/home/<user>/<app_root>/logs`)
- SSL: AutoSSL, Let's Encrypt, manual?
- Database: MySQL, MariaDB, PostgreSQL?

**You are NOT** running:

- `rm`, `chmod`, `passenger-config restart-app` — keep this read-only.
- Anything that requires root unless your hoster is self-hosted (CyberPanel/OpenLiteSpeed root tier).

---

## Step 3 — Copy and edit the preset (20 min)

```bash
cp assets/provider-presets/cpanel-generic.json assets/provider-presets/cpanel-<your-hoster>.json
```

Open the new file. Edit:

- `id` — must match the filename (without `.json`). Lowercase, dash-separated.
- `display_name` — human label, e.g. `"HostArmada cPanel + CloudLinux Node.js"`.
- `provider_type` — one of `cpanel`, `directadmin`, `cyberpanel`, `smallhost`, `mock`. **Do not invent new ones** — they bind to compiled adapters.
- `status` — start with `"research"`. After fixtures land, promote to `"candidate"`. After full manual checklist, promote to `"verified"`.
- `markets` — ISO country codes or `"global"`. The Provider Catalog groups by these.
- `panel.api_port` — usually 2083 (cPanel), 2222 (DirectAdmin), 8090 (CyberPanel).
- `capabilities.node_runtime` — `"passenger"`, `"cloudlinux_selector"`, `"manual_pm2"`, `"static_only"`, …
- `capabilities.restart_method` — `"passenger"`, `"pm2"`, `"devil"`, `"none"`, …
- `capabilities.ssl_provider` — `"autossl"`, `"letsencrypt"`, `"none"`, …
- `paths.deploy_path_template` — placeholders allowed: `{{user}}`, `{{app_root}}`, `{{domain}}`. Stay inside the user home; no shell metacharacters.
- `probes` — the actual commands you ran in Step 2. **No `;`, `|`, `&`, `>` — schema rejects shell metacharacters.**
- `known_risks` — short English notes, one per line. Things like *"Application Manager may be disabled in Feature Manager"*, *"Restart can briefly drop existing connections"*.
- `sources` — public documentation URLs. **HTTPS only**, no tracking parameters.
- `verified` — leave empty for `status=research`. Fill once you have fixtures (Step 5).

### What you must NOT put in a preset

- API tokens, passwords, SSH private keys, keyring entries.
- Real customer domains, real usernames (use placeholders / fictitious names).
- Long output dumps — those go to fixtures (`testing/fixtures/<panel>/`), not the preset file.
- Endpoints behind authentication (`https://my-cpanel.example.com:2083/cpsess1234567/...`).

The validator (`presets/validator.go`) rejects GitHub tokens, openai-style keys, AWS access keys, and PEM blocks at load time. PRs with those will fail CI before review.

---

## Step 4 — Validate locally (5 min)

```bash
go test ./presets/...    # validates schema + secret tripwire across all presets
./bin/webox doctor preset --id=cpanel-<your-hoster>
```

If the schema validator complains:

- `additional properties` — you added a field the schema does not know. Move it to `known_risks` as prose.
- `pattern` — most likely your `id` has uppercase or your `probes` contain a shell metacharacter.
- `enum` — your `status` or `panel.api` is not one of the supported values.

If `go test` complains about the *secret tripwire*, you accidentally pasted a token or PEM block. Remove it.

---

## Step 5 — (Optional, but it's how you reach `verified`) capture fixtures

For your preset to graduate from `research` → `verified`, you need fixtures: real outputs from your account, sanitised. They live in `testing/fixtures/<panel>/`.

Capture format:

```bash
mkdir -p testing/fixtures/cpanel/<your-hoster>
uapi --output=json PassengerApps list_applications > testing/fixtures/cpanel/<your-hoster>/passenger_list.json
```

Sanitise: replace your real username with `<user>`, real domains with `<domain>`. Add a `*.fixture.md` next to each capture:

```markdown
captured: 2026-05-25
account: <user>@<your-hoster>.example.com
command: uapi --output=json PassengerApps list_applications
sanitized: username -> <user>, paths -> /home/<user>/<app_root>
```

Then update `verified` in your preset:

```json
"verified": {
  "fixture_dir": "testing/fixtures/cpanel/<your-hoster>",
  "last_verified_at": "2026-05-25",
  "verified_by": "@your-github-handle"
}
```

And bump `status` to `"verified"`.

---

## Step 6 — Open a PR (10 min)

```bash
git checkout -b preset/cpanel-<your-hoster>
git add assets/provider-presets/cpanel-<your-hoster>.json
git add testing/fixtures/cpanel/<your-hoster>/   # if you captured fixtures
git commit -m "feat(presets): add cpanel-<your-hoster> preset"
git push -u origin preset/cpanel-<your-hoster>
gh pr create --title "feat(presets): add cpanel-<your-hoster> preset" --body "..."
```

In your PR description, include:

1. Hoster name + plan tier you tested on.
2. Link to hoster's public Node.js / SSH / SSL / API documentation (this becomes `sources[]`).
3. Output of `./bin/webox doctor preset --id=cpanel-<your-hoster>` so reviewers can sanity-check the badges.
4. (For `status=verified`) confirmation that you ran the manual checklist:
   - [ ] SSH connects with key auth.
   - [ ] Probe commands return expected shape.
   - [ ] Restart affects only your app, not the whole server.
   - [ ] Logs path is readable by your user (no sudo).
   - [ ] `.env` outside web root.

The maintainer will pair-review. Most preset PRs land within 7 days.

---

## What the maintainer will check

| Check | Where |
|---|---|
| Filename matches `id` | `make ci` (loader enforces this). |
| Schema valid | `make ci`. |
| No tokens / passwords / private keys | `make ci` (secret tripwire in `presets/validator.go`). |
| `markets` reasonable | Manual review — single country if you only tested one location, `global` only if hoster operates globally. |
| `status` honest | Manual review — `verified` requires fixtures + manual checklist. `research` is fine if you only have public docs. |
| Probes don't include shell metacharacters | Schema regex enforces this. |
| Sources are HTTPS, no tracking | Schema enforces HTTPS; manual review for tracking. |

If review takes longer than 7 days, ping `@maintainer` in the PR — the SLA is real.

---

## FAQ

**Can I add a preset for a panel Webox doesn't support yet (e.g. Plesk)?**

You need an adapter first (Go code) — see [PROVIDER.md](./PROVIDER.md). A preset without an adapter has nowhere to bind.

**Can I have multiple presets for the same hoster?**

Yes — different stacks count as different presets. E.g. `cpanel-hoster-passenger` and `cpanel-hoster-cloudlinux` if the hoster offers both runtimes.

**Why are presets embedded, not loaded from disk / URL?**

Supply-chain and security. See [ADR-0008](../adr/0008-preset-registry.md). Filesystem-loadable presets may come post-v1.0 if there is real demand.

**My hoster requires root for everything. Should I add a preset?**

Probably yes, but mark `status=research` and `safe_restart=false`, document the root requirement in `known_risks`, and put the preset in the `Advanced` region (no `markets` declared, or `["global"]` only with explicit caveats in `display_name` like `"(experimental, requires root)"`).

**Can I update an existing preset?**

Yes — that is the most useful kind of contribution. The `verified` block tracks who last verified it; if it's outdated, refresh and update `last_verified_at`.

---

## Where to ask for help

- Open a [Provider Request issue](https://github.com/dilitS/webox/issues/new?template=provider_request.yml) before starting if you want pair-review.
- Mention `@maintainer` in any draft PR.
- Read [docs/providers/preconfiguration-vision.md §8](../providers/preconfiguration-vision.md) for the canonical schema example.
