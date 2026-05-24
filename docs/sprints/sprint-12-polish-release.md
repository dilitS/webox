# Sprint 12 — Responsive Cockpit Polish & Overflow Ergonomics

> **Daty:** 2026-05-25 → 2026-06-01 (planowane 1 tydzień solo) · **Czas:** ~18-26h skupienia
>
> **Cel:** dopiąć ergonomię cockpitu po Sprincie 11. Sprint 12 skupia się na tym, jak interfejs zachowuje się w prawdziwym terminalu, a nie tylko jak wygląda na idealnym screenshotcie: gdy frame nie mieści się w oknie ma dać się przewijać, szerokości kart mają adaptować się do viewportu, `🌐 [Live Service Topology]` ma siedzieć pod `📂 [Active Projects]` po lewej stronie `🚀 [CI/CD PIPELINE]`, a pozostałe ekrany mają mieć ten sam chrome/styling co główny cockpit. Release-hardening (`rc1`, signing, release smoke-test) zostaje przesunięte do Sprintu 13 po retrospekcji, żeby nie mieszać ergonomii UI z cięciem release'a.

---

## TL;DR

Po sprincie 12:

- Cockpit dostaje **viewport scroll**: jeśli wyrenderowana ramka ma więcej linii niż aktualna wysokość terminala, operator może przewijać `PgUp`, `PgDn`, `Home`, `End`.
- Layout Ultra (`120×35`) zmienia strukturę z:
  - `Projects | Server+CI/CD`
  - `Logs`
  - `Topology`
  na:
  - `Projects | Server`
  - `Topology | CI/CD`
  - `Logs`
- Kolumny nie używają już jednego sztywnego ratio; szerokości reagują na zakres viewportu (`120-135`, `136-159`, `>=160`) i clampują się do minimów, żeby nie ucinać nazw projektów ani grafu topologii.
- Standard Cockpit (`100×30`) dostaje sekcję `Connections:` w `Overview` — tabelaryczny fallback grafu topologii z Sprintu 11.
- Wszystkie kluczowe ekrany (Init Wizard, Project Wizard, Resume Wizard, Import Preview, Project Detail, Live Logs) używają tego samego chrome: status bar, footer hints, grube ramki, ten sam język tytułów.
- Snapshoty pokazują nie tylko "ładny idealny ekran", ale też zachowanie przy overflow i przy Standard vs Ultra.

**Nie robimy w tym sprincie:**

- `v0.1.0-rc1`, signing, release smoke-test — przeniesione do Sprintu 13.
- Drugi provider (cPanel / DirectAdmin / CyberPanel) — sprint 13.
- OAuth Device Flow — sprint 13.
- DB leaf w topologii — dalej czeka na `config.Project.DB*` w v0.2.

---

## Pre-flight Checklist

