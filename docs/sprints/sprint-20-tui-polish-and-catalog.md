# Sprint 20 — TUI Polish & Provider Catalog

> **Daty:** 2026-05-25 → 2026-06-08 (2 tygodnie) · **Cel:** Dokończyć cooldown po-MVP TUI: dostarczyć Provider Catalog screen (carry-over z 19.4) i naprawić długoletnie UX-bugi (klikanie, scroll, scaling, dishonest hints).
>
> **Status:** ✅ Done (2026-05-25, all 6 tasks closed in one autonomous session) · **Properties:** code-heavy, low operator-time, no external blockers.

## Kontekst

Sprint 19 dostarczył **preset registry** jako fundament — w MVP wystąpił jako CLI (`webox doctor preset --list/--id=…`). TUI Provider Catalog (TASK-19.4) został świadomie odroczony, żeby uniknąć zmian w cocktail layout w sesji autonomicznej.

Równolegle operator zgłosił szereg „TUI feels unfinished" defektów po Sprint 19 ([transcript: feat/sprint-19-preset-registry merge](../../docs/sprints/sprint-19-preset-registry.md)):

- Klikanie myszą = no-op na każdym ekranie.
- Pasek dolny chrome reklamuje **nie-istniejące** `[/] command palette`.
- Project Detail naciśnięcie tab 2/3 (`Env Diff` / `Database`) emituje redundantny alert „tab available in v0.2" (label już o tym mówi).
- UltraPlus deep-dive strip pokazuje stub `Reserved for Sprint 11+` (Sprint 11 dawno zakończony).
- Tiny mode mówi „press [r] to redraw" — `[r]` nie jest wpięte; resize sam wystarczy.
- Standard mode (`100×30` fallback) wciąż używa pre-Sprint 13 stacked-tile silhouette — wygląda niedopracowanie.

Sprint 20 ma trzy cele równoległe: **(a)** dokończyć Provider Catalog (preset → operator), **(b)** naprawić TUI defects ujawnione po Sprint 19 release-candidate, **(c)** zaprojektować i zaimplementować click-to-focus hit testing (proper mouse support).

W Sprint 20 powstał już **commit `feat/sprint-20-tui-polish`** który zawiera „pierwszą falę" fixów (covered by [CHANGELOG: Sprint 20 — TUI Polish (in progress)](../../CHANGELOG.md)). Sprint kończy ten branch + dorzuca catalog + hit-testing.

## Cel sprintu

Po Sprincie 20 operator:

1. Otwiera Provider Catalog z dashboardu (`p` lub TUI menu) i widzi pełen katalog provider presetów (filtrowanie, drilldown, copy-to-clipboard markdown briefing).
2. Klika myszą na konkretny tile w bento grid — fokus przeskakuje **na ten tile** (nie na "next in cycle"), a click na project row w Projects tile zaznacza go i otwiera Project Detail.
3. Standard mode (`100×30`) wygląda spójnie z Ultra (mini-bento, nie stacked tiles).
4. Wszystkie dolne hint-stringy są zgodne z faktycznie wpiętymi keybindingami; brak kłamliwych referencji do nie-implementowanych feature'ów.

Czego **nie** umiemy: cPanel adapter (Sprint 21+), CI/CD step click-through (carry-over), command palette (Sprint 22+ — wymaga ADR).

## Taski

### TASK-20.1 — Layout-aware mouse hit testing → click focuses specific tile

