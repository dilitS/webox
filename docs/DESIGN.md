# Webox — Design / Architektura (Generacja 2026/2027)

> Status: Approved · Ostatnia aktualizacja: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [PRD.md](./PRD.md) (cele produktowe), [UX.md](./UX.md) (szczegółowy design system i makiety), [SECURITY.md](./SECURITY.md) (model zaufania), [TESTING.md](./TESTING.md) (strategia testowania).

---

## TL;DR

Webox to lekki, wysoce niezawodny monolit w Go zorganizowany według paradygmatu **MVU (Model-View-Update)**. Wersja 2.0 (generacja 2026/2027) rozwija architekturę o trzy krytyczne komponenty: **DAG-based Transactional Engine** (umożliwiający selektywny rollback i wznawianie przerwanych kreatorów), podstany routera TUI obsługujące **zintegrowany cockpit zakładkowy (Tabbed Cockpit)**, oraz asynchroniczny silnik wizualny **Concurrency & Motion Engine** (dynamiczne, oparte o pingi spinnery i sinusoidalne pulsowanie ramek).

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

Webox opiera się na architekturze jednokierunkowego przepływu danych (Elm/Charm Bubble Tea):

```
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
	CPINalled    bool
	LatencyMS    int
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

---

## 4. Rejestr providerów

Fabryka adapterów rejestruje się automatycznie podczas inicjalizacji pakietu:

```go
// providers/registry.go
package providers

type Factory func(properties map[string]string) (HostingProvider, error)

var registry = make(map[string]Factory)

func Register(providerType string, factory Factory) {
	registry[providerType] = factory
}
```

---

## 5. Warstwa SSH / SFTP Connection Pooling

Aby zapobiec banom za nadużycie połączeń (IP Rate Limiting) oraz skrócić czas reakcji interfejsu, Webox implementuje wewnętrzny **Connection Pool** o stałym rozmiarze (`max_connections=3` per host):

* **Dial Re-use:** Zamiast nawiązywania nowej sesji TCP/SSH przy każdej komendzie, sesja SSH (`ssh.Client`) jest współdzielona.
* **Keep-Alive Ticker:** Co 15 sekund Webox wysyła pakiet diagnostyczny SSH Global Request (`keepalive@openssh.com`), utrzymując otwarty tunel.
* **Graceful Release:** Nieużywane połączenia są zamykane po 60 sekundach bezczynności.

---

## 6. Model danych i atomowość zapisu config.json

Webox dba o nienaruszalność lokalnego stanu konfiguracji. Zapis pliku `~/.config/webox/config.json` odbywa się wyłącznie za pomocą mechanizmu transakcyjnego w pliku `config/config.go`:

1. **Lockfile Acquisition:** Tworzony jest plik blokady `config.lock`. Jeśli plik istnieje i PID procesu wewnątrz lockfile jest aktywny, zapis zostaje wstrzymany z komunikatem błędu concurrency.
2. **Atomic Write & fsync:** Nowy bufor konfiguracji jest zapisywany jako `config.json.tmp`. Następuje wywołanie `fsync()` na deskryptorze pliku w celu fizycznego zapisu na dysku.
3. **Atomic Replace:** Następuje wywołanie systemowe POSIX `os.Rename("config.json.tmp", "config.json")`. W systemach Windows operacja ta jest poprzedzona zwolnieniem blokady zapisu.

---

## 7. Zarządzanie sekretami (Keyring integration)

Krytyczna zasada: **Zero sekretów w pliku konfiguracyjnym**.
* **Integracja z Keyring:** Używamy `github.com/zalando/go-keyring`. Hasła do baz danych, tokeny GitHub Personal Access Token (PAT) oraz klucze aplikacji są zapisywane w systemowym pęku kluczy (macOS Keychain, Linux Secret Service przez dbus, Windows Credential Manager).
* **Headless Fallback:** Na maszynach serwerowych bez aktywnego serwera dbus, Webox inicjuje zdeklarowany magazyn szyfrowany symetrycznie za pomocą algorytmu **AES-256-GCM**. Klucz deszyfrujący (Master Key) jest wyprowadzany za pomocą funkcji **Argon2id** z hasła podanego przez użytkownika podczas startu sesji.

---

## 8. Trójpoziomowy Status Cache (Stale-While-Revalidate)

W celu zachowania płynności UI (kluczowe kryterium wydajności `K5`), interfejs TUI pobiera dane z asynchronicznej pamięci podręcznej.

```go
// status/statuscache.go
package status

import (
	"context"
	"sync"
	"time"
)

