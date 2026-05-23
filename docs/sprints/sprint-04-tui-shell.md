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

- [ ] Sprint 03 zamknięty z retro i `Outcome`.
- [ ] Re-read `docs/UX.md §2-§5` (design system + layouty).
- [ ] Re-read `docs/DESIGN.md §2.2-§2.3`, `§12`, `§16.2`.
- [ ] Re-read `docs/adr/0001-tui-zamiast-cli.md`.
- [ ] Verify `bubbletea` / `lipgloss` / `bubbles` ostatnie stabilne wersje przez Context7.
- [ ] Confirm `make ci` green on `main`.

---

## Taski

### TASK-04.1 — Add `bubbletea` / `lipgloss` / `bubbles` dependencies

- **Estymata:** S
- **Zależności:** Sprint 03 done
- **Acceptance Criteria:**
  - [ ] `go.mod` zawiera `github.com/charmbracelet/bubbletea`, `lipgloss`,
        `bubbles` przypięte do konkretnych tag'ów (nie `main`).
  - [ ] `go.mod` zawiera `github.com/charmbracelet/x/exp/teatest` na
        pinowanym commicie (test-only dependency).
  - [ ] `make tidy` + `make vulncheck` zielone — żaden z dep'ów nie
        odpala znanej luki.
  - [ ] `cmd/webox/run.go` start'uje pusty Bubble Tea program i kończy
        czysto na `q`. Snapshot test `cmd/webox/run_test.go` przechodzi.
  - [ ] CHANGELOG entry w sekcji **Added** + uzasadnienie wyboru
        wersji w body PR-a.
- **Docs:** `AGENTS.md §1.2` (deps), `DESIGN.md §2.2`.

### TASK-04.2 — Define `tui.Model`, states, messages

- **Estymata:** M
- **Zależności:** TASK-04.1
- **Acceptance Criteria:**
  - [ ] `tui/states.go` enumeruje `State` (InitWizard, Dashboard,
        ProjectDetail, CommandPalette, ConfirmDialog) i `DetailTab`
        (Overview tylko jako MVP-enabled; pozostałe disabled).
  - [ ] `tui/messages.go` definiuje wszystkie `tea.Msg` typy używane
        w sprincie (ConfigLoaded, StatusRefreshed, ResizeMsg, etc.).
  - [ ] `tui/model.go` definiuje pure `New(...)` i `Init() tea.Cmd`.
  - [ ] `Update(msg)` jest pure: brak `os.*`, `net.*`, `chan` operacji.
  - [ ] Tests: table-driven dla `Update` (input Msg → expected State).
- **Docs:** `DESIGN.md §2.3`, `§12`.

### TASK-04.3 — Lipgloss design tokens + theme

- **Estymata:** M
- **Zależności:** TASK-04.1
- **Acceptance Criteria:**
  - [ ] `tui/theme/theme.go` zawiera `Theme` struct z paletą OKLCH
        zgodną z `UX.md §2`. Single source of truth dla kolorów.
  - [ ] `tui/theme/styles.go` eksportuje styled `lipgloss.Style`
        instancje per komponent (Header, ProjectRow, StatusBadge,
        HelpHints, etc.).
  - [ ] Brak hardcoded ANSI escape w komponentach — wszystko przez
        `theme.Styles`.
  - [ ] Snapshot test smoke: renderuje każdy komponent na 80×24 i
        sprawdza brak nieoczekiwanych escape sequences.
- **Docs:** `UX.md §2`, `DESIGN.md §16.2`.

### TASK-04.4 — Init-wizard view (first-run detection)

- **Estymata:** M
- **Zależności:** TASK-04.2, TASK-04.3
- **Acceptance Criteria:**
  - [ ] `tui/views/init_wizard.go` renderuje pierwszy ekran z `UX.md
        §4.1` (System pre-requisites + SSH keypair box).
  - [ ] Stan `StateInitWizard` triggerowany przez `config.Load` zwrot
        `os.ErrNotExist`.
  - [ ] Klawisze: Tab / Shift+Tab nawigacja, Enter potwierdza, Esc
        wyjście.
  - [ ] **Brak** generowania klucza / wpisywania do panel'a w tym
        sprincie — Init Wizard tylko **wyświetla** stan systemowy.
        Generacja klucza / deploy do hosta to Sprint 05.
  - [ ] `teatest` snapshot dla 80×24 i 100×30.
