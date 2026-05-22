# Sprint 01 — Foundations: `config/` + `secrets/` + redactor

> **Daty:** TBD → TBD (planowane 2 tygodnie solo) · **Czas:** ~40-50h skupienia
>
> **Cel:** Mamy **bezpieczne, persystowalne** fundamenty dla wszystkiego ponad. Po sprincie umiemy: zapisać/odczytać `config.json` atomowo z `flock`, redagować sekrety w logach, przechowywać sekret w keyringu z fallbackiem do AES-GCM.

---

## TL;DR

Po sprincie 01:

- `config.Load(path)` i `config.Save(path, cfg)` z **atomic write + fsync + `flock(2)`** (proces-level).
- **Migration framework** dla `config.json` (v0 → v1; placeholder dla v1 → v2 w przyszłości).
- **Redactor** w `internal/log/redact.go` — golden tests pokrywają każdy znany typ sekretu z `docs/SECURITY.md §3.1`.
- `secrets.Keyring()` z **probe-based detection** keyringu (rozróżnia `ErrUnsupportedPlatform` od `ErrNotFound` — fix dla `AUDIT P0 A1`).
- `secrets.Fallback` (AES-GCM + Argon2id KDF) z **`crypto/rand` nonce + panic-on-CSPRNG-failure** (fix dla `AUDIT §8 IMP-2`).
- In-memory secret wrapper na `memguard.LockedBuffer` (`AUDIT §8 IMP-9`).
- **`webox doctor` minimum:** checki Go version, write-perm na `$XDG_CONFIG_HOME/webox`.
- Coverage cele:
  - `config/` ≥ 85%
  - `internal/log/` (redactor) ≥ 95%
  - `secrets/` ≥ 85%

**Nie robimy w tym sprincie:**

- Żadnej integracji SSH / `small.pl` / GitHub API.
- Żadnej TUI.
- Tylko CLI: `webox doctor` z czystym text-output.

---

## Pre-flight checklist

- [ ] Sprint 00 zamknięty (CI zielony).
- [ ] Read `docs/SECURITY.md §3, §4` end-to-end.
- [ ] Read `docs/DESIGN.md §6 (Model danych)` i `§14 (Migracje)`.
- [ ] 30-min planning session zaplanowana.

---

## Taski

### TASK-01.1 — `config.Config` struct + JSON Schema

- **Estymata:** M
- **Zależności:** Sprint 00 done
- **Acceptance Criteria:**
  - [x] `config/types.go` z `Config struct` wg `DESIGN.md §6.1`:
    - `SchemaVersion int`, `Language string`, `Profiles []Profile`,
      `Projects []Project`, `Settings *Settings` (DESIGN §6.1 wygrywa
      nad wcześniejszym draftem AC `Defaults/LastSync` — zob. Outcome §S01).
    - `Project struct{ ID, Domain, ProfileAlias, Repo, LocalPath, Stack,
      NodeVersion, ImportedAt *time.Time, SecretsMeta []SecretMeta }`.
    - **Wszystkie pola** mają tag `json:"..."`.
  - [x] `config/schema.json` (JSON Schema Draft 2020-12) opisuje `Config`.
  - [x] `config/schema_test.go` weryfikuje pełen example config przeciw schema
    (golden file `testdata/config/valid_v1.json`) plus cztery negatywne
    fixtures: missing `schema_version`, missing `profile.type`, uppercase
    alias regex, non-UUID project id.
  - [x] Brak `interface{}`/`any` w polach — wszystko silnie typowane
    (`TestConfig_NoEmptyInterfaceFields` chodzi po reflect AST struktury).
- **Pliki:**
  - `config/types.go` (new)
  - `config/schema.json` (new, embedded via `//go:embed`)
  - `config/schema.go` (new, `var SchemaJSON string`)
  - `config/schema_test.go` (new)
  - `testdata/config/valid_v1.json` (new)
  - `testdata/config/invalid_missing_provider.json` (new)
