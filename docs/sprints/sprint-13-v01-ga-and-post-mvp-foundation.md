# Sprint 13 — RC1 Hardening, GA Stabilization & v0.2 Shaping

> **Daty:** 2026-06-02 → 2026-06-15 (planowane 2 tygodnie solo) · **Czas:** ~28-36h skupienia
>
> **Cel:** zamienić dopracowany cockpit ze Sprintu 12 w kandydat do wydania. Sprint 13 ma jeden priorytet: **dowiezienie `v0.1.0-rc1`, obserwacja RC i decyzja o GA**. Wszelkie foundation/spike rzeczy pod v0.2 są tu tylko lekkim backlog shapingiem, nie pełną implementacją. To jest bezpośredni wniosek z retro Sprintu 12: nie mieszamy release hardening i dużych eksperymentów produktowo-architektonicznych w jednym sprincie.

---

## TL;DR

Po sprincie 13:

- **`v0.1.0-rc1` istnieje**: changelog złożony, artefakty zbudowane, smoke-test release tooling zielony, bug bash zakończony.
- **Okres obserwacji RC** jest opisany i przeprowadzony wewnątrz sprintu: wszystkie znalezione bugi mają albo fix + test regresji, albo świadomą decyzję o blokadzie GA.
- **`v0.1.0` GA** wychodzi tylko jeśli RC1 pozostaje bez P0/P1 blockerów. Jeśli nie, sprint kończy się na stabilnym RC1 i świadomym przesunięciu GA.
- **Sprint 14 backlog note** porządkuje kolejność większych tematów v0.2: second-provider research, OAuth Device Flow PoC, `config.json` schema v3 / DB fields, ADR dla DAG layout deferral.

**Nie robimy w tym sprincie:**

- pełnej implementacji drugiego providera
- OAuth Device Flow PoC w kodzie
- `config.json` schema v3
- generic DAG layout engine

Te rzeczy wracają dopiero po stabilizacji release path.

---

## Pre-flight Checklist

- [ ] Sprint 12 zakończony z zielonym `make ci`.
- [ ] `docs/retros/2026-05-24-sprint-12.md` przeczytane; scope split "ergonomia vs release" zaakceptowany.
- [ ] Re-read [ROADMAP §3.1](../ROADMAP.md), [PRD §6 priorytety](../PRD.md), [SECURITY §10](../SECURITY.md), [TESTING §3](../TESTING.md).
- [ ] Brak otwartych driftów między `docs/UX.md` a snapshotami cockpitowymi.

---

## Taski

### TASK-13.1 — Release hardening bug bash

- **Estymata:** M
- **Zależności:** none
- **Acceptance Criteria:**
  - [ ] Przejść 10 scenariuszy operatora dla `--mock` i podstawowego flow bez serwerów:
    1. init wizard
    2. dashboard standard
    3. dashboard ultra
    4. viewport scroll on overflow
    5. project detail overview
    6. live logs tab
    7. import preview
    8. resume wizard
    9. CI/CD modal
    10. tiny fallback
  - [ ] Każdy wykryty bug ma issue / fix / decyzję "not blocking RC".
  - [ ] Snapshoty i golden files odzwierciedlają finalny release-candidate UI.
