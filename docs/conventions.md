# Webox — Code & Workflow Conventions

> Status: Stable for v0.1 · Ostatnia aktualizacja: 2026-05-25 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [DESIGN.md](./DESIGN.md), [TESTING.md](./TESTING.md), [SECURITY.md](./SECURITY.md), [CONTRIBUTING.md](./CONTRIBUTING.md), [gotchas.md](./gotchas.md).
>
> Ten dokument jest **autorytetywnym katalogiem konwencji**. Linter (`golangci-lint v2`) wymusza większość, ale niektóre wymagają review (np. naming patterns, error wrapping styles). Linter NIE jest substytutem code review.

---

## 1. Naming

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
// BAD: type IHostingProvider interface
// GOOD: type HostingProvider interface

// Constructor: NewFoo, nie MakeFoo.
func NewPool(maxConn int) *Pool {}
```

| Element | Konwencja | Przykład |
|---|---|---|
| Pakiety | krótkie, lowercase, bez `_` | `providers`, `ssh`, `secrets` |
| Eksportowane typy / funkcje | `PascalCase` | `HostingProvider`, `RegisterProvider` |
| Wewnętrzne typy / funkcje | `camelCase` | `parseDevilOutput` |
| Stałe | `PascalCase` lub `ALL_CAPS` jeśli enum-like | `StateDashboard`, `MaxRetries` |
| Sentinel errors | `ErrFoo` | `ErrSubdomainExists` |
| Test files | `*_test.go` w tym samym pakiecie | `provider_test.go` |
| Interface naming | rzeczownik bez `I` prefix | `HostingProvider`, nie `IHostingProvider` |

---

## 2. Error handling

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

**Zasady:**

- **Nigdy** comparing przez `err.Error() == "..."`. Zawsze `errors.Is(err, ErrXxx)` lub `errors.As(err, &targetType)`.
- Sentinel errors w `errors.go` per pakiet — wszystkie eksportowane.
- Wrap z dodatkowym kontekstem: kto wywołał, na czym, dlaczego. NIE: parametry zawierające sekrety.
- `panic()` tylko w `init()` i fatal startup. Wszystko inne wraca `error`.

---

## 3. Context discipline

```go
// GOOD: Każda metoda I/O ma ctx jako PIERWSZY argument.
func (p *SmallHostProvider) CreateSubdomain(ctx context.Context, domain string) error {
    if err := ctx.Err(); err != nil {
        return err
    }
    // ...
}

// BAD: Nie zapisuj ctx jako pola struktury (poza wyjątkami typu Server).
type BadProvider struct {
    ctx context.Context // anti-pattern (poza krótko żyjącymi obiektami).
}
```

**Reguły:**

- Każda metoda providera, każde wywołanie SSH, każde HTTP — `context.Context` jako PIERWSZY argument.
- `ctx.Done()` MUSI być respektowany. `ctx.Err()` na początku metody dla early exit.
- Background tasks → `context.WithCancel` w `tea.Cmd`, nigdy direct goroutine z parent context.

---

## 4. Generics

```go
// Go 1.24 nadal NIE pozwala na generic methods na struct.

// BAD - nie kompiluje:
func (c *Cache) GetOrFetch[T any](key string, fetch func() T) T {}

// GOOD - funkcja pakietowa zamiast metody:
func GetOrFetch[T any](c *Cache, key string, fetch func() T) T {}
```

Patrz [DESIGN §8.4](./DESIGN.md#84-generics-limitation).

---

## 5. Logging

```go
import (
    "log/slog"

    "github.com/dilitS/webox/internal/log"
)

// GOOD: structured logging z slog.
log.Info("ssh connection established",
    "host", profile.Host,
    "user", profile.User,
    "duration_ms", elapsed.Milliseconds(),
)