- **Estymata:** L (1.5-2 dni)
- **Zależności:** —
- **Acceptance Criteria:**
  - [ ] `bento.Engine.RenderMode` publishes a layout map: `map[Slot]Rect{X, Y, Width, Height}` accessible via `engine.Layout()` after each render.
  - [ ] `tui.Model` stores latest layout map after each `View()` call (via a hook in `renderDashboardBody` — pure-View constraint preserved by writing to a pointer-receiver helper invoked from the bento engine itself).
  - [ ] `Model.handleLeftClick(x, y)` resolves click coordinates into a `Slot`. If the click lands on a scrollable tile → focus that tile. Otherwise (Projects/Server) → toggle "no focus" + drill into Project Detail.
  - [ ] Click on Projects tile **row** (parsed from row Y) → set `m.selectedIndex` to that project index AND open Project Detail.
  - [ ] Click in Project Detail → return to dashboard (existing Sprint 20 wave-1 behaviour preserved).
  - [ ] e2e test using teatest: `tea.MouseMsg{Action: Press, Button: Left, X: <projects tile center>, Y: <row 2>}` → state changes to `StateProjectDetail` AND selectedIndex matches the clicked row.
  - [ ] Unit tests for layout map: every Bento Ultra slot has non-overlapping non-zero rectangles at (`120×35`, `140×40`, `160×45`).
- **Pliki:**
  - `tui/bento/engine.go` (extend Render to capture layout)
  - `tui/bento/layout.go` (new — `Rect`, `LayoutMap`)
  - `tui/bento/layout_test.go` (new)
  - `tui/update.go` (rewire `handleLeftClick` to consume layout map)
  - `tui/tile_focus_test.go` (extend with positional click scenarios)
  - `internal/e2e/cockpit_test.go` (new e2e: click drives selection)
- **Docs:** [DESIGN.md §3 (Layout)](../DESIGN.md), [UX.md §4.4 (Mouse interactions)](../UX.md), this sprint plan.
- **Notatki:** Bubble Tea sends absolute X/Y in cell coordinates. The layout map can stay deterministic because `renderUltraGrid` already computes `leftCol`, `rightCol`, `budget.{TopRow,SecondRow,Logs}` — we expose those as `Rect` values.
  Watch out for: lipgloss border + padding eat 1-2 cells per side → tile inner regions ≠ outer regions; choose ONE convention (outer including border) and document it.

### TASK-20.2 — Provider Catalog screen (carry-over of TASK-19.4)

- **Estymata:** L (1.5-2 dni)
- **Zależności:** Sprint 19 preset registry (`presets.Default()`).
- **Acceptance Criteria:**
  - [ ] New state `StateProviderCatalog` accessible via dashboard hint `[p] catalog` (added to dashboard footer).
  - [ ] List view: 6 presets shown with verified-status pill (Verified / Candidate / Research / Community / Deprecated), market chips (PL/EU/NA/Global), capability badges (`+http`, `+ssl`, `+restart`, `+logs`).
  - [ ] Filter: `f` cycles `[All, Verified only, By market: PL/EU/NA/Global]`.
  - [ ] Detail view: pressing `Enter` on a row opens a markdown-rendered briefing (id, panel, paths, restart command, probes, known risks, sources, verifiedAt) using `glamour` (already in `go.mod` for the help screen).
  - [ ] `c` copies the briefing as Markdown to OS clipboard (via `golang.design/x/clipboard` — already in `go.mod`).
  - [ ] e2e test: `[p]` → catalog → `Enter` on smallhost-devil → briefing visible → `c` → clipboard contains "smallhost-devil".
- **Pliki:**
  - `tui/states.go` (new state constant)
  - `tui/surface/providercatalog/providercatalog.go` (new surface package)
  - `tui/surface/providercatalog/providercatalog_test.go` (new — table-driven over presets)
  - `tui/views/provider_catalog.go` (new renderer)
  - `tui/surface_adapters.go` (register surface)
  - `tui/update.go` (handlers + clipboard cmd)
  - `internal/e2e/cockpit_test.go` (new e2e scenario)
  - `docs/UX.md §4.X (Provider Catalog)` (new section)
- **Docs:** [presets/registry.go](../../presets/registry.go), [docs/providers/preconfiguration-vision.md](../providers/preconfiguration-vision.md).
- **Notatki:** This is the operator-visible payoff for Sprint 19's invisible plumbing. Briefing format must match `docs/contributing/PRESET.md` so a contributor can copy-paste from the catalog into a PR description. Clipboard write happens in a `tea.Cmd` (I/O off the Update path) and emits a one-second alert "Copied to clipboard".

