# Sprint 04 — TUI Shell (MVU, Navigation, Read-only Dashboard)

> **Daty:** TBD → TBD (planowane 1-2 tygodnie solo) · **Czas:** ~35-50h skupienia
>
> **Cel:** wprowadzić Bubble Tea / Lipgloss, zbudować MVU shell aplikacji, zaimplementować nawigację między ekranami i statyczny dashboard wczytujący dane z `config/` + `status/` + `providers/`. **Nie ma jeszcze kreatora projektu, GitHub workflow ani deploya** — to Sprint 05+.

---

## TL;DR

Po sprincie 04:

- `cmd/webox` startuje Bubble Tea program zamiast obecnego stub'a.
- `tui/` ma `Model`, `Update`, `View` opisane w DESIGN §2.2 + §12 — pure functions, każde I/O w `tea.Cmd`.
- Nawigacja: Init wizard (jeśli brak `config.json`) → Dashboard (lista projektów) → Project Detail (Overview tab tylko).
- Dashboard ma read-only dane: dla każdego projektu HTTP health, SSL days-left, Node version, last deploy — wszystko cache'owane przez `status/` z SWR semantics.
- Snapshot testy przez `teatest` na trzech kluczowych widokach.
- Brak akcji modyfikujących stan (Restart / SSL renew / Deploy) — to Sprint 05+.

**Nie robimy w tym sprincie:**

- Kreatora projektu (wizard wielokrokowy, LIFO rollback): Sprint 05.
- GitHub workflow generation: Sprint 06.
- Command Palette `/create`, `/import`, `/settings`: Sprint 07.
- Drugi provider, env merger, live log stream: STRETCH v0.2+.
- Bento Ultra (≥120×35) — MVP target = Standard Cockpit (`100×30`).

---

## Pre-flight Checklist

- [x] Sprint 03 zamknięty z retro i `Outcome`.
- [x] Re-read `docs/UX.md §2-§5` (design system + layouty).
- [x] Re-read `docs/DESIGN.md §2.2-§2.3`, `§12`, `§16.2`.
- [x] Re-read `docs/adr/0001-tui-zamiast-cli.md`.
- [x] Verify `bubbletea` / `lipgloss` / `bubbles` ostatnie stabilne wersje przez Context7.
- [x] Confirm `make ci` green on `main`.

---

## Taski

### TASK-04.1 — Add `bubbletea` / `lipgloss` / `bubbles` dependencies

- **Estymata:** S
- **Zależności:** Sprint 03 done
- **Acceptance Criteria:**
  - [x] `go.mod` zawiera `github.com/charmbracelet/bubbletea`, `lipgloss`,
        `bubbles` przypięte do konkretnych tag'ów (nie `main`).
  - [x] `go.mod` zawiera `github.com/charmbracelet/x/exp/teatest` na
        pinowanym commicie (test-only dependency).
  - [x] `make tidy` + `make vulncheck` zielone — żaden z dep'ów nie
        odpala znanej luki.
  - [x] `cmd/webox/run.go` start'uje pusty Bubble Tea program i kończy
        czysto na `q`. Snapshot test `cmd/webox/run_test.go` przechodzi.
  - [x] CHANGELOG entry w sekcji **Added** + uzasadnienie wyboru
        wersji w body PR-a.
- **Docs:** `AGENTS.md §1.2` (deps), `DESIGN.md §2.2`.

### TASK-04.2 — Define `tui.Model`, states, messages

- **Estymata:** M
- **Zależności:** TASK-04.1
- **Acceptance Criteria:**
  - [x] `tui/states.go` enumeruje `State` (InitWizard, Dashboard,
        ProjectDetail, CommandPalette, ConfirmDialog) i `DetailTab`
        (Overview tylko jako MVP-enabled; pozostałe disabled).
  - [x] `tui/messages.go` definiuje wszystkie `tea.Msg` typy używane
        w sprincie (ConfigLoaded, StatusRefreshed, ResizeMsg, etc.).
  - [x] `tui/model.go` definiuje pure `New(...)` i `Init() tea.Cmd`.
  - [x] `Update(msg)` jest pure: brak `os.*`, `net.*`, `chan` operacji.
  - [x] Tests: table-driven dla `Update` (input Msg → expected State).
- **Docs:** `DESIGN.md §2.3`, `§12`.

### TASK-04.3 — Lipgloss design tokens + theme

- **Estymata:** M
- **Zależności:** TASK-04.1
- **Acceptance Criteria:**
  - [x] `tui/theme/theme.go` zawiera `Theme` struct z paletą OKLCH
        zgodną z `UX.md §2`. Single source of truth dla kolorów.
  - [x] `tui/theme/styles.go` eksportuje styled `lipgloss.Style`
        instancje per komponent (Header, ProjectRow, StatusBadge,
        HelpHints, etc.).
  - [x] Brak hardcoded ANSI escape w komponentach — wszystko przez
        `theme.Styles`.
  - [x] Snapshot test smoke: renderuje każdy komponent na 80×24 i
        sprawdza brak nieoczekiwanych escape sequences.
