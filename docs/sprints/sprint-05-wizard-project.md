# Sprint 05 — Project Wizard + LIFO Rollback

> **Daty:** TBD → TBD (planowane 2-3 tygodnie solo) · **Czas:** ~45-65h skupienia
>
> **Cel:** przekształcić read-only TUI shell ze Sprintu 04 w pierwszy realny flow operatorski: pięciokrokowy wizard tworzenia projektu z profilem `smallhost`, wyborem stacka, walidacją domeny, opcjonalną bazą danych i prostym LIFO rollbackiem zapisującym `pending_cleanups.json`. **Nie generujemy jeszcze GitHub Actions workflow ani nie robimy pierwszego deploya** — to Sprint 06.

---

## TL;DR

Po sprincie 05:

- Dashboard ma wejście `n` / `/create` do wizardu nowego projektu.
- Init wizard przestaje być tylko statycznym ekranem: potrafi zebrać minimalny profil `smallhost` i zapisać `config.json` bez sekretów.
- Wizard projektu ma kroki: Profile → Stack → Database (skippable) → Subdomain → Execution.
- Execution wykonuje wyłącznie provider-side provisioning: subdomain, SSL i opcjonalną DB przez `providers.HostingProvider`.
- Każdy udany krok pushuje cleanup na prosty stos LIFO; crash-safe snapshot trafia do `pending_cleanups.json`.
- Błąd w połowie flow zatrzymuje wizard, pokazuje decyzję userowi i pozwala wykonać jawny rollback.

**Nie robimy w tym sprincie:**

- GitHub repo creation, deploy keys, Actions workflow, first deploy: Sprint 06.
- Live log stream, Env Merger, Database tab management: v0.2+.
- DAG rollback / selective resume: v0.3+; Sprint 05 ma tylko LIFO.
- Non-interactive CLI typu `webox create`: v0.3+.
- Drugi provider: poza MVP.

---

## Pre-flight Checklist

- [ ] Sprint 04 zamknięty z retro i `Outcome`.
- [ ] Re-read `PRD.md §6 F1/F3/F10/F21`.
- [ ] Re-read `DESIGN.md §2.3`, `§6`, `§10.0`, `§12`.
- [ ] Re-read `UX.md §4.1`, `§4.4`, `§11.1`, `§11.2`.
- [ ] Re-read `SECURITY.md §4`, `§5`, `§10.4`, `§10.6`.
- [ ] Confirm `make ci` green before adding wizard state transitions.

---

## Taski

### TASK-05.1 — Wizard state model + messages

- **Estymata:** M
- **Zależności:** Sprint 04 done
- **Acceptance Criteria:**
  - [ ] `tui/states.go` adds wizard substates without changing top-level `State` enum semantics.
  - [ ] `tui/messages.go` defines wizard messages for profile, stack, DB choice, domain validation, execution progress, rollback prompt.
  - [ ] `tui/model.go` keeps wizard state inside `Model`; no global mutable wizard variables.
  - [ ] `Update` tests cover forward/back navigation, cancel prompt, and invalid transition rejection.
  - [ ] `View` renders current wizard step without side effects.
- **Docs:** `DESIGN.md §2.3`, `§12`, `UX.md §11.2`.

### TASK-05.2 — Init wizard minimal profile setup

- **Estymata:** L
- **Zależności:** TASK-05.1
- **Acceptance Criteria:**
  - [ ] First-run flow collects `smallhost` profile alias, host, port, user, and `restart_method`.
  - [ ] Profile validation reuses `providers.New` / `smallhost.New` sentinels.
  - [ ] `config.Save` persists `config.json` atomically with no secret fields.
  - [ ] SSH password / PAT fields are not introduced in config or rendered views.
  - [ ] Tests cover missing config → profile form → saved config → dashboard transition.
- **Docs:** `PRD.md F1`, `DESIGN.md §6`, `SECURITY.md §4`.

### TASK-05.3 — Stack + DB selection

- **Estymata:** M
- **Zależności:** TASK-05.1
- **Acceptance Criteria:**
  - [ ] Supported stacks: `vite-react`, `node-express`, `static`.
  - [ ] DB step is skipped for `static` and `vite-react` unless user explicitly opts in.
  - [ ] DB kind is limited to provider-supported values (`mysql`, `postgres`) without free-form command tokens.
  - [ ] Tests cover smart skip, opt-in DB, and invalid stack rejection.
- **Docs:** `PRD.md F3/F21`, `UX.md §11.2`.

