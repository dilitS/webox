# Sprint 03 — Provider Contracts + `smallhost` Parser Skeleton

> **Daty:** TBD → TBD (planowane 1-2 tygodnie solo) · **Czas:** ~35-45h skupienia
>
> **Cel:** zbudować bezpieczny kontrakt `providers.HostingProvider` oraz pierwszy kawałek adaptera `smallhost` oparty o parsery outputu `devil`, bez jeszcze wykonywania pełnego kreatora projektu.

---

## TL;DR

Po sprincie 03:

- `providers/` ma stabilny interfejs `HostingProvider`, sentinele i registry.
- `providers/smallhost/` ma konstruktor, walidację configu i czyste path helpers.
- Parsery outputu `devil` dla subdomen, restartu, Node version i podstaw SSL/DB są TDD + fixtures-first.
- Adapter używa `ssh.Exec` z Sprintu 02, ale nie dodaje jeszcze TUI ani wizard flow.
- `testing/fixtures/devil/` ma provenance notes (`.fixture.md`) i malicious variants.

**Nie robimy w tym sprincie:**

- żadnej TUI,
- żadnego GitHub API/workflow writing,
- żadnego realnego deployu,
- żadnego drugiego providera,
- żadnego live fixture capture bez ręcznej sanitizacji i provenance note.

---

## Pre-flight Checklist

- [ ] Sprint 02 zamknięty z retro i `Outcome`.
- [ ] Read `docs/providers/smallhost.md` end-to-end.
- [ ] Read `docs/DESIGN.md §3`, `§4`, `§5`.
- [ ] Read `docs/SECURITY.md §3.3`, `§5`, `§10.4`.
- [ ] Confirm `make ci` green on `main`.

---

## Taski

### TASK-03.1 — `providers` interface + registry

- **Estymata:** M
- **Zależności:** Sprint 02 done
- **Acceptance Criteria:**
  - [ ] `providers/errors.go` with shared sentinels for config / unsupported provider / command output.
  - [ ] `providers/provider.go` defines `ProviderConfig`, `HostingProvider`, `DatabaseKind`, `DatabaseResult`, `Subdomain`, `Status`.
  - [ ] `providers/registry.go` supports `Register(name, factory)` and `New(config)`.
  - [ ] Registry rejects duplicate names and unknown providers with sentinels.
  - [ ] Tests cover duplicate registration, unknown provider, factory error propagation.
- **Docs:** `DESIGN.md §3`, `docs/providers/smallhost.md §2.2`.

### TASK-03.2 — `smallhost` constructor + config validation

- **Estymata:** M
- **Zależności:** TASK-03.1
- **Acceptance Criteria:**
  - [ ] `providers/smallhost.New(cfg)` validates alias/type/host/user/port.
  - [ ] Registers provider name `smallhost`.
  - [ ] Parses `properties.restart_method`, `ssh_pool_max`, `ssh_algorithms_legacy_compat`.
  - [ ] Rejects unsupported `restart_method`.
  - [ ] No hardcoded provider logic outside registry/factory.
- **Docs:** `AGENTS.md §2.2 Provider Pattern`, `docs/providers/smallhost.md §4`.

### TASK-03.3 — path helpers + validators

- **Estymata:** M
- **Zależności:** TASK-03.2
- **Acceptance Criteria:**
  - [ ] Pure helpers: `DeployPath(domain)`, `LogPath(domain)`, `EnvPath(domain)`, `StoragePath(domain)`.
  - [ ] Domain/subdomain validator follows `^[a-z0-9-]{1,63}$`, does not start/end with `-`.
  - [ ] Path helpers never accept `..`, `/`, whitespace injection.
  - [ ] Table-driven tests for valid/invalid domains.
- **Docs:** `docs/providers/smallhost.md §3`, `SECURITY.md §3.3`.

### TASK-03.4 — parser fixtures: `devil www add/list/restart`

- **Estymata:** L
- **Zależności:** TASK-03.1
- **Acceptance Criteria:**
  - [ ] Fixtures under `testing/fixtures/devil/` with `.fixture.md` provenance.
  - [ ] Parsers strip ANSI, reject >1MB output, tolerate `\r\n`.
  - [ ] `parseWwwAdd` handles success, exists, invalid node.
  - [ ] `parseWwwList` returns subdomains + node versions.
  - [ ] `parseWwwRestart` handles success, not found, not node app.
  - [ ] Malicious fixture with ANSI + command-injection-looking text does not leak into commands.
- **Docs:** `docs/providers/smallhost.md §2.1`, `AGENTS.md §4 TDD`.

### TASK-03.5 — parser fixtures: SSL + DB basics

- **Estymata:** L
- **Zależności:** TASK-03.4
- **Acceptance Criteria:**
  - [ ] `parseVhostList` extracts account IP needed for SSL.
  - [ ] `parseSSLAdd` handles success, DNS not configured, LE rate limit.
  - [ ] `parseDBAdd` extracts username/password without logging password.
  - [ ] `parseDBDelete` and `parseSSLDelete` treat not-found/no-cert as idempotent nil.
  - [ ] Fixture docs prove sanitization of any password-like values.
- **Docs:** `docs/providers/smallhost.md §2.1`, `SECURITY.md §3`, `§10.4`.

### TASK-03.6 — smallhost method skeleton over `ssh.Exec`

- **Estymata:** L
- **Zależności:** TASK-03.2, TASK-03.4, TASK-03.5
- **Acceptance Criteria:**
  - [ ] `CreateSubdomain`, `RestartNodeApp`, `CheckStatus`, `ListSubdomains` call `ssh.Exec`.
  - [ ] Command builder uses whitelist tokens, not shell escaping of raw user strings.
  - [ ] Errors map through sentinels using `errors.Is`.
  - [ ] Tests use fake executor interface and parser fixtures; no real SSH.
  - [ ] No secrets in logs/errors.
- **Docs:** `docs/providers/smallhost.md §2`, `SECURITY.md §3.3`.

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| Real `devil` output differs from drafted docs | H | Every parser fixture gets provenance; unknown output returns sentinel and safe diagnostic. |
| Parser accidentally logs DB password | H | Redactor tests already exist; DB parser tests must assert password is only in return struct, not error text. |
| Scope creep into full deploy wizard | M | Sprint 03 stops at provider methods + parsers. Wizard is later. |
| Provider interface churn | M | Keep interface minimal and map to PRD F3/F5/F6/F7/F8/F9 only. |

---

## Outcome (wypełnij po sprincie)

- ✅ Done: TASK-03.X, ...
- ⏭️ Carry-over: ...
- 📌 Decyzje: ...
- 🧠 Surprises: ...
- 📊 Metryki:
  - Coverage `providers/`: %
  - Coverage `providers/smallhost/`: %
  - Fixture count: %
- 🔒 Security validation:
  - [ ] No secrets in fixture files.
  - [ ] Parser malicious fixtures green.
  - [ ] `go test -race ./providers/... ./testing/...` green.
- ➡️ Następny sprint: `sprint-04-tui-shell.md`

---

## Retro Link

`docs/retros/YYYY-MM-DD-sprint-03.md`