- **Docs:** `UX.md §4.1`, `DESIGN.md §12`.

### TASK-04.5 — Dashboard view (read-only, SWR-backed)

- **Estymata:** L
- **Zależności:** TASK-04.2, TASK-04.3, TASK-04.4
- **Acceptance Criteria:**
  - [ ] `tui/views/dashboard.go` renderuje listę projektów po lewej +
        szczegóły wybranego projektu po prawej (Standard Cockpit
        100×30, patrz `UX.md §5`).
  - [ ] Każdy wiersz projektu pokazuje: domain, status badge
        (🟢/🟡/🔴/STALE), Node version.
  - [ ] Panel szczegółów (Overview tab tylko): HTTP health, SSL
        days-left, Node version, last deploy (placeholder w MVP — pole
        wypełniane w Sprint 06 przez GH API).
  - [ ] Dane przychodzą jako `StatusRefreshedMsg` po `tea.Tick`
        co 30 s (TTL z `status/ttl.go`).
  - [ ] Klawisze: `↑`/`↓` selekcja, `→`/`Tab` enter ProjectDetail,
        `q` quit, `?` help (overlay).
  - [ ] `teatest` snapshot z mock danymi (2 projekty: ONLINE +
        STALE).
- **Docs:** `UX.md §4.2-§5`, `DESIGN.md §8` (SWR), `services/httpcheck`.

### TASK-04.6 — Project Detail (Overview tab)

- **Estymata:** M
- **Zależności:** TASK-04.5
- **Acceptance Criteria:**
  - [ ] `tui/views/project_detail.go` renderuje Overview tab tylko
        (cards: ONLINE/OFFLINE, Node, SSL, deploy path, repo, last
        deploy).
  - [ ] Pozostałe taby (`[2] Env Diff`, `[3] Database`, `[4] Logs`)
        wyświetlone jako **disabled** z dimmed indicator
        "unlocked in v0.2".
  - [ ] Klawisze: `←`/`Esc` powrót do Dashboard, `r`/`s`/`v` widoczne
        ale **disabled** — bez wywołania akcji (alert "available in
        Sprint 05+").
  - [ ] `teatest` snapshot dla wybranego projektu.
- **Docs:** `UX.md §4.3`, `DESIGN.md §12`.

### TASK-04.7 — Background refresh tickers + cancel on quit

- **Estymata:** M
- **Zależności:** TASK-04.5
- **Acceptance Criteria:**
  - [ ] `tea.Tick` co 30 s wywołuje status refresh przez `status.GetOrFetch`
        dla aktualnie widocznych projektów (visible-only fetch).
  - [ ] Background refresh używa `services/httpcheck.ProbeHTTP` +
        `ProbeTLS` (już istnieją w Sprint 02).
  - [ ] `q` cancel'uje wszystkie in-flight operacje przez
        `context.CancelFunc` trzymaną w modelu.
  - [ ] Brak goroutine leak'a — test sprawdza
        `goleak.VerifyNone(t, ...)`.
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

## Outcome (wypełnij po sprincie)

- ✅ Done: TASK-04.X, ...
- ⏭️ Carry-over: ...
- 📌 Decyzje: ...
- 🧠 Surprises: ...
- 📊 Metryki:
  - Coverage `tui/`: %
  - Coverage `tui/views/`: %
  - Snapshot count: %
- 🔒 Security validation:
  - [ ] Brak sekretów w żadnym renderowanym widoku.
  - [ ] `webox.log` (debug) nie zawiera kluczy / hostów po redaktorze.
  - [ ] `go test -race ./tui/... ./cmd/webox/...` green.
- ➡️ Następny sprint: `sprint-05-wizard-project.md`

---

## Retro Link

`docs/retros/YYYY-MM-DD-sprint-04.md`