// BAD — NIGDY:
fmt.Printf("connecting to %s with password %s\n", host, password) // secret leak!
log.Info("token", "value", token) // secret leak!
```

**Zasady:**

- Tylko `log/slog` (stdlib) opakowany w `internal/log/redact.go`. Brak `logrus`/`zap`/etc.
- Brak ad-hoc `fmt.Printf` do diagnostyki (rzuca cienie security).
- Każdy nowy log msg sprawdzić: czy zmienne `password`/`token`/`key`/`secret` w nazwie mogłyby się tu znaleźć? Jeśli tak → redactor pattern obowiązkowy.

---

## 6. Testing helpers

```go
// t.Helper() w KAŻDEJ helper-funkcji.
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

## 7. Conventional Commits 1.0.0 (bez gitmoji)

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

**`<scope>`:**

- Pakiety top-level: `providers`, `tui`, `ssh`, `config`, `secrets`, `status`, `wizard`, `services`.
- Feature areas: `audit`, `scope`, `ci`, `release`.

### Przykład dobrego commit message

```
feat(providers): add smallhost CreateSubdomain implementation

Implements HostingProvider.CreateSubdomain for small.pl/Devil using
`devil www add <domain> nodejs <version>`. Parser handles three
known outputs: success, exists, invalid node version.

Refs: #42
```

### Przykład złego commit message

```
fix bug                                ← co? gdzie? dlaczego?
wip                                    ← nigdy do main.
:sparkles: add feature                 ← bez gitmoji.
update files                           ← bez kontekstu.
merge branch 'main' into feature/x     ← squash & merge zamiast tego.
```

---

## 8. PR structure

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

## 9. Architectural choices (always-applied)

- **Brak globalnych mutable variables.** Wszystko w `Model` lub explicit struct. Wyjątek: `init()` provider registry (zamknięty po starcie).
- **Wszystkie I/O przez `context.Context`** (patrz §3).
- **`Update()` jest pure function.** Brak `os.*`, `net.*`, channels w `tui/update.go`. Wszystko I/O w `tea.Cmd`.
- **`View()` jest pure function.** Brak mutacji stanu w `View`. Tylko `Model` → `string`.
- **Errors wrapped przez `fmt.Errorf("%w: ...", err)`.** Sentinel errors w `errors.go` pakietu.
- **Brak `panic()`** poza `init()` i fatal startup.
- **Idempotentne `Remove*` w providerach.** Brak zasobu = `nil` error. Inaczej DAG/LIFO rollback fragile.
- **`config.json` save = atomic.** `flock(2)` → write tmp → `fsync` → `rename` → `fsync(dir)`. Nigdy direct write.

---

## 10. Linter zignorowany przez `//nolint` — kiedy OK

```go
// BAD - nie rób tego:
//nolint:gocyclo
func setupSSL(...) {}

// GOOD - wymagane uzasadnienie:
//nolint:gocyclo // SetupSSL: 7 explicit error paths (rate limit, DNS,
//                 // existing cert, ACME, auth, network, parse), splitting
//                 // would reduce readability without measurable benefit.
func setupSSL(...) {}
```

**Reguła:** każde `//nolint` review'owane w PR. Brak uzasadnienia = automatic reject.

---

## 11. Zmiany dokumentacji są first-class

W Webox dokumentacja nie jest dodatkiem „po kodzie". Jeśli PR zmienia zachowanie, zakres, flow, security posture albo ergonomię operatora, reviewer ma prawo zatrzymać merge do czasu aktualizacji docs.

Minimalna zasada:

- zmiana zakresu → zaktualizuj [PRD.md](./PRD.md) lub [ROADMAP.md](./ROADMAP.md),
- zmiana kontraktu / modelu danych / sekretnych założeń → [DESIGN.md](./DESIGN.md),
- zmiana interakcji użytkownika / skrótów / ekranów → [UX.md](./UX.md),
- zmiana SSH / tokenów / `.env` / release chain → [SECURITY.md](./SECURITY.md),
- zmiana sposobu testowania lub release gate → [TESTING.md](./TESTING.md).

---

> _Last reviewed: 2026-05-25. Wydzielone z `AGENTS.md §5-6` jako część Sprint 15 docs refactor._
