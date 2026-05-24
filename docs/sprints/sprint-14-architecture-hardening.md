# Sprint 14 — Architecture Hardening (post-RC, pre-v0.2)

> **Daty:** 2026-06-16 → 2026-06-30 (planowane 2 tygodnie solo) · **Czas:** ~24-32h skupienia
>
> **Cel:** rozłożyć god-package `tui/` na surfaces, dopracować obrony jakości (per-tile scroll, SSH semaphore, integration test gate, host-key UX), i przygotować bezpieczne podłoże pod prace produktowe v0.2 (drugi provider, OAuth, config v3). **Bez** telemetrii, **bez** AI-driven features, **bez** plugin ecosystem — Sprint 14 to fundament technologiczny, nie produktowa rozbudowa.

---

## TL;DR

Sprint 13 dowiózł `v0.1.0-rc1` + chrome contract + e2e + perf gate. Sprint 14 wykorzystuje tę zieloną bazę żeby:

- **Domigrować surfaces** (init wizard / project detail / wizards / import preview) — projekt nie może wejść w v0.2 z god-package.
- **Wprowadzić per-tile scroll** — operator musi móc przewijać tylko logi bez ruszania całego dashboardu.
- **Dorzucić in-flight SSH semaphore** — koniec ryzyka thundering-herd przy 50+ projektach.
- **Naprawić host-key UX modal** — krótkoterminowy fallback zanim v0.2 dostarczy `webox doctor security --update-host-key`.
- **Domigrować integration test layer** — `internal/e2e` urosło do 5 scenariuszy, Sprint 14 dorzuca 5 kolejnych + uruchomienie w CI nightly.
- **Wprowadzić strukturalny tracing** (`--debug-trace`) — bez phone-home, tylko lokalny `~/.cache/webox/trace.jsonl` dla bug reports.

**Nie robimy w tym sprincie:**
- drugi provider (cPanel/DirectAdmin) — *research* tylko, kod w Sprincie 15+
- OAuth Device Flow PoC — Sprint 15
- `config.json` schema v3 / DB fields — Sprint 15
- DAG-based rollback engine — Sprint 16+
- żadnej telemetrii / phone-home
- AI / ML anomaly detection
- plugin marketplace

---

## Pre-flight Checklist

- [ ] Sprint 13 zakończony z zielonym `make ci` + zielonym `make bench-check`.
- [ ] `v0.1.0` GA wytagowany **albo** świadoma decyzja o `v0.1.0-rc2` z spisanym rationale.
- [ ] Krytyka §3.1–§3.9 z code review przeglądnięta i każdy punkt ma decyzję `do-in-14 / defer-to-15+`.
- [ ] `docs/UX.md` §4.2 + `docs/DESIGN.md` §2.4 (chrome contract) nadal aktualne — żadnych driftów z kodem.

---

## Taski

### TASK-14.1 — Migracja pozostałych powierzchni na `surface.Surface`

- **Estymata:** L
- **Zależności:** Sprint 13 surface foundation (`tui/surface/`, `tui/surface_adapters.go`).
- **Acceptance Criteria:**
  - [ ] `tui/surface/<state>/` istnieje dla każdej z: `initwizard`, `projectdetail`, `projectwizard`, `resumewizard`, `importpreview`.
  - [ ] Każda powierzchnia implementuje `Body / Crumb / Footer / AcceptsScroll`.
  - [ ] `tui/surface_adapters.go::surfaceFor` zwraca surface dla każdego z migrowanych stanów.
  - [ ] `tui/view.go::renderRootBody` switch jest pusty (każda linia `case ...` przeszła do surface).
  - [ ] `TestSurfaceFor_UnmigratedStatesReturnNil` zaktualizowany lub usunięty.
  - [ ] Per surface co najmniej 1 unit test (`tui/surface/<state>/<state>_test.go`).
  - [ ] Coverage ≥ 80% w `tui/surface/...`.
