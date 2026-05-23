# Sprint 11 — Live Service Topology Map

> **Daty:** TBD → TBD (planowane 1.5-2 tygodnie solo) · **Czas:** ~30-45h skupienia
>
> **Cel:** dostarczyć kafelek Bento Ultra `[Live Infrastructure Topology]` zgodnie z [UX §3.4](../UX.md#34-wizualny-graf-topologii-us%C5%82ug-live-service-topology-map): ASCII box-drawing graph `GitHub Repo ───(GHA Deploy)───▶ Production Server ─┬─▶ Subdomain ─(Proxy)─▶ Local Port :3000` z live edge states (ONLINE / BUILDING / OFFLINE / DEGRADED). Bazujemy na danych z `status/` (już zbieranych przez Sprint 02/04/09/10), nie wprowadzamy nowych źródeł danych — Sprint 11 to **layer wizualizacji** istniejących sygnałów.

---

## TL;DR

Po sprincie 11:

- `tui/bento/tiles/topology.go` implementuje `BentoTile` dla Topology Map.
- `tui/components/asciigraph/` — renderer ASCII box-drawing graphów z box-drawing characters (`┌─┐│└─┘├─┤` itp.).
- Edge states sterowane z `status.Cache` — ONLINE (Primary cienka linia + `✓`), BUILDING (Yellow pulsująca linia przerywana `═ ═ ═ ▶`), OFFLINE (Error gruba linia z `⚡ ⚡ ⚡` + badge `✗ DISCONNECTED`), DEGRADED (Degraded violet linia + `⚠`).
- Pulsation animation: edge animations re-render co 500ms (BUILDING) lub co 1000ms (OFFLINE) — context-cancellable, 60fps cap.
- Slot `Right` w `120×35` (wymaga `width≥120 && height≥35`); placeholder w `100×30` Standard fallback (tabelaryczna lista połączeń).
- Per-project topology — graph przebudowuje się przy switch projektu na dashboardzie.

**Nie robimy w tym sprincie:**

- Multi-project topology aggregator — STRETCH v0.2+ (wymaga `160×45` Bento Ultra+).
- Interactive node click (np. click MySQL node → otwórz `/db` view) — `/db` jest STRETCH v0.2+, więc nie ma celu.
- Animated edges przy SSL renew / deploy in-progress beyond simple `BUILDING` state — STRETCH v0.2+.
- Export topology as PNG/SVG — out of scope (Webox to TUI, nie raporter).

---

## Pre-flight Checklist

- [ ] Sprint 10 zamknięty z retro i `Outcome`.
- [ ] Re-read [UX §3.4](../UX.md#34-wizualny-graf-topologii-us%C5%82ug-live-service-topology-map), [UX §4.2](../UX.md#42-dashboard-20--bento-box-grid-system-12035-mvp--16045-stretch).
- [ ] Confirm `status/` cache zawiera już wszystkie potrzebne sygnały: HTTP health, SSL valid, Node version, GHA last run, SSH connectivity, DB connectivity (DB → opcjonalnie, projekt może nie mieć DB).
- [ ] Confirm `make ci` green on `main` after Sprint 10 merge.

---

## Taski

### TASK-11.1 — `tui/components/asciigraph/` renderer

- **Estymata:** L
- **Zależności:** Sprint 10 done
- **Acceptance Criteria:**
  - [ ] `tui/components/asciigraph/render.go` exposes `Render(g Graph, width int) string`.
  - [ ] `Graph{Nodes []Node, Edges []Edge}`; `Node{ID, Label, State (Online/Offline/Building/Degraded)}`; `Edge{From, To string, Label string, State EdgeState}`.
  - [ ] Renderer używa box-drawing characters: `┌─┐│└─┘├┤┬┴┼` dla node boxes, `───`/`═══` dla edges (single line = ONLINE; double line = ERROR; dashed `═ ═ ═` = BUILDING).
  - [ ] Layout algorithm: hard-coded layout dla MVP (3-level tree: GitHub → Server → {Subdomain, DB}). General DAG layout jest **out of scope** — Sprint 11 wystarcza dla typowego małego projektu.
  - [ ] State → kolor zgodnie z paletą OKLCH (Primary/Success/Yellow/Red/Violet).
  - [ ] Tests: golden snapshot dla 4 stanów (all-online, one-offline, building, degraded).
- **Docs:** [UX §3.4](../UX.md#34-wizualny-graf-topologii-us%C5%82ug-live-service-topology-map).

### TASK-11.2 — Topology graph builder z `status.Cache` + project config

- **Estymata:** M
- **Zależności:** TASK-11.1
- **Acceptance Criteria:**
  - [ ] `tui/bento/tiles/topology.go` ma `BuildGraph(project config.Project, statuses status.Snapshot) Graph`.
  - [ ] Standardowa topologia per projekt:
    - Node `gh-repo` (Label: `<owner>/<repo>`, State: ONLINE jeśli repo readable, OFFLINE jeśli `gh` auth błąd lub repo nie istnieje).
    - Edge `gh-repo → server` (Label: `GHA Deploy`, State: BUILDING jeśli aktywny run, ONLINE jeśli last run success, OFFLINE jeśli last run failed).
    - Node `server` (Label: `<profile.Host>`, State: ONLINE jeśli SSH probe OK, OFFLINE jeśli SSH error).
    - Edge `server → subdomain` (Label: `Proxy`, State: ONLINE jeśli HTTP 200/3xx, DEGRADED jeśli SSL <14 days lub HTTP 4xx, OFFLINE jeśli HTTP 5xx/timeout).
    - Node `subdomain` (Label: `<project.Domain>`, State: matches HTTP/SSL combined).
    - Opcjonalnie Node `db` + Edge `server → db` (Label: `MySQL Tunnel`/`Postgres Tunnel`, State: ONLINE jeśli `services/sshdb.Probe` OK, OFFLINE w przeciwnym razie).
  - [ ] Project bez DB → graph bez DB node/edge.
  - [ ] Tests: builder dla różnych project configs + statuses combinations.
- **Docs:** [UX §3.4](../UX.md#34-wizualny-graf-topologii-us%C5%82ug-live-service-topology-map).

### TASK-11.3 — Topology tile z live animations

- **Estymata:** M
- **Zależności:** TASK-11.1, TASK-11.2
- **Acceptance Criteria:**
  - [ ] `tui/bento/tiles/topology.go` implementuje `BentoTile`; slot `Right` (lub `BottomRight` w `160×45`), MinSize `(60, 14)`.
  - [ ] Live state aktualizowany przez `status.Cache` (no new API calls).
  - [ ] BUILDING edges pulsują (toggle styl co 500ms — `═ ═ ═` ↔ ` ═ ═ ═`).
  - [ ] OFFLINE edges pulsują (toggle co 1000ms — `⚡ ⚡ ⚡` ↔ `   ⚡   `).
  - [ ] 60fps render cap (jak w live logs).
  - [ ] Switch projektu na dashboardzie → graph re-build (nie cancel/restart streamów, tylko re-render z istniejących statuses).
  - [ ] Tests: teatest scenariusz dashboard switch projektu + graph re-render.

### TASK-11.4 — Standard Cockpit fallback (tabelaryczna lista)

- **Estymata:** S
- **Zależności:** TASK-11.3
- **Acceptance Criteria:**
  - [ ] Dla `width<120 || height<35` (Standard Cockpit) Topology tile nie jest renderowany; zamiast tego `views.RenderDashboard` `Overview` ma sekcję `Connections:` z tabelaryczną listą:
    ```
    Connections:
      GitHub → Server : ✓ Active (2h ago, success)
      Server → App    : ✓ Online (200 OK, 88ms)
      Server → MySQL  : ✓ Connected
    ```
  - [ ] Tests: snapshot dashboard 100×30 zawiera `Connections:` section.

### TASK-11.5 — Performance i goroutine hygiene

- **Estymata:** S
- **Zależności:** TASK-11.3
- **Acceptance Criteria:**
  - [ ] Topology tile re-render < 5ms na M-series Mac (benchmark).
  - [ ] Pulsation timer cancellable na `q`/`Esc` + cancel propagation przez context.
  - [ ] `goleak.VerifyNone` w teście quit transition.
  - [ ] CPU usage: dashboardize z all-tiles aktywnymi (CI/CD + logs + topology + header) < 8% na M-series Mac przy 1 active project.

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| Box-drawing characters nie renderują w niektórych terminalach (no UTF-8) | M | Fallback ASCII-only graph (`+--+`, `|`, `+`); auto-detect via `os.Getenv("LANG")` lub `--no-utf8` flag. |
| Layout algorithm fragile dla projektów z >3 services | M | MVP wspiera tylko hard-coded 3-level tree; bigger graphs → fallback do tabular listy. v0.2+ ma plan na DAG layout. |
| Pulsation animations rozpraszają user'a | M | `f` w project detail toggle animations on/off; setting persisted w `config.json`. |
| Real-time state inconsistency między tilemi (CI/CD pokazuje green, Topology pokazuje OFFLINE) | M | Wspólny `status.Cache` jako single source of truth; oba tile czytają z tego samego cache w tej samej klatce. |
| Edge between Server i DB faktycznie nie jest live (DB tunnel ad-hoc) | M | DB state pobierany tylko gdy `project.DB != nil`; brak DB → brak edge. |

---

## Dependencies signoff

Sprint 11 **nie dodaje** nowych zewnętrznych zależności.

---

## Outcome (wypełnij po sprincie)

- ✅ Done: ...
- ⏭️ Carry-over: ...
- 📌 Decyzje: ...
- 🧠 Surprises: ...
- 📊 Metryki:
  - Coverage `tui/components/asciigraph/`: ?
  - Coverage `tui/bento/tiles/topology.go`: ?
  - Render benchmark (ms): ?
  - All-tiles CPU%: ?
- 🔒 Security validation:
  - [ ] Brak nowych network calls (wszystko z `status.Cache`).
  - [ ] `go test -race ./tui/...` green.
- ➡️ Następny sprint: `sprint-12-polish-release.md` (release hardening — bug bash, RC1 → v0.1).

---

## Retro Link

`docs/retros/<data>-sprint-11.md` (do utworzenia po sprincie)
