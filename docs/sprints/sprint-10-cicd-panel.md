# Sprint 10 — Live CI/CD Pipeline Panel

> **Daty:** TBD → TBD (planowane 1-1.5 tygodnia solo) · **Czas:** ~25-35h skupienia
>
> **Cel:** dostarczyć kafelek Bento Ultra `[CI/CD Pipeline: Main Branch]` (lewa-środkowa pozycja w layout z [UX §4.2](../UX.md#42-dashboard-20--bento-box-grid-system-12035-mvp--16045-stretch)) który pokazuje live workflow steps z GitHub Actions: `[1] Git Checkout ✓`, `[2] Install Deps ✓`, `[3] Code Lint ✓`, `[4] Build Artifact ✓`, `[5] Unit Tests ✓`, `[6] Deploy (Prod) ✓` z kolorowanymi badge'ami i click-through (`F8`) do `gh run view` ostatnich 50 linii w modal. **Bazujemy w 100% na `services/github` ze Sprintu 06** — nie dodajemy nowych GitHub API calls poza tymi które już istnieją.

---

## TL;DR

Po sprincie 10:

- `tui/bento/tiles/cicd.go` implementuje `BentoTile` dla CI/CD panel.
- `services/github.GetWorkflowSteps(ctx, owner, repo, runID) ([]Step, error)` — nowa metoda dodana do istniejącego klienta (`gh run view --json jobs,steps`).
- Polling TTL 10s przez `status.Cache` (key: `gh:steps:<owner>/<repo>:<workflow>`).
- Kolorowe step results: ✓ Success (Green), ✗ Failure (Red), ⏳ In Progress (Yellow), ⊘ Skipped (Muted).
- `F8` (lub Enter na kafelku) otwiera modal z `gh run view <runID> --log` ostatnich 50 linii (redagowane przez `internal/log.Redact`).
- Pipeline header: `Build #412: SUCCESS ✓ (14:12 GMT)` lub `Build #413: IN_PROGRESS ⏳ (2m 14s)` z live update.

**Nie robimy w tym sprincie:**

- Workflow dispatcher (re-run failed) — bezpieczeństwo: wymaga `repo:write` PAT scope, MVP czyta tylko.
- Workflow file editor — STRETCH v0.2+.
- Multi-workflow comparison — STRETCH v0.2+ (MVP pokazuje `main` branch latest run).
- Topology Map — Sprint 11.

---

## Pre-flight Checklist

- [ ] Sprint 09 zamknięty z retro i `Outcome`.
- [ ] Re-read [UX §4.2](../UX.md#42-dashboard-20--bento-box-grid-system-12035-mvp--16045-stretch), [DESIGN §8](../DESIGN.md#8-tr%C3%B3jpoziomowy-status-cache-stale-while-revalidate).
- [ ] Re-read `services/github/` ze Sprintu 06 — czy `GetLatestRun` ma już potrzebne fields, czy musimy dodać `GetWorkflowSteps`.
- [ ] Verify `gh run view --json` schema na bieżącej wersji `gh` CLI.
- [ ] Confirm `make ci` green on `main` after Sprint 09 merge.

---

## Taski

### TASK-10.1 — `services/github.GetWorkflowSteps`

- **Estymata:** M
- **Zależności:** Sprint 09 done; `services/github` ze Sprintu 06.
- **Acceptance Criteria:**
  - [ ] `services/github/steps.go` exposes `GetWorkflowSteps(ctx, owner, repo, runID int64) ([]Step, error)`.
  - [ ] `Step{Number int, Name string, Status string, Conclusion string, StartedAt, CompletedAt time.Time, DurationMs int64}`.
  - [ ] Primary transport: `gh run view <runID> --json jobs --jq '.jobs[].steps[]'`.
  - [ ] REST fallback: `GET /repos/{owner}/{repo}/actions/runs/{run_id}/jobs` z PAT.
  - [ ] Cassette tests w `services/github/ghmock/` (success run, in-progress, failed step, skipped step).
  - [ ] Sentinel errors: `ErrRunNotFound`, `ErrStepsParseError` (rozszerza istniejące z Sprint 06).
  - [ ] PAT redaction tests (jak w Sprint 06).
- **Docs:** [PRD §6 F15](../PRD.md#6-ficzery--z-priorytetami), [DESIGN §8](../DESIGN.md#8-tr%C3%B3jpoziomowy-status-cache-stale-while-revalidate).

### TASK-10.2 — CI/CD tile z polling przez status.Cache

- **Estymata:** M
- **Zależności:** TASK-10.1
- **Acceptance Criteria:**
  - [ ] `tui/bento/tiles/cicd.go` implementuje `BentoTile`; slot `Right` (lub `Top-Right` w `160×45`), MinSize `(60, 12)`.
  - [ ] Polling przez `status.Cache.GetOrFetch(ctx, "gh:steps:<owner>/<repo>:<workflow>", 10*time.Second, fetcher)`.
  - [ ] Live update: `tea.Tick(10*time.Second, ...)` invokuje fetch; UI re-render na nowych danych.
  - [ ] In-progress badge `⏳ IN_PROGRESS` z elapsed time live update co 1s (czysto klientowy timer, nie new API call).
  - [ ] Step rendering: numbered list z badge'ami zgodnie z [UX §3.1](../UX.md#31-badges-statusu-premium).
  - [ ] Brak projektów z `Repo` field → tile renderuje placeholder `"No GitHub-linked project selected. Press [n] to create a new project."`.
  - [ ] Tests: cassette-driven test successful run, in-progress, failed step.
- **Docs:** [UX §4.2](../UX.md#42-dashboard-20--bento-box-grid-system-12035-mvp--16045-stretch).

### TASK-10.3 — `F8` modal: workflow logs viewer

- **Estymata:** M
- **Zależności:** TASK-10.2, TASK-08.4 (double-border modal)
- **Acceptance Criteria:**
  - [ ] `F8` (lub Enter) na CI/CD tile otwiera modal z `gh run view <runID> --log` ostatnich 50 linii.
  - [ ] Modal używa double-border component ze Sprintu 08.
  - [ ] Każda linia logu **redagowana** przez `internal/log.Redact` przed renderem (corpus ze Sprintu 09 active).
  - [ ] Scroll przez `↑`/`↓`, `Esc` zamyka.
  - [ ] Modal header: `Workflow Run #<runID> · <conclusion> · <duration>`.
  - [ ] Failure run: cały modal red border, header `FAILED ✗`, log lines highlight ostatnia ERROR line.
  - [ ] Tests: cassette dla success run logs i failure run logs; redactor smoke (PAT w logu nie pojawia się w rendered output).
- **Docs:** [UX §2.2](../UX.md#22-system-warstw-i-g%C5%82%C4%99bi-dynamic-layering), [SECURITY §6](../SECURITY.md).

### TASK-10.4 — Dashboard integration + project switcher

- **Estymata:** S
- **Zależności:** TASK-10.2
- **Acceptance Criteria:**
  - [ ] Switch projektu na dashboardzie invalidates CI/CD tile cache (`status.Cache.Delete("gh:steps:...")`).
  - [ ] CI/CD tile pokazuje workflow runs **wybranego** projektu (jeden tile, jeden projekt — multi-project comparison to STRETCH v0.2+).
  - [ ] Project bez `Repo` field → tile renderuje informacyjny placeholder, nie błąd.
  - [ ] Tests: teatest scenariusz dashboard switch projektu → tile re-fetch z cassette.

### TASK-10.5 — Rate limiting + GitHub API graceful degradation

- **Estymata:** S
- **Zależności:** TASK-10.1
- **Acceptance Criteria:**
  - [ ] Detekcja `ErrRateLimited` z `services/github` (już istnieje z Sprintu 06).
  - [ ] Rate limit → CI/CD tile pokazuje placeholder `"GitHub rate limit reached. Cached data shown. Reset in <X>min."` z `X-RateLimit-Reset` header.
  - [ ] Sukcesywne fetch'e nie retry'ują się gdy rate limit aktywny (backoff respect).
  - [ ] Tests: cassette zwracająca 429 → tile graceful degradation.
- **Docs:** [DESIGN §8](../DESIGN.md#8-tr%C3%B3jpoziomowy-status-cache-stale-while-revalidate), [SECURITY §10.4](../SECURITY.md).

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| Step rendering nie pasuje do `120×35` (over-flow) | M | MinSize gating; jeśli `width<60`, tile renderuje compact view (tylko status badge, bez step names). |
| PAT w workflow logs (np. `echo $MY_TOKEN`) | H | Redactor smoke active; user może sam wstawić secret w workflow — ostrzeżenie w `webox doctor github` (Sprint 07). |
| `gh run view --log` zwraca tysiące linii (large workflow) | M | Hard cap 50 ostatnich linii; modal ma scrollback ale nie ładuje całości. |
| Rate limit przy 5 projektach × poll co 10s = 30 req/min | M | Cache TTL 10s eliminuje większość; `gh` CLI auth = 5000/h limit, anon = 60/h; przekroczenie limitu graceful degradation. |
| Workflow z dispatched run (manualnie) bez ostatecznego run vs konflikt — UI pokazuje stale data | M | Polling co 10s wystarczy; live indicator w header tile (`⚡ refreshing` podczas fetch). |

---

## Dependencies signoff

Sprint 10 **nie dodaje** nowych zewnętrznych zależności. Wszystko buduje na `services/github` ze Sprintu 06 (`gh` CLI primary, REST+PAT fallback).

---

## Outcome (wypełnij po sprincie)

- ✅ Done: ...
- ⏭️ Carry-over: ...
- 📌 Decyzje: ...
- 🧠 Surprises: ...
- 📊 Metryki:
  - Coverage `services/github/steps.go`: ?
  - Coverage `tui/bento/tiles/cicd.go`: ?
  - Cassettes: success / in-progress / failed / rate-limited (4/4).
- 🔒 Security validation:
  - [ ] PAT redaction in workflow logs (modal).
  - [ ] PAT redaction in cache key (key nie zawiera PAT).
  - [ ] `go test -race ./services/github ./tui/bento/...` green.
- ➡️ Następny sprint: `sprint-11-topology-map.md`

---

## Retro Link

`docs/retros/<data>-sprint-10.md` (do utworzenia po sprincie)
