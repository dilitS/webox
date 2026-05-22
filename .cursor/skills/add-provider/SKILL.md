---
name: add-provider
description: Add a new hosting provider adapter to Webox (HostingProvider interface, factory registration, fixtures, docs, experimental flag). Use when the user asks to add cPanel, DirectAdmin, CyberPanel, or any new hosting panel, or when implementing the smallhost adapter from scratch.
---

# Add a Provider — Webox

## Overview

A "provider" is an adapter for one hosting panel (small.pl/Devil, cPanel UAPI, DirectAdmin JSON API, CyberPanel API). Every provider implements the **same contract** (`HostingProvider`) so the rest of Webox is panel-agnostic.

This skill is the canonical playbook from research to PR.

## Phases

```
- [ ] 1. Research   — capture real output, map methods to commands/endpoints
- [ ] 2. Doc        — write docs/providers/<name>.md (mirror smallhost.md)
- [ ] 3. Skeleton   — providers/<name>/ package, factory, doc.go
- [ ] 4. Parsers    — golden fixtures + parser per method (TDD)
- [ ] 5. Methods    — implement every HostingProvider method
- [ ] 6. Tests      — integration with sshmock + cassettes
- [ ] 7. Register   — providers.Register("<name>", New) in init()
- [ ] 8. Flag       — experimental gate via WEBOX_EXPERIMENTAL=1
- [ ] 9. PR         — checklist filled, CHANGELOG entry, ADR if needed
```

### 1. Research

For a new provider, capture **real** output before writing code:

```bash
# SSH to a test account
ssh testuser@new-provider.example.com

# Run each candidate command, save output verbatim
devil www add ...  > testing/fixtures/<name>/www_add_ok.txt
...
```

Sanitize: replace login → `testuser`, IP → `203.0.113.10`, email → `test@example.com`. Add `*.fixture.md` next to each fixture:

```markdown
captured: 2026-05-22
account: testuser@new-provider.example.com
command: <exact command>
sanitized: login -> testuser, IP -> 203.0.113.10
```

### 2. Doc

Mirror `docs/providers/smallhost.md` structure section-by-section:

- §1 Charakterystyka panelu (mechanism, API surface, quirks).
- §2 Mapowanie metod `HostingProvider` na komendy/endpointy (table + signature reference).
- §3 Ścieżki plików (deploy / logs / .env location).
- §4 Properties bag (provider-specific config knobs).
- §5 Edge cases i znane dziwactwa.
- §6 Deployment workflow szablon (with rsync excludes and .env perm check).
- §7 Otwarte pytania / TODO (TO BE VERIFIED items).

**Do not** copy-paste — research-derived content is unique per provider.

### 3. Skeleton

```text
providers/<name>/
├── doc.go             # package doc
├── provider.go        # SmallHostProvider equivalent
├── parse_*.go         # one file per parser
├── parse_*_test.go    # tests
├── errors.go          # sentinel errors
└── workflow.tmpl.yml  # GHA template (//go:embed)
```

`doc.go`:

```go
// Package <name> implements the HostingProvider interface for <Panel Name>.
//
// <Panel Name> is a <web|cli> hosting panel with <key characteristic>.
// This adapter speaks <SSH+CLI|REST|UAPI> and is suitable for <market segment>.
//
// Reference docs: ../../docs/providers/<name>.md
package <name>
```

### 4. Parsers (TDD)

Run the `tdd-loop` skill for **every** parser. Cycle through:

```
parse_www_add.go    + parse_www_add_test.go    + 4 fixtures
parse_mysql_add.go  + parse_mysql_add_test.go  + 4 fixtures
parse_ssl_add.go    + parse_ssl_add_test.go    + 4 fixtures
parse_www_list.go   + parse_www_list_test.go   + 4 fixtures
...
```

Minimum 4 fixtures per parser: `success`, `known_error`, `malicious_ansi`, `format_unknown`.

### 5. Methods

Implement every method on `*<Name>Provider`. **Do not** stub.

