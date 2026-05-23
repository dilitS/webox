# Sprint 08 — Bento Ultra Layout Engine + OKLCH Theme + Sprint-Leak Cleanup

> **Daty:** TBD → TBD (planowane 1.5-2.5 tygodnia solo) · **Czas:** ~35-50h skupienia
>
> **Cel:** dostarczyć adaptive layout engine (`100×30` Standard Cockpit fallback / `120×35` Bento Ultra default / `160×45` Bento Ultra+ extended), pełną paletę OKLCH zgodną z `UX.md §2`, premium status badges, double-border modals, gradientowy logotyp, spinner adaptacyjny **oraz** wyczyścić wszystkie wycieki „Sprint NN" z runtime UI. Po sprincie 08 wizualnie Webox wygląda jak premium cockpit z [UX §4.2](../UX.md#42-dashboard-20--bento-box-grid-system-12035-mvp--16045-stretch), ale **bez live data** w nowych kafelkach (CI/CD, Live Logs, Topology) — te dostarczają Sprinty 09-11.

---

## TL;DR

Po sprincie 08:

- `tui/bento/` zawiera adaptive layout engine z `BentoTile` interface i registry.
- `tui/theme/` ma pełną paletę OKLCH (Primary/Success/Warning/Error/Degraded/Muted + Surface Levels) zgodną z [UX §2.1](../UX.md#21-paleta-kolor%C3%B3w-oklch--hsl-precision).
- Premium components: status badges (`UX §3.1`), gradient header z logo (`UX §2.4`), double-border modals (`UX §2.2`), spinner adaptacyjny (`UX §3.3`).
- Auto-detect rozmiaru terminala z trzema progami; fallback path działa dla `<100×30`.
- **Wszystkie wycieki „Sprint NN" z runtime UI usunięte** (10 miejsc znalezionych w audycie 2026-05-23).
- Snapshot teatest goldens dla 3 rozmiarów (`100×30`, `120×35`, `160×45`) + 1 fallback (`80×24`).
- Coverage `tui/views/` ≥ 60% (z obecnego 0%).

**Nie robimy w tym sprincie:**

- Live log stream — Sprint 09.
- Live CI/CD panel — Sprint 10.
- Topology Map — Sprint 11.
- Sound Engine, Env Merger, fast-chord bindings — STRETCH v0.2+.

---

## Pre-flight Checklist

- [ ] Sprint 07 zamknięty z retro i `Outcome`.
- [ ] Re-read [ADR-0007](../adr/0007-bento-ultra-eskalacja-mvp.md) (kontekst eskalacji).
- [ ] Re-read [UX §2-§4](../UX.md#2-design-system-20), [UX §5](../UX.md#5-wymagania-terminala-i-progi-elastyczno%C5%9Bci).
- [ ] Re-read [DESIGN §2.3](../DESIGN.md#23-zasady-przep%C5%82ywu-danych-mvu), [DESIGN §12](../DESIGN.md#12-maszyna-stan%C3%B3w-tui).
- [ ] Verify `lipgloss` ostatnia stabilna wersja przez Context7 (OKLCH support).
- [ ] Confirm `make ci` green on `main` after Sprint 07 merge.
- [ ] Audyt wycieków „Sprint NN" w UI (zachowane w retro 2026-05-23, 10 miejsc):
  - `tui/views/init_wizard.go:87` — `"Sprint 05 captures the profile only; keypair work lands in v0.2."`
  - `tui/views/dashboard.go:20,37,46,88` — komentarze i runtime stringi z „Sprint 04/05".
  - `tui/commands.go:422` — `// FetchProjectStatuses performs the Sprint 04 read-only status probes`.
  - `tui/messages.go:11,22` — komentarze odwołujące się do Sprintów.
  - `tui/states.go:4,10,18,28` — komentarze odwołujące się do Sprintów (acceptable jako wewnętrzne komentarze; runtime stringi wycinamy).
  - `tui/theme/theme.go:4` — komentarz nagłówkowy.
  - `tui/wizard_runner.go:71,74,170,173` — komentarze opisujące Sprint 05/06 ewolucję (jako komentarze OK; nie dotyka runtime UI).
  - `tui/last_deploy_test.go:37` — string `"pending Sprint 06"` zwracany jako placeholder (zmień na `"—"` lub `"pending"`).
  - `tui/views/renderers_coverage_test.go:87` — `if !strings.Contains(out, "Sprint 04")` — zaktualizuj assert.
  - `tui/wizard_runner_test.go:330` — komentarz testowy (OK).

---

## Taski

### TASK-08.1 — `tui/bento/` layout engine + `BentoTile` interface

- **Estymata:** L
- **Zależności:** Sprint 07 done
- **Acceptance Criteria:**
  - [ ] `tui/bento/engine.go` definiuje `Engine` z `Render(width, height int) string` i auto-detekcją trzech rozmiarów: `Standard` (`width<120 || height<35`), `Ultra` (default, `120≤width<160` lub `35≤height<45`), `UltraPlus` (`width≥160 && height≥45`).
  - [ ] `tui/bento/tile.go` definiuje `BentoTile` interface: `ID() string`, `Render(size Size, focused bool) string`, `MinSize() (w, h int)`, `Slot() Slot` (`Left`/`Right`/`Top`/`Bottom`/`Center`/`Fullspan`).
  - [ ] `tui/bento/registry.go` ma `Register(tile BentoTile)` i `Tiles() []BentoTile` używane przez `tui/view.go`.
  - [ ] Domyślne kafelki Sprint 08: `ProjectsTile` (listProjects), `OverviewTile` (selected project details), `MetricsPlaceholderTile` (header bar — full implementation Sprint 09), `LogsPlaceholderTile` (Sprint 09), `CICDPlaceholderTile` (Sprint 10), `TopologyPlaceholderTile` (Sprint 11). Placeholders renderują "Coming in Sprint NN" w sposób **niesponiżający produkcji** — jasny komunikat z białą ramką i deferred-feature visual marker.
  - [ ] Fallback `<70×22` → fullscreen warning z [PRD §10.3](../PRD.md#103-terminal).
  - [ ] Tests: table-driven sizing matrix; każda kombinacja `(width, height)` zwraca poprawny `Mode`.
- **Docs:** [UX §4.2](../UX.md#42-dashboard-20--bento-box-grid-system-12035-mvp--16045-stretch), [UX §5](../UX.md#5-wymagania-terminala-i-progi-elastyczno%C5%9Bci), [ADR-0007](../adr/0007-bento-ultra-eskalacja-mvp.md).

### TASK-08.2 — Pełna paleta OKLCH + Surface Levels w `tui/theme/`

- **Estymata:** M
- **Zależności:** TASK-08.1
- **Acceptance Criteria:**
  - [ ] `tui/theme/theme.go` ma `Theme` z polami: `Primary`, `Success`, `Warning`, `Error`, `Degraded`, `Muted`, `SurfaceBase`, `SurfaceLow`, `SurfaceHigh`, `TextBright`, `TextDim` — wszystkie jako `lipgloss.Color` z mappingiem hex + OKLCH komentarz.
  - [ ] Dark theme + Light theme dostępne; auto-wybór przez `lipgloss.HasDarkBackground()`.
  - [ ] `tui/theme/styles.go` eksportuje `Header`, `Panel`, `ActivePanel`, `BentoTile`, `BentoTileActive`, `ProjectRow`, `SelectedProjectRow`, `StatusBadge(state)`, `Alert`, `HelpHints`, `Muted`, `GradientHeader`.
  - [ ] `StatusBadge` zwraca premium badge zgodnie z [UX §3.1](../UX.md#31-badges-statusu-premium): odwrócony kolor tekstu na kolorowym tle, zaokrąglone krawędzie, jedna z 5 ról: ONLINE/BUILDING/OFFLINE/STALE/DEGRADED.
  - [ ] Komentarz w `theme.go` wskazuje [ADR-0007](../adr/0007-bento-ultra-eskalacja-mvp.md) jako źródło decyzji.
  - [ ] Tests: smoke render dla każdego stylu × 80×24, brak nieoczekiwanych escape sequences poza ANSI control codes.
- **Docs:** [UX §2.1](../UX.md#21-paleta-kolor%C3%B3w-oklch--hsl-precision), [UX §2.2](../UX.md#22-system-warstw-i-g%C5%82%C4%99bi-dynamic-layering), [UX §3.1](../UX.md#31-badges-statusu-premium).

### TASK-08.3 — Gradient header z logo + spinner adaptacyjny

- **Estymata:** M
- **Zależności:** TASK-08.2
- **Acceptance Criteria:**
  - [ ] `tui/components/header.go` renderuje gradient header z logotypem zgodnie z [UX §2.4](../UX.md#24-identyfikacja-wizualna-i-branding-logo-g%C5%82%C3%B3wne) (linear gradient OKLCH od fioletu do neon-blue).
  - [ ] Compact badge `❖ webox cockpit v0.1 ❖` dla wąskich nagłówków.
  - [ ] `tui/components/spinner.go` zwraca `tea.Cmd` z dwiema klatkami: `Dot` (interval 50ms) dla RTT<30ms, `Pulse` (interval 200ms) dla RTT>150ms. RTT czytane z `status.Cache` (kontrakt: cache trzyma latency per server).
  - [ ] Mikro-animacja Splash Fade-In na `Init()` — 3 klatki × 80ms (interpolacja Muted → Primary).
  - [ ] Tests: gradient renderer deterministyczny dla seed RGB; spinner emituje `tea.Tick` z poprawnym interval.
- **Docs:** [UX §2.4](../UX.md#24-identyfikacja-wizualna-i-branding-logo-g%C5%82%C3%B3wne), [UX §3.3](../UX.md#33-spinner-adaptacyjny-morphing--latency-aware).

### TASK-08.4 — Double-border modals (Confirm / Command Palette / Help)

- **Estymata:** S
- **Zależności:** TASK-08.2
- **Acceptance Criteria:**
  - [ ] `tui/components/modal.go` renderuje modal z podwójną ramką (`║`, `═`) i drop-shadow simulation (`█▓▒░` na dolnej + prawej krawędzi).
  - [ ] `Modal{Title, Body, Buttons}` API; integracja z `tui.Model` przez `StateConfirmDialog`.
  - [ ] Confirm dialog dla actions disabled w MVP (Restart/SSL Renew/Deploy) jest **zachowany** jak był; tylko styl ramki podniesiony do premium.
  - [ ] Tests: snapshot dla modal 60×8.
- **Docs:** [UX §2.2](../UX.md#22-system-warstw-i-g%C5%82%C4%99bi-dynamic-layering), [UX §7](../UX.md#7-confirm-dialogs-i-tryb-expert).

### TASK-08.5 — Czyszczenie wycieków „Sprint NN" z runtime UI

- **Estymata:** S
- **Zależności:** TASK-08.1
- **Acceptance Criteria:**
  - [ ] `tui/views/init_wizard.go:87` → `"Default keypair generation lands in v0.2; for now Webox captures the SSH profile only."` (bez „Sprint 05").
  - [ ] `tui/views/dashboard.go:37` → string `"Sprint 04: actions are read-only"` usunięty (akcje aktywne od Sprintu 07).
  - [ ] `tui/views/dashboard.go:46` → `"No projects yet. Press [n] to start the new-project wizard."` (bez „Sprint 05").
  - [ ] `tui/views/dashboard.go:88` → tylko `"[r] Restart  [s] SSL Renew  [d] Deploy"` (bez „disabled in Sprint 04" — bo nie są już disabled po Sprincie 07).
  - [ ] `tui/views/dashboard.go:20` (komentarz) → "RenderDashboard renders the project list and overview tile" (bez „Sprint 04").
  - [ ] `tui/commands.go:422` (komentarz) → "FetchProjectStatuses performs read-only status probes per project".
  - [ ] `tui/messages.go:11,22` (komentarze) → bez odwołań do numerów sprintów; zachowaj merit techniczny.
  - [ ] `tui/states.go:4,10,18,28` (komentarze) → bez „Sprint 04/05/07"; opisuj **co robi stan**, nie kiedy został dodany.
  - [ ] `tui/theme/theme.go:4` (komentarz) → "Theme is the single source of truth for cockpit colors. Hex values are anchored in OKLCH (UX.md §2.1)."
  - [ ] `tui/wizard_runner.go` komentarze (71,74,170,173) — pozostają, są wewnętrzne i opisują ewolucję modułu (acceptable).
  - [ ] `tui/last_deploy_test.go:37` → assert na `"—"` lub `"pending"` zamiast `"pending Sprint 06"`.
  - [ ] `tui/views/renderers_coverage_test.go:87` — zaktualizuj assert do nowego footer copy.
  - [ ] `tui/wizard_runner_test.go:330` (komentarz testowy) — pozostaje.
  - [ ] `make lint` + `make test` green po zmianach.
- **Docs:** żadne — tylko cleanup runtime stringów.

### TASK-08.6 — Auto-detect terminal size + adaptive rendering

- **Estymata:** M
- **Zależności:** TASK-08.1, TASK-08.2
- **Acceptance Criteria:**
  - [ ] `tui/model.go` rozszerzony o `BentoMode` (`Standard`/`Ultra`/`UltraPlus`/`Tiny`), aktualizowany przez `tea.WindowSizeMsg`.
  - [ ] `tui/view.go` routuje rendering przez `bento.Engine.Render` zamiast bezpośrednio do `views.RenderDashboard` dla `Mode != Standard`.
  - [ ] Standard Cockpit (`100×30`) zachowany jako fallback path (legacy `views.RenderDashboard` używany jako Standard mode renderer).
  - [ ] Feature flag `WEBOX_LAYOUT` (env var): `standard` / `bento_ultra` / `auto` (default `auto`). User może wymusić Standard nawet na `120×35`.
  - [ ] Tests: `BentoMode` selector test, `WEBOX_LAYOUT` override test.
- **Docs:** [UX §5](../UX.md#5-wymagania-terminala-i-progi-elastyczno%C5%9Bci), [ADR-0007 Implementation notes](../adr/0007-bento-ultra-eskalacja-mvp.md#implementation-notes).

### TASK-08.7 — Snapshot teatest goldens + coverage uplift

- **Estymata:** M
- **Zależności:** TASK-08.1 do TASK-08.6
- **Acceptance Criteria:**
  - [ ] `tui/views/testdata/` zawiera goldens dla: `dashboard_100x30.golden`, `dashboard_120x35.golden`, `dashboard_160x45.golden`, `dashboard_80x24_fallback.golden`, `init_wizard_100x30.golden`, `project_detail_120x35.golden`.
  - [ ] Goldens stripped ANSI (deterministic).
  - [ ] Teatest smoke dla każdego rozmiaru — start program, send `tea.WindowSizeMsg`, capture `tm.Output()`.
  - [ ] Coverage `tui/views/` ≥ 60% (uplift z obecnych 0%).
  - [ ] Coverage `tui/bento/` ≥ 75% (nowy pakiet, łatwiej).
  - [ ] Coverage `tui/theme/` ≥ 85% (już 83.3%, drobny uplift).
- **Docs:** `TESTING.md §4` (snapshot strategy).

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| Layout engine za skomplikowany, breakage przy rozmiarach pośrednich | H | Three-mode strategy z explicit thresholds; tests pokrywają corner cases (119×34, 159×44). |
| Premium gradient header niewspierany na starszych terminalach | M | `lipgloss` auto-degraduje do 256-color; fallback do solid Primary jeśli `NoColor`. |
| Snapshot goldens nieczytelne (dużo ANSI) | M | Stripped ANSI w goldens; kolory walidujemy manualnie pre-release przez `make demo`. |
| `WEBOX_LAYOUT=bento_ultra` domyślnie off blokuje user discovery | M | Po teatest green w PR — flaga włącza się automatycznie (mode `auto`); env var pozostaje jako escape hatch. |
| Cleanup „Sprint NN" psuje testy szukające tych stringów | H | TASK-08.5 jawnie wymienia `renderers_coverage_test.go:87` i `last_deploy_test.go:37`; pełna lista w pre-flight. |
| BentoTile interface okazuje się za wąski dla Sprint 11 (Topology) | M | Interface designed by 4 use cases (Sprint 09/10/11); Sprint 11 może dorzucić `Streamable` rozszerzenie bez breaking change w `BentoTile`. |

---

## Dependencies signoff

Sprint 08 **nie dodaje** nowych zewnętrznych zależności. Wszystko buduje na istniejącym `lipgloss` / `bubbletea` / `bubbles` / `teatest`. Jeśli pojawi się potrzeba (np. OKLCH math library) — wymaga ADR + maintainer sign-off zgodnie z [AGENTS §1.2](../../AGENTS.md#12-kluczowe-biblioteki-sprawdzone-przez-context7).

---

## Outcome (wypełnij po sprincie)

- ✅ Done: ...
- ⏭️ Carry-over: ...
- 📌 Decyzje: ...
- 🧠 Surprises: ...
- 📊 Metryki:
  - Coverage `tui/bento/`: ?
  - Coverage `tui/theme/`: ?
  - Coverage `tui/views/`: ?
  - Liczba snapshot goldens: ?
- 🔒 Security validation:
  - [ ] Brak nowych runtime stringów wyciekających sprint identifiers / internal IDs.
  - [ ] Brak nowych external network calls.
  - [ ] `go test -race ./tui/...` green.
- ➡️ Następny sprint: `sprint-09-live-log-stream.md`

---

## Retro Link

`docs/retros/<data>-sprint-08.md` (do utworzenia po sprincie)