type cacheEntry struct {
	data      any
	expiresAt time.Time
}

type Cache struct {
	mu    sync.RWMutex
	store map[string]cacheEntry
}

func GetOrFetch[T any](
	c *Cache,
	key string,
	ttl time.Duration,
	fetch func(ctx context.Context) (T, error),
	ctx context.Context,
) (data T, isStale bool, err error) {
	c.mu.RLock()
	entry, found := c.store[key]
	c.mu.RUnlock()

	now := time.Now()
	if found && now.Before(entry.expiresAt) {
		return entry.data.(T), false, nil
	}

	// Stale-While-Revalidate Strategy
	if found {
		// Zwróć stare dane, ale natychmiast wyzwól asynchroniczny fetch w tle
		go func() {
			newData, err := fetch(context.Background())
			if err == nil {
				c.mu.Lock()
				c.store[key] = cacheEntry{data: newData, expiresAt: time.Now().Add(ttl)}
				c.mu.Unlock()
			}
		}()
		return entry.data.(T), true, nil
	}

	// Blokujący fetch w przypadku braku jakichkolwiek danych (Cold Start)
	newData, err := fetch(ctx)
	if err != nil {
		var zero T
		return zero, false, err
	}

	c.mu.Lock()
	c.store[key] = cacheEntry{data: newData, expiresAt: now.Add(ttl)}
	c.mu.Unlock()
	return newData, false, nil
}
```

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

Webox realizuje funkcję **Configuration Drift Detection** w celu wykrycia zmian dokonanych bezpośrednio przez panel WWW Devil poza kontrolą TUI:

* **Stale Domain Detection:** Podczas każdego cyklu auto-refresh, Webox wykonuje w tle polecenie `ListSubdomains` na aktywnym profilu. Jeśli zmapowany lokalnie projekt nie występuje na zwróconej liście serwerowej, w modelu TUI projekt otrzymuje stan `STALE` i timestamp wykrycia driftu.
* **Env File Drift:** Zakładka `Env Diff` podczas aktywowania pobiera asynchronicznie zawartość pliku `.env` z serwera produkcyjnego (przez SFTP) i porównuje hasze MD5 linijka po linijce z lokalnym plikiem roboczym. Różnice są natychmiast przekazywane do renderera Lipgloss.

### 11.1 Architektura Dwukierunkowego Env Merger (TUI Env Merger Engine)

Aby zapewnić pełną spójność i transakcyjność operacji na zmiennych środowiskowych, Webox 2.0 implementuje silnik synchronizacji różnicowej w pakiecie `env/merger.go`.

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

---

## 14. Dystrybucja i mechanizm sprawdzania wersji

* **Async Version Checker:** 2 sekundy po uruchomieniu aplikacji, system w asynchronicznej goroutynie odpytuje endpoint GitHub API (`GET /repos/webox/releases/latest`).
* **UX Alert Integration:** Jeśli wersja serwerowa jest wyższa niż zakodowana w aplikacji stała `Version`, model TUI otrzymuje flagę `updateAvailable = true` oraz wartość nowej wersji. Nagłówek Dashboardu wyświetla wtedy nieinwazyjny, błyszczący badge `Update: vX.Y.Z available`.

---

## 15. Diagnostyka (Doctor & Redacted Logger)

* **Redacted Local Logger:** Przy włączonym trybie debugowania (`webox --debug`), logi diagnostyczne są zapisywane w pliku `webox.log`. Wszystkie dane wrażliwe (hasła, tokeny, ścieżki prywatne) są filtrowane przez silnik regex przed zapisem na dysk.
* **Doctor System Diagnostics:** Komenda `webox doctor` dokonuje bezinwazyjnego audytu lokalnego środowiska deweloperskiego, sprawdzając spójność bazy kluczy Keyring, uprawnienia do zapisu konfiguracji oraz statusy pingów TCP do serwerów hostingowych.

---

## 16. Concurrency & Visual Motion Engine (Pulsacje i Spinnery)

Wprowadzamy zaawansowany silnik wizualny, który wykorzystuje asynchroniczne pętle czasowe (Goroutines & Channels) w Bubble Tea do animowania klatek interfejsu.

### 16.1 Sinusoidalne Pulsowanie Ramek (Border Pulsing)

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

Pakiet `sound` w Webox 2.0 zapewnia bezinwazyjną generację retro-akustyki w czasie rzeczywistym. Architektura ta jest zaprojektowana tak, aby operacje audio były całkowicie asynchroniczne i nie powodowały opóźnień (lagów) w głównym wątku renderowania TUI Bubble Tea.

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