- **Docs:** [DESIGN §2.4](../DESIGN.md#24-chrome-contract-status-bar--body--footer); krytyka §3.1.

### TASK-14.2 — Per-tile scroll + focus rotation

- **Estymata:** M
- **Zależności:** TASK-14.1 (potrzebne żeby `Surface` deklarowało scrollowalne kafelki).
- **Acceptance Criteria:**
  - [ ] `bento.BentoTile` dostaje opcjonalny interfejs `ScrollableTile` (`Scroll(delta int)` + `ScrollOffset()`).
  - [ ] `Tab` / `Shift+Tab` cykluje fokus pomiędzy scrollowalnymi kafelkami; aktywny kafelek widoczny (gradient ramki).
  - [ ] `PgUp`/`PgDn`/`Home`/`End` + mouse wheel pchają fokusowany kafelek, nie globalny viewport.
  - [ ] Globalny scroll body (z Sprint 13) zostaje, ale tylko gdy żaden kafelek nie jest fokusowany.
  - [ ] Unit testy: 4 scenariusze fokusu (rotacja w przód, w tył, brak fokusu, fokus na nie-scrollowalnym kafelku).
  - [ ] `internal/e2e/`: 1 scenariusz e2e — scroll logów bez przesuwania CI/CD.
- **Docs:** [UX §6.2](../UX.md#62-zestawienie-klawiszologii-tui) — dodać sekcję per-tile scroll.

### TASK-14.3 — SSH in-flight semaphore + retry policy

- **Estymata:** M
- **Zależności:** none
- **Acceptance Criteria:**
  - [ ] `ssh/pool.go` (lub `status/fetcher.go`) zawiera `golang.org/x/sync/semaphore` z budgetem `max(8, len(profiles)/2)`.
  - [ ] Retry policy: 3 próby, exponential backoff `200ms / 500ms / 1.2s`, brak retry dla `ssh.ErrHostKeyMismatch`.
  - [ ] Race test: 100 projektów × 1000 tików; assert peak goroutines ≤ `budget × 1.5`.
  - [ ] CHANGELOG entry pod `Changed` (zmiana zachowania pod obciążeniem).
- **Docs:** [DESIGN §8](../DESIGN.md#8-trójpoziomowy-status-cache-stale-while-revalidate); krytyka §3.4.

### TASK-14.4 — Host-key mismatch modal (krótkoterminowy fallback)

- **Estymata:** S
- **Zależności:** none
- **Acceptance Criteria:**
  - [ ] Gdy SSH dial zwróci `host key mismatch`, TUI otwiera blokujący modal z:
    - pełną ścieżką do `~/.ssh/known_hosts`,
    - dokładną komendą `ssh-keygen -R <host>`,
    - linkiem do dokumentacji,
    - przyciskiem `[Esc] Close` (modal nie kontynuuje połączenia samodzielnie).
  - [ ] Modal nie zawiera plaintekstowego klucza ani fingerprintu — tylko hostname.
  - [ ] Unit test snapshot modalu (`tui/snapshot_test.go`).
  - [ ] CHANGELOG entry pod `Added`.
- **Docs:** [SECURITY §3.2](../SECURITY.md), krytyka §3.9. Pełny doctor security command nadal w v0.2 (TASK-15.x).

### TASK-14.5 — E2E layer rozbudowa + nightly CI

- **Estymata:** M
- **Zależności:** TASK-14.1 (potrzebne nowe surfaces żeby testować ich flows).
- **Acceptance Criteria:**
  - [ ] `internal/e2e/` ma ≥ 10 scenariuszy: dotychczasowe 5 + 5 nowych:
    1. init wizard happy path (welcome → alias → host → port → user → review → save).
    2. project wizard validation error (subdomain conflict).
    3. resume wizard → dyskretne porzucenie (phrase confirm).
    4. import preview → akceptacja 1 wiersza → status refresh.
    5. CICD modal → otwarcie → przewinięcie logów → zamknięcie.
  - [ ] GitHub Actions `nightly.yml` uruchamia `go test ./internal/e2e/...` co 24h; wyniki publikowane jako artefakt.
  - [ ] Pełny budget: < 10 s wall clock, < 30 s `-race` wall clock.
- **Docs:** [TESTING §3](../TESTING.md), krytyka §3.6.

### TASK-14.6 — Lokalny strukturalny tracing (opt-in, no phone-home)

- **Estymata:** S
- **Zależności:** none
- **Acceptance Criteria:**
  - [ ] `webox --debug-trace` zapisuje per-frame JSONL do `~/.cache/webox/trace.jsonl` (auto-rotation przez lumberjack: 5 MB × 3 plików).
  - [ ] Format: `{ts, msg_type, duration_ms, state, viewport_w, viewport_h}` — **bez** wartości secret-prone (path, alias, dialog text).
  - [ ] Redactor z `internal/log/redact.go` przepuszczany na każde pole `string` przed zapisem (regression test wbudowuje sentinel `ghp_XXXX` i sprawdza że NIE pojawia się w pliku).
  - [ ] Flaga jest **wyłącznie lokalna** — `--debug-trace` nigdy nie loguje do stdout, nie wysyła nigdzie HTTP, brak metryk pasywnych.
  - [ ] `webox doctor` raportuje czy `trace.jsonl` istnieje i jego rozmiar (operator widzi że trace działa).
- **Docs:** [SECURITY §10](../SECURITY.md); krytyka §3.5.

### TASK-14.7 — `clipTileBlock` refactor na structured `TileBlock`

- **Estymata:** S
- **Zależności:** TASK-14.1 (surfaces mogą już wymagać scroll-aware tile API).
- **Acceptance Criteria:**
  - [ ] `bento.BentoTile.RenderBlock(width, maxHeight int) TileBlock` (nowa opcjonalna metoda).
  - [ ] `TileBlock{Header, TitleRow, Body []string, Footer, AccentRGB}` — engine decyduje co clipować.
  - [ ] Stary `Render()` zostaje (backwards-compat), ale `RenderBlock()` ma pierwszeństwo gdy tile go implementuje.
  - [ ] `clipTileBlock` / `framedIndicatorLine` przeniesione do operowania na `TileBlock`.
  - [ ] Magic constants `borderRows = 2`, `bordersAndHeader = 3` usunięte (struktura zamiast heurystyki string-level).
  - [ ] Bench (`make bench-check`) nie regresuje (próg 5 ms zostaje, ale baseline może lekko spaść).
- **Docs:** krytyka §3.2.

### TASK-14.8 — v0.2 backlog freeze (docs only)

- **Estymata:** S
- **Zależności:** TASK-14.1–14.7 zakończone
- **Acceptance Criteria:**
  - [ ] `docs/sprints/sprint-15-v02-foundation-plan.md` powstaje z 4 sztywno-zafreezowanymi tematami:
    1. cPanel adapter implementacja (Provider Pattern walidacja).
    2. OAuth Device Flow PoC za `WEBOX_EXPERIMENTAL=1`.
    3. `config.json` schema v3 + migracja v2→v3 + opcjonalne DB.
    4. ADR-0010: i18n migration plan (gradual view-by-view).
  - [ ] Każdy temat ma referencję do odpowiedniego ADR + estymatę.
  - [ ] `docs/ROADMAP.md` zaktualizowany: v0.2 daty przesunięte / potwierdzone.
- **Docs:** [ROADMAP §3](../ROADMAP.md).

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| Migracja surfaces (TASK-14.1) wprowadzi regresje w mniej testowanych stanach (resume, import). | H | `internal/e2e` TASK-14.5 dorzuca scenariusze dla nich; per-surface unit test obowiązkowy w AC. |
| Per-tile scroll (TASK-14.2) zacznie konfliktować z istniejącymi `↑/↓` keybindings (lista projektów, log buffer). | M | AC zawiera explicit "globalny scroll tylko bez fokusu kafelka"; e2e weryfikuje brak konfliktu. |
| `--debug-trace` zacznie być traktowany jako wstęp do telemetry. | M | AC: "**wyłącznie lokalna**, brak HTTP". Wpis w `SECURITY.md` że żaden trace plik nie opuszcza maszyny operatora. |
| Sprint 14 ujawni wzrost złożoności tile API > planowany budget refactoru (TASK-14.7). | M | TASK-14.7 jest oznakowany S — jeśli puchnie do M+, carry-over do Sprintu 15 (nie blokuje surfaces ani per-tile scroll). |

---

## Dependencies signoff

Sprint 14 może wymagać:

- `golang.org/x/sync` (`semaphore`) — już w `go.mod`, sign-off zbędny.
- `gopkg.in/natefinch/lumberjack.v2` — już w `go.mod` (logger rotation).

**Nowe zależności:** zero. Jeśli któraś okaże się potrzebna → wymaga sign-off maintainera i ADR.

---

## Outcome (wypełnij po sprincie)

- ✅ Done: ...
- ⏭️ Carry-over → Sprint 15: ...
- 📌 Decyzje:
  - Surfaces migracja kompletna: TAK / NIE
  - Per-tile scroll: TAK / NIE
  - Host-key modal: TAK / NIE
  - Tracing: TAK / NIE
- 🧠 Surprises: ...
- 📊 Metryki:
  - Coverage end-of-sprint: ?%
  - `internal/e2e` scenarios: ?
  - Bench worst ns/op: ?
- 🔒 Security validation:
  - [ ] `govulncheck` zielony
  - [ ] Brak nowych path-traversal / redact-bypass vectors
  - [ ] `trace.jsonl` nie zawiera żadnego sentinela `ghp_TESTSENTINEL`
- ➡️ Następny sprint: `sprint-15-v02-foundation-plan.md`

---

## Retro Link

`docs/retros/<data>-sprint-14.md` (do utworzenia po zakończeniu sprintu)