- **Docs:** [UX §4](../UX.md#4-layouty-ekran%C3%B3w-20), [TESTING §3](../TESTING.md).

### TASK-13.2 — Release tooling smoke-test + baseline

- **Estymata:** M
- **Zależności:** TASK-13.1
- **Acceptance Criteria:**
  - [ ] `make release-dry-run` przechodzi lokalnie bez warningów blokujących release.
  - [ ] `goreleaser check` green.
  - [ ] Manualny smoke-test binarki: `webox --version`, `webox doctor --json`, `webox --mock`.
  - [ ] Powstaje lekki zapis release baseline (`docs/perf/release-baseline.md` albo równoważny note) z czasem renderu/mock boot i rozmiarem artefaktów.
- **Docs:** [DESIGN §14](../DESIGN.md#14-dystrybucja-i-mechanizm-sprawdzania-wersji).

### TASK-13.3 — CHANGELOG + tag `v0.1.0-rc1`

- **Estymata:** S
- **Zależności:** TASK-13.2
- **Acceptance Criteria:**
  - [ ] `CHANGELOG.md` ma sekcję `[v0.1.0-rc1]`.
  - [ ] `[Unreleased]` zawiera tylko pracę po RC.
  - [ ] Annotated tag `v0.1.0-rc1` powstaje dopiero na zielonym CI.
  - [ ] GitHub Release zostaje przygotowany jako draft.
- **Docs:** [ROADMAP §3.1](../ROADMAP.md).

### TASK-13.4 — RC1 observation window + regression fixes

- **Estymata:** M
- **Zależności:** TASK-13.3
- **Acceptance Criteria:**
  - [ ] Każdy bug znaleziony po RC1 ma klasyfikację severity (`P0/P1/P2`).
  - [ ] Każdy fix blokujący GA ma test regresji.
  - [ ] Na koniec okna obserwacji istnieje jawna decyzja: `GA now` albo `hold`.
- **Docs:** [TESTING §3](../TESTING.md), `docs/retros/2026-05-24-sprint-12.md`.

### TASK-13.5 — `v0.1.0` GA decision gate

- **Estymata:** S
- **Zależności:** TASK-13.4
- **Acceptance Criteria:**
  - [ ] Jeśli brak P0/P1 blockerów, powstaje tag `v0.1.0` i publikowany release.
  - [ ] Jeśli są blockery, sprint kończy się na stabilnym RC1 + spisanej decyzji o przesunięciu GA.
  - [ ] Niezależnie od wyniku, decyzja i rationale są wpisane do `Outcome`.
- **Docs:** [ROADMAP §3.1](../ROADMAP.md).

### TASK-13.7 — Chrome contract + responsive Bento (DONE 2026-05-24)

- **Estymata:** M (delivered)
- **Zależności:** TASK-13.1 (bug bash exposed the gaps)
- **Origin:** RC bug bash uncovered three viewport regressions: (1) full-frame overflow scrolled host terminal history instead of staying inside the TUI, (2) right column tiles wasted vertical space when their left siblings were shorter, (3) Live Service Topology pushed sibling tiles down because both its border weight and the edge connectors were heavier than necessary.
- **Outcome:**
  - `tui/view.go` now composes every surface in three pinned slots (top chrome / scrollable body / bottom chrome). The dashboard reuses the bento status bar via `WithStatusBar`; every other surface gets a pinned bar from `renderChromeTop`. Footer publishes a dynamic `↕ scroll: PgUp/PgDn · Home/End · Mouse · (offset/max)` hint when the body overflows.
  - `tui/update.go::updateMouse` enables mouse-wheel scrolling using the post-1.3 Bubble Tea API (`MouseActionPress` + `MouseButtonWheelUp / WheelDown`; step 3 lines).
  - `tui/bento/engine.go::planRowBudgets` carves explicit budgets per row; `clipTileBlock` keeps the tile frame intact and surfaces `┃ … +N more lines · scroll inside tab/modal ┃` (rendered via `framedIndicatorLine` so the side borders match the tile's accent colour and the cockpit frame stays geometrically clean).
  - `tui/components/asciigraph/asciigraph.go` swaps node borders to `lipgloss.NormalBorder()` (light boxes inside the heavy tile frame) and compacts the edge from 3 lines to 2 (label-on-glyph + arrow). Topology now matches CI/CD height; Server matches Active Projects height; the right column no longer wastes vertical real estate.
  - `tui/views/dashboard.go` (Standard Cockpit `100×30`) restyled to the bento grammar (bracketed emoji headers, rounded selection pills, thick borders) and lets `tui.View` pin the global status bar + footer hints around it.
  - Snapshots refreshed (`mock-cockpit-140x40.txt`, `sprint-08-standard-100x30.txt`, etc.) and golden tests updated to the new chrome contract.
  - Docs updated: `docs/UX.md` §4.2 + §6.2 (chrome contract + scroll keys + indicator); `docs/DESIGN.md` §2.4 (three-slot composition + bento height-budget algorithm).
- **Acceptance Criteria:**
  - [x] `make ci` zielony; coverage 81%.
  - [x] Snapshots reflect the new chrome contract.
  - [x] CHANGELOG `[Unreleased]` entry added under `Added`.
  - [x] UX.md + DESIGN.md updated.
  - [x] No linter regressions (`gocritic`, `mnd`, `staticcheck` — all clean).

### TASK-13.6 — v0.2 shaping note (docs only)

- **Estymata:** S
- **Zależności:** TASK-13.4
- **Acceptance Criteria:**
  - [ ] `docs/research/v02-shaping-note.md` ustala kolejność po release:
    1. second-provider research
    2. OAuth Device Flow PoC
    3. `config.json` schema v3 / DB topology wiring
    4. ADR dla generic DAG layout deferral / confirmation
  - [ ] Każdy temat ma status: `next`, `later`, `blocked by maintainer decision`.
  - [ ] Sprint 14 można zaplanować bez ponownego mieszania release work i eksperymentów.
- **Docs:** [CONTRIBUTING §3](../CONTRIBUTING.md#3-jak-doda%C4%87-nowy-provider), [SECURITY §6](../SECURITY.md), [DESIGN §6](../DESIGN.md#6-model-danych-i-atomowo%C5%9B%C4%87-zapisu-configjson), [DESIGN §18](../DESIGN.md#18-silnik-dynamicznej-topologii-live-service-topology-map-engine).

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| RC1 ujawni blocker w samym interaction modelu (scroll/layout/focus) | H | Sprint 13 ma osobny observation window task; nie udajemy, że RC = GA. |
| Release tooling okaże się bardziej kruche niż sam produkt | M | Smoke-test i draft release przed jakimkolwiek tagiem GA. |
| Pokusa "przy okazji zróbmy OAuth/config v3" rozmyje sprint | H | Scope lock: tylko docs-level shaping, zero implementation work dla v0.2 tematów. |
| Snapshoty i UX docs znowu się rozjadą po szybkich poprawkach RC | M | Każdy fix UI aktualizuje snapshot lub golden file w tym samym PR/commit. |

---

## Dependencies signoff

Sprint 13 **nie powinien** dodawać nowych zewnętrznych zależności. Jeśli release tooling wymaga nowego helpera, idzie przez maintainer sign-off.

---

## Outcome (wypełnij po sprincie)

- ✅ Done: ...
- ⏭️ Carry-over → Sprint 14: ...
- 📌 Decyzje:
  - RC1 cut: TAK / NIE
  - GA cut: TAK / NIE
  - v0.2 first topic: ___
- 🧠 Surprises: ...
- 📊 Metryki:
  - Coverage end-of-sprint: ?%
  - RC bugs found: ?
  - Release artefacts size: ? MB
- 🔒 Security validation:
  - [ ] `govulncheck` na RC/GA tag: 0 findings
  - [ ] No secrets leaked in release notes / logs / screenshots
  - [ ] Draft release artefacts map to the tagged commit SHA
- ➡️ Następny sprint: `sprint-14-v02-foundation.md` (do utworzenia po decyzji GA)

---

## Retro Link

`docs/retros/<data>-sprint-13.md` (do utworzenia po zakończeniu RC1/GA gate)
