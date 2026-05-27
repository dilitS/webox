# Webox — Top Gotchas & Anti-patterns

> Status: Living document · Ostatnia aktualizacja: 2026-05-27 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [AUDIT.md](./AUDIT.md), [SECURITY.md](./SECURITY.md), [DESIGN.md](./DESIGN.md), [conventions.md](./conventions.md).
>
> Wszystkie gotchas tu spisane były zaobserwowane podczas audytu albo realnej implementacji Webox. Jeśli odkryjesz nową — **dopisz**.

---

## 1. Keyring detection przez `Get` bez kontekstu

**Anti-pattern:**

```go
if _, err := keyring.Get(service, account); err != nil {
    fallback()
}
```

**Problem:** `keyring.Get` zwraca `ErrNotFound` jeśli sekret nigdy nie był zapisany — to NIE sygnał o braku keyringa. `ErrUnsupportedPlatform` jest prawdziwym sygnałem.

**Fix:** Probe write+read+delete sentinela. `errors.Is(err, keyring.ErrUnsupportedPlatform)` → fallback. `ErrNotFound` na probe Get **po** Set = bug backendu, nie brak keyringa.

Patrz [SECURITY §4.2](./SECURITY.md#42-fallback-dla-środowisk-headless).

---

## 2. AES-GCM nonce z `time.Now()`

**Anti-pattern:**

```go
nonce := make([]byte, 12)
binary.BigEndian.PutUint64(nonce, uint64(time.Now().UnixNano()))
```

**Problem:** Nonce reuse w AES-GCM łamie cały szyfr (nie tylko jeden message — wszystkie). Time-based nonce ma kolizje przy szybkich kolejnych zapisach.

**Fix:**

```go
nonce := make([]byte, 12)
if _, err := rand.Read(nonce); err != nil {
    panic(err)
}
```

**JEDYNE** dopuszczalne źródło nonce to `crypto/rand`. Test jednostkowy weryfikuje że dwa kolejne write'y dają różne nonce.

Patrz [SECURITY §4.2.1](./SECURITY.md#421-generowanie-nonce-krytyczne-dla-aes-gcm).

---

## 3. PID-based lockfile

**Anti-pattern:**

```go
if process, err := os.FindProcess(pid); err == nil {
    // assume process exists
}
```

**Problem:** Na Linux/macOS `os.FindProcess` NIGDY nie zwraca błędu. PID jest re-wykorzystywany przez OS — nasz lockfile może wskazywać na zupełnie inny proces.

**Fix:** `flock(2)` via `github.com/gofrs/flock`. Kernel-managed, cross-platform.

Patrz [DESIGN §6](./DESIGN.md#6-model-danych-i-atomowość-zapisu-configjson).

---

## 4. Generic method na struct

**Anti-pattern:**

```go
func (c *Cache) Get[T any]() T {}
```

**Problem:** Go 1.24/1.25 wciąż nie wspiera generic methods na struct. Tylko generic functions.

**Fix:**

```go
func GetOrFetch[T any](c *Cache, key string, fetch func() T) T {}
```

---

## 5. `go get` cicho podbija `go` directive

**Anti-pattern:** `go get <dep>@latest` i nieświadomy commit z wyższym `go` directive w `go.mod`.

**Problem:** Niektóre transitive deps deklarują wysoki Go floor (`go 1.26`) i `go mod tidy` automatycznie podbije Twój floor.

**Fix:** Po **każdym** dodaniu depa:

1. Przeczytaj `go.mod`.
2. Sprawdź transitive `go.mod` nowych modułów.
3. Pinuj ostatnią wersję kompatybilną z aktualnym floorem (`go 1.25.0`).

**Wyjątek:** jeśli `govulncheck` wskazuje realnie wywoływany kod i fix wymaga wyższego flooru, podbij floor świadomie, opisz dlaczego w PR + CHANGELOG i zaktualizuj [docs/dependencies.md](./dependencies.md).

---

## 6. Hardcoded provider name w business logic

**Anti-pattern:**

```go
if profile.Type == "smallhost" {
    // smallhost-specific logic
}
```

**Problem:** Łamie Provider Pattern (patrz [ADR-0003](./adr/0003-provider-pattern.md)). Każdy nowy provider wymaga audytu całego codebase.

**Fix:**

```go
provider := registry.New(profile)
provider.CreateSubdomain(ctx, ...)
```

Logika biznesowa **nigdy** nie zna `smallhost` po nazwie. Wszystko przez `providers.HostingProvider`. Nawet w MVP gdy jest jeden provider.

---

## 7. `os.Rename` na różnych filesystem

**Anti-pattern:** Próba rename z `/tmp/` do `~/.config/` (różne mount points).

**Problem:** `os.Rename` na Linux failuje przez `EXDEV (cross-device link)`. Atomic save się rozpada.

**Fix:** Tmp file **w tym samym katalogu** co target. `os.CreateTemp(filepath.Dir(targetPath), "*.tmp")`.

---

## 8. Sekret w `fmt.Errorf("...%s...", password)`

**Anti-pattern:**

```go
return fmt.Errorf("auth failed for user %s with password %s", user, password)
```

**Problem:** Error wraca po stosie i ląduje w `webox.log`, w stack trace, w issue report.

**Fix:**

```go
return fmt.Errorf("auth failed for user %s: %w", user, ErrInvalidCredentials)
```

Nigdy nie zawieraj plaintekstowych sekretów w błędach. Sentinel errors (`ErrInvalidCredentials`) zamiast tego.

---

## 9. `rsync --delete` bez `--exclude`

**Anti-pattern:**

```bash
rsync -avz --delete ./build/ user@host:~/app/
```

**Problem:** Usuwa `public/uploads/`, `.env`, `node_modules/.cache/` na zdalnym. Klient może płakać.

**Fix:** Zawsze `--exclude`:

```bash
rsync -avz --delete \
    --exclude='.env' \
    --exclude='node_modules/' \
    --exclude='public/uploads/' \
    --exclude='logs/' \
    ./build/ user@host:~/app/
```

Patrz `providers/smallhost.md §6`.

---

## 10. GitHub Actions `uses: actions/checkout@v4`

**Anti-pattern:**

```yaml
- uses: actions/checkout@v4
```

**Problem:** Tag `v4` może być przepisany przez supply-chain attack. Maintainer actions może wgrać złośliwy kod pod tym samym tagiem.

**Fix:**

```yaml
- uses: actions/checkout@<full-40-char-SHA>  # v4.1.7
```

Wszystkie GitHub Actions używamy **TYLKO** z pinned full SHA. Komentarz z tag dla czytelności.

---

## 11. `t.Parallel()` w testach stubujących package-level globals

**Anti-pattern:**

```go
func TestDoctor_OK(t *testing.T) {
    t.Parallel()  // ← BUG
    oldNow := nowFn
    nowFn = func() time.Time { return time.Now().UTC() }
    defer func() { nowFn = oldNow }()
    // ...
}
```

**Problem:** Test podmienia `dispatchDoctor`, `newRunner`, `nowFn` i jednocześnie inne testy lecą równolegle — race condition.

**Fix:** Takie testy uruchamiaj **sekwencyjnie**, albo chowaj seam za instancją / mutexem. `go test -race` ma być zielony bez wyjątków.

---

## 12. Bubble Tea side effect w `Update`

**Anti-pattern:**

```go
case KeyMsg:
    go fetchData()  // raw goroutine, unsupervised
    return m, nil
```

**Problem:** Update() ma być **pure function**. Raw goroutine omija `tea.Cmd` lifecycle, nie da się anulować, nie wiadomo kiedy się skończy.

**Fix:**

```go
case KeyMsg:
    return m, fetchDataCmd(ctx)
```

`fetchDataCmd` to `func() tea.Msg` — Bubble Tea zarządza lifecycle.

Patrz [DESIGN §2.3](./DESIGN.md#23-zasady-przep%C5%82ywu-danych-mvu).

---

## 13. `//nolint` bez uzasadnienia

**Anti-pattern:**

```go
//nolint:gocyclo
func setupSSL(...) {}
```

**Problem:** Nie wiadomo dlaczego. Reviewer nie wie czy to OK. Następny dev zignoruje cały linter „w stylu zespołu".

**Fix:**

```go
//nolint:gocyclo // SetupSSL: 7 explicit error paths (rate limit, DNS,
//                 // existing cert, ACME, auth, network, parse), splitting
//                 // would reduce readability without measurable benefit.
func setupSSL(...) {}
```

---

## 14. Capturing wrażliwych danych w `webox.log`

**Anti-pattern:**

```go
slog.Info("github API response", "body", string(respBody))
```

**Problem:** GitHub API zwraca tokeny w error responses (np. validation errors echo input). Token w log = leak.

**Fix:**

```go
slog.Info("github API response",
    "status", resp.StatusCode,
    "body_size", len(respBody),
)
// body przepuść przez redactor PRZED zapisem do diagnostyki:
slog.Debug("github API response body (redacted)",
    "body", log.Redact(string(respBody)),
)
```

`internal/log/redact.go` ma listę wzorców (ghp_, github_pat_, sk-, BEGIN ... PRIVATE KEY, etc.). Każdy nowy wzorzec dodajemy razem z testem.

Patrz [DESIGN §15.2](./DESIGN.md#152-redacted-logger--wzorce).

---

## 14a. `t.Parallel()` + `t.Setenv()` to combo niedopuszczalne

**Anti-pattern:**

```go
func TestThing(t *testing.T) {
    t.Parallel()
    t.Setenv("WEBOX_CPANEL_MUTATIONS", "1")
    // ...
}
```

**Problem:** `t.Setenv` panicuje przy `t.Parallel()` — pakiet `testing` celowo wyłącza tę kombinację, bo zmienne środowiskowe procesu są globalne i równoległe testy modyfikujące różne wartości tej samej zmiennej będą się ścigać. Pomyłka kosztuje: testy padają dopiero przy `-race` lub na CI gdzie scheduler ujawnia kolejność, a lokalnie bywa zielono.

**Fix:** Wybierz jedno:

- **opcja A — `t.Parallel()` tak, `t.Setenv` nie.** Wstrzyknij wartość env vara przez seam (parametr funkcji, pole struktury, opcjonalna konfiguracja). Pakiet `cmd/webox` korzysta z tego wzorca — opcje doctora dostają `cpanelOpts.httpsTransport`, nie globalnego env vara.
- **opcja B — `t.Setenv` tak, `t.Parallel()` nie.** Niech test biegnie sekwencyjnie. Bezpieczne tylko gdy test jest szybki (< 200 ms) i nie wstrzymuje całego pakietu.

Każdy nowy test stosujący `t.Setenv` musi mieć komentarz wyjaśniający dlaczego nie da się odpuścić globalnej zmiennej.

**Wystąpienia:** Sprint 22 `internal/e2e/cpanel_test.go` (przepisany na seam), Sprint 23 — flagowane w retro jako recurring class. Patrz `docs/retros/2026-05-27-sprint-23.md`.

---

## 14b. Contextual-token parser bez stanowej maszyny

**Anti-pattern:** parser CLI rozpoznaje token (`github`, `preset`, `cpanel`, `directadmin`) globalnie — `simpleFlagHandled` widzi `cpanel` w `webox doctor cpanel` i w `webox provider new cpanel <X>` jako tę samą rzecz.

**Problem:** kolizja parsera w wielu kontekstach. `webox doctor cpanel --token=…` chce konsumować token w trybie diagnostyki, `webox provider new cpanel jakas-nazwa` chce konsumować ten sam string jako provider name. Bez state-aware logiki jeden z dwóch przypadków cicho znika lub łapie zły fragment komendy.

**Fix:** Parser w `cmd/webox/run.go` przed dotknięciem tokena pyta:

```go
if isProviderNewContext(parsed) && parsed.needsProviderName() {
    return consumeAsProviderName(parsed, tok)
}
if isPresetOrCpanelOrDirectadminContext(parsed) {
    return applyContextualToken(parsed, tok)
}
```

Każdy contextual handler (`applyContextualToken`, `simpleFlagHandled`, `prefixedFlagHandled`, `applyAPIPortFlag`, `applySSHPortFlag`) musi pierwsze sprawdzić w jakim kontekście jest parser, dopiero potem zinterpretować token. Negative tests są obowiązkowe: każdy flag z prefiksem (`--token`, `--loginkey`, `--api-port`, `--no-api`) musi mieć test rejekcji poza kontekstem.

**Wystąpienia:** Sprint 21 (cpanel + preset), Sprint 23 (directadmin dorzucony). Patrz `cmd/webox/cpanel_parser_test.go`, `cmd/webox/directadmin_parser_test.go`, `cmd/webox/preset_test.go`.

---

## 14c. `Pool.Close` ≠ "server-side counter frozen"

**Anti-pattern:** test SSH keepalive zakłada że po `pool.Close()` returns immediately `server.GlobalRequestCount(...)` jest finalne.

**Problem:** `pool.Close` honoruje kontrakt _client-side_ (`wg.Wait()` blokuje dopóki goroutine keepalive nie zwróci), ale po stronie serwera istnieje druga warstwa async: wewnętrzna goroutine `crypto/ssh.Server` czyta SSH-packets z drutu, parsuje globalne requesty i wkłada je do channela `requests`. `sshmock.handleGlobalRequests` zwiększa licznik **przed** `reply(req)`. Jeśli klient zamknął połączenie gdy jeden już-sparowany request siedział w channelu, server-side increment ląduje **po** powrocie `pool.Close()`.

**Fix:** Każdy assert oparty o server-side counter po `pool.Close()` musi czekać aż licznik się ustabilizuje (dwa kolejne snapshoty oddzielone interwałem ≥ 1 keepalive period zwracają tę samą wartość). Helper w `ssh/exec_test.go::waitForStableCount` — szablon dla nowych testów.

**Wystąpienia:** Sprint 22 (PR-17 macOS flake `just-after=1 final=2`), Sprint 23 (PR-18 to samo). Naprawione w 2026-05-27.

---

## 15. Polish-only docs / mixed PL/EN w contributor surface

**Anti-pattern:** README po polsku, CONTRIBUTING po polsku, ADR-y po polsku → contributor z UK się nie zaangażuje.

**Problem:** Webox jest globalny OSS. Contributor pool poza Polską > contributor pool w Polsce.

**Fix:** Rule from Sprint 15:

- **Root** `README.md`, `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md` — **EN-only**.
- **`docs/`** — może być mixed (PL/EN), ale każdy nowy doc tworzony post-Sprint 15 EN-first.
- **Issue templates** EN.
- **Commit messages** EN (Conventional Commits English).
- **Code comments** EN.

PL community ma `docs/CONTRIBUTING.md` (legacy detailed) jako reference, ale entry-point dla nich też jest EN root level.

---

> _Last reviewed: 2026-05-27. Sprint 23 retro dorzucił sekcje 14a-14c (t.Parallel+t.Setenv, contextual-token parser, server-side drain race). Wydzielone z `AGENTS.md §7` jako część Sprint 15 docs refactor._