### TASK-05.4 — Subdomain validation + provider preflight

- **Estymata:** M
- **Zależności:** TASK-05.2, TASK-05.3
- **Acceptance Criteria:**
  - [ ] Domain input uses `smallhost.ValidateDomain` before reaching command builders.
  - [ ] Provider preflight calls `CheckStatus` and fails closed on missing `devil`.
  - [ ] Duplicate subdomain maps through `providers.ErrSubdomainExists` and stays recoverable in UI.
  - [ ] Tests use fake provider; no real SSH.
- **Docs:** `docs/providers/smallhost.md §2`, `SECURITY.md §3.3`.

### TASK-05.5 — LIFO rollback stack package

- **Estymata:** L
- **Zależności:** TASK-05.1
- **Acceptance Criteria:**
  - [ ] `wizard/rollback.go` defines `CleanupStep`, `Stack`, `Push`, `Pop`, `RunRollback`.
  - [ ] `wizard/pending_cleanups.go` serializes snapshots to `pending_cleanups.json` using atomic save semantics.
  - [ ] Remove operations are idempotent; missing resource == success.
  - [ ] Tests cover push order, reverse execution, serialization round-trip, corrupted pending file, and context cancellation.
  - [ ] No DAG/topological sort code appears in `wizard/`.
- **Docs:** `DESIGN.md §10.0`, `AUDIT.md §8 IMP-1`.

### TASK-05.6 — Provider-side execution step

- **Estymata:** L
- **Zależności:** TASK-05.4, TASK-05.5
- **Acceptance Criteria:**
  - [ ] Execution step creates subdomain, sets up SSL, and optionally creates DB through `HostingProvider`.
  - [ ] After each successful operation, matching cleanup is pushed and persisted.
  - [ ] Failure stops execution and renders remediation choices: rollback or keep resources and exit.
  - [ ] Tests use fake provider scripts for success, SSL failure, DB failure, and rollback failure.
  - [ ] No GitHub repo, workflow, deploy key, or deploy action in this sprint.
- **Docs:** `PRD.md F3/F10`, `DESIGN.md §10.0`, `providers/smallhost.md`.

### TASK-05.7 — Dashboard integration + post-create status

- **Estymata:** M
- **Zależności:** TASK-05.6
- **Acceptance Criteria:**
  - [ ] `n` from Dashboard opens the project wizard; `q` still cancels safely.
  - [ ] Successful wizard appends project metadata to `config.json` and returns focus to the new project.
  - [ ] Status cache invalidates `http:`, `ssl:`, and `ssh:node:` prefixes for the new project.
  - [ ] `Project Detail` shows created provider-side values but last deploy remains Sprint 06 placeholder.
  - [ ] Teatest smoke covers dashboard → wizard → success with fake provider.
- **Docs:** `UX.md §4.2`, `§11.2`, `DESIGN.md §8`.

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| Wizard zacznie robić GitHub/deploy za wcześnie | H | Sprint 05 kończy się na provider-side provisioning; GitHub is explicitly Sprint 06. |
| LIFO snapshot zapisze sekret | H | `CleanupStep.Params` pozwala tylko na metadata potrzebne do usunięcia zasobu; testy skanują secret-shaped strings. |
| Rollback nie będzie idempotentny | H | Każdy `Remove*` traktuje not-found jako success; fake provider testuje powtórzony rollback. |
| `config.Load` first-run semantics nadal mylą | M | Sprint 05 dokumentuje osobny `tui` first-run detector; nie zmieniamy settled `config.Load` API bez ADR. |
| Wizard UI stanie się zbyt duży dla 88×28 | M | Render tests dla 80×24/88×28/100×30; single-pane layout obowiązkowy. |

---

## Outcome (wypełnij po sprincie)

- ✅ Done: TASK-05.X, ...
- ⏭️ Carry-over: ...
- 📌 Decyzje: ...
- 🧠 Surprises: ...
- 📊 Metryki:
  - Coverage `wizard/`: %
  - Coverage wizard-related `tui/`: %
  - Teatest wizard scenarios: %
- 🔒 Security validation:
  - [ ] `pending_cleanups.json` contains no plaintext secrets.
  - [ ] `config.json` remains secret-free after init/profile/project flows.
  - [ ] `go test -race ./wizard ./tui/... ./cmd/webox/...` green.
- ➡️ Następny sprint: `sprint-06-github-deploy-workflow.md`

---

## Retro Link

`docs/retros/YYYY-MM-DD-sprint-05.md`