- **Docs:** `UX.md §2`, `DESIGN.md §16.2`.

### TASK-04.4 — Init-wizard view (first-run detection)

- **Estymata:** M
- **Zależności:** TASK-04.2, TASK-04.3
- **Acceptance Criteria:**
  - [x] `tui/views/init_wizard.go` renderuje pierwszy ekran z `UX.md
        §4.1` (System pre-requisites + SSH keypair box).
  - [x] Stan `StateInitWizard` triggerowany przez detekcję
        nieobecnego `config.json` na poziomie TUI load command (patrz
        Outcome: settled `config.Load` zwraca `DefaultConfig()` dla
        brakującego pliku).
  - [x] Klawisze: Tab / Shift+Tab nawigacja, Enter potwierdza, Esc
        wyjście.
  - [x] **Brak** generowania klucza / wpisywania do panel'a w tym
        sprincie — Init Wizard tylko **wyświetla** stan systemowy.
        Generacja klucza / deploy do hosta to Sprint 05.
  - [x] `teatest` smoke (`TestTeatestSmokeDashboardSnapshot`) +
        deterministyczne render testy dla 80×24 i 100×30 widoków.
- **Docs:** `UX.md §4.1`, `DESIGN.md §12`.

### TASK-04.5 — Dashboard view (read-only, SWR-backed)

- **Estymata:** L
- **Zależności:** TASK-04.2, TASK-04.3, TASK-04.4
- **Acceptance Criteria:**
  - [x] `tui/views/dashboard.go` renderuje listę projektów po lewej +
        szczegóły wybranego projektu po prawej (Standard Cockpit
        100×30, patrz `UX.md §5`).
  - [x] Każdy wiersz projektu pokazuje: domain, status badge
        (ASCII: ONLINE/OFFLINE/STALE/UNKNOWN; emoji/Nerd Font layer
        zostaje w Sprint 07+ polish), Node version.
  - [x] Panel szczegółów (Overview tab tylko): HTTP health, SSL
        days-left, Node version, last deploy (placeholder w MVP — pole
        wypełniane w Sprint 06 przez GH API).
  - [x] Dane przychodzą jako `StatusRefreshedMsg` po `tea.Tick`
        co 30 s (TTL z `status/ttl.go`).
  - [x] Klawisze: `↑`/`↓` selekcja, `→`/`Tab` enter ProjectDetail,
        `q` quit, `?` help (overlay).
  - [x] `teatest` smoke z fixture configu (2 projekty); render test
        `dashboard` w `view_test.go` weryfikuje ONLINE + STALE.
- **Docs:** `UX.md §4.2-§5`, `DESIGN.md §8` (SWR), `services/httpcheck`.

### TASK-04.6 — Project Detail (Overview tab)