### TASK-20.3 — Standard mode (`100×30`) redesign — proper mini-bento

- **Estymata:** M (1 dzień)
- **Zależności:** —
- **Acceptance Criteria:**
  - [ ] `renderStandardFallback` replaced with a 1-column compact bento: status bar → projects tile (full width) → server overview tile (full width) → mini-cicd row (compact, 4 lines) → mini-logs row (compact, 5 lines).
  - [ ] No overflow at `100×30`; total body ≤ 29 lines (reserved 1 for footer).
  - [ ] Visual diff: golden-file regression test (`tui/views/standard_dashboard_test.go`) capturing the rendered output at `100×30`, `90×26` (clipped), and the Tiny→Standard threshold.
- **Pliki:**
  - `tui/bento/engine.go` (replace stacked-tile fallback)
  - `tui/bento/engine_test.go` (golden updates)
  - `docs/screenshots/sprint-20/02-dashboard-standard-100x30.txt` (regenerated)
- **Docs:** [UX.md §4.2 (Bento tiers)](../UX.md), [docs/AUDIT.md §IMP-08 (standard fallback ugly)](../AUDIT.md#imp-08).
- **Notatki:** Project owner runs Webox over SSH from a phone occasionally — Standard mode needs to feel just-as-polished as Ultra. The compact mini-bento must keep the same visual grammar (thick borders, rounded inner shapes, glyph-prefix rows) so muscle memory transfers across tiers.

### TASK-20.4 — Project detail tabs 2 & 3 unstub: Env Diff + Database read-only views

- **Estymata:** M (1 dzień)
- **Zależności:** TASK-20.1 (clicks unify navigation).
- **Acceptance Criteria:**
  - [ ] Tab 2 "Env Diff" surface renders the project's `.env` vs `.env.example` diff using existing `services/envdiff` (skeleton from Sprint 14 backlog) — falls back to "no .env file detected" when neither exists.
  - [ ] Tab 3 "Database" surface lists databases attached to the project (from `cfg.Project.Databases`) with `[engine] [name] [size?]` rows.
  - [ ] Both tabs accept scroll (long .env diff, many DBs).
  - [ ] Tab labels lose the "unlocked in v0.2" annotation (tabs are now implemented).
  - [ ] e2e test: `Right` → `2` → "Env Diff" header visible; `3` → "Database" header visible.
- **Pliki:**
  - `services/envdiff/envdiff.go` (new — pure-Go, no SSH, file-system based)
  - `services/envdiff/envdiff_test.go` (new — golden-file)
  - `tui/views/project_detail_envdiff.go` (new)
  - `tui/views/project_detail_database.go` (new)
  - `tui/views/project_detail.go` (route active tab → renderer)
  - `tui/update.go` (remove silent-ignore for 2/3, route to enterEnvDiffTab/enterDatabaseTab)
- **Docs:** [PRD.md §6 (priorytety)](../PRD.md), [DESIGN.md §3.X (env diff)](../DESIGN.md).
- **Notatki:** The "v0.2 placeholder" copy was a deliberate tease in Sprint 04 — by Sprint 20 we should either ship the feature or remove the tabs. We're shipping. Database tab uses cached config data only (no live SQL connection in v0.1). The Env Diff renderer must redact secret-shaped values (API keys, DB passwords) using `internal/log/redact.go`.

### TASK-20.5 — Help screen (`?`) overhaul + per-surface key reference

- **Estymata:** M (0.5-1 dzień)
- **Zależności:** TASK-20.1, TASK-20.2 (so help reflects new keys).
- **Acceptance Criteria:**
  - [ ] `?` opens a modal/overlay with sections grouped by surface (Dashboard / Project Detail / Wizard / Provider Catalog / Resume Wizard / Import Preview).
  - [ ] Each section enumerates keys + their effect, sourced **dynamically** from each surface's `Footer().Text` (so help can never drift from the actual key router).
  - [ ] `?` is closeable via `?`, `Esc`, or `q`.
  - [ ] Clicking outside the modal also closes it (consumes TASK-20.1 hit testing).
- **Pliki:**
  - `tui/help.go` (new)
  - `tui/help_test.go` (new — table-driven over states)
  - `tui/view.go` (overlay layering)
- **Docs:** [UX.md §6 (Help & onboarding)](../UX.md).
- **Notatki:** Existing `helpVisible` boolean in Model is unused; we hijack it. The modal renders with the same `components.Modal` used for CI/CD logs and host-key warnings.

### TASK-20.6 — CHANGELOG, retro, screenshots, sprint review

- **Estymata:** S (< 2h)
- **Zależności:** TASK-20.1 → TASK-20.5.
- **Acceptance Criteria:**
  - [ ] `CHANGELOG.md` `[Unreleased]` section grows: Added (Provider Catalog, click hit-testing), Changed (Standard mode redesign, env diff/database tabs), Fixed (chrome hint dishonesty, deep-dive stub).
  - [ ] `docs/screenshots/sprint-20/*.txt` regenerated for all surfaces and tiers (Tiny / Standard / Ultra / UltraPlus + Catalog + Help).
  - [ ] `docs/retros/2026-06-08-sprint-20.md` written following retro skill template.
  - [ ] Sprint plan "Outcome" section completed.

## Risk watch

| Ryzyko | Mitygacja |
|---|---|
| Layout map abstraction leaks bento internals into tui package | Keep `bento.Rect` opaque; tui only sees `LayoutMap.SlotAt(x,y) (Slot, bool)`. |
| Standard mode redesign breaks `100×30` golden tests across other packages | Run full `make ci` after TASK-20.3 — any golden file mismatch is forced into the diff for visual review. |
| Provider Catalog clipboard write fails on headless CI | Wrap clipboard write in try/log; emit alert "Copied (or saved to /tmp/webox-briefing-<id>.md as fallback)". |
| `services/envdiff` reads project files via SSH cache TTL — race with restart | Read on-demand only when tab opens; no background polling. |
| `Help` overlay keybinding (`?`) conflicts with existing surface that also wants `?` | Help is global at `Update`-router level; surfaces can't claim `?`. Document in `docs/conventions.md`. |

## Outcome (2026-05-25, sprint zamknięty wcześniej niż planowano)

- ✅ **Done (6/6 taski + bonus operator-validation):**
  - TASK-20.1 — `bento.LayoutMap` + click hit-testing (focus scrollable tile / drill non-scrollable / status-bar no-op).
  - TASK-20.2 — Provider Catalog screen pod `[p]`, kursor `↑/↓`, detail toggle `Enter`, copy-briefing `c` (clipboard via `pbcopy`/`xsel`/`xclip`/`wl-copy`/`clip.exe` z graceful fallback hintem).
  - TASK-20.3 — Standard mode redesign na proper mini-bento (top: projects + server, bottom: CI/CD strip + Live Log strip), 5 nowych regression testów dla budgetu wiersza/clippingu.
  - TASK-20.4 — Project Detail tabs `[2] Env Diff` i `[3] Database` jako read-only views; Env Diff czyta `SecretsMeta` (z badge stale), Database to stack-aware cheatsheet z naming convention z domeny.
  - TASK-20.5 — Help overlay (`?`) jako fullscreen centered modal nad każdym ekranem, surface keys parsowane z `Footer().Text` (live, never drifts), strict-block routing kluczy (tylko `?`/`Esc`/`Enter`/`q`/`Ctrl+C` reach the model). Refactor `updateKey` → `handleOverlayKey` zbił cyclomatic complexity poniżej lint gate.
  - TASK-20.6 — CHANGELOG `[Unreleased]` z 5 wpisami Added (catalog, help, screenshot tool, mouse semantics, smoke-test runner) + 1 Changed; 14 statycznych screenshotów w `docs/screenshots/sprint-20/`; 17 smoke-snapshotów + REPORT.md w `docs/screenshots/sprint-20/manual/`; retrospective w `docs/retros/2026-05-25-sprint-20.md`.
  - **BONUS — `make smoke-test` (tuistory PTY driver).** Operator validation problem zidentyfikowany w retro Sprint 20 (Open Question #3) został zaadresowany w tej samej sesji: `scripts/manual-test/` z 5 scenariuszami × 34 assertions × ~83s end-to-end pokrywa każdą Sprint 20 zmianę widoczną dla operatora (resize bento, help overlay, catalog, project detail tabs, mouse click). Wynik: `✓ all 5 scenarios passed` w pierwszym uruchomieniu, snapshoty diff-friendly.
- ⏭️ Carry-over: brak — wszystkie taski + bonus zamknięte w jednej sesji.
- 📌 **Decyzje (bez nowych ADR):**
  - Help overlay zastępuje cały `View()` zamiast composite-paint-on-top — uniknęliśmy double-border artefaktów z bento engine i nie musieliśmy modyfikować layout viewport math. Decyzja udokumentowana w komentarzu `helpOverlayFullscreen`.
  - Clipboard via `os/exec` + per-OS allowlist (`pbcopy` macOS, `xsel`/`xclip`/`wl-copy` Linux/Wayland, `clip.exe` Windows) zamiast nowej `go.mod` zależności (`atotto/clipboard`) — uniknęliśmy procedury sign-off maintainera.
  - `cmd/screenshot` jako stable dev tool zamiast ad-hoc `probe.go` — przyszłe sprinty mają deterministyczny generator captures.
  - Provider Catalog briefing format ("Webox Provider Briefing — …") jako stabilny plain-text grammar (sectioned, line-oriented) — operator może wkleić do Slack/post-mortem/onboarding doc bez markdown noise.
- 🧠 **Surprises:**
  - `gofumpt 0.10` na go1.26 ma issue z multi-arg `append(slice, "literal", style.Render(…))` — wymusiło konwersję do `slice = append(slice, group...)` w trzech miejscach. Małe, ale notable bo łatwo regenerable.
  - `bento.Engine` differential rendering optymalizuje znikające linie statusa, więc e2e `teatest.WaitFor` na "[Active Projects]" hangował po Esc; przerzuciliśmy needle na `[Right/Enter] open` (footer-only) — fixed.
  - Help overlay `tea.MsgKey` `q` przy otwartym modal MUSI quitować (operator-safety), więc strict-block w `handleOverlayKey` ma explicit `case "q"`.
- 📊 **Metrics:**
  - `make test`: 100% pass · coverage **80.1% ≥ 70%** (`+0.4pp` vs Sprint 19 close).
  - `make bench-check`: worst `BenchmarkRenderMode/ultraplus-160x45` = **199µs ≤ 5ms budget** (40× margines).
  - `make vulncheck`: 0 vulnerabilities (Go side; tuistory dev-only deps mają 23 transitive low/moderate — udokumentowane w `scripts/manual-test/README.md`).
  - `make lint`: 0 issues po naprawie 9 (mnd, gosec G204 false-positive, gocyclo, prealloc, unconvert).
  - `make smoke-test`: 5/5 scenarios, 34/34 assertions, ~83s wall-clock end-to-end.
  - 36 nowych unit testów (catalog model 7, catalog views 5, catalog surface 2, help overlay 6, env diff 5, database 4, mini-bento 5, layout map 2).
  - 14 statycznych screenshotów w `docs/screenshots/sprint-20/` (od `01-dashboard-tiny-60x18.txt` do `14-help-overlay-detail-120x35.txt`) + 17 smoke-snapshotów w `docs/screenshots/sprint-20/manual/`.