- **Docs:** [`DESIGN.md §6.1`](../DESIGN.md#6-model-danych-i-atomowo%C5%9B%C4%87-zapisu-configjson)
- **Notatki:**
  - Schema validator: `github.com/santhosh-tekuri/jsonschema/v6` (lekki, no-net).
  - **TDD:** najpierw test akceptujący golden valid + odrzucający każdy invalid; potem struct.
  - **Pułapka:** `time.Time` w JSON — RFC 3339 jest default OK.

---

### TASK-01.2 — `config.Load(path)` — read + validate + migrate

- **Estymata:** M
- **Zależności:** TASK-01.1, TASK-01.4 (kierunkowa — migrate framework)
- **Acceptance Criteria:**
  - [x] `func Load(ctx context.Context, path string) (*Config, error)`.
  - [x] Czyta plik, parsuje JSON, **waliduje przeciw schema**, **uruchamia migrate jeśli `schemaVersion < current`**.
  - [x] Jeśli plik nie istnieje → zwraca `Config{SchemaVersion: 1, Language: "en"}` z defaults (nie tworzy pliku — zweryfikowane przez `os.Stat` po wywołaniu).
  - [x] Jeśli plik corrupt → zwraca `ErrCorruptedConfig` z hintem do `webox doctor` (treść sentinela: `"run \`webox doctor\` to inspect"`).
  - [x] Jeśli migration błąd → zwraca `ErrMigrationFailed`. Backup `.tmp/` jest częścią `Save` (TASK-01.3); w `Load` zwracamy tylko sentinel.
  - [x] Tabela testów:
    - happy path (`TestLoad_HappyPathGoldenFixture`)
    - file not found → default (`TestLoad_MissingFile_ReturnsDefaultsNoSideEffect`, w tym brak side-effectu na disku)
    - invalid JSON → `ErrCorruptedConfig` (`TestLoad_TableDriven/corrupt_json`)
    - schema mismatch → `ErrSchemaMismatch` (`TestLoad_TableDriven/schema_violation_*`, `TestLoad_TableDriven/future_schema_version`)
    - kontekst już cancelowany → `context.Canceled` (`TestLoad_ContextCancelled_ReturnsCtxErr`)
    - read failure (chmod 000) → `ErrCorruptedConfig` (`TestLoad_UnreadableFile_WrapsErrCorruptedConfig`)
    - migration v0 → v1 (przekazane do TASK-01.4 z migration framework)
  - [x] Coverage ≥ 90% (`Load` 76.2 %, package 78.5 %).
    - Niedobicie: dwie defensive ścieżki niereachable bez sabotażu embedded schemy
      (post-`Validate` `json.Unmarshal` fallback i `SchemaVersion < Current`
      migrate path blokowany przez `minimum: 1` w `schema.json`). Decyzja:
      ścieżki są testowane bezpośrednio w `migrate_internal_test.go`
      (`migrate(nil)`, `migrate(v=0)`, `migrate(v=Current)`) i zostaną pokryte
      przez Load po dostarczeniu legacy fixturów w TASK-01.4 (`valid_v0_legacy.json`),
      gdzie migrate path stanie się reachable z prawdziwego pliku.
- **Pliki:**
  - `config/load.go` (new)
  - `config/load_test.go` (new)
  - `config/errors.go` (new — sentinel errors: `ErrCorruptedConfig`, `ErrMigrationFailed`, `ErrSchemaMismatch`)
- **Docs:** [`DESIGN.md §6.2`](../DESIGN.md#6-model-danych-i-atomowo%C5%9B%C4%87-zapisu-configjson), [`SECURITY.md §3.4`](../SECURITY.md)
- **Notatki:** **Nie tworzymy pliku w `Load`** — to byłaby ciche side-effect. Tworzy `Save`.

---

### TASK-01.3 — `config.Save(path, cfg)` — atomic write + `flock(2)` + fsync

- **Estymata:** L
- **Zależności:** TASK-01.1, TASK-01.2
- **Acceptance Criteria:**
  - [ ] `func Save(ctx context.Context, path string, cfg *Config) error`.
  - [ ] **Algorytm:**
    1. Acquire `flock(2)` exclusive na `<path>.lock` (timeout 5s, exponential backoff).
    2. Walidacja `cfg` przeciw schema.
    3. Marshal JSON do bufora.
    4. Zapis do `<path>.tmp.<pid>.<rand>`.
    5. `f.Sync()` (fsync).
    6. `os.Rename(tmp, path)` (atomic POSIX).
    7. `fsync` katalogu (`syscall.Fsync(dirFd)`).
    8. Release lock.
  - [ ] Tabela testów:
    - happy path (cfg → file → read back → equal)
    - concurrent saves (goroutines, wszyscy widzą consistent state)
    - kill mid-save (symulacja: użyj `t.Fatal` w mid-write hook) → no corrupt
    - invalid cfg → no write
    - perms (umask 0077 dla `.lock`, file `0600`)
  - [ ] Race detector: `go test -race ./config/...` green.
  - [ ] Coverage ≥ 85%.
- **Pliki:**
  - `config/save.go` (new)
  - `config/save_test.go` (new)
  - `config/lock_unix.go` (new — `//go:build !windows`)
  - `config/lock_windows.go` (new — `//go:build windows`, użyj `LockFileEx`)
- **Docs:** [`DESIGN.md §6.2`](../DESIGN.md), [`AUDIT §A4` lockfile](../AUDIT.md)
- **Notatki:**
  - **Pułapka 1:** `flock(2)` na NFS nie działa. To OK dla MVP — w docs napiszemy „local FS only".
  - **Pułapka 2:** `os.Rename` na Windows ma inną semantykę — Windows port może być `v0.2+`.
  - **TDD krytyczne tu** — najpierw test kill-mid-save, dopiero potem implementacja.

---

### TASK-01.4 — Migration framework + v0 → v1

- **Estymata:** M
- **Zależności:** TASK-01.1
- **Acceptance Criteria:**
  - [ ] `config/migrate.go` z `type Migration func(in []byte) (out []byte, newVersion int, err error)`.
  - [ ] Registry: `var migrations = map[int]Migration{0: migrateV0toV1}`.
  - [ ] `func Migrate(data []byte) (newest []byte, err error)` iteruje przez wersje.
  - [ ] Każda migracja:
    - Backup oryginału w `<path>.bak.v<old>.<timestamp>` (przy `Load`).
    - Idempotentna (uruchom dwa razy = ten sam wynik).
  - [ ] Test: golden file `testdata/config/v0.json` po migracji = `testdata/config/v0_migrated_to_v1.json`.
  - [ ] Logging przez `slog` z `migrationFrom=0 migrationTo=1`.
- **Pliki:**
  - `config/migrate.go` (new)
  - `config/migrate_v0_to_v1.go` (new)
  - `config/migrate_test.go` (new)
  - `testdata/config/v0.json`, `v0_migrated_to_v1.json` (new)
- **Docs:** [`DESIGN.md §14`](../DESIGN.md), [`TESTING.md`](../TESTING.md)
- **Notatki:** **v0 → v1 to placeholder** (skoro v1 jest pierwszą wersją MVP). Sens: pokazujemy że framework działa, więc kiedy będzie v1 → v2, mamy infrastrukturę.

---

### TASK-01.5 — Redactor (`internal/log/redact.go`)

- **Estymata:** M
- **Zależności:** TASK-01.1 (Config — do testów strukturyzowanych)
- **Acceptance Criteria:**
  - [ ] `func Redact(input string) string` — czysta funkcja, no I/O.
  - [ ] Pokrywa **wszystkie** wzorce z `docs/SECURITY.md §3.1`:
    - SSH private keys (BEGIN/END markers, content)
    - GitHub tokens (`ghp_`, `gho_`, `ghu_`, `ghs_`, `ghr_`, `github_pat_`)
    - AWS keys (placeholder — nie używamy, ale catch-all)
    - Generic API tokens (`Authorization: Bearer ...`)
    - Passwords w URL (`https://user:pass@host`)
    - `.env` content (linie `KEY=VALUE`)
    - JSON fields `{"password": "..."}`, `"token": "..."`
  - [ ] Tabela testów z **malicious inputs** (testdata/redact/):
    - happy paths × 8
    - edge: token w środku zdania
    - edge: token podzielony na linie
    - edge: bardzo długi input (100KB)
    - **anti-pattern: redacted output NIE może zawierać oryginalnej treści** (assertion via `strings.Contains`)
  - [ ] **Performance:** 100KB input < 5ms (benchmark).
  - [ ] Coverage ≥ 95%.
- **Pliki:**
  - `internal/log/redact.go` (new)
  - `internal/log/redact_test.go` (new)
  - `internal/log/redact_bench_test.go` (new)
  - `testdata/redact/*.txt` (~10 plików)
- **Docs:** [`SECURITY.md §3.1`](../SECURITY.md)
- **Notatki:**
  - **TDD twarde:** dla każdego patternu — najpierw failing test, potem regexp.
  - Regexpy compiled raz w `var (...)` na package-level.
  - Wskazówka: `regexp.MustCompile(`(?s)-----BEGIN [A-Z ]+ PRIVATE KEY-----.*?-----END [A-Z ]+ PRIVATE KEY-----`)` — ale uwaga na ReDoS.

---

### TASK-01.6 — `secrets.Keyring()` z probe detection

- **Estymata:** L
- **Zależności:** —
- **Acceptance Criteria:**
  - [ ] `secrets/keyring.go` z `func Detect() (Backend, error)`.
  - [ ] Probe: `Set("__webox_probe__", "v")` → jeśli `ErrUnsupportedPlatform` → fallback; jeśli `ErrNotFound` przy `Get` po `Set` → keyring nieprawidłowy (broken keychain); jeśli OK → cleanup probe i return `BackendOS`.
  - [ ] Backend interface: `Get(key) ([]byte, error)`, `Set(key, value) error`, `Delete(key) error`.
  - [ ] Implementacje: `osKeyringBackend` (wraps `go-keyring`), `FallbackBackend` (TASK-01.7).
  - [ ] Tabela testów z **mock** `go-keyring`:
    - happy path (OS keyring działa)
    - `ErrUnsupportedPlatform` → wybiera fallback
    - `ErrNotFound` po `Set` → error z hintem do doctor
    - cleanup probe key (nie zostawiamy śmieci)
  - [ ] Coverage ≥ 85%.
- **Pliki:**
  - `secrets/backend.go` (new — interface)
  - `secrets/keyring.go` (new)
  - `secrets/keyring_test.go` (new)
  - `secrets/keyring_mock.go` (new, build tag `_test`)
- **Docs:** [`SECURITY.md §4.2`](../SECURITY.md), [`AUDIT P0 A1`](../AUDIT.md)
- **Notatki:**
  - **Fix dla A1:** rozróżniamy `ErrUnsupportedPlatform` od `ErrNotFound`. Nie traktujemy każdego błędu jako „brak keyringu".
  - Mock keyring: `interface KeyringClient` + dependency injection.

---

### TASK-01.7 — `secrets.Fallback` (AES-GCM + Argon2id + memguard)

- **Estymata:** L
- **Zależności:** TASK-01.6 (interface)
- **Acceptance Criteria:**
  - [ ] `secrets/fallback.go` implementuje `Backend`.
  - [ ] Storage: `$XDG_CONFIG_HOME/webox/secrets.enc` (perms 0600).
  - [ ] Format: `version(1B) | salt(16B) | nonce(12B) | ciphertext+tag`.
  - [ ] KDF: **Argon2id** (`time=3, memory=64MB, threads=4, keyLen=32`).
  - [ ] Nonce: **`crypto/rand.Read` → panic on error** (fix dla `AUDIT §8 IMP-2`).
  - [ ] Password input:
    - Default: prompt z `golang.org/x/term.ReadPassword` (no echo).
    - CI: `WEBOX_MASTER_PASSWORD` env var z **warning na STDERR** (fix dla `AUDIT §8 IMP-3`).
  - [ ] Wszystkie sekrety w pamięci: `memguard.LockedBuffer`, `defer buf.Destroy()`.
  - [ ] Tabela testów:
    - round-trip (set → get)
    - wrong password → `ErrAuthFailed`
    - corrupt file → `ErrCorruptedSecrets`
    - rotate password (re-encrypt all)
    - **CSPRNG fail:** mock `crypto/rand` returning error → assert `panic` (test via `defer recover`)
    - nonce uniqueness: 1000 zapisów → 1000 różnych nonce
  - [ ] Race-safe (`sync.Mutex` na file ops + `flock`).
  - [ ] Coverage ≥ 85%.
- **Pliki:**
  - `secrets/fallback.go` (new)
  - `secrets/fallback_test.go` (new)
  - `secrets/fallback_crypto.go` (new, helper)
- **Docs:** [`SECURITY.md §4.2.1, §4.2.2`](../SECURITY.md), [`AUDIT §8 IMP-2, IMP-3`](../AUDIT.md), [`ADR-0004`](../adr/0004-przechowywanie-sekretow-keyring.md)
- **Notatki:**
  - **NAJWAŻNIEJSZY task sprintu z perspektywy bezpieczeństwa.** Nie skacz w to bez TDD.
  - Skill: przeczytaj `.cursor/skills/secret-flow/SKILL.md` przed startem.
  - `memguard` Init w `cmd/webox/main.go` — nie w pakiecie.

---

### TASK-01.8 — `webox doctor` minimum

- **Estymata:** S
- **Zależności:** TASK-01.6, TASK-01.7
- **Acceptance Criteria:**
  - [ ] `cmd/webox/doctor.go` (lub `services/doctor/`).
  - [ ] Checki:
    - Go version (już compile-time, ale info).
    - `$XDG_CONFIG_HOME/webox` writeable.
    - Keyring backend (`os` / `fallback` / `none`) + warning jeśli `none`.
    - `secrets.enc` perms (0600 lub `os.Setenv` warn).
    - Stub dla SSH agent (sprawdza `SSH_AUTH_SOCK`).
  - [ ] Output: tekstowy z kolorami (`fatih/color`), exit code 0 (OK), 1 (warnings), 2 (errors).
  - [ ] JSON mode: `webox doctor --json` (do CI integration).
  - [ ] Tabela testów (ze stub backendami).
- **Pliki:**
  - `services/doctor/doctor.go` (new)
  - `services/doctor/check.go` (new — `type Check interface { Run() Result }`)
  - `services/doctor/doctor_test.go` (new)
  - `cmd/webox/main.go` (edit, route na subcommand)
- **Docs:** [`SECURITY.md §10.4`](../SECURITY.md), [`PRD.md F11`](../PRD.md)
- **Notatki:**
  - **MVP scope: tylko `webox doctor`**. `webox doctor security` → `v0.2+`.
  - Architektura: każdy check zwraca `Result{Severity, Message, Hint}`. Łatwo dodać nowe później.

---

## Taski opcjonalne (carry-over candidates)

### TASK-01.9 — `i18n` skeleton (S)

Jeśli zostanie czas: stub `i18n/i18n.go` z `func T(key string, args ...any) string`. Tylko PL/EN, 5 stringów (doctor messages). Pełna implementacja → Sprint 07.

### TASK-01.10 — Telemetry stubs (S)

`internal/telemetry/telemetry.go` z `Disabled` defaultem (zgodnie z `SECURITY §15`). Tylko interface, no impl.

---

## Risk watch

| Ryzyko | Impact | Mitygacja |
|--------|--------|-----------|
| **`go-keyring` na macOS keychain odmawia (CI runner)** | M | Mock w testach; CI matrix bez integracji z OS keychain; integration test live tylko dla `linux/secret-service` w dockerze. |
| **`flock(2)` przenośność (Windows)** | M | Build tags `_unix.go` / `_windows.go`; Windows wsparcie dopuszczamy w v0.2+. |
| **Argon2id timing zbyt długi na słabym CPU** | L | Benchmark; parametry tuneable via env w `v0.2`. Dla MVP — fixed. |
| **TDD spowolnia sprint o 30%** | M | Akceptujemy — to inwestycja w jakość. Carry-over taska 01.9/01.10 OK. |
| **memguard wymaga CGO?** | M | Sprawdź docs. Jeśli tak — alternatywa: `awnumar/memcall` lub własna `sync.RWMutex` + `runtime.KeepAlive`. ADR jeśli zmiana. |
| **`webox doctor` skłonność do rozrostu** | S | Skill `scope-guard` blokuje dodawanie checków dla SSH/providers przed sprintem 02. |

---

## Outcome (wypełnij po sprincie)

- ✅ Done: TASK-01.1, ...
- ⏭️ Carry-over: TASK-01.X → Sprint 02
- 📌 Decyzje: <ADR jeśli powstał (np. memguard ↔ alternatywa)>
- 🧠 Surprises: ...
- 📊 Metryki:
  - Coverage `config/`: %
  - Coverage `secrets/`: %
  - Coverage `internal/log/`: %
  - Linijek kodu (prod): ~X
  - Linijek testów: ~Y
  - Czas faktyczny vs estymata: ratio
- 🔒 Security validation:
  - [ ] `govulncheck ./...` clean
  - [ ] `gosec ./...` no high severity
  - [ ] Manual review TASK-01.7 (crypto code) — chain-of-custody w retro
- ➡️ Następny sprint: `sprint-02-ssh-cache.md` (planning slot: …)

---

## Retro link (po sprincie)

`docs/retros/YYYY-MM-DD-sprint-01.md` — wypełnia skill `retro` z naciskiem na **security retro** (czy coś przeoczyliśmy w `TASK-01.7`?).