- **Estymata:** M
- **Zależności:** TASK-04.5
- **Acceptance Criteria:**
  - [x] `tui/views/project_detail.go` renderuje Overview tab tylko
        (cards: ONLINE/OFFLINE, Node, SSL, deploy path, repo, last
        deploy).
  - [x] Pozostałe taby (`[2] Env Diff`, `[3] Database`, `[4] Logs`)
        wyświetlone jako **disabled** z dimmed indicator
        "unlocked in v0.2".
  - [x] Klawisze: `←`/`Esc` powrót do Dashboard, `r`/`s`/`v` widoczne
        ale **disabled** — bez wywołania akcji (alert "available in
        Sprint 05+").
  - [x] Render test `project detail overview` pokrywa wybrany
        projekt; teatest smoke pokrywa wejście do detalu z dashboardu.
- **Docs:** `UX.md §4.3`, `DESIGN.md §12`.

### TASK-04.7 — Background refresh tickers + cancel on quit

- **Estymata:** M
- **Zależności:** TASK-04.5
- **Acceptance Criteria:**
  - [x] `tea.Tick` co 30 s wywołuje status refresh przez
        `status.GetOrFetchMeta` dla aktualnie widocznych projektów
        (visible-only fetch).
  - [x] Background refresh używa `services/httpcheck.ProbeHTTP` +
        `ProbeTLS` (już istnieją w Sprint 02).
  - [x] `q` cancel'uje wszystkie in-flight operacje przez
        `context.CancelFunc` trzymaną w modelu.
  - [x] Brak goroutine leak'a — `TestQuitTransitionDoesNotLeakGoroutines`
        owija `goleak.VerifyNone(t, ...)` wokół pure quit
        transition; teatest harness ma własną signal goroutine i jest
        wyłączony spod goleak świadomie.
- **Docs:** `DESIGN.md §2.3`, `§9`.

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| Bubble Tea API drift między wersjami | M | Pin do konkretnego tag'a, snapshot testy szybko wykrywają breaking change. |
| `teatest` jest eksperymentalny — możliwe wycofanie | M | Commit hash w `go.mod`, nie `latest`. Patrz AGENTS.md §1.2. |
| Snapshot tests false-positives przez OS-specific ANSI | M | `teatest` strip'uje ANSI w `golden.txt`. Kolory walidujemy manualnie pre-release. |
| Scope creep w wizard tworzenia projektu | H | Sprint 04 zatrzymuje się na read-only dashboard. Każda akcja modyfikująca = automatic reject. |
| Layout breakage przy 80×24 (minimum z PRD §10.3) | M | Snapshot testy uruchamiane na 80×24, 100×30, 120×35. |

---

## Dependencies signoff

TASK-04.1 dodaje 4 nowe direct dependencies. Każda wymaga
uzasadnienia w PR + sign-off maintainera (AGENTS.md §1.2):

1. `github.com/charmbracelet/bubbletea` — MVU framework. **Bez
   alternatywy** w Go ecosystem dla tej klasy TUI.
2. `github.com/charmbracelet/lipgloss` — declarative styling. Część
   tej samej rodziny co bubbletea, użycie razem jest standardem.
3. `github.com/charmbracelet/bubbles` — gotowe komponenty (spinner,
   textinput). Alternatywa: ręczna implementacja każdego komponentu.
4. `github.com/charmbracelet/x/exp/teatest` — testing harness.
   **Eksperymentalny**, pinujemy commit hash. Alternatywa:
   manualne snapshot'y `bytes.Buffer` (więcej boilerplate).

---

## Outcome (wypełnione 2026-05-23)

- ✅ Done: TASK-04.1, TASK-04.2, TASK-04.3, TASK-04.4, TASK-04.5,
  TASK-04.6, TASK-04.7.
- ⏭️ Carry-over:
  - Snapshot coverage is smoke-level, not golden-file strict. `teatest`
    proves the Bubble Tea harness boots and renders the dashboard, but the
    current assertions are substring-based to avoid baking unstable ANSI
    teardown output into goldens too early.
  - `StateInitWizard` first-run detection is implemented in the TUI load
    command by checking the config path before `config.Load`. This preserves
    Sprint 01's contract where `config.Load` returns `DefaultConfig()` for a
    missing file instead of changing a settled config API.
  - Dashboard status refresh is read-only and uses HTTP/TLS probes plus
    project metadata for Node version. Real SSH `node --version` fan-out is
    intentionally deferred until wizard/profile flows provide a selected
    provider session in Sprint 05+.
- 📌 Decyzje:
  - Keep the Charm imports on the `github.com/charmbracelet/...` v1 module
    line (`bubbletea v1.3.10`, `lipgloss v1.1.0`, `bubbles v1.0.0`) because
    repo docs and rules explicitly name those import paths. Context7 shows
    v2 examples under `charm.land/...`; switching paths would be an
    architectural dependency migration, not a Sprint 04 bootstrap.
  - Add `go.uber.org/goleak` as a direct test dependency because TASK-04.7
    requires a leak assertion. The teatest harness itself leaves an internal
    signal goroutine, so leak coverage is attached to the pure quit
    transition rather than the teatest smoke.
  - Use ASCII status labels (`ONLINE`, `OFFLINE`, `STALE`) in the initial
    implementation. The UX emoji/nerd-font layer remains design intent; the
    first shell prioritizes deterministic tests and terminal portability.
- 🧠 Surprises:
  - `teatest.FinalOutput` mostly returns Bubble Tea teardown control
    sequences after quitting; the useful screen snapshot must be captured
    from `tm.Output()` while the program is still running.
  - `config.Load` missing-file semantics conflict with the literal wording
    of TASK-04.4. Preserving the older config invariant is safer than
    changing a public package contract to satisfy a sprint note.
  - Pulling `teatest` by pseudo-version also pulls a newer
    `github.com/charmbracelet/x` pseudo-module. This is acceptable, but it
    makes `make vulncheck` and future dependency review more important.
- 📊 Metryki:
  - Coverage `tui/`: 54.8%.
  - Coverage `tui/theme/`: 83.3%.
  - Coverage `tui/views/`: 0.0% package-level; render helpers are covered
    indirectly through `tui.View()` tests, but this remains below the
    package target and needs direct view tests before release hardening.
  - Global coverage after Sprint 04: 79.6%.
  - Snapshot count: 1 teatest smoke (`TestTeatestSmokeDashboardSnapshot`) +
    3 pure render view cases.
- 🔒 Security validation:
  - [x] Brak sekretów w renderowanych widokach Sprint 04 (only metadata:
        domains, repo slug, Node version, status placeholders).
  - [x] No debug log file wiring changed; `webox.log` is not written by the
        TUI shell yet.
  - [x] `go test -race ./tui/... ./cmd/webox` green.
  - [x] `make lint`, `make test`, `make vulncheck`, `make cover-check`, and
        `make build` green.
- ➡️ Następny sprint: [`sprint-05-wizard-project.md`](sprint-05-wizard-project.md)

---

## Retro Link

[`docs/retros/2026-05-23-sprint-04.md`](../retros/2026-05-23-sprint-04.md)
