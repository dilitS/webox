# Webox — UX / Design System (Generacja 2026/2027)

> Status: Approved · Ostatnia aktualizacja: 2026-05-23 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [PRD.md](./PRD.md) (cele biznesowe), [DESIGN.md](./DESIGN.md) (architektura TUI i silnika), [adr/0006](./adr/0006-jezyk-interfejsu-en-domyslny.md) (język interfejsu), [adr/0007](./adr/0007-bento-ultra-eskalacja-mvp.md) (eskalacja Bento Ultra do MVP), [AUDIT.md](./AUDIT.md) (scope decisions).

---

## TL;DR

UX/UI Webox wkracza w generację **2026/2027** jako zintegrowany **Terminal Cockpit klasy premium**. **MVP (v0.1)** po [ADR-0007](./adr/0007-bento-ultra-eskalacja-mvp.md) dostarcza **pełną siatkę Bento-Box Grid (`120×35`)** z adaptive layout (fallback do `100×30` Standard Cockpit dla mniejszych terminali), **Live Log Stream** (§4.3 Tab [4]), **Live Service Topology Map** (§3.4), GitHub Actions live panel i server metrics header bar. 🔶 **STRETCH (v0.2+)** zostają: **Bento Ultra+ (`≥ 160×45`)** z dodatkowymi kafelkami (multi-server agregator, TTL panels), **Sound Engine** (§12), **`/env` Merger** (§9.3) i **fast-chord bindings**. Tradycyjny, liniowy kreator zastępujemy **Self-Healing Wizardem** prezentującym wizualny graf zależności z opcją chirurgicznego wznawiania i korekcji błędów na żywo — to **w MVP**, ale jako liniowy step-by-step z prostym LIFO rollback (graf jako wizualizacja, **nie** DAG-based engine — patrz [DESIGN §10](./DESIGN.md#10-dag-based-transactional-engine-wznawialny-rollback)).

> **Konwencja scope w tym dokumencie:** `🔵 MVP (v0.1)` = w zakresie pierwszego release'u. `🔶 STRETCH (v0.2+)` = zaprojektowane, ale **niezimplementowane** w MVP. Patrz [ROADMAP §3.3](./ROADMAP.md#33-czego-nie-ma-w-mvp).
>
> **Konwencja mockupów:** Wszystkie nazwy domen, profili, użytkowników i wartości env w mockupach ASCII (`s1.small.pl`, `biuromody`, `sui.biuromody.smallhost.pl`, `mysql://biuro_local:secPassword@…`, etc.) są **fikcyjne i ilustracyjne** — pochodzą z pierwotnego dogfooding setupu autora i służą tylko jako wzorzec wizualny. Zastępując je dla własnych testów używaj swojego profilu / aliasu / hosta. Żadne z tych wartości nie reprezentują działającego credentialu.

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

> 🔵 **MVP (v0.1) — dostarczone w Sprincie 11, dopracowane w Sprincie 12.** Kafelek `🌐 [Live Service Topology]` renderuje się w Ultra (`120×35`) i Ultra+ (`160×45`) jako **lewa dolna karta** pod `📂 [Active Projects]` i po lewej od `🚀 [CI/CD PIPELINE]`. Dla terminali `<120×35` (Standard Cockpit fallback) topology degraduje się do tabelarycznej listy `Connections:` wewnątrz kafelka `Overview`.

Renderer to czysta funkcja w `tui/components/asciigraph/asciigraph.go`. Producer (`tui/topology.go::buildTopologySnapshot`) folduje `config.Project` + `ProjectStatus` + `cicdSnapshotEntry` w jeden `asciigraph.Graph`. Pulse jest sterowany przez `m.nowFn().Second()%2`, więc edge'y BUILDING/OFFLINE migoczą na tick refresh dashboardu bez dodatkowego timera (i bez ryzyka leak goroutine).

```text
┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ 🌐 [Live Service Topology]                                                            ┃
┃ ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓                                        ┃
┃ ┃ 📦 dilitS-demo/shopease-web  ●             ┃                                        ┃
┃ ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛                                        ┃
┃         │ GHA Deploy                                                                  ┃
┃         ▼ ✓                                                                           ┃
┃ ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓                                        ┃
┃ ┃ 🖥 us-east-1  ●                             ┃                                       ┃
┃ ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛                                        ┃
┃         │ Proxy → node-express                                                        ┃
┃         ▼ ✓                                                                           ┃
┃ ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓                                        ┃
┃ ┃ 🌐 ShopEase-Web  ●                         ┃                                        ┃
┃ ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛                                        ┃
┃ ↻ live · All systems nominal                                                          ┃
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
```

#### Zasady Dynamiki Wizualnej Grafu:

| Stan edge'a | Konektor | Strzałka | Glyph wertykalny | Kolor | Sygnał źródłowy |
|---|---|---|---|---|---|
| **ONLINE** | `──────────` | `✓` | `│` | Success (#04B575) | HTTP 2xx + SSL ≥ 14 dni + last CI success |
| **BUILDING** | `╌╌ ╌╌ ╌╌ ╌` ↔ ` ╌╌ ╌╌ ╌╌ ` (pulse) | `▶` | `╎` | Warning (#FFB800) | CI run `in_progress` / `queued` / `pending` |
| **DEGRADED** | `━━━━━━━━━━` | `⚠` | `│` | Degraded (#D846EF) | SSL < 14 dni, HTTP 4xx, lub CI `cancelled`/`skipped` |
| **OFFLINE** | `⚡   ⚡   ⚡` ↔ `⚡ ⚡ ⚡ ⚡ ⚡` (pulse) | `✗` | `║` | Error (#FF4444) | HTTP 5xx, timeout, lub `ProjectOffline` |
| **UNKNOWN** | `··········` | `?` | `│` | Muted (#4E5A85) | brak danych (pierwsza klatka, przed pierwszym probe) |

Node states używają tych samych kolorów + okrągłych markerów: `●` (online), `◐` (building), `◑` (degraded), `○` (offline), `·` (unknown).

#### Hard-coded layout (v0.1)

Renderer **nie jest** generic DAG layout engine — to świadoma decyzja. Hard-coded 3-level tree (`Repo → Server → {Subdomain, [DB]}`) wystarcza dla typowego solo projektu small.pl/Devil. Generic DAG layout zostaje odroczony do v0.3+ przez ADR-0010 (do utworzenia w Sprincie 13). Project z 4+ services (np. cache layer) fallbackuje do tabelarycznej `Connections:` listy.

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

### 4.2 Dashboard 2.0 — Bento-Box Grid System (`120×35` MVP / `≥160×45` STRETCH)

> 🔵 **MVP (v0.1)** — Bento Ultra przy `120×35` eskalowane do MVP przez [ADR-0007](./adr/0007-bento-ultra-eskalacja-mvp.md). Dostarczane w Sprincie 08 (layout engine + theme), Sprint 09 (live logs + header bar metrics), Sprint 10 (CI/CD panel), Sprint 11 (topology map). Dla `100×30 ≤ width < 120×35` (Standard Cockpit) renderujemy zubożony layout split-pane. Dla `width ≥ 160×45` (Bento Ultra+) dorzucamy dodatkowe kafelki — to **🔶 STRETCH (v0.2+)** (multi-server agregator, TTL panels).

Gdy rozmiar okna terminala przekracza próg komfortu (`120×35`), Webox transformuje interfejs w pięcio­modułowy pulpit:

1. **Status bar** (full-width, top) — `WEBOX vX.Y.Z [LIVE]` brand-pill po lewej + pipe-delimited stream metryk po prawej (`HH:MM:SS · profile · Uptime · Load · RAM · Ping`). Pill `LIVE/STALE/PENDING/OFFLINE` zmienia kolor w zależności od świeżości danych (`status.GitHubStepsTTL` + `ssh:metrics:` cache).
2. **Active Projects** (lewa kolumna, górny rząd, magenta ramka) — lista projektów z kropkami stanu (`●` Success / Warning / Error / Muted) i zaokrąglonym pillem selekcji.
3. **SERVER: `<project>`** (prawa kolumna, górny rząd, magenta ramka) — iconified key-value (Profile / Stack / Node.js / Status / HTTP / SSL / Repo / Last Deploy) + per-line status dot.
4. **Live Service Topology** (lewa kolumna, drugi rząd, cyan ramka) — graf `📦 Repo → 🖥 Server → 🌐 App`, zsynchronizowany semantycznie z `Connections:` fallbackiem w Standard Cockpit.
5. **CI/CD PIPELINE: Main Branch** (prawa kolumna, drugi rząd, cyan ramka) — `Build #N: STATUS` badge + ponumerowane kroki z badgami `✓ ✗ ⏳ … ⊘ ⊗ ?` + skróty `[F8] View logs · [Enter] Open run`.
6. **Live Server Logs** (full-width, dół, magenta ramka) — timestampowane linie z kolorowanym poziomem (`INFO` cyan, `WARN` warning, `ERROR` red, `DEBUG` accent) i adnotacją `(redacted)` gdy redaktor sekretów coś przepuścił.

Aktywny panel jest podświetlony grubszą ramką w kolorze swojego akcentu. Jeśli pełny frame ma więcej linii niż wysokość terminala, operator przewija **wyłącznie część korpusu** (slot `body` w kontrakcie chrome — patrz [DESIGN §2.5 Chrome contract](./DESIGN.md#25-chrome-contract-status-bar--body--footer)) przez `PgUp` / `PgDn` / `Home` / `End` lub kółko myszy; **status bar (top chrome)** i **footer z hintem nawigacji (bottom chrome)** pozostają przyklejone do krawędzi terminala. Gdy body przekracza dostępną wysokość, footer pokazuje pasek `↕ scroll: PgUp/PgDn · Home/End · Mouse · (offset/max)`. Każdy kafelek niezależnie respektuje **height budget** — rzędy są wyrównywane do siebie (Topology = wysokości CI/CD, Server = wysokości Active Projects), a kafelek, którego treść przekracza budżet, zamiast pchać sąsiada w dół, zwraca dyskretny pasek `┃ … +N more lines · scroll inside tab/modal ┃`, zachowując pionowe bordery `┃` zgodne z kolorem akcentu kafelka.

Borderowanie infrastruktury wewnątrz topology map (`[Live Service Topology]`) używa **lekkich** glifów (`┌─┐└─┘`), żeby hierarchia czytała się jako *grid > tile > nodes* zamiast trzech konkurujących wag ramek; chrome kafelka pozostaje grubą ramką (`┏━┓`).

```text
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ WEBOX v0.1.0 [LIVE]    14:32:01 │ us-east-1 │ Uptime: 24d 11h │ Load: 0.12, 0.28, 0.31 │ RAM: 3.4/8.0 GB (42%) │ Ping: 18ms │
├────────────────────────────────────────────────┬───────────────────────────────────────────────────────────────────────────┤
│ ╭─[Active Projects]──────────────────────────╮ │ ╭─[SERVER: ShopEase-Web]────────────────────────────────────────────────╮ │
│ │  ( ShopEase-Web                  )       ● │ │ │ ⊟ Profile:     us-east-1                                              │ │
│ │    API-Gateway                           ● │ │ │ ◎ Stack:       node-express                                            │ │
│ │    Auth-Service                          ● │ │ │ ◆ Node.js:     v20.11.0 ●                                              │ │
│ │    Dashboard                             ● │ │ │ ✓ Status:      ONLINE ●                                                │ │
│ │    Dashboard-Admin                       ● │ │ │ ⇄ HTTP:        200 OK                                                  │ │
│ │    Payment-UI                            ● │ │ │ ⚿ SSL:         Valid (114 days remaining) ●                            │ │
│ ╰────────────────────────────────────────────╯ │ │ ⌬ Repo:        dilitS-demo/shopease-web                                │ │
│ ╭─[Live Service Topology]───────────────────╮ │ │ ⏱ Last Deploy: 2h ago · success                                        │ │
│ │ [ GitHub Repo ] ──▶ [ Production Server ] │ │ ╰────────────────────────────────────────────────────────────────────────╯ │
│ │        │                                  │ │ ╭─[CI/CD PIPELINE: Main Branch]─────────────────────────────────────────╮ │
│ │        └────▶ [ ShopEase-Web ]            │ │ │ Build #412  SUCCESS ✓  (1m 42s)                                        │ │
│ ╰────────────────────────────────────────────╯ │ │ [1] Git Checkout      ✓                                                │ │
│                                                │ │ [2] Install Deps      ✓                                                │ │
│                                                │ │ [3] Code Lint         ✓                                                │ │
│                                                │ │ [4] Build Artifact    ✓                                                │ │
│                                                │ │ [5] Unit Tests        ✓                                                │ │
│                                                │ │ [6] Deploy (Prod)     ✓     [F8] View logs · [Enter] Open run          │ │
│                                                │ ╰────────────────────────────────────────────────────────────────────────╯ │
├────────────────────────────────────────────────┴───────────────────────────────────────────────────────────────────────────┤
│ ╭─[Live Server Logs]───────────────────────────────────────────────────────────────────────────────────────────────────╮  │
│ │ [14:32:10] INFO  - API-Gateway:  Incoming GET /users (status: 200)                                                   │  │
│ │ [14:32:11] WARN  - Auth-Service: High latency detected (450ms)                                                       │  │
│ │ [14:32:12] INFO  - ShopEase-Web: Served /products in 88ms                                                            │  │
│ │ [14:32:14] DEBUG - Worker #2:    Cache hit for key 'prod:list'                                                       │  │
│ ╰──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯  │
│ q:quit  ↑↓:navigate  →/Tab:cockpit focus  n:new  /:command  ?:help                                                          │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

> **Offline preview.** Cały powyższy layout można obejrzeć bez serwera komendą `webox --mock` (lub `WEBOX_MOCK=1 webox`). Mock dostarcza deterministyczne dane (sześć projektów, jeden pipeline SUCCESS, sześć linii logów, fixed clock `14:32:01 UTC`) i nie wykonuje żadnych połączeń sieciowych.

---

### 4.3 Tabbed Cockpit (Wielomodułowe Szczegóły Projektu)

Gdy użytkownik wciśnie `→` lub `Tab` na projekcie, przechodzi do panelu szczegółów. Górna belka zamienia się w system zakładek. Nawigacja odbywa się za pomocą klawiszy `H`/`L` (lewo/prawo) lub cyfr `1`–`4`.

| Karta | Skrót | Scope | Status |
|---|---|---|---|
| [1] Overview | `1` | 🔵 MVP (v0.1) | implementowane |
| [2] Env Diff | `2` | 🔶 STRETCH (v0.2+) | wymaga `/env` post-MVP |
| [3] Database | `3` | 🔶 STRETCH (v0.2+) | wymaga `/db` post-MVP |
| [4] Logs | `4` | 🔵 MVP (v0.1) | Live log stream eskalowany do MVP przez [ADR-0007](./adr/0007-bento-ultra-eskalacja-mvp.md), dostarczany w Sprincie 09 |

> W MVP karty `[1] Overview` i `[4] Logs` są aktywne — `[1]` to widok statyczny z `[r] Restart`, `[s] SSL Renew`, `[v] Logs (last 200 lines)`; `[4]` to live log stream (Sprint 09). Karty `[2] Env Diff` i `[3] Database` (`H`/`L`, cyfry 2–3) są disabled z dimmed indicator `unlocked in v0.2`.

#### Karta [2] — Env Diff (Dwukierunkowy Podgląd Różnic Zmiennych) — STRETCH v0.2+

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

#### Karta [3] — Database Management — STRETCH v0.2+

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

#### Karta [4] — Live Log Stream — MVP v0.1 (Sprint 09)

> 🔵 **MVP (v0.1)** — eskalowane z STRETCH przez [ADR-0007](./adr/0007-bento-ultra-eskalacja-mvp.md). Dostarczane w Sprincie 09: SSH `tail -f` przez `ssh.Pool`, ring buffer 1000 linii, ANSI level coloring, **redactor pre-render** (każda linia przez `internal/log.Redact` przed dodaniem do bufora), 60fps throttle cap, context-cancellable na `q`/`Esc`.

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

### 6.1 Szybkie skróty akordowe (Fast-Chord Bindings) — STRETCH v0.2+

> 🔶 **STRETCH (v0.2+)** — chord bindings są niewystarczająco priorytetowe dla MVP. W MVP klawisze `r`, `s`, `v` w widoku Overview wystarczają. Chordy implementujemy po pomiarze realnej żądanej kadencji operacji w wczesnym dogfooding'u. Patrz [AUDIT A6](./AUDIT.md#a6-scope-creep-w-designmd-i-uxmd-poza-zakresem-mvp-z-roadmapmd-33).

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
| `PgUp` / `PgDn` | Przewijanie korpusu (body slot) w górę / w dół jednym ekranem. Status bar i footer pozostają przyklejone. |
| `Home` / `End` | Skok do początku / końca aktualnego korpusu (zachowuje top + bottom chrome). |
| `Mouse Wheel` | Przewijanie korpusu krokiem 3 linii (`MouseActionPress` na `WheelUp/WheelDown` — drag/long-press nie zapętla scrolla). |

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

### 9.3 Interaktywny Dwukierunkowy TUI Env Merger — STRETCH v0.2+

> 🔶 **STRETCH (v0.2+)** — wymaga `/env` post-MVP. Cała sekcja jest architektonicznym planem dla v0.2+. Patrz [DESIGN §11.1](./DESIGN.md#111-architektura-dwukierunkowego-env-merger-tui-env-merger-engine), [ROADMAP §3.3](./ROADMAP.md#33-czego-nie-ma-w-mvp), [AUDIT A6](./AUDIT.md#a6-scope-creep-w-designmd-i-uxmd-poza-zakresem-mvp-z-roadmapmd-33).

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

## 12. TUI Soundscapes & Akustyka Operacyjna — STRETCH (osobny RFC)

> 🔶 **STRETCH (osobny RFC, post-v0.2+)** — sound engine **nie jest** w żadnym planowanym release MVP/v0.2. Wymaga osobnego RFC z konkretną wartością user (research show'uje, że TUI z dźwiękiem przy daily-driver tools są **rzadko** chwalone i często wyciszane natychmiast po pierwszym kontakcie). Specyfikacja zachowana jako materiał do dyskusji. Patrz [DESIGN §17](./DESIGN.md#17-architektura-silnika-d%C5%BAwi%C4%99kowego-w-go-package-sound), [AUDIT A6+C1](./AUDIT.md#a6-scope-creep-w-designmd-i-uxmd-poza-zakresem-mvp-z-roadmapmd-33).

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

* **Klawisz szybkiego wyciszenia (`Alt+M` lub `Ctrl+M`):** Pozwala na błyskawiczne włączenie/wyłączenie dźwięków w dowolnym momencie działania aplikacji.

  > **Dlaczego nie `Ctrl+S`:** w wielu terminalach (`xterm`, `Terminal.app`, `gnome-terminal`) `Ctrl+S` to **XON/XOFF flow control**, który zatrzymuje rendering TTY do czasu `Ctrl+Q`. User wcisnąwszy `Ctrl+S` zobaczyłby zamrożony ekran i pomyślał, że webox crashnął. `Alt+M` (Meta+M) jest standardem w innych TUI (Vim, Emacs, htop). Patrz [AUDIT D6](./AUDIT.md#d6-uxmd-12-sound--ctrls-jako-szybkie-wyciszenie-ale-w-wielu-terminalach-ctrls-to-xonxoff-flow-control).

* **Wizualna reprezentacja w Status Barze:**
  * Przy aktywnym dźwięku: `[ 󰓃 █▄▅▇ ]` (dynamicznie animowany korektor graficzny).
  * Przy wyciszeniu: `[ 󰓄 MUTED ]` (szary, nieaktywny badge).
