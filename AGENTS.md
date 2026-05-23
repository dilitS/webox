# AGENTS.md — Webox

> Operator handbook dla AI coding agents pracujących nad Webox.
>
> Status: Stable for v0.1 implementation phase · Ostatnia aktualizacja: 2026-05-22 · Właściciel: @maintainer
>
> Ten plik jest **kontraktem** między człowiekiem-maintainerem a agentem. Agent czyta go **przed** każdym task'em. Jeśli zalecenie z `AGENTS.md` koliduje z user request, **agent zatrzymuje się i pyta**.

---

## TL;DR

Webox to **monolit w Go 1.24+** z TUI opartym o Bubble Tea/Lipgloss, GoReleaser do dystrybucji, `golangci-lint v2` jako linter, **MVP scope = small.pl/Devil only**. Implementacja jest **docs-first** — jakikolwiek kod sprzeczny z `docs/` zostaje odrzucony w review. **TDD jest obowiązkowe** dla: parserów outputu providerów, redaktora sekretów, status cache, walidatorów, maszyny stanów TUI. Sekrety **nigdy** nie wchodzą do logów ani plików tekstowych poza `keyring`/`secrets.enc`. Commity są **Conventional Commits 1.0.0** bez gitmoji. Każda znacząca zmiana → wpis w `CHANGELOG.md` sekcji `[Unreleased]`.

---

## Spis treści

