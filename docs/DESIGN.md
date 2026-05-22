# Webox — Design / Architektura

> Status: Approved · Ostatnia aktualizacja: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [PRD.md](./PRD.md) (cele produktowe), [UX.md](./UX.md) (szczegółowy design system i makiety), [SECURITY.md](./SECURITY.md) (model zaufania), [TESTING.md](./TESTING.md) (strategia testowania), [AUDIT.md](./AUDIT.md) (audyt przed-implementacyjny).

---

## TL;DR

Webox to lekki monolit w Go zorganizowany według paradygmatu **MVU (Model-View-Update)** z [Bubble Tea](https://github.com/charmbracelet/bubbletea). Architektura **MVP (v0.1)** opiera się na czterech filarach: (1) **Provider Pattern** z jednym adapterem `smallhost` i pełnym interfejsem `HostingProvider` dla przyszłych providerów, (2) **DAG-based Transactional Engine** dla rollback-safe wizardu, (3) **SSH Connection Pool** z `keepalive` dla responsywnego dashboardu, (4) **Stale-While-Revalidate Status Cache** dla płynności UI. **Sekrety wyłącznie w keyringu** (fallback AES-GCM z Argon2id) zgodnie z [ADR-0004](./adr/0004-przechowywanie-sekretow-keyring.md). Sekcje oznaczone `🔶 STRETCH (v0.2+)` opisują rozszerzenia poza MVP — implementowane dopiero po dostarczeniu `v0.1`, z mocą [ROADMAP §3.3](./ROADMAP.md#33-czego-nie-ma-w-mvp).

> **Konwencja scope:** sekcje oznaczone `🔵 MVP (v0.1)` są w zakresie pierwszego release'u. Sekcje `🔶 STRETCH (v0.2+)` są **architekturalnie zaprojektowane**, ale **nie implementowane** w MVP. Lista co i kiedy w [ROADMAP.md](./ROADMAP.md).

---

## Spis treści

1. [Cel dokumentu](#1-cel-dokumentu)
2. [Wysokopoziomowa architektura i przepływ danych](#2-wysokopoziomowa-architektura-i-przepływ-danych)
3. [Provider Pattern (Kontrakty v2)](#3-provider-pattern-kontrakty-v2)
4. [Rejestr providerów](#4-rejestr-providerów)
5. [Warstwa SSH / SFTP Connection Pooling](#5-warstwa-ssh--sftp-connection-pooling)
6. [Model danych i atomowość zapisu config.json](#6-model-danych-i-atomowość-zapisu-configjson)
7. [Zarządzanie sekretami (Keyring integration)](#7-zarządzanie-sekretami-keyring-integration)
8. [Trójpoziomowy Status Cache (Stale-While-Revalidate)](#8-trójpoziomowy-status-cache-stale-while-revalidate)
9. [Obsługa błędów sieciowych i reconnect](#9-obsługa-błędów-sieciowych-i-reconnect)
10. [DAG-based Transactional Engine (Wznawialny Rollback)](#10-dag-based-transactional-engine-wznawialny-rollback)
11. [Detekcja rozbieżności konfiguracji (Drift & Stale detection)](#11-detekcja-rozbieżności-konfiguracji-drift--stale-detection)
    * 11.1 [Architektura Dwukierunkowego Env Merger (TUI Env Merger Engine)](#111-architektura-dwukierunkowego-env-merger-tui-env-merger-engine)
12. [Maszyna stanów TUI (Tabbed Cockpit Spec)](#12-maszyna-stanów-tui-tabbed-cockpit-spec)
13. [Integracja z GitHubem (Actions & Deployment Keys)](#13-integracja-z-githubem-actions--deployment-keys)
14. [Dystrybucja i mechanizm sprawdzania wersji](#14-dystrybucja-i-mechanizm-sprawdzania-wersji)
15. [Diagnostyka (Doctor & Redacted Logger)](#15-diagnostyka-doctor--redacted-logger)
16. [Concurrency & Visual Motion Engine (Pulsacje i Spinnery)](#16-concurrency--visual-motion-engine-pulsacje-i-spinnery)
17. [Architektura Silnika Dźwiękowego w Go (Package sound)](#17-architektura-silnika-dźwiękowego-w-go-package-sound)
18. [Silnik Dynamicznej Topologii (Live Service Topology Map Engine)](#18-silnik-dynamicznej-topologii-live-service-topology-map-engine)

---

## 1. Cel dokumentu

Niniejszy dokument opisuje techniczną specyfikację architektury monolitu Webox. Wyjaśnia, w jaki sposób implementowane są zaawansowane mechanizmy UX/UI określone w [UX.md](./UX.md) przy zachowaniu czystości kodu Go, pełnego pokrycia testami oraz deterministycznego przepływu stanów.

---

## 2. Wysokopoziomowa architektura i przepływ danych

### 2.1 Layout repo

Planowane drzewo katalogów (pierwsze PR-y stworzą konkretne pliki):

```text
webox/
├── cmd/webox/              # main package, entrypoint
├── tui/                    # Bubble Tea state machine, view rendering
│   ├── views/              # pure render functions per stan
│   ├── states.go           # enum stanów i sub-stanów (§12)
│   └── update.go
├── providers/              # adaptery paneli hostingowych (§3)
│   ├── provider.go         # interfejs HostingProvider + ProviderConfig
│   ├── registry.go         # rejestr + factory (§4)
│   ├── smallhost/          # adapter small.pl / Devil (MVP)
│   └── mock/               # mock provider dla testów (TESTING §3)
├── ssh/                    # connection pool + sftp helpers (§5)
├── config/                 # load/save config.json + migracje (§6)
├── secrets/                # keyring + AES-GCM fallback (§7 + SECURITY §4)
├── status/                 # SWR cache (§8 + ADR-0005)
├── wizard/                 # DAG-based transactional engine (§10)
├── services/               # GitHub API client, HTTP probes (§13)
├── i18n/                   # translation loader (UX §10)
├── assets/                 # embedded workflow_deploy.tmpl.yml (§13.5)
├── testing/                # fixtures + sshmock + ghmock (TESTING)
└── docs/                   # te dokumenty
```

### 2.2 Przepływ danych

Webox opiera się na architekturze jednokierunkowego przepływu danych (Elm/Charm Bubble Tea):

```text
┌────────────────────────────────────────────────────────────────────────┐
│                              tui/  (Bubble Tea)                        │
│  ┌─────────────────┐       ┌─────────────────┐       ┌──────────────┐  │
│  │   TUI Model     │ ────▶ │     View()      │ ────▶ │  Stdout TTY  │  │
│  │ (State & Tabs)  │       │ (Pure Render)   │       │ (Lipgloss)   │  │
│  └─────────────────┘       └─────────────────┘       └──────────────┘  │
│           ▲                                                           │
│           │ Update() (Pure State Mutation)                            │
│           │                                                           │
│    [User Keypress] ─── (Emit Msg) ───┐                                │
│    [Background Tick] ────────────────┼────────────────────────────────┘
│                                      ▼
│                            ┌───────────────────┐
│                            │    tea.Cmd Msg    │ (Asynchronous Side Effects)
│                            └─────────┬─────────┘
│                                      │
│             ┌────────────────────────┼────────────────────────┐
│             ▼                        ▼                        ▼
│      ┌──────────────┐        ┌──────────────┐        ┌──────────────┐
│      │  providers/  │        │  services/   │        │   secrets/   │
│      │ (SSH Exec)   │        │ (GitHub API) │        │  (Keyring)   │
│      └──────────────┘        └──────────────┘        └──────────────┘
```

### 2.3 Zasady przepływu danych (MVU)

Reguły, których trzymamy się **bezwyjątkowo** — wymuszają testowalność (`teatest` snapshot) i przewidywalność:

1. **`Update()` jest czystą funkcją.** Bierze `Msg`, zwraca nowy `Model` + opcjonalny `tea.Cmd`. **Brak** efektów ubocznych (`os.*`, `net.*`, channels) wewnątrz `Update`.
2. **Wszystkie efekty I/O są opakowane w `tea.Cmd`.** Funkcja `tea.Cmd` to `func() tea.Msg` — może blokować, ale runtime Bubble Tea wykonuje ją w goroutynie i wynik wraca jako `Msg` do `Update`.
3. **`View()` jest czystą funkcją renderującą.** Bierze `Model`, zwraca `string`. **Brak** mutacji stanu w `View`.
4. **Brak globalnych mutowalnych zmiennych.** Cały stan w `Model` lub w explicitnym `struct` przekazanym do `tea.NewProgram`.
5. **Tickery (`tea.Tick`)** dla auto-refresh dashboardu i animacji spinnera — zawsze w `Cmd`, nigdy w `Update`.
6. **Cancellation** propagowane przez `context.Context` do każdej operacji I/O. `Model` trzyma `context.CancelFunc` dla aktywnej długoterminowej operacji (np. wizard step), `q`/`Esc` wywołuje `cancel()`.
7. **Testy:** `Update` testowany jako pure function (input `Msg` → output `Model`), `View` snapshot testowany (`teatest`). I/O testowane osobno (provider × mock SSH).

---

## 3. Provider Pattern (Kontrakty v2)

Warstwa providerów całkowicie odcina logikę biznesową od specyfiki poszczególnych paneli hostingowych. Każdy adapter implementuje interfejs `HostingProvider` zdefiniowany w `providers/provider.go`:

```go
package providers

import (
	"context"
	"errors"
)

type ProviderStatus struct {
	SSHConnected bool
	CLIInstalled bool
	LatencyMS    int
}

type ProviderConfig struct {
	Alias      string            // np. "main"
	Type       string            // np. "smallhost"
	Host       string            // np. "s1.small.pl"
	Port       int               // 22 jeśli 0
	User       string            // login SSH
	Properties map[string]string // patrz §3.3
}

type HostingProvider interface {
	Name() string
	CreateSubdomain(ctx context.Context, domain string, nodeVersion string) error
	SetupSSL(ctx context.Context, domain string) error
	CreateDatabase(ctx context.Context, dbType string, dbName string) (user string, password string, err error)
	RestartNodeApp(ctx context.Context, domain string) error
	GetDeployPath(domain string) string
	GetLogPath(domain string) string
	CheckStatus(ctx context.Context) (*ProviderStatus, error)
	ListSubdomains(ctx context.Context) ([]string, error)
	RemoveSubdomain(ctx context.Context, domain string) error
	RemoveDatabase(ctx context.Context, dbName string) error
	RemoveSSL(ctx context.Context, domain string) error
}
```

### 3.1 Założenia kontraktu

- Wszystkie metody przyjmują `context.Context` z obsługą `Done()` (anulacja, deadline).
- Operacje `Remove*` są **idempotentne**: brak zasobu == sukces (`nil`). Inaczej DAG-rollback z §10 byłby fragile.
- `GetDeployPath` / `GetLogPath` są **czyste funkcje** (bez I/O) — wynik wyłącznie z `ProviderConfig.Properties`. Implementacja nie loguje, nie woła SSH.
- Adapter nie loguje sekretów — odpowiedzialność warstwy redaktora (§15).
- Sygnatury Go zdefiniowane wyżej są **kanoniczne** i każda zmiana = breaking change (`MAJOR` bump w [ROADMAP §2.1](./ROADMAP.md#21-semver)).

### 3.2 Kontrakt — HostingProvider (referowany z innych dokumentów)

Pełna lista metod ze szczegółami semantyki, możliwych błędów i mapowaniem na komendy konkretnego panelu żyje w plikach `docs/providers/<nazwa>.md`. Plik `docs/providers/smallhost.md` jest **wzorcem** — pozostałe muszą zachować tę samą strukturę nagłówków.

### 3.3 Properties bag

`ProviderConfig.Properties map[string]string` zawiera klucze specyficzne dla danego panelu. Konwencje:

- klucze w `snake_case`, wartości jako **string** (deserializacja per provider),
- każdy provider dokumentuje swój zestaw kluczy w sekcji `Properties bag` swojego pliku w `docs/providers/`,
- nieznany klucz = ignorowany (forward compatibility), brak wymaganego klucza = `ErrInvalidProviderConfig`,
- `restart_method` jest **wymagany** dla wszystkich providerów — wartości enum per provider (`smallhost`: `"devil"`, `cpanel`: `"passenger"` / `"app_manager"`, ...).

### 3.4 Defensywne parsowanie outputu

Adapter parsuje stdout/stderr panelu w trybie strict, zgodnie z [SECURITY.md §3.3](./SECURITY.md#33-defensywne-parsowanie-outputu). Krótkie reguły:

1. Strip ANSI escape sequences przed regexem.
2. Walidacja rozmiaru (max 1 MB per komenda).
3. Named regex groups, **nigdy** `eval`/`exec` na zawartości.
4. Nieparsujący się output → `ErrUnknownOutputFormat` + log diagnostyczny.
5. Każdy parser ma **golden file** z poprawnym + złośliwym przypadkiem ([TESTING.md §7](./TESTING.md#7-test-fixtures)).

```

---

## 4. Rejestr providerów

Fabryka adapterów rejestruje się automatycznie podczas inicjalizacji pakietu:

```go
// providers/registry.go
package providers

// Factory tworzy konkretną instancję providera na podstawie ProviderConfig (§3).
// Każdy adapter rejestruje się w init() pakietu — patrz CONTRIBUTING §3.2.
type Factory func(cfg ProviderConfig) (HostingProvider, error)

var registry = make(map[string]Factory)

// Register zapisuje fabrykę pod kluczem providerType (np. "smallhost").
// Wywoływane z init() pakietu adaptera; podwójna rejestracja = panic.
func Register(providerType string, factory Factory) {
	if _, exists := registry[providerType]; exists {
		panic("provider already registered: " + providerType)
	}
	registry[providerType] = factory
}

// New wyszukuje fabrykę po type i tworzy instancję; brak typu = ErrUnknownProvider.
func New(cfg ProviderConfig) (HostingProvider, error) { /* ... */ }
```

---

## 5. Warstwa SSH / SFTP Connection Pooling

Aby zapobiec banom za nadużycie połączeń (IP Rate Limiting) oraz skrócić czas reakcji interfejsu, Webox implementuje wewnętrzny **Connection Pool** o stałym rozmiarze (`max_connections=3` per host):

* **Dial Re-use:** Zamiast nawiązywania nowej sesji TCP/SSH przy każdej komendzie, sesja SSH (`ssh.Client`) jest współdzielona.
* **Keep-Alive Ticker:** Co 15 sekund Webox wysyła pakiet diagnostyczny SSH Global Request (`keepalive@openssh.com`), utrzymując otwarty tunel.
* **Graceful Release:** Nieużywane połączenia są zamykane po 60 sekundach bezczynności.

### 5.1 Algorithmy negocjacji

Webox eksplicitnie deklaruje listę dozwolonych algorytmów w `ssh.ClientConfig` — zgodnie z [SECURITY §5.5](./SECURITY.md#55-algorytmy). Lista jest **konserwatywna**, ale konfigurowalna przez `properties.ssh_algorithms_legacy_compat` na poziomie providera (default `false`) dla edge case'ów ze starymi serwerami negocjującymi tylko `ssh-rsa` (SHA-1).

### 5.2 Host key verification

`HostKeyCallback` w `ssh.ClientConfig` wskazuje na własny callback Webox, który:

1. Czyta `~/.config/webox/known_hosts` (dedykowany, **nie** `~/.ssh/known_hosts` — patrz [SECURITY §5.1](./SECURITY.md#51-lokalizacja-known_hosts)).
2. Match → continue. Brak wpisu → TOFU UI flow ([SECURITY §5.3](./SECURITY.md#53-pierwsze-po%C5%82%C4%85czenie-tofu)). Mismatch → strict block, **nigdy** auto-accept.

### 5.3 Connection pool — struktura

| Element | Wartość |
|---|---|
| Max połączeń per host | 3 (override przez `properties.ssh_pool_max`) |
| Keep-alive interval | 15 s (`keepalive@openssh.com` global request) |
| Idle timeout | 60 s (po którym pool zamyka połączenie) |
| Acquire timeout | 5 s (po którym `Acquire` zwraca `ErrPoolBusy`) |
| Reconnect strategy | 3 próby z backoffem `3s, 6s, 12s` (patrz §9) |
| Concurrency primitive | `sync.Mutex` + buffered channel jako semafora |

Wszystkie operacje (`Exec`, `Run`, `OpenSession`) idą przez `pool.Acquire(ctx, host)` → użycie sesji → `pool.Release(host, session)`. Brak ręcznego zarządzania `ssh.Client` poza pakietem `ssh/`.

### 5.4 SFTP

Operacje plikowe (`PutFile`, `GetFile`, `Stat`) używają tej samej sesji SSH przez `github.com/pkg/sftp` (SFTP nadbudowane na SSH). Dla `Put` z atomowością: upload do `<path>.tmp`, `fsync` przez SFTP, `Rename` do `<path>`.

---

## 6. Model danych i atomowość zapisu config.json

Webox dba o nienaruszalność lokalnego stanu konfiguracji. Zapis pliku `~/.config/webox/config.json` odbywa się wyłącznie za pomocą mechanizmu transakcyjnego w pakiecie `config/`:

1. **Lockfile Acquisition:** webox otwiera `config.lock` z `flock(2)` (POSIX) lub `LockFileEx` (Windows) przez wrapper `github.com/gofrs/flock`. **Nie polegamy** na PID-only sprawdzeniu — PID jest racey (proces może crashować, PID jest re-wykorzystywany przez OS). `flock` rozwiązuje problem natywnie. Jeśli lock nie jest dostępny po 1 s — `ErrConfigLocked` + komunikat *"Webox is already running. Close the other instance or wait."*.
2. **Atomic Write & fsync:** Nowy bufor konfiguracji jest zapisywany jako `config.json.tmp` w tym samym katalogu (wymagane dla atomic rename na tym samym filesystem). Następuje wywołanie `f.Sync()` (mapowane na `fsync(2)`) w celu fizycznego zapisu na dysku przed atomicznym rename'em.
3. **Atomic Replace:** Następuje wywołanie `os.Rename("config.json.tmp", "config.json")`. Na POSIX jest to natywnie atomic (single inode swap). Na Windows wymaga `MoveFileExW` z `MOVEFILE_REPLACE_EXISTING`.
4. **Backup on schema migration:** przed `Save()` wykonującym migrację schematu (`v1 → v2`), webox kopiuje stary plik do `config.json.<schema_version>.bak` (patrz §6.4).
5. **Release lock:** `flock` zwolnione w `defer`.

### 6.1 Schema config.json

Plik `~/.config/webox/config.json` zawiera **tylko metadane**, **bez sekretów** (zgodnie z [SECURITY §4](./SECURITY.md#4-przechowywanie-sekret%C3%B3w)):

| Pole | Typ | Wymagane | Opis |
|---|---|---|---|
| `schema_version` | int | tak | Numer wersji schematu. `v0.1` = `1`. Każda zmiana niezgodna → bump + migracja (§6.4). |
| `language` | string | nie (default `"en"`) | Kod języka UI (`en`, `pl`, ...). Patrz [ADR-0006](./adr/0006-jezyk-interfejsu-en-domyslny.md). |
| `profiles` | array | tak | Lista profili hostingowych. |
| `profiles[].alias` | string | tak | Alias profilu (`main`, `client-x`). Regex `^[a-z0-9-]{1,32}$`. |
| `profiles[].type` | string | tak | Typ providera (`smallhost`, `cpanel`, ...). |
| `profiles[].host` | string | tak | Host SSH (np. `s1.small.pl`). |
| `profiles[].port` | int | nie (default `22`) | Port SSH. |
| `profiles[].user` | string | tak | Login SSH. |
| `profiles[].properties` | object | nie | `map[string]string` (§3.3). |
| `projects` | array | tak | Lista projektów. |
| `projects[].id` | string | tak | UUID. |
| `projects[].domain` | string | tak | Pełna domena (`sub.user.smallhost.pl`). |
| `projects[].profile_alias` | string | tak | FK do `profiles[].alias`. |
| `projects[].repo` | string | nie | `org/name` slug GitHuba. |
| `projects[].local_path` | string | nie | Lokalna ścieżka do repo. |
| `projects[].stack` | string | nie | `vite-react`, `node-express`, `static`, ... |
| `projects[].node_version` | string | nie | Wersja Node z serwera. |
| `projects[].imported_at` | RFC3339 | nie | Set dla importowanych (banner *"Settings incomplete"*). |
| `projects[].secrets_meta` | array | nie | Metadane sekretów aplikacji (patrz [SECURITY §10.6](./SECURITY.md#106-rotacja-sekret%C3%B3w-aplikacji--metadane-i-warningi)). |
| `settings` | object | nie | Globalne preferencje (`expert_mode`, `refresh_interval_s`, ...). |

Pełny przykład jako fixture w `testing/fixtures/config/valid_v1_0.json` (patrz [TESTING §7](./TESTING.md#7-test-fixtures)).

### 6.2 Permisje

`~/.config/webox/config.json` ma permisje `0600` (owner only). Webox sprawdza permisje przy load i warning gdy są szersze ([SECURITY §7](./SECURITY.md#7-audyt-sekret%C3%B3w-i-tryb-doctor)).

### 6.3 Atomic save — pseudokod

```text
1. acquire flock(config.lock, exclusive, timeout=1s)
2. read existing config (jeśli istnieje), keep original bytes
3. validate JSON Schema na nowej wartości
4. write to config.json.tmp (0600 perms)
5. fsync(config.json.tmp)
6. rename(config.json.tmp, config.json)
7. fsync(directory) — wymagane dla rename durability na ext4
8. release flock
```

### 6.4 Migracje schematu

Przy load:

1. webox czyta `schema_version` z pliku.
2. Jeśli `version > supported_version` → `ErrConfigSchemaNewer` (downgrade nie wspierany).
3. Jeśli `version < supported_version` → łańcuch migratorów `migrate_v1_to_v2`, `migrate_v2_to_v3`, ... aż do current. Każdy migrator jest **idempotentny** (np. uruchomienie `v1 → v2` na configu już `v2` jest no-op).
4. Po pomyślnej migracji — backup oryginału jako `config.json.<old_version>.bak` + zapis nowej wersji przez §6.3.

Lista migratorów w `config/migrations.go`. Każdy ma test jednostkowy (TESTING §2.1).

---

## 7. Zarządzanie sekretami (Keyring integration)

Krytyczna zasada: **Zero sekretów w pliku konfiguracyjnym**.
* **Integracja z Keyring:** Używamy `github.com/zalando/go-keyring`. Hasła do baz danych, tokeny GitHub Personal Access Token (PAT) oraz klucze aplikacji są zapisywane w systemowym pęku kluczy (macOS Keychain, Linux Secret Service przez dbus, Windows Credential Manager).
* **Headless Fallback:** Na maszynach serwerowych bez aktywnego serwera dbus, Webox inicjuje zdeklarowany magazyn szyfrowany symetrycznie za pomocą algorytmu **AES-256-GCM**. Klucz deszyfrujący (Master Key) jest wyprowadzany za pomocą funkcji **Argon2id** z hasła podanego przez użytkownika podczas startu sesji.

---

## 8. Trójpoziomowy Status Cache (Stale-While-Revalidate)

W celu zachowania płynności UI (kluczowe kryterium wydajności `K5`), interfejs TUI pobiera dane z asynchronicznej pamięci podręcznej. Zgodnie z [ADR-0005](./adr/0005-cache-statusow-projektow.md) cache implementujemy jako **wzorzec funkcyjny** — pakietową funkcję `GetOrFetch`, **nie** generyczną metodę na strukturze (Go nie wspiera generyków na metodach, a opracowanie typowane przez `any` było źródłem błędów w prototypie — patrz [CHANGES.md §1 6.1](../CHANGES.md#1-poprawki-merytoryczne-z-tabeli-%C2%A76-briefu)).

### 8.1 Kontrakt funkcji

Pakiet `status` udostępnia generyczną funkcję pakietową:

```text
status.GetOrFetch[T any](
    cache *Cache,
    key   string,
    ttl   time.Duration,
    fetch func(ctx context.Context) (T, error),
    ctx   context.Context,
) (data T, isStale bool, err error)
```

Semantyka:

- **Cache hit (świeży):** zwraca `data, false, nil` natychmiast bez wywołania `fetch`.
- **Cache stale (przeterminowany, ale obecny):** zwraca starą wartość z `isStale=true`, asynchronicznie odpala `fetch` w tle przez `singleflight` (`golang.org/x/sync/singleflight`).
- **Cache miss (cold start):** blokująco woła `fetch(ctx)`, zapisuje wynik z `expiresAt = now + ttl`.
- **Anulacja:** `ctx` z deadline'em propagowany do `fetch`. Anulacja po stronie konsumenta nie blokuje fetch'a w tle — ten ma własny `context.Background()` z 30 s timeoutem.

Cache trzyma `sync.RWMutex` + `map[string]cacheEntry`. Singleflight zapewnia, że dwie równoległe operacje na tym samym kluczu wywołają `fetch` tylko raz.

### 8.2 Parametry TTL i invalidacji

Tabela jednoznacznie definiuje TTL i triggery invalidacji eventowej (zgodnie z [ADR-0005 §Parametry cache](./adr/0005-cache-statusow-projektow.md#parametry-cache)):

| Klucz cache | TTL | Cold-start fetch | Invalidacja eventowa |
|---|---|---|---|
| `http:<domain>` | 30 s | `GET https://<domain>` (1 s timeout) | `Restart`, `Deploy`, `RemoveSubdomain` |
| `ssh:node:<domain>` | 60 s | `node --version` przez SSH pool | `ChangeNodeVersion` |
| `ssl:<domain>` | 300 s | TLS handshake + `peer.NotAfter` | `SetupSSL`, `RenewSSL`, `RemoveSSL` |
| `gh:lastDeploy:<repo>` | 60 s | GitHub Actions API `runs` | `Deploy` (push trigger) |

Po wywołaniu jednej z eventowych akcji wizard / dashboard wykonuje `cache.Invalidate(prefix)` — najprostszy sposób, bo prefixy są deterministyczne (`http:`, `ssh:node:`, `ssl:`, `gh:lastDeploy:`).

### 8.3 Świeżość w UI

Badge "buffered" w prawym górnym rogu komórki dashboardu zmienia kolor:

| Wiek danych | Stan UI |
|---|---|
| `<= ttl` | brak badge'a — świeże |
| `(ttl, ttl + 60s]` | `(buffered Ns ago)` — szary |
| `(ttl + 60s, ttl + 180s]` | żółty |
| `> ttl + 180s` | pomarańczowy, sugeruje force refresh (`Ctrl+R`) |

### 8.4 Wzorzec funkcyjny — uzasadnienie

Wcześniejszy prototyp w `archive/PRD_v0_monolith.md` próbował zdefiniować `func (c *Cache) GetOrFetch[T any](...)`. **Go 1.24 nadal nie pozwala na generyki na metodach** — kompilator odrzuci taki kod. Implementację robimy więc:

1. Jako funkcja pakietowa `status.GetOrFetch[T any]` ponad strukturą `Cache` (działa).
2. Z opcjonalnymi wrapperami per-domain w pakiecie wywołującym, np. `status.HTTPCacheGet(c, domain)` (typowo `T = int` dla status code), które po prostu wywołują `GetOrFetch[int]` z properly typowanym fetchem.

---

## 9. Obsługa błędów sieciowych i reconnect

W przypadku zerwania połączenia SSH w trakcie wykonywania procedur operatorskich:
1. **Zatrzymanie stanu TUI:** TUI przechodzi w stan przejściowy, zamrażając interakcję użytkownika i wyświetlając w help barze alert: `Connection lost. Attempting reconnect (1/3)...`.
2. **Exponential Backoff:** System podejmuje maksymalnie 3 próby nawiązania sesji SSH z interwałem kolejno `3s`, `6s` oraz `12s`.
3. **Execution Recovery:** Jeśli połączenie zostanie odzyskane, ostatnia niedokończona komenda z bufora jest bezpiecznie ponawiana. W przypadku trwałego błędu połączenia, system zapisuje stan do pliku `pending_cleanups.json` i przechodzi do diagnozy.

---

## 10. DAG-based Transactional Engine (Wznawialny Rollback)

Zastępujemy prosty stos LIFO acyklicznym grafem skierowanym (DAG) dla operacji scaffoldingowych i instalacyjnych. Pozwala to na precyzyjne śledzenie zależności i wznawianie przerwanej pracy (Resumable Workflows).

```
          [1. Pre-flight Check]
             /             \
            ▼               ▼
     [2. Subdomain]     [3. Scaffold Files]
            \               /
             ▼             ▼
         [4. Provision Database]
                    │
                    ▼
          [5. SSL Let's Encrypt]
                    │
                    ▼
          [6. GitHub Git Push]
```

### 10.1 Struktura silnika transakcyjnego

```go
// wizard/dag.go
package wizard

type NodeID string
type NodeState int

const (
	StatePending NodeState = iota
	StateRunning
	StateSuccess
	StateFailed
	StateSkipped
)

type StepNode struct {
	ID           NodeID
	DependsOn    []NodeID
	State        NodeState
	ExecuteFunc  func(ctx context.Context, params map[string]any) (map[string]any, error)
	RollbackFunc func(ctx context.Context, results map[string]any) error
}

type ExecutionDAG struct {
	TransactionID string
	Nodes         map[NodeID]*StepNode
	Params        map[string]any
	Results       map[NodeID]map[string]any
}
```

### 10.2 Logika wznowienia i selektywnego rollbacku

Gdy dany węzeł DAG (np. `Provision Database`) zgłosi błąd, silnik:
1. Oznacza stan węzła jako `StateFailed`.
2. Wstrzymuje wykonywanie węzłów zależnych (np. `SSL Let's Encrypt`).
3. Zapisuje pełen zrzut stanu grafu oraz zebrane dotychczas rezultaty (`Results`) do pliku `pending_cleanups.json`.
4. Wyświetla interfejs naprawczy w TUI. Jeśli użytkownik skoryguje parametry i wybierze **Resume**:
   * Silnik podmienia parametry wejściowe `Params`.
   * Resetuje stan węzła z `StateFailed` na `StatePending`.
   * Uruchamia ponownie procesor grafu, pomijając węzły o stanie `StateSuccess` (redukcja czasu i kosztów zasobów na serwerze).

---

## 11. Detekcja rozbieżności konfiguracji (Drift & Stale detection)

> 🔵 **MVP (v0.1)** — Stale Domain Detection. Env File Drift jest **🔶 STRETCH (v0.2+)** (wymaga `/env` post-MVP).

Webox realizuje funkcję **Configuration Drift Detection** w celu wykrycia zmian dokonanych bezpośrednio przez panel WWW Devil poza kontrolą TUI:

* **Stale Domain Detection (MVP):** Podczas każdego cyklu auto-refresh, Webox wykonuje w tle polecenie `ListSubdomains` na aktywnym profilu. Jeśli zmapowany lokalnie projekt nie występuje na zwróconej liście serwerowej, w modelu TUI projekt otrzymuje stan `STALE` i timestamp wykrycia driftu. Zgodnie z `F23` w [PRD §6](./PRD.md#6-ficzery--z-priorytetami).
* **Env File Drift (STRETCH):** zakładka `Env Diff` (post-MVP) pobiera zawartość pliku `.env` z serwera i porównuje z lokalnym. Szczegóły poniżej w §11.1.

### 11.1 Architektura Dwukierunkowego Env Merger (TUI Env Merger Engine)

> 🔶 **STRETCH (v0.2+)** — wymaga sub-widoku `/env` post-MVP. W `v0.1` ta sekcja jest tylko architekturalnym planem; implementacja po dostarczeniu MVP. [ROADMAP §3.3](./ROADMAP.md#33-czego-nie-ma-w-mvp).

Aby zapewnić pełną spójność i transakcyjność operacji na zmiennych środowiskowych, Webox implementuje silnik synchronizacji różnicowej w pakiecie `env/merger.go`.

#### Algorytm Synchronizacji Różnicowej:
1. **Asynchroniczne pobranie (Pull):** W momencie wejścia w interakcję z Env Resolverem, silnik za pomocą połączenia SFTP z Connection Poola pobiera zawartość pliku `.env` z serwera produkcyjnego.
2. **Parsowanie i Mapowanie (Key-Value AST):** Oba pliki (lokalny i serwerowy) are parsed na abstrakcyjne struktury asocjacyjne z zachowaniem kolejności oraz komentarzy.
3. **Wizualizacja Rozbieżności (Drift Resolution State):**
   * Jeśli klucz istnieje tylko w jednym pliku, otrzymuje flagę `ORPHANED`.
   * Jeśli wartości dla tego samego klucza się różnią, generowany jest hash MD5 obu ciągów i oznaczany jako `DRIFT`.
4. **Transakcyjny Zapis Różnicowy (Chirurgiczny Push):**
   Gdy użytkownik zatwierdza scalenie klawiszem `Enter`, silnik wykonuje operację dwuetapową:
   * **Kopia bezpieczeństwa (Backup):** Z poziomu SFTP tworzony jest plik `.env.bak` na serwerze produkcyjnym.
   * **Atomic Overwrite:** Nowy plik `.env` jest zapisywany. Jeśli zapis się powiedzie, Webox natychmiast wysyła komendę `RestartNodeApp`. W przypadku awarii restartu, system automatycznie przywraca `.env` z pliku `.env.bak`, chroniąc przed przestojem (Downtime mitigation).

---

## 12. Maszyna stanów TUI (Tabbed Cockpit Spec)

Rozszerzamy główne stany interfejsu TUI zdefiniowane w [UX.md §12](./UX.md#12-maszyna-stanów-tui-tabbed-cockpit-spec) o podstany zakładek szczegółów projektu. Zapobiega to przeładowaniu pojedynczego kontrolera.

```go
// tui/states.go
package tui

type State int

const (
	StateInitWizard State = iota
	StateDashboard
	StateProjectDetail // Główny stan szczegółów projektu
	StateCommandPalette
	StateConfirmDialog
)

type DetailTab int

const (
	TabOverview DetailTab = iota
	TabEnvDiff
	TabDatabase
	TabLogs
)

type Model struct {
	state      State
	activeTab  DetailTab // Aktywna pod-karta w szczegółach projektu
	width, height int
	// ... pozostałe pola modelu
}
```

### Przepływ nawigacji (Tabbed Routing)

* Gdy `state == StateProjectDetail`, klawisze `H` / `L` lub cyfry `1`-`4` mutują wyłącznie pole `activeTab` w modelu.
* Wywołanie `view()` dla `StateProjectDetail` deleguje render do dedykowanych funkcji w pakiecie `tui/views`:
  * `TabOverview` → `views.RenderOverview(m)`
  * `TabEnvDiff` → `views.RenderEnvDiff(m)`
  * `TabDatabase` → `views.RenderDatabase(m)`
  * `TabLogs` → `views.RenderLogs(m)`
* Klawisz `Esc` lub `←` resetuje stan bezpośrednio do `StateDashboard`.

---

## 13. Integracja z GitHubem (Actions & Deployment Keys)

Bezpieczeństwo wdrożeń CI/CD (Blast Radius mitigation):
* **Surgical Deployment Keys:** Webox **nie współdzieli** głównego klucza SSH użytkownika z GitHubem. Dla każdego nowo tworzonego projektu, Webox generuje dedykowaną parę kluczy SSH w locie (`ssh-keygen -t ed25519 -f temp_deploy_key`).
* **GitHub Secret Provisioning:** Część prywatna klucza ląduje bezpośrednio w GitHub Secrets danego repozytorium pod nazwą `SSH_PRIVATE_KEY` (za pomocą API GitHuba).
* **Devil Authorization:** Część publiczna klucza jest dopisywana przez SFTP wyłącznie do pliku `~/.ssh/authorized_keys` na koncie współdzielonym serwera, z jawnym komentarzem `# Webox Deploy Key for Repo [name]`.

### 13.5 Szablon workflow parametryzowany

Webox generuje `.github/workflows/deploy.yml` w repo projektu na podstawie szablonu **osadzonego w binarce** przez `embed.FS`:

```text
//go:embed assets/workflow_deploy.tmpl.yml
var workflowTemplate embed.FS
```

Parametryzacja przez `text/template` z polami opisanymi w [`docs/providers/smallhost.md §6`](./providers/smallhost.md#6-deployment-workflow-szablon). Każdy provider ma własny szablon (`assets/workflow_deploy_smallhost.tmpl.yml`, etc.).

Po renderowaniu webox **waliduje** wynik:

1. YAML parsing (`gopkg.in/yaml.v3`) — błąd → fatal (nie commitujemy zepsutego YAML do repo).
2. Schema check (sprawdzenie obecności wymaganych pól: `name`, `on`, `jobs`).
3. Diff vs istniejący `deploy.yml` jeśli plik już jest — webox prosi user'a o confirm zanim nadpisze.
4. Commit przez API GitHuba (`PUT /repos/:owner/:repo/contents/.github/workflows/deploy.yml`), nie przez lokalny `git push` (zachowuje historię nawet jeśli user nie ma repo zclonowanego).

Szablon **musi** używać `actions/checkout`, `actions/setup-node` i `shimataro/ssh-key-action` z **pinned SHAs**, nie tags (zgodnie z [SECURITY §8.2](./SECURITY.md#82-supply-chain--release-pipeline)).

`rsync` w workflow używa **excludes** dla persistent dirs (np. `--exclude='uploads/' --exclude='.env'`) — patrz audit C6 i `providers/smallhost.md §6`.

---

## 14. Dystrybucja i mechanizm sprawdzania wersji

* **Async Version Checker:** 2 sekundy po uruchomieniu aplikacji, system w asynchronicznej goroutynie odpytuje endpoint GitHub API (`GET /repos/webox/releases/latest`).
* **UX Alert Integration:** Jeśli wersja serwerowa jest wyższa niż zakodowana w aplikacji stała `Version`, model TUI otrzymuje flagę `updateAvailable = true` oraz wartość nowej wersji. Nagłówek Dashboardu wyświetla wtedy nieinwazyjny, błyszczący badge `Update: vX.Y.Z available`.

---

## 15. Diagnostyka (Doctor & Redacted Logger)

* **Redacted Local Logger:** Przy włączonym trybie debugowania (`webox --debug`), logi diagnostyczne są zapisywane w pliku `webox.log`. Wszystkie dane wrażliwe (hasła, tokeny, ścieżki prywatne) są filtrowane przez silnik regex przed zapisem na dysk.
* **Doctor System Diagnostics:** Komenda `webox doctor` dokonuje bezinwazyjnego audytu lokalnego środowiska deweloperskiego, sprawdzając spójność bazy kluczy Keyring, uprawnienia do zapisu konfiguracji oraz statusy pingów TCP do serwerów hostingowych.

### 15.1 Kategorie checków

| Kategoria | Przykłady | Severity domyślny |
|---|---|---|
| `system` | binarki (git, ssh-keygen, gh), Go version, perms `~/.config/webox/` | fatal |
| `network` | TCP probe :22 dla profili, DNS lookup time | warn |
| `server` | `devil --version`, `node --version` per profile | warn |
| `security` | scan `config.json` pod kątem plaintext secrets, perms 0600, keyring sentinel write/read | fatal |
| `github` | `gh auth status`, `gh api /rate_limit` | warn |

### 15.2 Redacted logger — wzorce

Lista regexów do redakcji w `internal/log/redact.go`. Każdy match → `*** REDACTED ***`:

| Wzorzec | Co matchuje |
|---|---|
| `gh[ps]_[A-Za-z0-9]{36,255}` | GitHub PAT (classic) + fine-grained |
| `github_pat_[A-Za-z0-9_]{82}` | GitHub fine-grained PAT |
| `ey[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+` | JWT |
| `-----BEGIN [A-Z ]+PRIVATE KEY-----` | PEM private key block |
| `(?i)(password\|passwd\|secret\|token)\s*[:=]\s*\S+` | key=value pairs |
| `/Users/[^/]+\|/home/[^/]+` | home directories (privacy) |
| `\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b` | emails |

Testy redaktora w `testing/fixtures/log/redact_corpus.txt` (gold standard input → expected redacted output, patrz TESTING §2.3).

### 15.3 JSON Schema raportu doctor

`webox doctor --json` zwraca structured output dla skryptów CI/automation. Pełny przykład poniżej, schemat w `assets/doctor_report.schema.json` (osadzony przez `embed.FS`):

```json
{
  "schema_version": 1,
  "generated_at": "2026-05-20T12:34:56Z",
  "webox_version": "0.1.0",
  "platform": { "os": "darwin", "arch": "arm64", "go_version": "1.24.0" },
  "checks": [
    {
      "id": "system.git_installed",
      "category": "system",
      "label": "Git binary",
      "severity": "fatal",
      "status": "ok",
      "duration_ms": 12,
      "detail": "git version 2.43.0"
    },
    {
      "id": "security.config_clean",
      "category": "security",
      "label": "No plaintext secrets in config",
      "severity": "fatal",
      "status": "fail",
      "duration_ms": 5,
      "detail": "Pattern 'github_pat_' detected in profiles[0].properties (REDACTED)",
      "remediation_url": "https://docs.webox.dev/security/secrets-leak"
    }
  ],
  "summary": { "ok": 16, "warn": 1, "fail": 1, "skipped": 0 }
}
```

`severity ∈ {info, warn, fatal}`. `status ∈ {ok, warn, fail, skipped}`. Exit code z `webox doctor`: `0` (wszystko ok), `1` (warn-only), `2` (fail).

---

## 16. Concurrency & Visual Motion Engine (Pulsacje i Spinnery)

> 🔵 **MVP (v0.1)** — Latency-Aware Spinner (§16.2) jest częścią MVP, bo wpływa na percepcję responsywności. 🔶 **STRETCH (v0.2+)** — Sinusoidal Border Pulsing (§16.1) to czysto kosmetyczna animacja, implementacja dopiero po MVP.

Silnik wizualny wykorzystuje asynchroniczne pętle czasowe (Goroutines & Channels) w Bubble Tea do animowania klatek interfejsu.

### 16.1 Sinusoidalne Pulsowanie Ramek (Border Pulsing) — STRETCH

> 🔶 Implementacja **po** v0.1. Sekcja zachowana jako specyfikacja architektoniczna.

Podczas operacji w tle (np. długotrwałe wdrażanie kodu lub restart), ramka aktywnego Bento Boxa pulsuje kolorystycznie. Mechanizm ten jest sterowany matematycznie za pomocą funkcji sinus:

```go
type PulseMsg time.Time

func TickPulse() tea.Cmd {
	return tea.Tick(time.Millisecond*80, func(t time.Time) tea.Msg {
		return PulseMsg(t)
	})
}

// Wewnątrz pętli Update()
case PulseMsg:
	m.ticks++
	// Obliczenie jasności HSL w zakresie 40% - 75%
	lightness := 57.5 + 17.5*math.Sin(float64(m.ticks)*0.2)
	m.pulsingColor = oklchToHex(0.60, 0.24, 280, lightness)
	return m, TickPulse()
```

Dzięki temu Lipgloss generuje płynne przejście kolorów ramki Bento w locie, dając niesamowity efekt animacji bez obciążania procesora.

### 16.2 Latency-Aware Spinner Scheduler

Częstotliwość odświeżania spinnera w Bubble Tea jest kontrolowana dynamicznie. Zamiast stałego interwału czasowego, system mierzy średnie opóźnienie ping SSH dla aktywnego profilu hostingowego i dostosowuje interwał odpytywania ramek:

```go
func (m *Model) GetSpinnerTick() time.Duration {
	rtt := m.sshPool.GetAverageRTT()
	if rtt < 30 * time.Millisecond {
		return 50 * time.Millisecond // Szybkie łącze -> płynny spinner 50ms
	}
	if rtt < 120 * time.Millisecond {
		return 100 * time.Millisecond // Standardowy spinner 100ms
	}
	return 200 * time.Millisecond // Wolne łącze -> powolny spinner 200ms w celu redukcji obciążenia TTY
}
```

---

## 17. Architektura Silnika Dźwiękowego w Go (Package `sound`)

> 🔶 **STRETCH (v0.2+ lub LATER)** — silnik audio nie jest częścią MVP. Specyfikacja zachowana jako materiał do osobnego RFC. **Uwaga techniczna:** wcześniejsza wersja tej sekcji proponowała `/dev/dsp` jako interfejs ALSA — to przestarzałe (OSS legacy). Realna implementacja użyje `oto` / `beep` Go bibliotek z platform-detect (`afplay` na macOS, `paplay`/`aplay` na Linux z PulseAudio/PipeWire) lub ANSI BEL fallback. Punkty audio rozważymy dopiero po dostarczeniu MVP, w osobnym RFC, z weryfikacją user value.

### 17.1 Struktura Pakietu

```go
// sound/player.go
package sound

import (
	"context"
	"sync"
	"time"
)

type SoundType int

const (
	SoundClick SoundType = iota
	SoundTabSwitch
	SoundConfirm
	SoundChimeSuccess
	SoundDroneError
)

type SoundEngine struct {
	mu        sync.Mutex
	isMuted   bool
	soundChan chan SoundType
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewEngine() *SoundEngine {
	ctx, cancel := context.WithCancel(context.Background())
	se := &SoundEngine{
		soundChan: make(chan SoundType, 10),
		isMuted:   true, // Domyślnie wyciszony (Opt-In policy)
		ctx:       ctx,
		cancel:    cancel,
	}
	go se.startEventLoop()
	return se
}

func (se *SoundEngine) ToggleMute() bool {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.isMuted = !se.isMuted
	return se.isMuted
}

func (se *SoundEngine) Play(t SoundType) {
	se.mu.Lock()
	muted := se.isMuted
	se.mu.Unlock()

	if muted {
		return
	}

	select {
	case se.soundChan <- t:
	default:
		// Bufor pełny - zrzuć paczkę dźwiękową, aby nie blokować interfejsu
	}
}
```

### 17.2 Niskopoziomowy Event Loop i Generowanie PCM
Generowanie dźwięku wykorzystuje bezpośrednie odpytanie systemowych interfejsów audio lub wysyłanie odpowiednich kodów częstotliwości ANSI:
* **macOS/Linux:** Silnik wykorzystuje natywne wywołania niskopoziomowe przez API `/dev/dsp` lub platform-native CLI (np. `afplay` w macOS, `aplay` w Linux), uruchamiane w odseparowanej od wątku TUI goroutynie.
* **ANSI BEEP Fallback:** Na maszynach, gdzie fizyczne audio jest niedostępne (np. sesja SSH bez przekierowania dźwięku), silnik emituje krótkie sekwencje kodów ucieczki terminala (`\a` - BEL) o modulowanym czasie trwania, tworząc efekt mikro-klików w głośniku systemowym emulatora.

---

## 18. Silnik Dynamicznej Topologii (Live Service Topology Map Engine)

> 🔶 **STRETCH (v0.2+)** — wymaga trybu Bento Ultra (`≥120×35`), który sam jest stretch (patrz [UX.md §4.2](./UX.md#42-dashboard-20--bento-box-grid-system-12035)). MVP używa Standard Cockpit (100×30) i listy projektów bez wizualnej topologii. Specyfikacja zachowana dla planowania v0.2+.

Generowanie wizualnego grafu połączeń realizowane jest przez moduł `tui/views/topology.go`, który przetwarza stany zasobów z trójpoziomowej pamięci podatnej (`StatusCache`) i buduje dynamiczny rysunek ASCII w locie.

### 18.1 Algorytm Budowania Grafu w TUI

```go
// tui/views/topology.go
package views

import (
	"strings"
	"github.com/charmbracelet/lipgloss"
)

type TopologyNode struct {
	Label  string
	Status string // "ONLINE", "BUILDING", "OFFLINE"
}

func BuildServiceTopology(nodes []TopologyNode, width int) string {
	if width < 120 {
		return "" // Zwróć pusty string pod progami elastyczności (Bento Ultra only)
	}

	// Style Lipgloss
	primaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4E5A85"))

	var builder strings.Builder

	// Renderowanie ścieżki i weryfikacja połączeń
	// Silnik dynamicznie podmienia znaki ramkowe w zależności od statusu
	// np. ONLINE: ───▶, BUILDING: ═ ═ ▶, OFFLINE: ⚡⚡⚡
	
	return builder.String()
}
```

### 18.2 Cykliczny Refresh i Redukcja Narzutu (CPU Protection)
Aby animowanie dynamicznego grafu (w tym pulsowanie uszkodzonych połączeń i ruch strzałek wdrożeniowych) nie obciążało procesora:
1. **Zdarzenia tylko przy zmianach:** Dane o infrastrukturze są pobierane w interwale SWR (Stale-While-Revalidate) co 10 sekund.
2. **Taktowanie Animacji:** Klatki animacji graficznej (np. miganie koloru czerwonego `⚡ ⚡` lub przesunięcie kropek w `═ ═ ═ ▶`) są taktowane co `500ms`, co daje estetyczny, powolny ruch, generując pomijalne obciążenie TTY (poniżej 0.5% zużycia CPU).
