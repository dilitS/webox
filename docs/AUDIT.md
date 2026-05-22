# Webox — Pre-implementation Audit

> Status: Draft · Ostatnia aktualizacja: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [PRD.md](./PRD.md), [DESIGN.md](./DESIGN.md), [UX.md](./UX.md), [SECURITY.md](./SECURITY.md), [TESTING.md](./TESTING.md), [ROADMAP.md](./ROADMAP.md), [CHANGES.md](../CHANGES.md).

## TL;DR

Pełny audyt dokumentacji przed wejściem w implementację. Cel: zidentyfikować luki logiczne, sprzeczności między dokumentami, błędy techniczne w przykładach kodu i scope creep, zanim którykolwiek z nich zatruje bazę kodu. Audyt zawiera **39 znalezisk** podzielonych na cztery priorytety oraz 5 decyzji otwartych. Wszystkie znaleziska oznaczone `P0` muszą zostać poprawione przed rozpoczęciem implementacji `v0.1`. Audyt jest świadomie publikowany jako osobny plik, żeby każda korekta była atomowa i recenzowalna w PR.

## Spis treści

1. [Metodologia](#1-metodologia)
2. [Znaleziska P0 — krytyczne, blokujące implementację](#2-znaleziska-p0--krytyczne-blokujące-implementację)
3. [Znaleziska P1 — wysokie, niezgodne z PRD/ROADMAP](#3-znaleziska-p1--wysokie-niezgodne-z-prdroadmap)
4. [Znaleziska P2 — średnie, błędy techniczne i nieaktualne biblioteki](#4-znaleziska-p2--średnie-błędy-techniczne-i-nieaktualne-biblioteki)
5. [Znaleziska P3 — niskie, redakcyjne i edge case'y](#5-znaleziska-p3--niskie-redakcyjne-i-edge-casey)
6. [Decyzje otwarte do potwierdzenia](#6-decyzje-otwarte-do-potwierdzenia)
7. [Plan korekt](#7-plan-korekt)

---

## 1. Metodologia

Audyt obejmuje:

- **15 dokumentów** w `docs/` + `README.md` w root + `CHANGES.md` + `CODE_OF_CONDUCT.md`.
- **Wszystkie 6 ADR-ów** (0001–0006).
- **4 docs providerów** (smallhost referencyjny, cpanel/directadmin/cyberpanel jako research).
- **Cross-reference check** — każdy link w docs sprawdzony pod kątem istnienia anchora.
- **Weryfikacja zewnętrzna przez Context7** dla bibliotek: `bubbletea`, `lipgloss`, `bubbles`, `go-keyring`, `golangci-lint v2`, `goreleaser`, `bubbletea-app-template`.

Audyt **nie** obejmuje:

- Archiwalnego monolitu `archive/PRD_v0_monolith.md` (zachowany świadomie, nie zmieniamy go).
- Kodu (jeszcze nie istnieje).
- Realnych testów na koncie small.pl (zaplanowane w `providers/smallhost.md §7`).

---

## 2. Znaleziska P0 — krytyczne, blokujące implementację

### A1. `SECURITY.md §4.2` — błędny algorytm detekcji keyringa

**Lokalizacja:** [`docs/SECURITY.md §4.2`](./SECURITY.md#42-fallback-dla-środowisk-headless), kroki 1–2.

**Problem:** detekcja używa `keyring.Get("webox-sentinel", "test")` i interpretuje **dowolny** błąd jako brak keyringa. Tymczasem `go-keyring` na pierwszym uruchomieniu zwraca `keyring.ErrNotFound` (brak takiego sekretu) **przy działającym keyringu** — algorytm uznałby normalny system za headless i przeszedł na fallback.

**Źródło:** dokumentacja `go-keyring` (Context7) — pakiet eksportuje sentinel errors: `ErrNotFound`, `ErrSetDataTooBig`, `ErrUnsupportedPlatform`. Tylko `ErrUnsupportedPlatform` lub błąd niezdefiniowany jako `ErrNotFound` powinien skutkować fallbackiem.

**Korekta:** detekcja **probe write + read + delete** sentinela, gdzie:

- `ErrUnsupportedPlatform` → fallback,
- `errors.Is(err, keyring.ErrNotFound)` → keyring **działa**, sekret po prostu jeszcze nie istnieje,
- każdy inny błąd (dbus error string sygnalizowany jako generic) → log warning + fallback.

### A2. `DESIGN.md §3 + §4` — niezgodność sygnatury `Factory` z resztą dokumentacji

**Lokalizacja:** [`docs/DESIGN.md §3`](./DESIGN.md#3-provider-pattern-kontrakty-v2), [`§4`](./DESIGN.md#4-rejestr-providerów) vs [`docs/CONTRIBUTING.md §3.2`](./CONTRIBUTING.md#32-krok-2--implementacja-interfejsu) vs [`docs/providers/smallhost.md §2.2`](./providers/smallhost.md#22-sygnatury-referencja).

**Problem:** trzy różne podpisy fabryki:

| Plik | Sygnatura |
|---|---|
| `DESIGN.md §4` | `Factory func(properties map[string]string) (HostingProvider, error)` |
| `CONTRIBUTING.md §3.2` | `func(cfg providers.ProviderConfig) (providers.HostingProvider, error)` |
| `providers/smallhost.md §2.2` | `func New(cfg providers.ProviderConfig) (providers.HostingProvider, error)` |

**Korekta:** jedna definicja `ProviderConfig` w DESIGN.md jako struktura zawierająca `Alias`, `Type`, `Host`, `User`, `Properties map[string]string` — wszystkie pozostałe pliki referują tylko do niej.

### A3. `DESIGN.md §8` — żywy kod generyczny mimo deklaracji w `CHANGES.md §1 6.1` o jego usunięciu

**Lokalizacja:** [`docs/DESIGN.md §8`](./DESIGN.md#8-trójpoziomowy-status-cache-stale-while-revalidate), wiersze 163–224 (blok kodu Go).

**Problem:** `CHANGES.md §1 6.1` deklaruje: *"Usunięto kod, opisano wzorzec funkcyjny jako funkcja pakietowa + tabela TTL i invalidacji eventowej"*. W rzeczywistości `DESIGN.md §8` wciąż zawiera **pełny blok 60+ linii kodu Go** z generykami i ręczną mutex obsługą. To wprowadza w błąd osobę zaczynającą implementację.

**Korekta:** zastąpić blok kodu **kontraktem funkcji + tabelą TTL i polityką invalidacji eventowej** (zgodnie z [`ADR-0005 §Parametry cache`](./adr/0005-cache-statusow-projektow.md#parametry-cache)).

### A4. `DESIGN.md §3` — typo `CPINalled` w polu struktury

**Lokalizacja:** [`docs/DESIGN.md §3`](./DESIGN.md#3-provider-pattern-kontrakty-v2), pole `ProviderStatus.CPINalled bool`.

**Problem:** zamiast `CLIInstalled` jest `CPINalled` — błąd literowy w eksportowanym polu trafiłby do publicznego API.

**Korekta:** `CPINalled` → `CLIInstalled`.

### A5. Sieć martwych kotwic w cross-references

**Problem:** Wiele dokumentów referuje do anchorów, które fizycznie nie istnieją w docelowym pliku. Przykłady:

| Plik referujący | Anchor | Czy istnieje? |
|---|---|---|
| `PRD.md §6 F23` | `DESIGN.md#11-konflikty-z-ręcznymi-zmianami-w-panelu` | Nie — `DESIGN.md §11` to "Detekcja rozbieżności konfiguracji". |
| `PRD.md §12.4` | `DESIGN.md#15-telemetria-i-logi-diagnostyczne` | Nie — `DESIGN.md §15` to "Diagnostyka (Doctor & Redacted Logger)". |
| `SECURITY.md §6.1 link` | `DESIGN.md#7-sekrety` | Nie — `DESIGN.md §7` to "Zarządzanie sekretami (Keyring integration)". |
| `CONTRIBUTING.md §2.5` | `DESIGN.md#23-zasady-przepływu-danych-mvu` | Nie — `DESIGN.md` nie ma §2.3. |
| `TESTING.md §5.1` | `DESIGN.md#23-zasady-przepływu-danych-mvu` | Nie — j.w. |
| `providers/smallhost.md §6` | `DESIGN.md#135-szablon-workflow-parametryzowany` | Nie — `DESIGN.md §13` nie ma §13.5. |
| `SECURITY.md §9.4` | `DESIGN.md#153-crash-reports` | Nie — `DESIGN.md §15` jest flat, bez §15.3. |
| `CONTRIBUTING.md §3.2` | `DESIGN.md#32-kontrakt--hostingprovider` | Nie — `DESIGN.md §3` jest flat. |

**Korekta:** każdy anchor musi zostać albo dodany do pliku docelowego, albo link skierowany do istniejącej sekcji.

### A6. Scope creep w `DESIGN.md` i `UX.md` poza zakresem MVP z `ROADMAP.md §3.3`

**Lokalizacja:** [`docs/DESIGN.md`](./DESIGN.md) i [`docs/UX.md`](./UX.md) — wiele sekcji.

**Problem:** `ROADMAP.md §3.3` wprost deklaruje, że poniższych rzeczy **nie ma w MVP**, ale `DESIGN.md` i `UX.md` opisują je jako podstawowe ficzery:

| Element | Status w PRD/ROADMAP | Status w DESIGN/UX |
|---|---|---|
| `Live log stream` | `F14 P1`, [`ROADMAP §3.3`](./ROADMAP.md#33-czego-nie-ma-w-mvp): *"Brak live log stream"* | [`UX.md §4.3 Tab [4]`](./UX.md#karta-4--live-log-stream) — pokazane jako pełna zakładka MVP |
| `/env`, `Env Diff`, `Env Merger` | [`ROADMAP §3.3`](./ROADMAP.md#33-czego-nie-ma-w-mvp): *"Brak /db, /env, /storage, /domain"* | [`UX.md §4.3 Tab [2]`](./UX.md#karta-2--env-diff-dwukierunkowy-podgląd-różnic-zmiennych), [`§9.3`](./UX.md#93-interaktywny-dwukierunkowy-tui-env-merger), [`DESIGN.md §11.1`](./DESIGN.md#111-architektura-dwukierunkowego-env-merger-tui-env-merger-engine) — pełen interaktywny merger |
| `/db` (Database tab) | j.w. | [`UX.md §4.3 Tab [3]`](./UX.md#karta-3--database-management) — pełna zakładka |
| `Live Service Topology Map` | brak w PRD §6 jako ficzer | [`DESIGN.md §18`](./DESIGN.md#18-silnik-dynamicznej-topologii-live-service-topology-map-engine), [`UX.md §3.4`](./UX.md#34-wizualny-graf-topologii-usług-live-service-topology-map) — pełen silnik |
| `Sound Engine` (PCM audio, retro chimes) | brak w PRD §6 jako ficzer | [`DESIGN.md §17`](./DESIGN.md#17-architektura-silnika-dźwiękowego-w-go-package-sound), [`UX.md §12`](./UX.md#12-tui-soundscapes--akustyka-operacyjna) — pełen pakiet |
| `Sinusoidal Border Pulsing` | brak w PRD §6 | [`DESIGN.md §16.1`](./DESIGN.md#161-sinusoidalne-pulsowanie-ramek-border-pulsing) — animacja sinusoidalna |
| `Bento Ultra ≥120×35` jako default | [`PRD §10.3`](./PRD.md#103-terminal) deklaruje *zalecane* 100×30, *minimalne* 88×28 | [`UX.md §4.2`](./UX.md#42-dashboard-20--bento-box-grid-system-12035) — Bento Ultra **przy ≥120**, czyli powyżej "zalecanego" PRD |
| `Fast-chord bindings` (`g r`, `g d`, `g e`, `g l`) | brak w PRD §6 jako P0 | [`UX.md §6.1`](./UX.md#61-szybkie-skróty-akordowe-fast-chord-bindings) jako element MVP |
| `DEGRADED` status badge | brak w PRD §6 jako status | [`UX.md §3.1`](./UX.md#31-badges-statusu-premium) jako 5. status |

**Korekta:** podzielić `DESIGN.md` i `UX.md` na sekcje **`v0.1 (MVP)`** i **`Stretch / v0.2+`**. Wszystkie ficzery niewymienione w PRD §6 jako P0 trafiają do tej drugiej grupy, z wyraźnym banerem `> ⚠ Stretch goal — wykraczające poza MVP. Implementacja po dostarczeniu v0.1.`. Sekcje przeniesione **nie są usuwane** (pomysł architektoniczny zostaje udokumentowany), ale są jasno odsunięte od ścieżki krytycznej.

### A7. `DESIGN.md` — brakujące podrozdziały referowane z innych plików

**Lokalizacja:** [`docs/DESIGN.md`](./DESIGN.md).

**Problem:** następujące sekcje są referowane z innych plików, ale **nie istnieją**:

- `§2.1 Layout repo`
- `§2.2 Przepływ danych`
- `§2.3 Zasady przepływu danych (MVU)`
- `§3.2 Kontrakt — HostingProvider`
- `§3.3 Properties bag`
- `§3.4 Defensywne parsowanie outputu`
- `§5.3 Connection pool`
- `§6.4 Migracje schematu`
- `§13.5 Szablon workflow parametryzowany`
- `§15.3 Crash reports`

**Korekta:** dodać brakujące podrozdziały. Część jest semantycznie obecna pod inną nazwą — wystarczy zsynchronizować nagłówki (i tym samym anchory).

### A8. `DESIGN.md §6` — race condition w PID check lockfile

**Lokalizacja:** [`docs/DESIGN.md §6`](./DESIGN.md#6-model-danych-i-atomowość-zapisu-configjson), krok 1.

**Problem:** *"Jeśli plik istnieje i PID procesu wewnątrz lockfile jest aktywny, zapis zostaje wstrzymany"* — PID jako mechanizm rozpoznania żywego procesu jest niewystarczający na POSIX, bo PID może być pownie wykorzystany przez **inny proces** po crash'u (race window).

**Korekta:** użyć `flock(2)` (Linux/macOS) lub `os.O_EXCL | os.O_CREATE` z lockfilem zawierającym dodatkowo `start_time` procesu. Na Windows — `LockFileEx`. Implementacja może użyć `github.com/gofrs/flock`.

---

## 3. Znaleziska P1 — wysokie, niezgodne z PRD/ROADMAP

### B1. `TESTING.md §5.3` — testy MVP zawierają zakładki spoza zakresu v0.1

**Lokalizacja:** [`docs/TESTING.md §5.3`](./TESTING.md#53-co-testujemy-w-tui), wiersz *"Reveal `.env` (key `v` + confirm)"*.

**Problem:** test referuje do funkcji `/env` reveal — która **nie jest częścią MVP** (`ROADMAP §3.3` wyklucza `/env`).

**Korekta:** wykreślić test z listy MVP. Pozostawić w v0.2.

### B2. `TESTING.md §2.1` — odwołanie do nieistniejącego anchora `DESIGN §12 Maszyna stanów`

**Lokalizacja:** [`docs/TESTING.md §2.1`](./TESTING.md#21-unit--60--wszystkich-testów).

**Problem:** `DESIGN.md §12` jest zatytułowane *"Maszyna stanów TUI (Tabbed Cockpit Spec)"* — opisuje zakładki, a nie pełnej maszyny stanów wizard'a. Brakuje listy stanów wizard.

**Korekta:** dodać do `DESIGN.md §12` osobny podrozdział `§12.1 Lista stanów top-level` z pełną listą `state*`.

### B3. `CONTRIBUTING.md §2.1` + `TESTING.md §6.1` — przestarzałe nazwy linterów `golangci-lint`

**Lokalizacja:** [`docs/CONTRIBUTING.md §2.1`](./CONTRIBUTING.md#21-linter), [`docs/TESTING.md §6.1`](./TESTING.md#61-workflow-ciyml).

**Problem:** lista linterów używa starych nazw z `golangci-lint v1`. W v2 mamy zmiany: `gas → gosec`, `goerr113 → err113`, `gomnd → mnd`, `logrlint → loggercheck`, `megacheck → staticcheck`. Wymagana też deklaracja `version: "2"` w konfiguracji.

**Korekta:** zaktualizować pliki do v2.

### B4. `CONTRIBUTING.md §1.1` — `golangci-lint 1.60+` nieaktualne, projekt celuje w Go 1.24

**Korekta:** `golangci-lint 2.x+`, dodać dopisek o `CGO_ENABLED=0` dla buildu release.

### B5. `UX.md §4.2` — Bento przy ≥120×35, ale `PRD §10.3` deklaruje *zalecane* 100×30

**Problem:** sprzeczność: jeśli "zalecane" 100×30 daje **Standard Cockpit** (a nie Bento), to większość power-userów nigdy nie zobaczy Bento — co czyni Bento de facto post-MVP.

**Korekta:** Bento (≥120×35) jako **stretch goal**, MVP target to Standard Cockpit (100×30).

### B6. `PRD.md §8 K6` — kryterium zależne od community

**Korekta:** dopisek *"Niespełnienie K6 w 6 miesięcy nie blokuje GA, ale przesuwa GA o kolejne 6 miesięcy lub do momentu spełnienia kryterium."*

### B7. `SECURITY.md §6.1` — `fine-grained PAT` z `Administration: Read and write`

**Problem:** `Administration: Read and write` na `org` poziomie pozwala na **tworzenie/usuwanie repo całej organizacji** — to przesadne uprawnienie.

**Korekta:** rozdzielić scenariusze: *(a)* webox tworzy nowe repo automatycznie → wymaga `repo:create` (rare scope, bardzo ostrożnie), *(b)* webox konfiguruje istniejące repo → wymaga tylko `Contents`, `Workflows`, `Actions`, `Secrets`, `Metadata`. Tryb (a) opt-in, tryb (b) default.

### B8. Brak `CHANGELOG.md` na poziomie root

**Lokalizacja:** root repo, [`docs/CONTRIBUTING.md §2.4`](./CONTRIBUTING.md#24-struktura-pr) — checkbox `- [ ] CHANGELOG entry added.`

**Korekta:** utworzyć szablon `CHANGELOG.md` w root z sekcją `Unreleased` + format [Keep a Changelog](https://keepachangelog.com/).

---

## 4. Znaleziska P2 — średnie, błędy techniczne i nieaktualne biblioteki

### C1. `DESIGN.md §17` — `/dev/dsp` jako interfejs audio Linuksa jest passé

**Problem:** `/dev/dsp` to OSS, zastąpione przez ALSA → PulseAudio → PipeWire już dekadę temu. Na typowym Ubuntu/Fedora 2026 `/dev/dsp` nie istnieje. Sound engine to scope creep — usuwamy zgodnie z A6.

### C2. `TESTING.md §5.1` — `teatest` w `x/exp` — niestabilny path

**Korekta:** dodać note że `teatest` jest experimental i może zmienić ścieżkę importu — wymagany monitor zmian w Charm.

### C3. `DESIGN.md §16.2` — `GetAverageRTT()` przy zerowej liczbie próbek

**Korekta:** dopisać: *"Implementacja `GetAverageRTT()` zwraca `0` jeśli brak próbek; `GetSpinnerTick` traktuje `rtt == 0` jako `Standard` (`100ms`) do czasu pierwszej próbki."*

### C4. `SECURITY.md §4.3` — `zerocopy.Wipe` jako wymyślona biblioteka

**Problem:** Go nie gwarantuje wymazania pamięci po wartości — GC kopiuje. Pełny zero-knowledge wymaga `mlock`'owanego bufora.

**Korekta:** zamiast wymyślonego `zerocopy.Wipe` powołać się na `memguard.LockedBuffer`.

### C5. `DESIGN.md §9` — `Exponential Backoff` z 3 krokami: 3s, 6s, 12s

**Korekta:** doprecyzować: `dial timeout 15s` per próba, `3 próby reconnect`, między nimi `3s, 6s, 12s` jitter (random fraction 50%–100% backoffu). Worst case: `15 + 6 + 15 + 12 + 15 = 63s`.

### C6. `providers/smallhost.md §6` — workflow `deploy.yml` używa `rsync` z `--delete`

**Problem:** `--delete` usuwa pliki na zdalnym, których brak w lokalnym `dist/`. To **bardzo niebezpieczne** dla `public/uploads/` (assety klientów).

**Korekta:** użyć **excludes**: `rsync -avz --delete --exclude='uploads/' --exclude='.env'`.

### C7. `DESIGN.md §13.5` (do utworzenia) — brak miejsca na szablon workflow

**Korekta:** dodać `DESIGN.md §13.5 Szablon workflow parametryzowany` — opis embedowania `.github/workflows/deploy.yml` jako `embed.FS` w binarce.

### C8. `SECURITY.md §5.5` — `ssh-rsa SHA-1` w odrzucone

**Problem:** Niektóre serwery wciąż negocjują tylko `ssh-rsa` (z SHA-1) — webox **odrzuci** połączenie.

**Korekta:** dodać **konfigurowalność** w `properties.ssh_algorithms_legacy_compat` z domyślnym `false`.

### C9. Brak realnego pliku `.go` z `package` declaration w żadnym `providers/`

**Korekta:** brak. Pierwsza implementacja PR doda `doc.go` i pierwsze testy.

### C10. `MIGRATION_NOTES.md` — anchor `archive/PRD_v0_monolith.md` używa `../` ale plik jest w root

**Korekta:** brak. False alarm.

---

## 5. Znaleziska P3 — niskie, redakcyjne i edge case'y

### D1. `README.md` — brak sekcji `Installation`

**Korekta:** dodać sekcję `Installation (pre-MVP)` z informacją *"Project is in pre-MVP / docs-first phase. No binary is published yet. Track progress in [ROADMAP.md](./docs/ROADMAP.md)."*

### D2. `docs/README.md` — wzmianka *"Status: Draft"* mimo że dokument jest miejscem entry point

**Korekta:** `Status: Stable navigation guide` lub po prostu zostawić bez `Status`.

### D3. `PRD.md §10.3` — pomysł 60×20 jako "blockout" mimo że minimum to 88×28

**Korekta:** PRD §10.3 zsynchronizować z `UX §5` (Single-Pane fallback od 70).

### D4. `ROADMAP.md §3.5` — estymata `8-12 tygodni jednoosobowej` jest aspiracyjna

**Korekta:** zaktualizować na `12-20 tygodni jednoosobowej (P50 wartość: 16 tygodni)`.

### D5. `CONTRIBUTING.md §1.1` — wymóg `go 1.24` ale `bubbletea-app-template` używa `go 1.22` jako baseline

**Korekta:** uściślić, że projekt celuje `go 1.24+`, a `.golangci.yml` ustawia `run.go: '1.24'`.

### D6. `UX.md §12 Sound` — `Ctrl+S` jako "szybkie wyciszenie", ale w wielu terminalach `Ctrl+S` to **XON/XOFF flow control**

**Korekta:** zmienić na `Ctrl+M` lub `Alt+M`. Sound jest stretch.

### D7. `docs/SECURITY.md §10.4` — `Web root per provider` brakuje `cyberpanel`

**Korekta:** dopisać `cyberpanel: ~/<DOMAIN>/public_html/` (zgodnie z `providers/cyberpanel.md §6`).

### D8. Wszystkie ADR — brakuje pola `Reviewers`

**Korekta:** dodać `Reviewers: @maintainer (sole author, awaiting external review at v0.1)` w header'ze każdego ADR.

### D9. Brak `.editorconfig` na poziomie root

**Korekta:** dodać `.editorconfig`.

### D10. Brak `Makefile`

**Korekta:** dodać minimalny `Makefile`.

### D11. `.gitignore` jest minimalny

**Korekta:** rozszerzyć `.gitignore` o standardowe Go entries.

### D12. Polski tekst w `DESIGN.md §11.1` używa "are parsed" zamiast "są parsowane"

**Korekta:** zmienić na "są parsowane".

### D13. `UX.md §4.4` — graf zależności wizard kolejność niezgodna z `DESIGN.md §10`

**Korekta:** zsynchronizować graf w UX z DAG w DESIGN.

---

## 6. Decyzje otwarte do potwierdzenia

Audyt nie rozstrzyga poniższych, bo są to świadome trade-off'y.

### E1. Czy sound engine i Bento Ultra wracają w `v0.2` czy ich nie ma w ogóle?

**Sugestia audytu:** Bento `≥120×35` jako `v0.2 stretch`. Sound engine — **nie** w `v0.2`, do osobnego RFC z konkretną wartością user.

### E2. Czy dla `v0.1` przyjmujemy konwencję commit messages z `gitmoji` czy bez?

**Sugestia audytu:** Conventional Commits **bez** gitmoji.

### E3. Czy `webox.log` rotuje przez built-in czy przez `lumberjack`?

**Sugestia audytu:** `gopkg.in/natefinch/lumberjack.v2`.

### E4. Czy testy integracyjne z realnym `small.pl` wymagają sandboxa per maintainer, czy współdzielonego konta `webox-test@small.pl`?

**Sugestia audytu:** współdzielone konto `webox-test@small.pl`.

### E5. Czy `webox doctor --json` wprowadzamy w `v0.1` czy CLI flags wszystkie w `v0.3+`?

**Sugestia audytu:** `webox doctor --json` w `v0.1`. Reszta CLI w `v0.3+`.

---

## 7. Plan korekt

Plan wyboru zmian w kolejności PR-ów:

### PR-1 — Krytyczne fixy P0 (A1–A8)

1. `SECURITY.md §4.2` — algorytm detekcji keyringa.
2. `DESIGN.md §3 + §4` — unifikacja `ProviderConfig` + `Factory`.
3. `DESIGN.md §8` — usunięcie generycznego kodu, dodanie kontraktu funkcyjnego + tabeli TTL.
4. `DESIGN.md §3` — `CPINalled` → `CLIInstalled`.
5. Anchor fix sweep (A5).
6. Scope creep purge — przeniesienie sound, topology, env merger, live log do `Stretch / v0.2+` (A6).
7. Brakujące podrozdziały (A7) — placeholdery lub pełne sekcje.
8. `DESIGN.md §6` — `flock(2)` zamiast PID check (A8).

### PR-2 — Niezgodności P1 (B1–B8)

9. `TESTING.md §5.3` — wykreślenie `/env` testów z MVP (B1).
10. `DESIGN.md §12.1` — lista stanów top-level (B2).
11. `CONTRIBUTING.md §2.1` i `TESTING.md §6.1` — `golangci-lint v2` (B3+B4).
12. `UX.md §4.2` — Bento jako stretch (B5).
13. `PRD.md §8 K6` — dopisek o niespełnieniu nie blokującym GA (B6).
14. `SECURITY.md §6.1` — dwa scenariusze tokena (B7).
15. `CHANGELOG.md` szablon (B8).

### PR-3 — Cleanup P2/P3

16. `DESIGN.md §17` — sound engine drift do v0.2 stretch (C1).
17. `SECURITY.md §4.3` — `memguard` zamiast wymyślonego `zerocopy` (C4).
18. `DESIGN.md §16.2` — domyślny `100ms` dla cold-start RTT (C3).
19. `providers/smallhost.md §6` — `rsync --exclude='uploads/'` (C6).
20. Reszta P2/P3 — patrz tabela.

### PR-4 — Środowisko deweloperskie

21. `.editorconfig` (D9).
22. `Makefile` (D10).
23. `.gitignore` extensions (D11).
24. `AGENTS.md` na poziomie root (wsparcie agentów AI).
25. `.cursor/rules/`, `.cursor/skills/`, `.cursor/hooks/` (jako narzędzie procesu wytwórczego).

Po wszystkich PR-ach: ten plik AUDIT.md przechodzi w status `Status: Resolved` lub jest archiwizowany do `docs/archive/audit-pre-mvp.md`.

---

## Statystyki

| Priorytet | Liczba znalezisk | Wpływ |
|---|---|---|
| P0 | 8 | Blokujące implementację — sprzeczne kontrakty, brak anchorów, scope creep, błędna detekcja keyringa. |
| P1 | 8 | Niezgodne z PRD/ROADMAP, zaktualizowane biblioteki. |
| P2 | 10 | Drobne błędy techniczne, edge case'y. |
| P3 | 13 | Redakcyjne, tła operacyjne. |
| **Razem** | **39** | |

Plus **5 decyzji otwartych** (E1–E5).

> Audit nie zamyka projektu — otwiera go do **kontrolowanej** implementacji. Każda korekta zostanie wprowadzona przez świadomy PR z linkiem do tej tabeli.