1. [Stack technologiczny](#1-stack-technologiczny)
2. [Guardrails — co MUSI być zachowane](#2-guardrails--co-musi-być-zachowane)
3. [Co JEST i NIE JEST w MVP](#3-co-jest-i-nie-jest-w-mvp)
4. [Workflow implementacji (TDD)](#4-workflow-implementacji-tdd)
5. [Konwencje kodu Go](#5-konwencje-kodu-go)
6. [Konwencje commitów i PR](#6-konwencje-commitów-i-pr)
7. [Częste pułapki i jak ich unikać](#7-częste-pułapki-i-jak-ich-unikać)
8. [Kiedy ZADAWAĆ pytania, kiedy DECYDOWAĆ](#8-kiedy-zadawać-pytania-kiedy-decydować)
9. [Retrospektywa per task](#9-retrospektywa-per-task)

---

## 1. Stack technologiczny

### 1.1 Język i toolchain

| Element | Wersja / Wybór | Uzasadnienie |
|---|---|---|
| Język | **Go 1.24+** | `go 1.24` w `go.mod`. CI matrix testuje 1.24 + 1.25-rc gdy dostępne. |
| Module system | Go modules | `go.mod` + `go.sum`; `vendor/` nie commitujemy. |
| Build tool | `go build` + Makefile | `make build`, `make test`, `make lint`. |
| Linter | `golangci-lint v2.x+` | Config: `.golangci.yml` z `version: "2"`. Nazwy v2: `gas→gosec`, `goerr113→err113`, `gomnd→mnd`. |
| Formatter | `gofumpt` + `goimports` | `make fmt`. Ostro: nic poza nimi. |
| Vulnerability scan | `govulncheck` | CI gate (`make vulncheck`). |
| Release | `GoReleaser 2.x` | `make snapshot` / `make release-dry-run`. |
| Signing | `cosign` (keyless OIDC) + SLSA | Obowiązkowe od v0.1. |
| Coverage | `go test -coverprofile=` + `make cover-check` | Minimum 70% (MVP), 80% (v0.2). |

### 1.2 Kluczowe biblioteki (sprawdzone przez Context7)

| Pakiet | Cel | Uwagi krytyczne |
|---|---|---|
| `github.com/charmbracelet/bubbletea` | TUI MVU framework | `tea.Cmd`, `Update()`, `View()` — patrz [DESIGN §2.3](./docs/DESIGN.md#23-zasady-przep%C5%82ywu-danych-mvu). |
| `github.com/charmbracelet/lipgloss` | Terminal styling | Deklaratywny, OKLCH colors. **Nie** używać do 60fps renderingu. |
| `github.com/charmbracelet/x/exp/teatest` | TUI testing harness | **Eksperymentalna** ścieżka — pinujemy commit hash w `go.mod`, nie `latest`. |
| `github.com/charmbracelet/bubbles` | Common TUI components | Spinner, textinput, table — używamy gdy pasują. |
| `golang.org/x/crypto/ssh` | SSH client | Native Go, **bez** zależności od systemowego `ssh`. |
| `github.com/pkg/sftp` | SFTP nad SSH | Atomic put: `<file>.tmp` + `Rename`. |
| `github.com/zalando/go-keyring` | Keyring | Sentinel errors: `ErrUnsupportedPlatform`, `ErrNotFound`. **Detekcja przez probe write/read/delete** (patrz [SECURITY §4.2](./docs/SECURITY.md#42-fallback-dla-środowisk-headless)). |
| `github.com/gofrs/flock` | Cross-platform file locks | Dla `config.json` lock. **NIE polegamy na PID check** (patrz [DESIGN §6](./docs/DESIGN.md#6-model-danych-i-atomowość-zapisu-configjson)). |
| `github.com/awnumar/memguard` | Locked secret buffers | `LockedBuffer.Destroy()` zamiast wymyślonego `zerocopy.Wipe`. |
| `golang.org/x/sync/singleflight` | Deduplikacja równoległych fetch'ów | Dla SWR cache (patrz [DESIGN §8](./docs/DESIGN.md#8-trójpoziomowy-status-cache-stale-while-revalidate)). |
| `crypto/rand` | CSPRNG | **Jedyne** dopuszczalne źródło nonce dla AES-GCM (patrz [SECURITY §4.2.1](./docs/SECURITY.md#421-generowanie-nonce-krytyczne-dla-aes-gcm)). |
| `golang.org/x/crypto/argon2` | KDF | Parametry: `memory=64MB, iterations=3, parallelism=2`. |
| `gopkg.in/yaml.v3` | YAML parsing | Dla walidacji `deploy.yml` przed commit do repo. |
| `gopkg.in/natefinch/lumberjack.v2` | Log rotation | `webox.log` rotuje przez tę bibliotekę. |
| `github.com/google/uuid` | UUID v4 dla `projects[].id` | Nigdy nie sekwencyjny ID. |

### 1.3 Struktura repo (planowana)

Pełna lista w [`docs/DESIGN.md §2.1`](./docs/DESIGN.md#21-layout-repo). Najważniejsze:

```text
cmd/webox/         entrypoint
tui/               Bubble Tea state machine
providers/         HostingProvider interface + adapters
ssh/               connection pool + sftp
config/            config.json + migrations + flock
secrets/           keyring + AES-GCM fallback + redactor
status/            SWR cache
wizard/            LIFO rollback stack (DAG → v0.3+)
services/          GitHub API client
i18n/              translation loader
assets/            //go:embed workflow templates + doctor schema
testing/           fixtures, sshmock, ghmock cassettes
```

**Zasada cardinala:** każdy nowy pakiet ma `doc.go` z opisem co i dlaczego.

---

## 2. Guardrails — co MUSI być zachowane

> Te zasady są **niepodlegające negocjacji** bez explicit override od maintainera. Agent który je łamie, **traci review**.

### 2.1 Bezpieczeństwo

| Reguła | Mechanizm |
|---|---|
| **Sekret NIGDY w `config.json`** | JSON Schema validation. `config/validate.go` rzuca `ErrSecretInConfig` jeśli wzorzec (`ghp_`, `ghs_`, `github_pat_`, `sk-`, `BEGIN ... PRIVATE KEY`) jest matched. |
| **Sekret NIGDY w log** | `internal/log/redact.go` z regex listą (patrz [DESIGN §15.2](./docs/DESIGN.md#152-redacted-logger--wzorce)). Każdy nowy wzorzec dodajemy razem z testem. |
| **Sekret NIGDY w stack trace** | `defer recover()` w głównej goroutine sanitizuje. Variables z `password`/`token`/`key` w nazwie nie są loggowane przy crash. |
| **AES-GCM nonce TYLKO z `crypto/rand`** | Zabronione: `time.Now()`, licznik, hash deterministyczny. Test jednostkowy weryfikuje że dwa kolejne write'y dają różne nonce. |
| **SSH host key — strict block on mismatch** | Brak `--insecure`, brak `auto-accept`. TOFU tylko przy pierwszym połączeniu z out-of-band confirmation. |
| **Workflow templates osadzone przez `embed.FS`** | Brak dynamicznego pobierania z sieci. Pinned SHA versions dla GitHub Actions w workflow (`uses: actions/checkout@<sha>`, **nie** `@v4`). |
| **`.env` na serwerze: `0600`, poza web root** | Post-deploy SSH check w `deploy.yml` failuje deploy jeśli perms inne lub w `public_html/`. |

### 2.2 Architektura

| Reguła | Co konkretnie |
|---|---|
| **Provider Pattern jest świętością** | Logika biznesowa **nigdy** nie zna `smallhost` po nazwie. Wszystko przez `providers.HostingProvider`. Nawet w MVP gdy jest jeden provider. |
| **`Update()` jest pure function** | Brak `os.*`, `net.*`, channels w `tui/update.go`. Wszystko I/O w `tea.Cmd`. |
| **`View()` jest pure function** | Brak mutacji stanu w `View`. Tylko `Model` → `string`. |
| **Brak globalnych mutable variables** | Wszystko w `Model` lub explicit struct. Wyjątek: `init()` provider registry (zamknięty po starcie). |
| **Wszystkie I/O przez `context.Context`** | Każda metoda providera, każde wywołanie SSH, każde HTTP — `ctx.Done()` musi być respektowany. |
| **Sentinel errors w `errors.go` per pakiet** | `var ErrSubdomainExists = errors.New("provider: subdomain exists")`. **Nigdy** comparing przez `err.Error() == "..."`. Zawsze `errors.Is(err, ErrSubdomainExists)`. |
| **Idempotentne `Remove*`** | Brak zasobu == `nil`. Inaczej DAG/LIFO rollback fragile. |
| **`config.json` save = atomic** | `flock(2)` → write tmp → `fsync` → `rename` → `fsync(dir)`. Nigdy direct write. |

### 2.3 Testowanie

| Reguła | Mechanizm |
|---|---|
| **TDD dla parserów, walidatorów, redaktora, cache** | Test pierwszy, kod drugi. Patrz [§4 Workflow](#4-workflow-implementacji-tdd). |
| **Każdy fixture devil ma `.fixture.md`** | Pochodzenie, sanityzacja, data capture. |
| **Każdy parser ma golden file z malicious input** | `\r\n` injection, ANSI escape, 1MB+ output, mock błędnego formatu. |
| **`go test -race` w CI** | Brak path do merge'a bez `-race`. |
| **TUI testy przez `teatest` snapshot** | Stripped ANSI w snapshot. Kolory walidowane manualnie pre-release. |
| **Coverage ≥ 70% (MVP), ≥ 80% (v0.2)** | `make cover-check`. CI fails poniżej. |

### 2.4 Scope discipline

| Reguła | Konsekwencja |
|---|---|
| **MVP = small.pl/Devil only** | Wszelkie referencje do `cpanel`/`directadmin`/`cyberpanel` w kodzie poza `provider.go` interface = automatic reject. |
| **Sound engine, Bento Ultra, Topology Map, Env Merger, Live log stream — STRETCH v0.2+** | Każda implementacja tych ficzerów w MVP PR-ze = automatic reject. Wyjątek: explicit ADR + maintainer sign-off. |
| **Brak operatorskich CLI commands poza `webox doctor`** | Operacje typu create/restart/import/provider CRUD idą przez TUI. Dozwolone są ograniczone startup/debug/diagnostic flags opisane w [ADR-0001](./docs/adr/0001-tui-zamiast-cli.md). |
| **DAG-based rollback engine = v0.3+** | MVP używa LIFO stack. Patrz [DESIGN §10](./docs/DESIGN.md#10-dag-based-transactional-engine-wznawialny-rollback). |

---

## 3. Co JEST i NIE JEST w MVP

### 3.1 W MVP v0.1 (musi działać przed release)

- Init wizard (pierwsze uruchomienie, generowanie SSH keypair, auto-deploy klucza publicznego).
- Provider profile management dla `smallhost`.
- Project creation wizard: subdomain → SSL → DB (opcjonalnie) → GitHub repo + secrets + workflow → first deploy.
- Dashboard: lista projektów + detail panel (Overview only).
- Status: HTTP ping, SSL cert info, Node version, last deploy.
- Restart, view logs (`tail -n 200`), SSL renew.
- Import existing projects (read-only, detects gaps).
- LIFO rollback przy częściowym fail wizard'a.
- Keyring secrets + AES-GCM fallback z Argon2id.
- `webox doctor` + `webox doctor --json`.
- Host-key mismatch resolution przez TUI phrase-confirm flow w `v0.1`; opcjonalna CLI (`webox doctor security --update-host-key`) dopiero `v0.2+`.

### 3.2 NIE w MVP (STRETCH v0.2+)

- Live log stream (`tail -f` via SSH).
- `/db`, `/env`, `/storage`, `/domain` tabs i ich Command Palette commands.
- TUI Env Merger (interactive `.env` diff/merge).
- Multi-provider dashboard agregujący projekty z różnych serwerów.
- DAG-based transactional engine (LIFO wystarcza dla MVP).
- Sound engine (osobny RFC post-v0.2).
- Bento Ultra dashboard (≥120×35) — MVP target = Standard Cockpit (100×30).
- Fast-chord bindings (`g r`, `g d`, etc.).
- Live Service Topology Map.
- `webox auth login github` (OAuth Device Flow) — MVP używa `gh` CLI lub PAT.
- Drugi provider (cPanel, DirectAdmin, CyberPanel) — research only.
- Jump host / SSH bastion / ProxyJump.
- In-app updater.
- Non-interactive operatorskie CLI commands poza `webox doctor`.

### 3.3 Czego NIGDY nie będziemy robić

- Zdalna telemetria. Zero. Cokolwiek.
- Plugin marketplace (security blast radius).
- Webox jako vault (Webox **orkiestruje** sekrety, nie jest 1Password/Vault).
- VPS provisioning (Coolify/Forge robią to lepiej).
- Docker / Kubernetes management.

---

## 4. Workflow implementacji (TDD)

### 4.1 Każdy task ma 5 faz

> Skill `tdd-loop` w `.cursor/skills/tdd-loop/SKILL.md` ma rozwiniętą procedurę. Tu — esencja.

```
1. Read    → przeczytaj relevantny PRD/DESIGN/UX/SECURITY/TESTING fragment.
              Sprawdź czy issue/feature ma już docs entry. Jeśli nie — pytaj.
2. Plan    → opisz w 3-5 zdaniach co zamierzasz zrobić. Zaznacz na które
              guardrailse to wpływa.
3. Red     → napisz failing test FIRST (parser → fixture, validator → table-driven).
              Test musi failować z konkretnym komunikatem ("expected X, got Y").
4. Green   → napisz minimalny kod żeby test przeszedł. Brak premature
              abstraction. Brak refactor'u "przy okazji".
5. Refactor→ po zielonym teście — refactor. Linter, golangci-lint, fmt.
              Dodaj wpis do CHANGELOG.md `[Unreleased]`. Commit conventional.
```

### 4.2 Kiedy TDD jest twardo obowiązkowe

- Parsery outputu providera (`devil www add`, `devil mysql add`, etc.).
- Walidatory wejścia użytkownika (regex subdomeny, port, alias).
- Redaktor sekretów (`internal/log/redact.go`).
- Status cache (SWR semantics, TTL, race).
- Maszyna stanów TUI (`tui/update.go`).
- Keyring detekcja (`secrets/keyring.go`).
- `config.json` load/save/migracje.
- `pending_cleanups.json` serialization + resume.

### 4.3 Kiedy TDD jest soft (testy after, nie before)

- Bubble Tea View rendering (snapshot via `teatest`, ale nie "test-first" — najpierw makieta z `docs/UX.md`, potem snapshot).
- Wrappers zewnętrznych bibliotek (SFTP put/get — testowane przez integration z mock SSH).
- Glue code w `cmd/webox/main.go`.

### 4.4 Procedura uruchomienia testów lokalnie

```bash
make tidy       # sync modules
make lint       # golangci-lint v2
make test       # -race -coverprofile
make cover-check # ≥70% threshold
make vulncheck  # govulncheck
make ci         # cały bundle, identyczny do CI
```

Jeśli `make ci` przechodzi lokalnie, CI przejdzie. Jeśli CI failuje a lokalnie nie — bug w `Makefile`, nie w teście.

---

## 5. Konwencje kodu Go

### 5.1 Naming

```go
// Pakiet: krótki, lowercase, bez _ ani myślnika.
package providers

// Eksportowane typy / funkcje: PascalCase.
type HostingProvider interface { /* ... */ }
func RegisterProvider(name string, factory Factory) {}

// Wewnętrzne: camelCase.
func parseDevilOutput(raw []byte) (*Result, error) {}

// Sentinel errors: ErrFoo.
var ErrSubdomainExists = errors.New("provider: subdomain already exists")

// Interface: rzeczownik bez I prefix.
// ❌ type IHostingProvider interface
// ✅ type HostingProvider interface

// Constructor: NewFoo, nie MakeFoo.
func NewPool(maxConn int) *Pool {}
```

### 5.2 Error handling

```go
// Wrap z context.
if err := pool.Acquire(ctx, host); err != nil {
    return fmt.Errorf("acquire ssh session for %s: %w", host, err)
}

// Compare przez errors.Is.
if errors.Is(err, providers.ErrSubdomainExists) {
    // handle idempotent case
}

// Typed errors gdy potrzeba payload (rzadko).
type ParseError struct {
    Source string
    Line   int
    Err    error
}
func (e *ParseError) Error() string { return fmt.Sprintf("...") }
func (e *ParseError) Unwrap() error { return e.Err }
```

### 5.3 Context discipline

```go
// ✅ Każda metoda I/O ma ctx.
func (p *SmallHostProvider) CreateSubdomain(ctx context.Context, domain string) error {
    if err := ctx.Err(); err != nil {
        return err // early exit jeśli już cancelled
    }
    // ...
}

// ❌ Nie zapisuj ctx jako pola struktury (poza wyjątkami typu Server).
type BadProvider struct {
    ctx context.Context // ❌ anti-pattern (poza krótko żyjącymi obiektami).
}
```

### 5.4 Generics

```go
// Go 1.24 nadal NIE pozwala na generic methods.

// ❌ NIE DZIAŁA:
func (c *Cache) GetOrFetch[T any](key string, fetch func() T) T {}

// ✅ Funkcja pakietowa zamiast:
func GetOrFetch[T any](c *Cache, key string, fetch func() T) T {}

// Patrz docs/DESIGN.md §8.4.
```

### 5.5 Logging

```go
import (
    "log/slog"

    "github.com/dilitS/webox/internal/log"
)

// ✅ structured logging z slog.
log.Info("ssh connection established",
    "host", profile.Host,
    "user", profile.User,
    "duration_ms", elapsed.Milliseconds(),
)

// ❌ NIGDY:
fmt.Printf("connecting to %s with password %s\n", host, password) // ❌ secret leak
log.Info("token", "value", token)                                  // ❌ secret leak
```

### 5.6 Testing helpers

```go
// t.Helper() w każdej helper-funkcji.
func assertEnvFileFormat(t *testing.T, content string) {
    t.Helper()
    if !strings.Contains(content, "DATABASE_URL=") {
        t.Fatalf("expected DATABASE_URL in env file, got: %s", content)
    }
}

// Table-driven dla parserów.
func TestParseDevilWwwAdd(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *AddResult
        wantErr error
    }{
        {
            name:  "success",
            input: loadFixture(t, "devil/www_add_ok.txt"),
            want:  &AddResult{Domain: "test.user.smallhost.pl"},
        },
        {
            name:    "exists",
            input:   loadFixture(t, "devil/www_add_exists.txt"),
            wantErr: ErrSubdomainExists,
        },
        // ... wszystkie golden files.
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := parseDevilWwwAdd(tt.input)
            if !errors.Is(err, tt.wantErr) {
                t.Errorf("err = %v, want %v", err, tt.wantErr)
            }
            // ...
        })
    }
}
```

---

## 6. Konwencje commitów i PR

### 6.1 Conventional Commits 1.0.0 (bez gitmoji)

```
<type>(<scope>): <opis ≤72 chars>

<body opcjonalny — DLACZEGO, nie CO>

<footer opcjonalny — BREAKING CHANGE, Refs: #123>
```

| `type` | Kiedy używać |
|---|---|
| `feat` | Nowy ficzer end-user-visible. |
| `fix` | Bug fix. |
| `refactor` | Zmiana struktury bez zmiany behavior. |
| `perf` | Optymalizacja wydajności. |
| `test` | Dodanie/edycja testów. |
| `docs` | Tylko docs/. |
| `chore` | Tooling, deps, config. |
| `ci` | Workflow CI. |
| `build` | GoReleaser, Makefile. |
| `revert` | Rewert poprzedniego commit'a. |

`<scope>`:

- Pakiety top-level: `providers`, `tui`, `ssh`, `config`, `secrets`, `status`, `wizard`, `services`.
- Feature areas: `audit`, `scope`, `ci`, `release`.

### 6.2 Przykłady dobrego commit'a

```
feat(providers): add smallhost CreateSubdomain implementation

Implements HostingProvider.CreateSubdomain for small.pl/Devil using
`devil www add <domain> nodejs <version>`. Parser handles three
known outputs: success, exists, invalid node version.

Refs: #42
```

```
fix(secrets): correctly detect headless keyring via probe sentinel

Distinguishes ErrUnsupportedPlatform (true fallback trigger) from
ErrNotFound (secret simply not stored yet). Previous code treated
both as fallback signal, breaking normal first-run on macOS.

Refs: AUDIT A1
```

### 6.3 Anti-patterns w commitach

```
❌ "fix bug"                              — co? gdzie? dlaczego?
❌ "wip"                                  — nigdy do main.
❌ ":sparkles: add feature"               — bez gitmoji.
❌ "update files"                         — bez kontekstu.
❌ "merge branch 'main' into feature/x"   — squash & merge zamiast tego.
```

### 6.4 PR structure

```markdown
## Summary
- Co PR robi w 2-3 bullet'ach.

## Motivation
- Dlaczego (issue link, audit finding, user pain).

## Implementation notes
- Architectural decyzje, trade-offs, ryzyko.

## Test plan
- [ ] Unit tests for parser X
- [ ] Integration test with mock SSH
- [ ] Manual: `make doctor` exits 0

## Checklist
- [ ] `make ci` passes locally
- [ ] CHANGELOG.md [Unreleased] entry added
- [ ] Docs updated (if behavior change)
- [ ] No secrets in any committed file
- [ ] Linked AUDIT finding (if applicable)
```

---

## 7. Częste pułapki i jak ich unikać

### 7.1 Top 10 gotchas (wszystkie zaobserwowane podczas audytu)

| Pułapka | Anti-pattern | Co robić |
|---|---|---|
| **Keyring detection przez `Get` bez kontekstu** | `if _, err := keyring.Get(...); err != nil { fallback() }` | Probe write+read+delete sentinela. `errors.Is(err, keyring.ErrUnsupportedPlatform)` → fallback. `ErrNotFound` na probe Get **po** Set = bug backendu, nie brak keyringa. |
| **AES-GCM nonce z `time.Now()`** | `nonce := make([]byte,12); binary.BigEndian.PutUint64(nonce, uint64(time.Now().UnixNano()))` | `if _, err := rand.Read(nonce); err != nil { panic(err) }`. |
| **PID-based lockfile** | Sprawdzanie czy `os.FindProcess(pid)` zwraca proces | `flock(2)` via `gofrs/flock`. PID jest re-wykorzystywany. |
| **Generic method na struct** | `func (c *Cache) Get[T any]() T {}` | Funkcja pakietowa `GetOrFetch[T any](c *Cache, ...)`. Go 1.24 wciąż nie wspiera. |
| **`go get` cicho podbija `go` directive** | `go get <dep>@latest` i commit z `go 1.25.0` mimo że projekt miał zostać na `go 1.24` | Po **każdym** dodaniu depa przeczytaj `go.mod`, sprawdź transitive `go.mod` nowych modułów i pinuj ostatnią wersję kompatybilną z naszym floor (`go 1.24`). |
| **Hardcoded provider name w business logic** | `if profile.Type == "smallhost" { ... }` | `provider := registry.New(profile); provider.CreateSubdomain(...)`. |
| **`os.Rename` na różnych filesystem** | Próba rename z `/tmp/` do `~/.config/` (różne mount). | Tmp file **w tym samym katalogu** co target. |
| **Sekret w `fmt.Errorf("...%s...", password)`** | Error message zawiera plaintext sekret. | `fmt.Errorf("auth failed: %w", ErrInvalidCredentials)`. |
| **`rsync --delete` bez `--exclude`** | Usuwa `public/uploads/`, `.env` na zdalnym. | Zawsze excludes (patrz `providers/smallhost.md §6`). |
| **GitHub Actions `uses: actions/checkout@v4`** | Tag może być przepisany przez supply-chain attack. | `uses: actions/checkout@<full-40-char-SHA>`. |
| **`t.Parallel()` w testach stubujących package-level globals** | Test podmienia `dispatchDoctor`, `newRunner`, `nowFn` i jednocześnie inne testy lecą równolegle | Takie testy uruchamiaj **sekwencyjnie** albo chowaj seam za instancją / mutexem. `go test -race` ma być zielony bez wyjątków. |
| **Bubble Tea side effect w `Update`** | `case KeyMsg: go fetchData()` (raw goroutine) | `case KeyMsg: return m, fetchDataCmd(ctx)` (return `tea.Cmd`). |

### 7.2 Linter zignorowany przez `//nolint` bez powodu

```go
// ❌ NIE rób tego:
//nolint:gocyclo
func setupSSL(...) {}

// ✅ Wymagane uzasadnienie:
//nolint:gocyclo // SetupSSL: 7 explicit error paths (rate limit, DNS,
//              // existing cert, ACME, auth, network, parse), splitting
//              // would reduce readability without measurable benefit.
func setupSSL(...) {}
```

### 7.3 Tworzenie nowych adapterów providerów

Jeśli pracujesz nad nowym providerem (post-MVP):

1. Przeczytaj `docs/providers/smallhost.md` **w całości** — to wzorzec.
2. Skopiuj strukturę pliku (`docs/providers/<nazwa>.md`).
3. Zarejestruj fabrykę: `providers.Register("<nazwa>", New)` w `init()`.
4. Implementuj **wszystkie** metody `HostingProvider` (nie wybiórczo).
5. Pierwszy release providera = `experimental` flag (`WEBOX_EXPERIMENTAL=1`).

Patrz [CONTRIBUTING §3](./docs/CONTRIBUTING.md#3-jak-doda%C4%87-nowy-provider).

---

## 8. Kiedy ZADAWAĆ pytania, kiedy DECYDOWAĆ

> Agent ma uprawnienia do **technicznych** decyzji w wąsko określonych obszarach. **Produktowe** i **architektoniczne** decyzje wymagają maintainera.

### 8.1 Decyduj sam (don't ask, just do)

- Wybór konkretnej regex dla walidatora gdy spec mówi `^[a-z0-9-]{1,63}$`.
- Nazewnictwo funkcji wewnętrznych w pakiecie (jeśli zgodne z [§5.1](#51-naming)).
- Wybór nazw plików testowych (`*_test.go`).
- Decyzja czy table-driven czy plain test.
- Czy użyć `t.Run` subtests.
- Dodanie wpisu do `CHANGELOG.md [Unreleased]`.
- Dodanie testu na świeżo poprawiony bug.
- Refactor wewnątrz pakietu po `go test ./...` zielony.

### 8.2 Zapytaj maintainera (ask first)

- Dodanie nowej zewnętrznej zależności (`go.mod`).
- Zmiana publicznego API (`HostingProvider`, `ProviderConfig`).
- Decyzja czy ficzer jest MVP czy STRETCH (patrz [§3](#3-co-jest-i-nie-jest-w-mvp), jeśli nie ma jasnego mappingu).
- Zmiana struktury katalogów top-level.
- Zmiana strategii rollback (LIFO → DAG migration).
- Dodanie nowego pliku w `docs/adr/`.
- Dodanie nowej flagi CLI / env var.
- Zmiana którejkolwiek z guardrails (`AGENTS.md §2`).

### 8.3 Jak pytać efektywnie

```markdown
**Kontekst:** Pracuję nad `providers/smallhost/ssl.go`. SECURITY §5.5
mówi że Webox eksplicitnie deklaruje dozwolone algorytmy SSH.

**Pytanie:** Czy `properties.ssh_algorithms_legacy_compat=true` powinien
być per-profile (jak teraz w specu) czy per-projekt (bo różne projekty
mogą trafiać na różne node'y small.pl)?

**Moja propozycja:** Per-profile. Argument: w MVP cały profil to jeden
host SSH, więc niższe granularne ustawienie nie ma sensu.

**Alternatywa:** Per-host w `known_hosts` przy `properties` per wpis.

**Co wybrać?**
```

---

## 9. Retrospektywa per task

### 9.1 Po każdym ukończonym task'u — 3 pytania

> Skill `retro` w `.cursor/skills/retro/SKILL.md` ma rozwinięty template.

```markdown
## Retro: <task name>

**Co działało dobrze?**
- ...

**Co poszło źle / wymagałoby ulepszenia?**
- ...

**Co zmieniam w workflow / dokumentacji?**
- ...
```

### 9.2 Po każdym sprint'cie / fazie (eskalacja)

- Update `docs/AUDIT.md` jeśli pojawiły się nowe znaleziska.
- Rewizja `AGENTS.md` jeśli pojawiła się nowa pułapka (`§7`).
- Aktualizacja `.cursor/skills/*` jeśli skill okazał się za wąski/za szeroki.
- Audit `make ci` overhead — czy CI nie rośnie ponad SLA.

---

## 10. Materiały referencyjne

- [PRD.md](./docs/PRD.md) — co i dla kogo budujemy.
- [DESIGN.md](./docs/DESIGN.md) — jak to działa technicznie.
- [UX.md](./docs/UX.md) — jak to wygląda i zachowuje się dla usera.
- [SECURITY.md](./docs/SECURITY.md) — threat model i polityka sekretów.
- [TESTING.md](./docs/TESTING.md) — strategia testowa, piramida, mock'i.
- [ROADMAP.md](./docs/ROADMAP.md) — co kiedy, kryteria GA.
- [CONTRIBUTING.md](./docs/CONTRIBUTING.md) — workflow contributora.
- [AUDIT.md](./docs/AUDIT.md) — 39 znalezisk przed-implementacyjnych.
- [adr/](./docs/adr/) — sześć ADR-ów z architecture rationale.
- [providers/smallhost.md](./docs/providers/smallhost.md) — wzorzec dla providerów.
- [CHANGELOG.md](./CHANGELOG.md) — historia zmian.

---

> **Reguła ostateczna:** *Wątpisz? Pytaj. Pewny? Działaj. Działa? Przetestuj. Przetestowane? Skomituj. Skomitowane? Zaktualizuj `CHANGELOG.md`.*