```go
func (p *<Name>Provider) CreateSubdomain(ctx context.Context, domain string, nodeVersion string) error {
    // 1. Validate input via local validator.
    // 2. Build command via sanitizer (no fmt.Sprintf for user input).
    // 3. pool.Acquire → exec → release.
    // 4. Parse output via parse_www_add.
    // 5. Return sentinel error from errors.go on known failures.
}
```

Idempotent `Remove*` methods: missing resource == `nil`.

### 6. Tests

```go
// Integration with sshmock
func TestProvider_FullWizard(t *testing.T) {
    server := sshmock.Start(t,
        sshmock.OnCommand(`devil www add .*`, "testing/fixtures/<name>/www_add_ok.txt", 0),
        sshmock.OnCommand(`devil ssl www add .*`, "testing/fixtures/<name>/ssl_add_ok.txt", 0),
        sshmock.OnCommand(`devil mysql add .*`, "testing/fixtures/<name>/mysql_add_ok.txt", 0),
    )
    defer server.Stop()

    p := New(testProviderConfig(server.Addr()))
    ctx := context.Background()

    if err := p.CreateSubdomain(ctx, "test.example.com", "24"); err != nil {
        t.Fatal(err)
    }
    if err := p.SetupSSL(ctx, "test.example.com"); err != nil {
        t.Fatal(err)
    }
    _, _, err := p.CreateDatabase(ctx, "mysql", "test_db")
    if err != nil {
        t.Fatal(err)
    }
}
```

### 7. Register

In `providers/<name>/provider.go`:

```go
func init() {
    providers.Register("<name>", New)
}

func New(cfg providers.ProviderConfig) (providers.HostingProvider, error) {
    if err := validateConfig(cfg); err != nil {
        return nil, err
    }
    return &<Name>Provider{cfg: cfg}, nil
}
```

In `cmd/webox/main.go` or `providers/imports.go`:

```go
import (
    _ "github.com/dilitS/webox/providers/smallhost"
    _ "github.com/dilitS/webox/providers/<name>"  // ← new
)
```

### 8. Experimental flag

First version is gated:

```go
providers.Register("<name>", New, providers.Experimental)
```

UI hides the provider type in profile creation unless `WEBOX_EXPERIMENTAL=1`. Flag promoted to `Stable` after:

- 1 minor release without critical issues.
- Manual sign-off from maintainer.
- Real-account testing (not just sshmock).

### 9. PR

Use the `commit-policy` skill for commits. PR template:

```markdown
## Provider: <name>

- [ ] `docs/providers/<name>.md` complete (mirrors smallhost.md structure).
- [ ] All HostingProvider methods implemented (no stubs).
- [ ] Fixtures in `testing/fixtures/<name>/` with `.fixture.md` metadata.
- [ ] Coverage ≥ 75 % for `providers/<name>/`.
- [ ] Integration tests pass against `sshmock`.
- [ ] At least one manual test against real account (described below).
- [ ] Registered with `providers.Experimental` flag.
- [ ] Workflow template (`workflow_deploy_<name>.tmpl.yml`) embedded.
- [ ] `make ci` passes locally.
- [ ] CHANGELOG.md `[Unreleased] / Added` entry.
- [ ] Linked open questions in PR body.

## Manual test description

Account: testuser@new-provider.example.com
Scenarios tested:
- New Vite+React project end-to-end (no DB).
- New Node/Express project with MySQL.
- Restart, view logs, SSL renew on existing project.
- Wizard failure mid-step → LIFO rollback completes cleanly.
```

## Done criteria

- [ ] Provider doc complete with TO BE VERIFIED items resolved or documented.
- [ ] All parsers covered by TDD with 4+ fixtures each.
- [ ] All HostingProvider methods implemented, none stubbed.
- [ ] sshmock integration suite green.
- [ ] At least one full wizard test against real account.
- [ ] Provider registered as experimental.
- [ ] Workflow template embedded.
- [ ] Maintainer sign-off for `Stable` promotion (separate PR).
