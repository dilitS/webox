# How to Add a Hosting Provider to Webox

> Status: Stable · Last updated: 2026-05-25 · Audience: contributors building a new provider adapter.
>
> **Estimated time:** 4-8 hours for a "candidate" status adapter, 1-2 days for a "verified" status adapter with full fixtures.
>
> Sister documents: [docs/providers/smallhost.md](../providers/smallhost.md) (reference implementation), [docs/providers/preconfiguration-vision.md](../providers/preconfiguration-vision.md) (adapter vs preset), [docs/DESIGN.md §3](../DESIGN.md#3-provider-pattern) (Provider Pattern spec), [docs/conventions.md](../conventions.md) (code style).

---

## TL;DR

Webox's `HostingProvider` interface abstracts hosting panels (cPanel, DirectAdmin, CyberPanel, etc.). Adding a new provider takes ~4 hours if the panel has decent API and you follow this walkthrough. The path is:

1. **Decide:** preset vs adapter (preset = JSON config for an existing panel adapter, adapter = new Go code for a panel).
2. **Generate skeleton:** `webox provider new <name>`.
3. **Capture fixtures** from a real account (or document `TO BE VERIFIED`).
4. **Implement parsers TDD-style** (malicious fixtures first).
5. **Implement methods**: read-only first (ListProjects, GetStatus, GetLogs), then mutating (CreateSubdomain, CreateDatabase, IssueSSL).
6. **Add SSH mock integration tests** (`testing/sshmock`).
7. **Submit PR with pair-review request.**

Maintainer commits to pairing on the first PR — open an issue *before* writing code, we'll align on approach.

---

## Step 0 — Preset vs Adapter

Before writing code, decide which artifact you're adding. The two are different.

### Preset (no Go code)

A preset is a JSON file in `assets/provider-presets/` describing a *specific hosting variant* using an *existing* adapter. Example: `cpanel-krystal-cloudlinux.json` reuses the cPanel adapter (`providers/cpanel/`) but pins paths, runtime, restart method, AutoSSL availability, and capability badges specifically for Krystal's cPanel + CloudLinux Node.js Selector setup.

**Choose preset if:**

- The panel already has an adapter in Webox.
- You just want to customize paths/runtime/SSL for a specific hoster.
- You have an account on that hoster and can capture fixtures.

**Time:** ~1 hour. Workflow: copy `assets/provider-presets/cpanel-generic.json`, edit values, capture probe fixtures, submit PR.

Stop here and read `docs/contributing/PRESET.md` (when Sprint 19 lands). For now, follow the adapter path even if you only need a preset — preset registry is post-v0.2.

### Adapter (Go code)

An adapter is a Go package in `providers/<name>/` implementing `HostingProvider`. Example: `providers/cpanel/` wraps the cPanel UAPI plus SSH fallback.

**Choose adapter if:**

- The panel doesn't have an adapter in Webox yet (cPanel, DirectAdmin, CyberPanel, Plesk, etc.).
- The panel exposes an API or CLI you can drive from Go.
- You have access to a real account or extensive documentation.

**Continue this walkthrough.**

---

## Step 1 — Generate the skeleton

```bash
# Generate a new provider package + tests + fixtures directory.
./bin/webox provider new mypanel --preset blank
```

This creates:

```text
providers/mypanel/
├── provider.go            # HostingProvider implementation skeleton
├── provider_test.go       # TDD red test (failing on purpose)
├── parsers.go             # Output parsers
├── parsers_test.go        # Table-driven parser tests
├── doc.go                 # Package documentation
testing/fixtures/mypanel/
└── .fixture.md            # Fixture origin + sanitization instructions
```

The generator also edits `providers/imports.go` to register the new package.

**Smoke test:**

```bash
go build ./...
go test ./providers/mypanel/...
```

The test should fail (red) with `ErrNotImplemented`. That's expected — TDD red phase.

---

## Step 2 — Capture fixtures

A fixture is a real output from the panel's API/CLI saved as a text file. Fixtures drive parser TDD and integration tests.

### Capturing from a real account

```bash
# SSH to the host
ssh user@host.example.com

# Capture UAPI/CLI output as raw text
uapi --output=json Version get_version > /tmp/version.txt
uapi --output=json PassengerApps list_applications > /tmp/list_apps.txt
# ... etc.

# Pull fixtures back
scp user@host.example.com:/tmp/*.txt testing/fixtures/mypanel/
```

### Sanitization checklist

Before committing fixtures, **scrub**:

- [ ] Account user → `<user>` placeholder.
- [ ] Hostname → `<host>` placeholder.
- [ ] Real domains → `example.com` / `app.example.com`.
- [ ] Email addresses → `user@example.com`.
- [ ] Tokens / API keys → `[REDACTED]` (and revoke them after, even if scrubbed).
- [ ] IP addresses → `203.0.113.1` (RFC 5737 reserved range).
- [ ] SSL certificate fingerprints if present → hash but keep length.
- [ ] Database names with company / brand → `appdb`.
- [ ] File paths with personal data → `/home/<user>/<app>/`.

### Per-fixture metadata

Each fixture needs a sidecar `.fixture.md`:

```markdown
# Fixture: list_apps_ok.txt

- **Origin:** Real cPanel account, hosted at Krystal Lite plan.
- **Panel version:** cPanel 11.110.0
- **CloudLinux Node.js Selector:** v1.42.1
- **Captured:** 2026-05-30
- **Captured by:** @yourhandle
- **Sanitization:** user replaced with `<user>`, domain replaced with `app.example.com`, no other PII.
- **Reproduce:** `uapi --output=json PassengerApps list_applications`.
```

### Minimum fixtures per method

For each `HostingProvider` method, capture **at least three**:

- `<method>_ok.txt` — successful response (e.g., subdomain created).
- `<method>_exists.txt` — idempotent case (e.g., subdomain already exists).
- `<method>_error.txt` — failure (e.g., invalid argument).

Plus **one malicious fixture per parser**: `<method>_malicious.txt` with `\r\n` injection, ANSI escape sequences, 1MB+ output, and intentionally malformed JSON. The parser must NOT panic.

---

## Step 3 — Implement parsers (TDD red → green)

Open `providers/mypanel/parsers_test.go` and write table-driven tests **first**:

```go
func TestParseListApps(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    []App
        wantErr error
    }{
        {
            name:  "ok",
            input: loadFixture(t, "list_apps_ok.txt"),
            want:  []App{{Name: "shop-ease", Path: "/home/<user>/shop-ease"}},
        },
        {
            name:    "empty",
            input:   loadFixture(t, "list_apps_empty.txt"),
            want:    nil,
            wantErr: nil,
        },
        {
            name:    "malformed",
            input:   loadFixture(t, "list_apps_malicious.txt"),
            wantErr: ErrParseFailed,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := parseListApps([]byte(tt.input))
            if !errors.Is(err, tt.wantErr) {
                t.Errorf("err = %v, want %v", err, tt.wantErr)
            }
            if diff := cmp.Diff(tt.want, got); diff != "" {
                t.Errorf("mismatch (-want +got):\n%s", diff)
            }
        })
    }
}
```

Then implement `parsers.go` to make tests pass. **Defensive parsing rules:**

1. **Strip ANSI** before any regex.
2. **Named regex groups** — never positional.
3. **Fail-soft** — unknown output format returns `ErrParseFailed`, **never panic**.
4. **Bounded sizes** — reject inputs > 1MB unless explicitly streaming.
5. **No `unsafe`** — never `unsafe.Pointer` to skip type-checking.

---

## Step 4 — Implement methods

`HostingProvider` is in `providers/provider.go`. The interface has 9 methods (as of v0.1):

```go
type HostingProvider interface {
    // Read-only — implement first.
    ListProjects(ctx context.Context) ([]Project, error)
    GetStatus(ctx context.Context, projectID string) (*Status, error)
    GetLogs(ctx context.Context, projectID string, lines int) ([]string, error)
    Restart(ctx context.Context, projectID string) error

    // Mutating — implement after read-only is green.
    CreateSubdomain(ctx context.Context, req CreateSubdomainRequest) (*Subdomain, error)
    CreateDatabase(ctx context.Context, req CreateDatabaseRequest) (*Database, error)
    IssueSSL(ctx context.Context, req IssueSSLRequest) (*Certificate, error)

    // Cleanup — must be idempotent (no resource = nil error).
    RemoveSubdomain(ctx context.Context, projectID string) error
    RemoveDatabase(ctx context.Context, dbName string) error
}
```

### Order of implementation

1. **`ListProjects`** — proves your transport layer (HTTP/SSH) works end-to-end.
2. **`GetStatus`** — proves you can read individual resources.
3. **`GetLogs`** — proves SSH integration.
4. **`Restart`** — first mutating op; choose the safest restart method (no global server reload).
5. **`CreateSubdomain`** — most complex; chain panel + Passenger/Selector + DNS.
6. **`CreateDatabase`** — UAPI-style mysql or panel-native API.
7. **`IssueSSL`** — AutoSSL preferred, Let's Encrypt fallback.
8. **`Remove*`** — must succeed when the resource doesn't exist.

### Implementation rules

- **Context first:** every method takes `context.Context` as first arg, respects `ctx.Done()`.
- **Errors wrapped:** `fmt.Errorf("create subdomain for %s: %w", domain, err)`.
- **No secrets in errors:** never include passwords, tokens, keys in error messages.
- **Sentinel errors:** declare in `errors.go` (e.g., `ErrSubdomainExists`). Use `errors.Is` to compare.
- **Idempotency for `Remove*`:** missing resource = `nil`, not an error.

See [docs/conventions.md §2](../conventions.md#2-error-handling) for full error handling guide.

---

## Step 5 — Integration tests with `sshmock`

`testing/sshmock` provides a fake SSH server that replays captured fixtures. Use it in `providers/mypanel/integration_test.go`:

```go
//go:build integration

package mypanel_test

import (
    "context"
    "testing"

    "github.com/dilitS/webox/providers/mypanel"
    "github.com/dilitS/webox/testing/sshmock"
)

func TestMyPanel_ListProjects_Integration(t *testing.T) {
    srv := sshmock.New(t)
    srv.RegisterCommand("uapi --output=json PassengerApps list_applications", "list_apps_ok.txt")

    p, err := mypanel.New(mypanel.Config{
        Host: srv.Addr(),
        User: "<user>",
        // ...
    })
    if err != nil {
        t.Fatal(err)
    }

    projects, err := p.ListProjects(context.Background())
    if err != nil {
        t.Fatal(err)
    }
    // assert projects
}
```

**Run with:**

```bash
go test -tags=integration ./providers/mypanel/...
```

---

## Step 6 — Capability probe + provider doctor

Add a `doctor` subcommand for your provider in `cmd/webox/doctor_mypanel.go`:

```go
func runDoctorMyPanel(ctx context.Context, profile string) error {
    // Run a sequence of probes:
    // 1. SSH connectivity
    // 2. Panel version detection
    // 3. Node runtime probe
    // 4. SSL provider probe
    // 5. Database engine probe
    // Output: human-readable + --json flag for machine-readable.
}
```

Probes are documented in `docs/providers/preconfiguration-vision.md §5.3` (cPanel), §6.3 (DirectAdmin), §7.3 (CyberPanel).

---

## Step 7 — Document your provider

Create `docs/providers/<name>.md` following `docs/providers/smallhost.md` as template. Required sections:

- **Status:** `Research` / `Candidate` / `Verified` / `Experimental`.
- **Panel characteristics.**
- **Mapping table:** `HostingProvider` method → panel command / endpoint.
- **Paths:** deploy / logs / env / `~/.htaccess`.
- **`Properties` bag:** what config fields your adapter accepts.
- **Edge cases & known issues.**
- **TODO / Open questions.**

---

## Step 8 — Submit the PR

PR template (auto-loaded from `.github/pull_request_template.md`):

```markdown
## Provider: <name>

- [ ] `docs/providers/<name>.md` complete.
- [ ] All `HostingProvider` methods implemented (or explicit `TODO` with rationale).
- [ ] Fixtures in `testing/fixtures/<name>/` with `.fixture.md` per fixture.
- [ ] Coverage ≥ 75 % for `providers/<name>.go`.
- [ ] Integration tests against `sshmock` pass.
- [ ] At least one manual test against real account (described in PR body).
- [ ] Marked as `experimental` (first version) — to be promoted after 1 minor release without critical issues.
- [ ] Linked open questions / future work in PR body.
```

**Request pair-review:** add `@maintainer-username` and write „Open to pair-review on this PR — happy to walk through together." Maintainer will respond within 7 working days with detailed feedback.

---

## Difficulty levels (rough estimate)

| Provider | Difficulty | Why |
|---|---|---|
| 🟢 **cPanel UAPI** | Easy-Medium | Well-documented OpenAPI, stable, mainstream. Application Manager handles Node.js. |
| 🟡 **DirectAdmin** | Medium | Two API generations (legacy + new JSON), CloudLinux Node.js Selector vs Nginx Unit. Capability detection important. |
| 🔴 **CyberPanel** | Hard | Mixed CLI + API, OpenLiteSpeed configs may need root, no native Node.js lifecycle. Many `TO BE VERIFIED` items. |
| 🟡 **Plesk** | Medium | Plesk CLI + REST API; multiple Node.js variants. Less open documentation than cPanel. |
| 🔴 **hPanel (Hostinger)** | Hard | Proprietary panel, no public API, depends on SSH workarounds. Possibly not feasible. |

---

## Maintainer pair-review commitment

For the first PR of any new adapter, the maintainer commits to:

- Initial response within 7 working days.
- Pair-review session (async or sync via Discord/email) within 14 days.
- Co-walking the first 50% of methods if you're unfamiliar with Go.
- Helping with sanitization checklist before fixtures land.
- Co-signing the PR description as „pair-reviewed adapter — first contribution."

Open an **issue** before you start coding (or use one of the `good-first-issue` provider issues). We'll align on approach, save you rework.

---

> _Last reviewed: 2026-05-25. Created as part of Sprint 15 docs refactor + launch readiness._
