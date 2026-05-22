# Webox — UX / Design System (Generacja 2026/2027)

> Status: Approved · Ostatnia aktualizacja: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [PRD.md](./PRD.md) (cele biznesowe), [DESIGN.md](./DESIGN.md) (architektura TUI i silnika), [adr/0006](./adr/0006-jezyk-interfejsu-en-domyslny.md) (język interfejsu).

---

## TL;DR

UX/UI Webox wkracza w generację **2026/2027** jako zintegrowany **Terminal Cockpit klasy premium**. Odrzucamy proste, dwukolumnowe layouty na rzecz elastycznego systemu **Bento-Box Grid** (optymalizowanego dla rozdzielczości `≥ 120×35`), dynamicznych gradientów blokowych (`█▓▒░`) imitujących cieniowanie przestrzenne, zunifikowanego standardu **Nerd Font** z pełnym fallbackiem, oraz beztarciowej nawigacji za pomocą kart szczegółów (Tabs). Tradycyjny, liniowy kreator zastępujemy **Self-Healing Wizardem** prezentującym wizualny graf zależności z opcją chirurgicznego wznawiania i korekcji błędów na żywo.

---

## Spis treści

1. [Cel dokumentu](#1-cel-dokumentu)
2. [Design system 2.0](#2-design-system-20)
   * 2.1 [Paleta kolorów (OKLCH & HSL Precision)](#21-paleta-kolorów-oklch--hsl-precision)
   * 2.2 [System warstw i głębi (Dynamic Layering)](#22-system-warstw-i-głębi-dynamic-layering)
   * 2.3 [Integracja Nerd Font vs Fallback Unicode](#23-integracja-nerd-font-vs-fallback-unicode)
   * 2.4 [Identyfikacja Wizualna i Branding (Logo Główne)](#24-identyfikacja-wizualna-i-branding-logo-główne)
3. [Komponenty wizualne 2.0](#3-komponenty-wizualne-20)
   * 3.1 [Badges Statusu Premium](#31-badges-statusu-premium)
   * 3.2 [Dynamiczne Paski Postępu](#32-dynamiczne-paski-postępu-gradients-progress-bars)
   * 3.3 [Spinner Adaptacyjny](#33-spinner-adaptacyjny-morphing--latency-aware)
   * 3.4 [Wizualny Graf Topologii Usług (Live Service Topology Map)](#34-wizualny-graf-topologii-usług-live-service-topology-map)
4. [Layouty ekranów 2.0](#4-layouty-ekranów-20)
5. [Wymagania terminala i progi elastyczności](#5-wymagania-terminala-i-progi-elastyczności)
6. [System nawigacji i Key bindings (UX 2.0)](#6-system-nawigacji-i-key-bindings-ux-20)
7. [Confirm dialogs i tryb Expert](#7-confirm-dialogs-i-tryb-expert)
8. [Command Palette 2.0](#8-command-palette-20)
9. [Maskowanie sekretów i wizualny Diff](#9-maskowanie-sekretów-i-wizualny-diff)
   * 9.1 [Zasada absolutnego bezpieczeństwa](#9-maskowanie-sekretów-i-wizualny-diff)
   * 9.2 [Wizualny Env Diff](#9-maskowanie-sekretów-i-wizualny-diff)
   * 9.3 [Interaktywny Dwukierunkowy TUI Env Merger](#93-interaktywny-dwukierunkowy-tui-env-merger)
10. [Internacjonalizacja i lokalizacja](#10-internacjonalizacja-i-lokalizacja)
11. [Flowy użytkownika premium](#11-flowy-użytkownika-premium)
12. [TUI Soundscapes & Akustyka Operacyjna](#12-tui-soundscapes--akustyka-operacyjna)

---

## 1. Cel dokumentu

Ten dokument definiuje kompletną specyfikację wrażeń użytkownika (UX) oraz tożsamości wizualnej (UI) dla systemu Webox. Wszystkie makiety ASCII są zoptymalizowane pod kątem czytelności w nowoczesnych emulatorach terminala i służą jako bezpośredni wzorzec dla implementacji w bibliotece Lipgloss.

---

## 2. Design system 2.0

### 2.1 Paleta kolorów (OKLCH & HSL Precision)

Wersja 2.0 wprowadza paletę kolorów opartą na przestrzeni OKLCH, co gwarantuje stałą postrzegalną jasność (perceptual luminance) niezależnie od motywu. Kolory te są mapowane w Lipgloss jako dynamiczne tokeny adaptujące się do tła terminala.

| Rola | Dark Theme (Hex) | Light Theme (Hex) | OKLCH / HSL (Dark) | Zastosowanie |
|---|---|---|---|---|
| **Primary (Brand)** | `#7D56F4` | `#5B3FB5` | `oklch(60% 0.24 280)` | Aktywna selekcja, ramki Bento, nagłówki, spinner. |
| **Success** | `#04B575` | `#02855A` | `oklch(68% 0.21 160)` | Status ONLINE, ikona `✓`, 100% progress. |
| **Warning** | `#FFB800` | `#B27F00` | `oklch(78% 0.18 85)` | Status BUILDING, `⏳`, operacje w toku. |
| **Error** | `#FF4444` | `#CC2A2A` | `oklch(62% 0.22 25)` | Status OFFLINE, krytyczne błędy, `✗`. |
| **Degraded** | `#D846EF` | `#9D17BF` | `oklch(60% 0.25 320)` | Stan DEGRADED, błędy konfiguracji, częściowy SSL. |
| **Muted** | `#4E5A85` | `#8C96B8` | `oklch(45% 0.08 260)` | Elementy nieaktywne, klawisze pomocy, etykiety. |
| **Surface Base** | `#1A1B26` | `#FFFFFF` | `oklch(18% 0.02 260)` | Podstawowe tło emulatora terminala. |
| **Surface Low** | `#13141F` | `#F2F4F8` | `oklch(14% 0.02 260)` | Tło nieaktywnych paneli bocznych. |
| **Surface High** | `#24273A` | `#EBEFF5` | `oklch(24% 0.03 260)` | Ramki aktywnego kafelka, nakładki modalne (Overlays). |
| **Text Bright** | `#F8F8F2` | `#1A1B26` | `oklch(95% 0.01 260)` | Główny, wyraźny tekst, wartości aktywne. |
| **Text Dim** | `#8C98C1` | `#5A658A` | `oklch(70% 0.05 260)` | Komentarze, wskazówki, ścieżki na serwerze. |

---

### 2.2 System warstw i głębi (Dynamic Layering)

Terminal nie musi być płaski. Webox 2.0 symuluje głębię przestrzenną (3D Depth) poprzez trzy techniki:

1. **Warstwy Tła (Surface Levels):** 
   * Nieaktywne kafelki Bento otrzymują obramowanie o kolorze `Muted` i tło `Surface Low`.
   * Aktywny kafelek Bento, na którym spoczywa fokus użytkownika, otrzymuje ramkę w kolorze `Primary` i jaśniejsze tło `Surface High`.
2. **Cieniowanie Gradientowe krawędzi (Drop-Shadow Simulation):**
   * Wykorzystujemy bloki o zmiennym stopniu krycia (`█▓▒░`) na dolnej i prawej krawędzi nakładek modalnych (Confirm Dialogs, Command Palette), tworząc efekt fizycznego cienia rzucanego na kafelki pod spodem.
3. **Borders High-Intensity:**
   * Modale oraz ekrany krytyczne używają podwójnych ramek (`║`, `═`), natomiast standardowe moduły Bento używają pojedynczych zaokrąglonych ramek (`╭`, `╮`, `╯`, `╰`).

---

### 2.3 Integracja Nerd Font vs Fallback Unicode

Webox wspiera nowoczesne czcionki programistyczne (np. *Fira Code Nerd Font*, *SF Mono*). W przypadku wykrycia środowiska bez obsługi Nerd Font (lub na życzenie użytkownika w `/settings`), system przełącza się automatycznie na elegancki standard Unicode/Emoji.

| Nazwa Ikony | Glif Nerd Font (Zalecany) | Glif Fallback (Unicode) | Zastosowanie |
|---|:---:|:---:|---|
| **Database** | `󰆼` | `🐬` (MySQL) / `🐘` (Pg) | Sekcja baz danych |
| **SSL/Key** | `󰏚` | `🔐` | Statusy SSL, certyfikaty |
| **Git / Repo** | `󰊢` | `📦` | Połączenie z GitHubem |
| **Server Host** | `󰘳` | `🖥️` | Profil hostingowy |
| **Terminal/Shell**| `󰆍` | `▶` | Logi i procesy na żywo |
| **Check Success** | `󰄬` | `✓` | Powodzenie operacji |
| **Warning Pend** | `󰔚` | `⏳` | Operacje asynchroniczne |
| **Settings Gear** | `󰒓` | `⚙️` | Opcje konfiguracji |

---

### 2.4 Identyfikacja Wizualna i Branding (Logo Główne)

Wizerunek Webox w generacji **2026/2027** buduje silną tożsamość opartą o geometryczną nowoczesność i minimalizm. Logo główne TUI składa się z precyzyjnie dobranych, trójwierszowych bloków Unicode, które są zoptymalizowane pod kątem renderowania gradientowego w Lipgloss:

```text
  ▌ ▌   ▛▀▘   ▛▀▚   ▞▀▚   ▚ ▞
  ▌▄▌   ▙▄    ▛▀▚   ▌ ▐    █  
  ▙█▟   ▛▄▘   ▙▄▞   ▚▄▞   ▞ ▚
```

#### Zasady renderowania w Lipgloss:
1. **Dynamiczny Liniowy Gradient:** Logotyp jest rysowany z użyciem liniowego przejścia kolorów (Color Interpolation) w przestrzeni OKLCH od lewej do prawej krawędzi:
   * **Punkt startowy (lewy):** Głęboki fiolet `oklch(60% 0.24 280)`
   * **Punkt końcowy (prawy):** Neonowy błękit `oklch(65% 0.20 200)`
2. **Sub-branding Compact Badge:** Na ekranach o ograniczonej szerokości lub w nagłówkach mniejszych paneli stosuje się jednowierszową sygnaturę tekstową:
   ```text
   ❖ w e b o x  c o c k p i t  v 1.0  ❖
   ```
3. **Mikro-animacja powitalna (Splash Fade-In):** Przy starcie systemu, logo przechodzi 3-klatkowy asynchroniczny efekt rozjaśniania (Fade-In) od stłumionego `Muted` do pełnej luminancji OKLCH w czasie `240ms`.

---

## 3. Komponenty wizualne 2.0

### 3.1 Badges Statusu Premium

Badge w Webox 2.0 nie są zwykłymi napisami. Są renderowane jako bloki kolorów o wysokim kontraście (odwrócony kolor tekstu na kolorowym tle) z zaokrąglonymi krawędziami:

```text
  Dark Theme:
  ╭─────────╮  ╭──────────╮  ╭──────────╮  ╭─────────╮  ╭──────────╮
  │ ONLINE  │  │ BUILDING │  │ OFFLINE  │  │  STALE  │  │ DEGRADED │
  ╰─────────╯  ╰──────────╯  ╰──────────╯  ╰─────────╯  ╰──────────╯
   #04B575      #FFB800       #FF4444      #4E5A85      #D846EF
```

* **DEGRADED (Nowość):** Aplikacja odpowiada HTTP 200, ale jej certyfikat wygasa za mniej niż 3 dni lub zdetektowano błędy w pliku `.env` na produkcji.

---

### 3.2 Dynamiczne Paski Postępu (Gradients Progress Bars)

Paski postępu w kreatorze oraz przy wdrażaniu kodu używają dynamicznego gradientu blokowego na krawędzi postępu, imitując płynny ruch:

```text
  0%   [░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░]   0 %
  50%  [██████████████████▓▒░░░░░░░░░░░░░░░░░░░]  50 %
  100% [██████████████████████████████████████] 100 % (Success)
```

---

### 3.3 Spinner Adaptacyjny (Morphing & Latency-Aware)

Spinner Webox to nie tylko statyczny zestaw klatek. Jego prędkość (Tick Duration) oraz kształt zmieniają się dynamicznie na podstawie RTT (Round Trip Time) aktywnego połączenia SSH:

* **Szybkie Połączenie (RTT < 30ms):** Klasyczny `Dot` (`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`) z interwałem `50ms` (wrażenie błyskawicznego działania).
* **Wolne Łącze / Mobile (RTT > 150ms):** Zmiana spinnera na pulsujący blok `Pulse` (`░` → `▒` → `▓` → `█` → `▓` → `▒`) z interwałem `200ms`, dający wizualny spokój i informujący, że system czeka na odpowiedź sieci.

---

### 3.4 Wizualny Graf Topologii Usług (Live Service Topology Map)

W kafelku Bento dedykowanym dla topologii (dostępnym w trybie Bento Grid przy szerokości `≥ 120` znaków) Webox renderuje w czasie rzeczywistym schemat przepływu danych i relacji między elementami systemu. Pozwala to na natychmiastową diagnozę wąskich gardeł lub uszkodzonych węzłów.

```text
┌── Live Infrastructure Topology ────────────────────────────────────────────────────────────────────────┐
│                                                                                                          │
│  [ GitHub Repo ] ───(GHA Deploy)───▶ [ Production Server ]                                               │
│                                              │                                                           │
│                                              ├─▶ [ sui.biuromody... ] ───(Proxy)───▶ [ Local Port: 3000 ]  │
│                                              │                                                           │
│                                              └─▶ [ MySQL Tunnel ] ─────────────────▶ [ biuromody_sui ]     │
│                                                                                                          │
└──────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

#### Zasady Dynamiki Wizualnej Grafu:
* **Status ONLINE (Węzły sprawne):** Połączenia oraz ramki węzłów są rysowane cienką linią w kolorze `Primary`, a statusy posiadają mały, zielony glif `󰄬` lub `✓`.
* **Status BUILDING / DEPLOYING (W trakcie pracy):** Strzałka `───(GHA Deploy)───▶` zmienia się w animowaną linię przerywaną `═ ═ ═ ▶` z fioletowym pulsowaniem ramki.
* **Status ERROR / OFFLINE (Błąd krytyczny):** Ścieżka łącząca uszkodzony węzeł (np. brak połączenia z bazą) zmienia styl na grubą linię pulsującą na czerwono `⚡ ⚡ ⚡` z migającym badge `✗ DISCONNECTED`.

---

## 4. Layouty ekranów 2.0

### 4.1 Init Wizard — Pierwsze uruchomienie (64×22)

```text
 ╭────────────────── Webox — first run setup ───────────────────╮
 │                                                              │
 │  Step 1/2: System & Agent Environment                        │
 │                                                              │
 │  ┌── System Pre-requisites ───────────────────────────────┐  │
 │  │  󰊢 Git Engine:         v2.54.0                         │  │
 │  │  󰒓 GitHub CLI (gh):    v2.45.0 (Authenticated)         │  │
 │  │  󰏚 Keyring Backend:   macOS Secure Keychain (Active)  │  │
 │  └────────────────────────────────────────────────────────┘  │
 │                                                              │
 │  ┌── Default SSH Keypair ─────────────────────────────────┐  │
 │  │  Path: ~/.ssh/id_ed25519_webox                         │  │
 │  │  Fingerprint: SHA256:xGf9k+2Jb89... (Ed25519)          │  │
 │  │                                                        │  │
 │  │  [  Show Public Key  ]     ▶ [  Auto-inject to Host  ] │  │
 │  └────────────────────────────────────────────────────────┘  │
 │                                                              │
 │  [ Tab ] Navigate   [ Enter ] Confirm   [ Esc ] Quit         │
 ╰──────────────────────────────────────────────────────────────╯
```

---

### 4.2 Dashboard 2.0 — Bento-Box Grid System (`≥ 120×35`)

Gdy rozmiar okna terminala przekracza próg komfortu (`120×35`), Webox automatycznie transformuje interfejs w pełnoprawny, pięciomodułowy pulpit nawigacyjny. Aktywny panel (w tym przypadku *Projects*) jest podświetlony ramką o grubości 2 znaków (tutaj symbolicznie podwójną) i fioletowym kolorem.

```text
╭─────────────────────────────────────────────── Webox Cockpit v1.0 ─────────────────────────────────────────────────────────╮
│ [Profile: main · s1.small.pl · latency: 14ms]                                                                 [/] 20:17:50 │
│                                                                                                                            │
│  ╭─────────────────────────── Projects ──╮ ╭──────────────────────────────────────── sui.biuromody.smallhost.pl ───────────╮ │
│  │                                       │ │                                                                               │ │
│  │  ▶ sui.biuromody             🟢       │ │  ╭───────────╮                                                                │ │
│  │    makspomoc                 🟢       │ │  │  ONLINE   │  Node Version: v24.15.0                                        │ │
│  │    si                        🟡       │ │  ╰───────────╯  SSL Status:   󰏚 Valid (27 days remaining)                     │ │
│  │    legacy                    STALE    │ │                                                                               │ │
│  │                                       │ │  Domain Path:   /usr/home/biuromody/domains/sui.biuromody.smallhost.pl/       │ │
│  │  ───────────────────────────────────  │ │  GitHub Repo:   dilitS/mockupweb  (Branch: main)                              │ │
│  │  [n] New Project                      │ │  Last Deploy:   2h ago by dilitS ✓ (Commit: 3fdc34d)                          │ │
│  │  [i] Import Existing                  │ │                                                                               │ │
│  │                                       │ │  [r] Restart Application    [s] SSL Renew    [d] Manual Trigger Deploy        │ │
│  ╰───────────────────────────────────────╯ ╰───────────────────────────────────────────────────────────────────────────────╯ │
│  ╭──────────────────────── Quick Metrics ──╮ ╭─────────────────────────────── CI/CD Pipeline (GHA) ──────────────────────────╮ │
│  │                                       │ │                                                                               │ │
│  │  HTTP Health: 200 OK                  │ │  󰄬 Setup Build Workspace    ✓ 12s                                             │ │
│  │  Ping Latency: 42 ms                  │ │  󰄬 Production Build (Vite)   ✓ 45s                                             │ │
│  │  Active DBs:  🐬 Connected            │ │  ▶ Deploying SFTP Assets     ⏳ 14s (In Progress...)                           │ │
│  ╰───────────────────────────────────────╯ ╰───────────────────────────────────────────────────────────────────────────────╯ │
│  ╭────────────────────────────────────────────────── Live Micro-Logs ──────────────────────────────────────────────────────╮ │
│  │  [20:15:02] INFO: Production Express server running on port 3000                                                        │ │
│  │  [20:15:30] GET /api/v1/health_check - 200 OK (5ms)                                                                     │ │
│  │  [20:16:12] GET /assets/index-D8gH9s.js - 200 OK (2ms)                                                                  │ │
│  ╰─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯ │
│                                                                                                                            │
│  q:quit  ↑↓:navigate  →/Tab:cockpit focus  n:new  /:command  ?:help                                                        │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

---

### 4.3 Tabbed Cockpit (Wielomodułowe Szczegóły Projektu)

Gdy użytkownik wciśnie `→` lub `Tab` na projekcie, przechodzi do panelu szczegółów. Górna belka zamienia się w system zakładek. Nawigacja odbywa się za pomocą klawiszy `H`/`L` (lewo/prawo) lub cyfr `1`–`4`.

#### Karta [2] — Env Diff (Dwukierunkowy Podgląd Różnic Zmiennych)

Ta karta pozwala na graficzne porównanie lokalnego pliku `.env` z rzeczywistym plikiem `.env` wdrożonym na serwerze produkcyjnym, zapobiegając częstym błędom braku konfiguracji.

```text
 ┌─ sui.biuromody.smallhost.pl ── [1] Overview  ▶ [2] Env Diff  [3] Database  [4] Logs ──────────────────────────────────────┐
 │                                                                                                                           │
 │  Below is a comparison of your local and remote environment configurations. Missing values will trigger app failure.       │
 │                                                                                                                           │
 │  KEY               LOCAL FILE VALUE             SERVER PROD VALUE            STATUS                                       │
 │  ───────────────────────────────────────────────────────────────────────────────────────────────                          │
 │  PORT              3000                         3000                         󰄬 MATCH                                      │
 │  NODE_ENV          production                   production                   󰄬 MATCH                                      │
 │  DATABASE_URL      mysql://biuro:******         mysql://biuro:******         󰄬 MATCH                                      │
 │  API_KEY           sec_9876abc...               [MISSING PRODUCTION KEY]     ✗ ORPHANED ERROR                             │
 │  CACHE_TTL         600                          300                          ⚡ DRIFT (Server differs)                     │
 │                                                                                                                           │
 │  [s] Sync Server env with Local   [Ctrl+R] Force Reload   [v] Reveal selected secret   [e] Edit variable                  │
 └───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

#### Karta [3] — Database Management

```text
 ┌─ sui.biuromody.smallhost.pl ── [1] Overview  [2] Env Diff  ▶ [3] Database  [4] Logs ──────────────────────────────────────┐
 │                                                                                                                           │
 │  🐬 MySQL Relational Database (Active Engine on s1.small.pl)                                                              │
 │                                                                                                                           │
 │  ┌── Connection Parameters ─────────────────────────────────────────────────────────────────────────────┐  │
 │  │  Host Address:      127.0.0.1 (Internal Loopback via SSH Tunnel)                                    │  │
 │  │  Database Name:     biuromody_sui                                                                   │  │
 │  │  Database User:     biuromody_sui                                                                   │  │
 │  │  Linked Keyring:    webox-db-main-biuromody_sui (AES-256 Symmetrically Encrypted)                    │  │
 │  └───────────────────────────────────────────────────────────────────────────────────────────────────────┘  │
 │                                                                                                                           │
 │  ┌── Diagnostic metrics ─────────────────────────────────────────────────────────────────────────────────┐  │
 │  │  Latency:           0.8 ms (Local virtual link)                                                     │  │
 │  │  Tables count:      14 tables (Innodb Engine)                                                       │  │
 │  │  Size on disk:      4.2 MB                                                                          │  │
 │  └───────────────────────────────────────────────────────────────────────────────────────────────────────┘  │
 │                                                                                                                           │
 │  [t] Test Connection   [d] Export database dump (.sql)   [r] Reset Database Credentials                           │
 └───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

#### Karta [4] — Live Log Stream

```text
 ┌─ sui.biuromody.smallhost.pl ── [1] Overview  [2] Env Diff  [3] Database  ▶ [4] Logs ──────────────────────────────────────┐
 │  Active File: /usr/home/biuromody/domains/sui.biuromody.smallhost.pl/logs/error.log                     󰆍 Stream Mode     │
 │ ───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────── │
 │  2026-05-22 20:15:02 [info] Starting production Node/Express server on port 3000                                          │
 │  2026-05-22 20:15:02 [info] Establishing secure tunnel to MySQL...                                                        │
 │  2026-05-22 20:15:05 [debug] Successfully loaded 18 environment variables from .env                                       │
 │  2026-05-22 20:16:12 [warn] Database query latency warning: 'SELECT * FROM activities' took 118ms                         │
 │  2026-05-22 20:16:45 [error] GET /api/v1/invalid-route - 404 Route handler not found in router.js                         │
 │                                                                                                                           │
 │  ↑↓: Scroll buffer  [f] Toggle Auto-scroll (Tail -f: On)  [c] Clear local console buffer  [Esc] Back                       │
 └───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### 4.4 Self-Healing Wizard Step 5 (Dependency Graph)

Podczas tworzenia nowego projektu, zamiast nudnej, płaskiej listy kroków, Webox rysuje interaktywny **Graf Zależności**, który pokazuje rzeczywistą architekturę i kolejność uruchamiania zasobów.

```text
 ┌─ Step 5/5 — Project Scaffolding & Provisioning ─────────────────────────────────┐
 │                                                                                 │
 │  scaffold ──┐                                                                   │
 │             ├─▶ gh-repo ──┐                                                     │
 │  subdomain ─┘             ├─▶ configure-secrets ──▶ deploy (⏳ Active Task)     │
 │  ssl ─────────────────────┘                                                     │
 │                                                                                 │
 │  ┌── Operational Progress ──────────────────────────────────────────────────┐   │
 │  │  󰄬 1. Pre-flight Validation check                         ✓ 1.1s         │   │
 │  │  󰄬 2. Subdomain creation on small.pl (devil www add)      ✓ 4.2s         │   │
 │  │  󰄬 3. Let's Encrypt Wildcard SSL generation              ✓ 8.7s         │   │
 │  │  ▶ 4. GitHub repository and workflow generation           ⏳ 2.3s...     │   │
 │  │     ├── 󰊢 Initializing local git tree                                    │   │
 │  │     └── 󰒓 Generating GitHub secret: SSH_PRIVATE_KEY                      │   │
 │  │  ○ 5. Production deploy via Actions                        ○ Pending      │   │
 │  └──────────────────────────────────────────────────────────────────────────┘   │
 │                                                                                 │
 │  [Esc] Cancel operation and request surgical rollback                           │
 └─────────────────────────────────────────────────────────────────────────────────┘
```

#### Ekrany Awarii Kreatora (Chirurgiczna Korekcja & Wznowienie)

Gdy dany krok wizarda zawiedzie (np. nazwa bazy danych jest zajęta), Webox **nie wykonuje automatycznie** pełnego, niszczącego rollbacku. Zamiast tego zatrzymuje egzekucję, podświetla błędny węzeł na czerwono i daje użytkownikowi interaktywne menu naprawcze:

```text
 ┌─ ✗ Provisioning Failed: Database Name Conflict ───────────────────────────────────┐
 │                                                                                   │
 │  The step "Provision Database" failed. Error returned:                            │
 │  > devil: database name 'biuromody_sui' is already registered on s1.small.pl      │
 │                                                                                   │
 │  Please select a remediation action. Existing resources will be preserved.        │
 │                                                                                   │
 │  ┌── Remediation Menu ──────────────────────────────────────────────────────────┐ │
 │  │  ▶ [1] Correct DB name & Resume wizard execution                             │ │
 │  │    [2] Skip database step (Application will be marked as static site)        │ │
 │  │    [3] Surgical Rollback (Deletes subdomain, SSL; leaves GitHub repository)   │ │
 │  │    [4] Full Rollback (Destructive cleanup of all steps 1-4)                  │ │
 │  └──────────────────────────────────────────────────────────────────────────────┘ │
 │                                                                                   │
 │  ↑↓: Select option   [Enter] Execute remediation action                           │
 └───────────────────────────────────────────────────────────────────────────────────┘
```

---

## 5. Wymagania terminala i progi elastyczności

Webox 2.0 dynamicznie nasłuchuje zdarzeń zmiany rozmiaru terminala (`tea.WindowSizeMsg`) i płynnie reorganizuje interfejs w locie bez utraty stanu danych:

| Próg Szerokości (Cols) | Próg Wysokości (Rows) | Tryb Renderowania UI | Reakcja i Fallback |
|---|---|---|---|
| **≥ 120** | **≥ 35** | **Ultra Cockpit Mode (Bento)** | Wyświetla pełną siatkę Bento (5 kafelków: Projekty, Szczegóły, Logi, CI/CD Actions, Metryki). |
| **100 – 119** | **30 – 34** | **Standard Cockpit Mode** | Dashboard trójpanelowy (Projekty, Szczegóły, dynamicznie chowane logi u dołu). |
| **88 – 99** | **28 – 29** | **Classic Split-Pane** | Klasyczny podział dwupanelowy (Lewy: projekty, Prawy: szczegóły). Logi i metryki przeniesione do zakładek szczegółów. |
| **70 – 87** | **22 – 27** | **Single-Pane Focus** | Dashboard jednopanelowy. Lewy panel zajmuje 100% szerokości. Przejście do szczegółów projektu otwiera zupełnie nowy ekran. |
| **< 70** | **< 22** | **Blockout Screen** | Zatrzymanie renderu. Wyświetla pełnoekranowy komunikat ostrzegawczy z prośbą o zwiększenie rozmiaru okna terminala. Klawisz `q` zamyka aplikację. |

---

## 6. System nawigacji i Key bindings (UX 2.0)

### 6.1 Szybkie skróty akordowe (Fast-Chord Bindings)

Dla zaawansowanych użytkowników (Power-Users) Webox 2.0 wprowadza skróty akordowe (dwuklawiszowe kombinacje wzorowane na edytorze Vim/Neovim), które pozwalają na błyskawiczne wyzwalanie akcji na aktualnie zaznaczonym projekcie z poziomu listy głównej dashboardu — całkowicie **omijając konieczność otwierania szczegółów i potwierdzania**:

* `g` `r` — **Go Restart:** Błyskawiczny restart zaznaczonego projektu (miga fioletowy pasek statusu).
* `g` `d` — **Go Deploy:** Ręczne wyzwolenie GitHub Actions Deploy.
* `g` `e` — **Go Env:** Natychmiastowe przejście do zakładki `Env Diff` zaznaczonego projektu.
* `g` `l` — **Go Logs:** Natychmiastowe otwarcie streamu logów na pełnym ekranie.

---

### 6.2 Zestawienie klawiszologii TUI

#### Globalne (Wspólne dla każdego stanu)

| Klawisz | Akcja |
|---|---|
| `q` / `Ctrl+C` | Wyjście z aplikacji (z potwierdzeniem w wizardzie). |
| `/` | Otwarcie Command Palette w trybie Fuzzy Search. |
| `?` | Otwarcie pełnoekranowej pomocy z listą wszystkich skrótów. |
| `Esc` | Zamknięcie modali, powrót do ekranu nadrzędnego. |
| `Ctrl+R` | Natychmiastowe unieważnienie pamięci podręcznej (Cache Invalidation) i pełny przeładunek sieciowy. |

#### W zakładce Szczegółów Projektu (Cockpit Focus)

| Klawisz | Akcja |
|---|---|
| `H` / `L` | Nawigacja po zakładkach w lewo/prawo (`Overview` ↔ `Env` ↔ `DB` ↔ `Logs`). |
| `1`, `2`, `3`, `4` | Błyskawiczne przejście do konkretnej zakładki (np. `2` otwiera od razu `Env Diff`). |
| `v` | W zakładce `Env Diff`: Ujawnienie/zamaskowanie wybranej wartości zmiennej. |
| `Ctrl+Y` | Skopiowanie zaznaczonej wartości (np. hasła DB lub zmiennej) do systemowego schowka. |

---

## 7. Confirm dialogs i tryb Expert

Webox eliminuje tzw. „dialogue fatigue” (zmęczenie potwierdzeniami) za pomocą trójpoziomowej polityki bezpieczeństwa:

1. **Destructive Actions (Zawsze Wymagane):** Operacje takie jak usunięcie subdomeny z serwera, trwała utrata bazy danych, czy nadpisanie profilu hostingowego **zawsze** wymagają potwierdzenia. Nie można ich ominąć w żadnym trybie.
2. **Standard Actions (Bypassable):** Operacje typu restart, ręczny deploy, czy generowanie nowego certyfikatu domyślnie pokazują modal z dwoma przyciskami `[ Yes ]` oraz `[ No ]`.
3. **Tryb Expert (Opt-in):** Z poziomu modalu użytkownik może zaznaczyć pole `[x] Don't ask again for "Restart"`. Spowoduje to zapisanie flagi do pliku `config.json` w sekcji `settings.expert_mode.restart = true`. Od tego momentu skrót `r` na dashboardzie wykonuje restart natychmiastowo, dając jedynie sekundowy, zielony flash na ramce projektu (`Application Restart Triggered`).

---

## 8. Command Palette 2.0

Otwierana klawiszem `/` z dowolnego miejsca. Posiada fioletowy pasek wyszukiwania i dynamiczny fuzzy matching oparty na algorytmie Levenshteina.

```text
 ╭─────────────────────────────────────────────── Webox Cockpit v1.0 ─────────────────────────────────────────────────────────╮
 │  ┌──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐  │
 │  │  / restart_sui█                                                                                                      │  │
 │  │  󰆍 /restart:sui           Restart application sui.biuromody.smallhost.pl (s1.small.pl)                    ▶ Run     │  │
 │  │  󰏚 /ssl:renew:sui        Force renew SSL for domain sui.biuromody                                        ○ Select  │  │
 │  │  󰒓 /settings              Open global settings console                                                    ○ Select  │  │
 │  └──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘  │
 │                                                                                                                            │
 │  Type search query...  ↑↓: Navigate   Enter: Select   Esc: Close Palette                                                   │
 ╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

Komendy są grupowane w sekcjach kontekstowych, a post-MVP komendy (np. `/db`, `/storage`) są wyszarzone w wersji `v0.1` z jasną adnotacją: `/db will arrive in v0.2. Track roadmap at issue #142.`

---

## 9. Maskowanie sekretów i wizualny Diff

Zasada absolutnego bezpieczeństwa w UI Webox: **sekrety nigdy nie mogą być widoczne przez przypadek**.

* **Inline Masking:** Wszelkie klucze API, hasła bazodanowe oraz wartości zmiennych środowiskowych zakwalifikowane jako tajne są domyślnie renderowane jako ciąg ośmiu kropek: `••••••••`.
* **Surgical Reveal (`v`):** Zaznaczenie wiersza w `/env` lub w szczegółach bazy i wciśnięcie klawisza `v` ujawnia na żywo wyłącznie wybraną wartość. Pierwsze ujawnienie w danej sesji wyświetla jednorazowe ostrzeżenie: `Reveal secret? Make sure no one is looking over your shoulder.`. Ponowne wciśnięcie `v` natychmiast maskuje wartość z powrotem.
* **Wizualny Env Diff:** Wszelkie rozbieżności (Drifts) między plikiem lokalnym a serwerowym są wyraźnie podświetlane na żółto (`⚡ DRIFT`), a brakujące zmienne na czerwono (`✗ MISSED`), co całkowicie eliminuje sytuacje, w których wdrożona aplikacja nie startuje z powodu braku konfiguracji.

---

### 9.3 Interaktywny Dwukierunkowy TUI Env Merger

W przypadku wykrycia dryfu konfiguracji (`⚡ DRIFT`), użytkownik może wywołać klawiszem `m` dedykowane narzędzie do interaktywnego, bezpiecznego scalania plików konfiguracyjnych `.env` bez konieczności wychodzenia z terminala i manualnego przepisywania haseł.

```text
 ╭──────────────────────────── TUI Environment Conflict Resolver ────────────────────────────╮
 │ Conflict Key 4/7: DATABASE_URL                                                           │
 │                                                                                          │
 │   ┌── [H] Use Local Value ──────────────────┐   ┌── [L] Use Production Server Value ──────┐ │
 │   │  mysql://biuro_local:secPassword@127.0  │   │  mysql://biuromody_sui:******@db.smal   │ │
 │   │                                         │   │                                         │ │
 │   │  󰄬 (Selected for merging)              │   │  ○ (Discarded)                          │ │
 │   └─────────────────────────────────────────┘   └─────────────────────────────────────────┘ │
 │                                                                                          │
 │   ┌── Difference Inspector ─────────────────────────────────────────────────────────────┐  │
 │   │  - mysql://biuro_local:secPassword@127.0.0.1:3306/biuro_db                          │  │
 │   │  + mysql://biuromody_sui:******@db.smallhost.pl:3306/biuromody_sui                  │  │
 │   └─────────────────────────────────────────────────────────────────────────────────────┘  │
 │                                                                                          │
 │  ↑↓: Navigate keys  [Space]: Accept side  [e]: Edit selected  [Enter]: Commit & Restart  │
 ╰──────────────────────────────────────────────────────────────────────────────────────────╯
```

#### Klawiszologia i Interakcja:
* `H` (lub `←`) / `L` (lub `→`): Przełączanie fokusu pomiędzy lewym panelem (Local Value) a prawym panelem (Server Value).
* `Space`: Wybór danej wartości jako docelowej dla wybranego klucza. Pod wybranym panelem pojawia się zielona sygnatura `󰄬 (Selected for merging)`.
* `e`: Ręczne edytowanie wybranej wartości przed zatwierdzeniem. Otwiera jednolinijkowy miniedytor tekstowy w dolnej części ekranu.
* `Enter`: Chirurgiczne zastosowanie wygenerowanego pliku `.env` na produkcji (poprzez bezpieczną sesję SFTP), wyczyszczenie bufora i automatyczny, bezpieczny restart powiązanej aplikacji Node.

---

## 10. Internacjonalizacja i lokalizacja

Zgodnie z decyzją [ADR-0006](./adr/0006-jezyk-interfejsu-en-domyslny.md), interfejs jest **domyślnie angielski**.
Polski pakiet językowy (`translations/pl.json`) jest wbudowany i aktywuje się automatycznie przy wykryciu zmiennej środowiskowej systemowej `LANG=pl_PL.UTF-8` lub poprzez bezpośredni przełącznik w sekcji `/settings`.

Tłumaczenia są zorganizowane jako płaska struktura JSON z kontekstowymi prefiksami kluczy. Wszelkie brakujące klucze w plikach społecznościowych automatycznie spadają z powrotem (Fallback) do wartości z pliku `en.json`, zapobiegając awarii renderowania (Fail-Soft).

---

## 11. Flowy użytkownika premium

### 11.1 Flow A: Pierwsze Uruchomienie (Zero-Friction Bootstrapping)

1. Użytkownik wpisuje w konsoli `webox`.
2. Brak pliku `config.json` → System przechodzi automatycznie do stanu `stateInitWizard`.
3. Wyświetlenie sprawdzianu środowiska (Git, GitHub CLI, Keyring backend).
4. System wykrywa brak dedykowanego klucza SSH i pyta: `Create secure Webox keypair (~/.ssh/id_ed25519_webox)? [Yes]`.
5. Po wygenerowaniu, system pyta: `How would you like to install the public key on your server?`
   * **Option A (Rekomendowana/Automatyczna):** `Auto-inject to Host`. Użytkownik podaje jednorazowo hasło logowania SSH, a Webox pod maską wykonuje bezpieczną implementację `ssh-copy-id`, po czym natychmiast czyści hasło z pamięci.
   * **Option B (Ręczna):** `Show Public Key`. Wyświetla klucz w ładnej ramce z poleceniem skopiowania go do pliku `authorized_keys` w panelu Devil.
6. Webox testuje połączenie za pomocą bezhasłowego klucza. Przy sukcesie zapisuje `config.json` i przechodzi do krystalicznie czystego `stateDashboard`.

---

### 11.2 Flow B: Nowy Projekt z Self-Healing (Scaffold & Deploy)

1. Użytkownik klika `n` na dashboardzie.
2. **Krok 1 (Profile):** Wybór aktywnego profilu hostingowego.
3. **Krok 2 (Stack):** Wybór frameworku. Jeśli wybrano `Vite + React` lub `Static`, system inteligentnie pomija krok bazy danych, minimalizując Dialogue Fatigue.
4. **Krok 3 (Database):** Jeśli wybrano `Node/Express` lub `Next.js`, system pyta o chęć wykreowania bazy MySQL/PgSQL.
5. **Krok 4 (Subdomain):** Wpisanie nazwy subdomeny. W tle Webox odpytuje serwer small.pl o unikalność nazwy i weryfikuje dostępność rekordu DNS przez publiczny protokół DoH. Ścieżki na serwerze are rendered na żywo jako interaktywne drzewo katalogów.
6. **Krok 5 (Execution & Graph):** Wyświetlenie grafu zależności. Egzekucja asynchroniczna. 
   * **Happy Path:** Wszystkie węzły zmieniają kolor z `Muted` → `Warning` (w toku) → `Success` (zielony). Przejście do dashboardu, fokus na nowym projekcie.
   * **Failure Path (Self-Healing):** Jeśli krok Let's Encrypt zawiedzie (DNS nie zdążył się rozpropagować), kreator wstrzymuje pracę. Czerwony alert opisuje błąd techniczny. Użytkownik otrzymuje menu naprawcze, poprawia parametr lub wybiera opcję ponowienia próby po 30 sekundach, wznawiając wizard od punktu awarii. Zintegrowany mechanizm chroni przed powstawaniem zasobów-sierot (Orphaned Resources).

---

## 12. TUI Soundscapes & Akustyka Operacyjna

Webox 2.0 to zmysłowe doświadczenie terminalowe. Poprzez opcjonalny, retro-futurystyczny system dźwiękowy, operacje deweloperskie zyskują fizyczny wymiar. Wszystkie sygnały są generowane bez dodatkowych zależności systemowych, bezpośrednio z kodu Go.

### 12.1 Tabela Zdarzeń Dźwiękowych

| Zdarzenie UX | Typ Dźwięku | Częstotliwość (Hz) | Czas trwania (ms) | Charakterystyka akustyczna |
|---|---|---|---|---|
| **Nawigacja w Bento / Klawisz** | Kliknięcie mechaniczne | `1200 Hz` | `6 ms` | Bardzo krótki, precyzyjny impuls, imitujący mechaniczną klawiaturę. |
| **Zmiana karty szczegółów** | Impuls przełącznika | `800 Hz → 1000 Hz` | `15 ms` | Krótki świst o rosnącym tonie informujący o zmianie kontekstu. |
| **Zatwierdzenie wyboru (Enter)**| Potwierdzenie | `1500 Hz` | `40 ms` | Przyjemny, czysty dźwięk potwierdzający poprawność wyboru. |
| **Pełny sukces wdrożenia (GHA)**| Chime harmoniczny | `523 Hz, 659 Hz, 784 Hz` | `180 ms` | C-dur triada (arpeggio), radosny, harmoniczny sygnał pełnego sukcesu. |
| **Przerwanie Kreatora / Błąd** | Dron ostrzegawczy | `180 Hz` | `350 ms` | Niski, pulsujący dźwięk o charakterze alarmowym. |

### 12.2 Kontrola akustyczna (Mute System)
* **Klawisz szybkiego wyciszenia (`Ctrl+S`):** Pozwala na błyskawiczne włączenie/wyłączenie dźwięków w dowolnym momencie działania aplikacji.
* **Wizualna reprezentacja w Status Barze:**
  * Przy aktywnym dźwięku: `[ 󰓃 █▄▅▇ ]` (dynamicznie animowany korektor graficzny).
  * Przy wyciszeniu: `[ 󰓄 MUTED ]` (szary, nieaktywny badge).