- [x] Sprint 11 zamknięty z Outcome (2026-05-24).
- [ ] Re-read [UX §3.4](../UX.md#34-wizualny-graf-topologii-us%C5%82ug-live-service-topology-map), [UX §4.1-§4.3](../UX.md#4-layouty-ekran%C3%B3w-20), [DESIGN §2.3](../DESIGN.md#23-zasady-przep%C5%82ywu-danych-mvu).
- [ ] `make ci` green na `main`.
- [ ] Snapshot baseline z Sprintu 11 odtworzony lokalnie (`WEBOX_SNAPSHOT=1 go test ./tui -run TestCockpitSnapshots`).
- [ ] Potwierdzone, że zmiana nie wychodzi poza MVP scope (to ergonomia istniejących ekranów, nie nowy feature area).

---

## Taski

### TASK-12.1 — Viewport scroll dla overflowing screens

- **Estymata:** M
- **Zależności:** none
- **Acceptance Criteria:**
  - [ ] Jeśli wyrenderowany frame ma więcej linii niż `WindowSizeMsg.Height`, `View()` zwraca tylko aktualny wycinek viewportu zamiast przepychać terminal history.
  - [ ] Operator może przewijać `PgUp`, `PgDn`, `Home`, `End`.
  - [ ] Scroll działa dla dashboardu, init wizarda, project detail, project wizarda, resume/import preview oraz live logs (scroll całej ramki; wewnętrzny scroll logów zostaje na `↑/↓`).
  - [ ] Test jednostkowy potwierdza, że przy overflow liczba renderowanych linii == wysokość viewportu oraz że `PgDn` zmienia zawartość.
- **Docs:** [DESIGN §2.3](../DESIGN.md#23-zasady-przep%C5%82ywu-danych-mvu), [UX §4](../UX.md#4-layouty-ekran%C3%B3w-20).

### TASK-12.2 — Responsive Bento widths + left-column topology

- **Estymata:** M
- **Zależności:** TASK-12.1
- **Acceptance Criteria:**
  - [ ] Ultra layout zmienia kompozycję na:
    ```text
    StatusBar
    [Projects]  | [Server]
    [Topology]  | [CI/CD]
    [Logs................. full width .................]
    ```
  - [ ] `🌐 [Live Service Topology]` renderuje się pod `📂 [Active Projects]` i po lewej od `🚀 [CI/CD PIPELINE: Main Branch]`.
  - [ ] Width ratio jest adaptacyjne (nie jeden sztywny `36/64`) i clampowane minimami, żeby nie ucinać najdłuższych nazw demo projektów oraz grafu topologii.
  - [ ] Test layoutu sprawdza, że na tej samej linii `Live Service Topology` pojawia się przed `CI/CD PIPELINE`.
- **Docs:** [UX §3.4](../UX.md#34-wizualny-graf-topologii-us%C5%82ug-live-service-topology-map), [UX §4.2](../UX.md#42-dashboard-20--bento-box-grid-system-12035-mvp--16045-stretch).

### TASK-12.3 — Standard Cockpit `Connections:` fallback

- **Estymata:** S
- **Zależności:** TASK-12.2
- **Acceptance Criteria:**
  - [ ] Dla `100×30 ≤ viewport < 120×35` Overview renderuje sekcję:
    ```text
    Connections:
      GitHub → Server : ✓ Active (2h ago, success)
      Server → App    : ✓ Online (200 OK)
    ```
  - [ ] Producer reużywa `buildTopologySnapshot` — bez nowych źródeł danych.
  - [ ] Snapshot `100×30` zawiera `Connections:`.
- **Docs:** [UX §3.4](../UX.md#34-wizualny-graf-topologii-us%C5%82ug-live-service-topology-map), carry-over Sprint 11 TASK-11.4.

### TASK-12.4 — Cross-screen cockpit styling parity

- **Estymata:** M
- **Zależności:** TASK-12.1
- **Acceptance Criteria:**
  - [ ] Init Wizard, Project Wizard, Resume Wizard, Import Preview, Project Detail i Live Logs używają tego samego chrome co dashboard: status bar, footer hints, thick/double borders, ten sam język tytułów.
  - [ ] Snapshot / tests potwierdzają obecność brandu `WEBOX`, breadcrumba (`Init Wizard`, `Project Wizard`, ... ) i footer hint strip.
  - [ ] Jeśli ekran nie mieści się pionowo, viewport scroll działa tam tak samo jak na dashboardzie.
- **Docs:** [UX §4.1-§4.3](../UX.md#4-layouty-ekran%C3%B3w-20).

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| Globalny viewport scroll pogryzie się z istniejącą nawigacją (`↑/↓` wybór projektu, `↑/↓` logs buffer, `↑/↓` modal workflow logs) | H | Scroll całej ramki tylko na `PgUp/PgDn/Home/End`; wewnętrzne `↑/↓` zostają bez zmian. |
| Zbyt agresywne ściskanie lewej kolumny popsuje czytelność topologii | M | Adaptive ratios + minima per column; snapshot tests dla `120×35`, `140×40`, `160×45`. |
| Ujednolicenie chrome na małych ekranach pogorszy ciasne terminale | M | Tiny fallback (`<70×22`) zostaje bez zmian; dla większych ekranów overflow obsługuje viewport scroll. |
| Standard Cockpit zacznie dryfować od `buildTopologySnapshot` i będzie pokazywał inne stany niż kafelek topologii | M | `Connections:` budowane wyłącznie z `buildTopologySnapshot`, nie z oddzielnej logiki. |

---

## Dependencies signoff

Sprint 12 **nie dodaje** nowych zewnętrznych zależności. Do viewport scroll możemy użyć istniejących zależności (`bubbletea`, `bubbles`) albo własnego pure slicera — bez dotykania `go.mod`.

---

## Outcome (wypełnij po sprincie)

- ✅ Done:
  - viewport scroll (`PgUp` / `PgDn` / `Home` / `End`) dla overflowing screens
  - responsive Ultra grid `Projects | Server` / `Topology | CI/CD` / `Logs`
  - Standard Cockpit `Connections:` fallback zasilany z `buildTopologySnapshot`
  - cross-screen chrome parity + bracketed emoji titles na ekranach poza dashboardem
  - snapshot refresh (`docs/screenshots/*.txt`) + pełne `make ci`
- ⏭️ Carry-over → Sprint 13:
  - release tooling smoke-test
  - RC1 / GA tagging
  - formalny bug bash release'owy i okres obserwacji RC
  - benchmark/performance-budget formalizacja
- 📌 Decyzje:
  - viewport scroll dostał osobne klawisze, żeby nie psuć istniejącej nawigacji `↑/↓`
  - Standard fallback i Ultra tile współdzielą jedną semantykę topologii
  - Sprint 13 zostaje przepisany tak, by najpierw dowieźć release hardening, a dopiero potem wracać do foundation spikes
- 🧠 Surprises:
  - samo przeniesienie topologii do lewej kolumny nie wystarczyło; trzeba było jeszcze wyrównać wysokości wierszy, żeby grid czytał się jak prawdziwe `2x2`
  - `tea.WithAltScreen()` rozwiązuje host-terminal scroll, ale nie problem overflowu wewnątrz samej aplikacji
- 📊 Metryki:
  - Coverage post-sprint: 81.5%
  - Overflow scroll tests: 1 dedykowany regression test + pełne `go test ./tui/...`
  - Snapshot tiers updated: 5 files (`standard`, `ultra`, `ultra+`, `live logs`, `mock cockpit`)
- 🔒 Security validation:
  - [x] Zero nowych network calls.
  - [x] `go test -race ./...` green via `make ci`.
  - [x] No secret leakage in scrollable/rendered surfaces.
- ➡️ Następny sprint: `sprint-13-v01-ga-and-post-mvp-foundation.md` (release hardening + post-MVP foundation).

---

## Retro Link

`docs/retros/2026-05-24-sprint-12.md`
