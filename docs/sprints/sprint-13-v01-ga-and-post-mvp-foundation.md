# Sprint 13 — v0.1 GA + Post-MVP Foundation

> **Daty:** 2026-06-08 → 2026-06-21 (planowane 2 tygodnie solo) · **Czas:** ~30-40h skupienia
>
> **Cel:** wydać `v0.1.0` (GA) po dwutygodniowym okresie obserwacji RC1, **i** położyć fundamenty pod v0.2 — concretely: research drugiego providera (cPanel | DirectAdmin | CyberPanel — wybór decydujemy w sprincie), PoC OAuth Device Flow (`webox auth login github`), DB schema w `config.json` (przygotowanie pod topology DB leaf), pierwszy ADR dla generic DAG layout engine.
>
> **Reguła:** wszystko, co tu robimy, ma deliverable w v0.1.x (`fix`/`docs`/`chore`) **albo** zostaje za experimental flag `WEBOX_EXPERIMENTAL=1`. Żadna z research-spike rzeczy nie ląduje w main path v0.1 bez ADR + sign-off.

---

## TL;DR

Po sprincie 13:

- **`v0.1.0` GA tag** — wydanie publiczne, GitHub Release published, brew tap zaktualizowany.
- **`docs/research/provider-comparison.md`** — porównanie 3 paneli (cPanel / DirectAdmin / CyberPanel) wg [CONTRIBUTING §3](../CONTRIBUTING.md#3-jak-doda%C4%87-nowy-provider): authn, CLI/API surface, error model, SSL flow, ekonomia czasu impl. Decyzja maintainera → którego dodajemy w v0.2.
- **`services/githubauth/` PoC** — OAuth Device Flow client + tests + mock; **za** `WEBOX_EXPERIMENTAL` flag, dokumentowany w `docs/research/oauth-device-flow.md`.
- **`config.json` schema migration v3** — dodaje opcjonalne pola `Project.DBType`, `Project.DBName`, `Project.DBHost` (nullable). Migracja v2 → v3 idempotentna; topology builder wireuje DB leaf gdy obecne (wcześniej carry-over z Sprint 11/12).
- **ADR-0010 / DAG layout engine** — projektujemy generic DAG layout (do v0.3+), nie implementujemy. ADR zamyka decyzję, że `asciigraph` zostaje hard-coded 3-level dla v0.1/v0.2 i sięgamy po DAG dopiero gdy multi-server topology stanie się priorytetem.
- **CHANGELOG.md `[v0.1.0]`** zamknięty, `[Unreleased]` zawiera tylko v0.2 work-in-progress.

**Nie robimy w tym sprincie:**

- Implementacji drugiego providera (sam research).
- Full OAuth login flow w main path (tylko PoC + experimental flag).
- DAG layout engine (tylko ADR).
- Plugin marketplace (NIGDY, per AGENTS.md §3.3).
- Telemetrii (NIGDY, per AGENTS.md §3.3).

---

## Pre-flight Checklist

- [ ] `v0.1.0-rc1` tag istnieje na `main`, CI green, podpisany cosign.
- [ ] 2 tygodnie minimum od RC1 → GA (period obserwacji). Jeśli krócej, sprint przesuwa się.
- [ ] Zero P1/P0 bugów w GitHub Issues na RC1.
- [ ] Re-read [AGENTS.md §3.2/§3.3](../../AGENTS.md), [PRD §10 v0.2 hints](../PRD.md), [ROADMAP §3.2](../ROADMAP.md).
- [ ] Bench baseline z Sprint 12 zapisany w `docs/perf/baseline.json`.

---

## Taski

### TASK-13.1 — `v0.1.0` GA release

- **Estymata:** S
- **Zależności:** brak (Sprint 12 RC1 stable)
- **Acceptance Criteria:**
  - [ ] Annotated tag `v0.1.0` na zielonym CI.
  - [ ] GitHub Release **published** (nie draft) z release notes (kopia z `CHANGELOG.md [v0.1.0]`).
  - [ ] Goreleaser produkuje artefakty (6 platform), wszystkie podpisane cosign + SLSA.
  - [ ] Brew tap `dilitS/tap/webox` zaktualizowany (jeśli ZA tap został setupowany w Sprint 12; jeśli nie — bumpuje do v0.2 sprint).
  - [ ] Smoke-test post-release: `brew install dilitS/tap/webox && webox doctor --json` zwraca exit 0.
- **Docs:** [ROADMAP §3.1](../ROADMAP.md).

### TASK-13.2 — Provider research: cPanel vs DirectAdmin vs CyberPanel

- **Estymata:** L
- **Zależności:** TASK-13.1
- **Acceptance Criteria:**
  - [ ] `docs/research/provider-comparison.md` dla każdego z 3 paneli wypełnia tabelę:
    - **Auth model** (login+pass / API token / OAuth / SSH-only?)
    - **CLI surface** (jest natywne CLI? Jak `devil` dla small.pl?)
    - **API REST?** (endpointy potrzebne dla `HostingProvider`: CreateSubdomain, AddDB, ManageSSL, RestartApp, GetLogs)
    - **Error model** (sentinel? Codes? Text-only?)
    - **SSL flow** (Let's Encrypt automatic? Manual? Wildcard?)
    - **Konkurencja:** kto już to integruje (Forge, Ploi, Coolify)?
    - **Estymata impl czasu** (S/M/L/XL) dla `HostingProvider` + parser fixtures.
  - [ ] Rekomendacja: 1 panel do v0.2, drugi do v0.3 backlog.
  - [ ] Decyzja maintainera spisana w ADR-0011 (do utworzenia: `docs/adr/0011-second-provider-choice.md`).
- **Docs:** [CONTRIBUTING §3](../CONTRIBUTING.md#3-jak-doda%C4%87-nowy-provider).

### TASK-13.3 — OAuth Device Flow PoC (experimental)

- **Estymata:** M
- **Zależności:** TASK-13.1
- **Acceptance Criteria:**
  - [ ] `services/githubauth/devicelogin.go` implementuje GitHub Device Flow:
    1. POST `/login/device/code` → user code + verification URI
    2. UI: pokazuje code + URI, kopiuje do clipboarda (`golang-design/clipboard` lub equiv.)
    3. Polling POST `/login/oauth/access_token` co 5s; exponential backoff na `slow_down`
    4. Token zapisuje przez `secrets/keyring` (nigdy plaintext na disk)
  - [ ] Test z `httptest.Server` symulującym GitHub OAuth — coverage ≥ 80 %.
  - [ ] CLI subcommand `webox auth login github` ZA `WEBOX_EXPERIMENTAL=1` (poza tym flagiem komenda nie istnieje).
  - [ ] `docs/research/oauth-device-flow.md` opisuje threat model, alternatywy (Web App Flow, PAT), uzasadnienie wyboru Device Flow.
- **Docs:** [SECURITY §6](../SECURITY.md), GitHub OAuth Apps docs.

### TASK-13.4 — `config.json` schema v3 + DB topology wiring

- **Estymata:** M
- **Zależności:** TASK-13.1
- **Acceptance Criteria:**
  - [ ] `config.Project` zyskuje opcjonalne pola:
    ```go
    DBType string `json:"db_type,omitempty"`  // mysql | postgres | mariadb
    DBName string `json:"db_name,omitempty"`
    DBHost string `json:"db_host,omitempty"`  // default: same as ProfileAlias
    ```
  - [ ] JSON Schema (`config/schema/webox-config-v3.json`) walidator akceptuje v2 i v3; migracja v2 → v3 idempotentna (no-op, pola optional).
  - [ ] `tui/topology.go::buildTopologySnapshot` używa nowych pól — wireuje DB leaf gdy `project.DBName != ""`.
  - [ ] Snapshot `mock-cockpit-140x40.txt` regenerowany — pokazuje `🗄 mysql:webox` leaf w topology gdy `WEBOX_MOCK_DB=1`.
  - [ ] TDD: testy walidatora + migracji (per AGENTS.md §4.2 obowiązek).
- **Docs:** [DESIGN §6 config schema](../DESIGN.md#6-model-danych-i-atomowo%C5%9B%C4%87-zapisu-configjson), nowy ADR-0012 (`docs/adr/0012-config-schema-v3-db-fields.md`).

### TASK-13.5 — ADR-0010: Generic DAG layout engine (research only)

- **Estymata:** S
- **Zależności:** TASK-13.2 (żeby kontekst multi-providera był znany)
- **Acceptance Criteria:**
  - [ ] `docs/adr/0010-generic-dag-layout-engine.md` opisuje:
    - **Context:** dlaczego hard-coded 3-level wystarcza w v0.1/v0.2 (mały solo projekt small.pl), ale przestaje skalować w v0.3+ (multi-server, second provider, multi-service apps).
    - **Decision:** **odroczyć** generic DAG layout do v0.3+, nie implementować w v0.2.
    - **Consequences:** asciigraph zostaje narrowscoped renderer; jeśli operator dodaje 4+ węzłów (np. cache layer), fallback do tabelarycznej `Connections:` listy.
    - **Alternatives considered:** `gonum/graph`, `goccy/go-graphviz`, custom BFS layout.
  - [ ] ADR przeczytany i ACK przez maintainera.
- **Docs:** `docs/adr/README.md` index zaktualizowany.

### TASK-13.6 — Bug bash drugiej rundy + sprzątanie

- **Estymata:** S
- **Zależności:** TASK-13.1
- **Acceptance Criteria:**
  - [ ] Triage wszystkich GitHub Issues otwartych w okresie RC1 → GA (powinno być ich 0–3).
  - [ ] Każdy zamknięty bug ma test regresji.
  - [ ] `CHANGELOG.md [v0.1.1]` (lub [Unreleased] jeśli żadnych fixes) — release minor.

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| Period obserwacji RC1 ujawnia P0 bug → przesunięcie GA | H | Sprint 13 ma bufor: TASK-13.1 może być przeniesione na Sprint 14 jeśli RC1 nie jest stable. |
| OAuth Device Flow PoC ujawnia problem z keyringiem na Linux headless | M | Już mamy AES-GCM fallback (SECURITY §4.2); PoC testuje oba w `httptest`. |
| Provider research zakończy się "no clear winner" — paraliż decyzji | M | Maintainer **musi** podjąć decyzję do końca sprintu; jeśli nie, default na cPanel (największa share). |
| Schema v3 migracja zepsuje istniejące `config.json` v2 użytkowników | H | Pola **optional**; migracja v2 → v3 to no-op; testy regresyjne na 10 fixturach config v2. |
| Brew tap setup blokowany przez homebrew-core review delay | M | Tap własny (`dilitS/tap`) zamiast push do core; mniejsze friction. |

---

## Dependencies signoff

Sprint 13 **może** dodać:

- `github.com/golang-design/clipboard` (TASK-13.3) — wymaga ADR-0013 + maintainer sign-off przed mergem.
- Nic innego.

---

## Outcome (wypełnij po sprincie)

- ✅ Done: ...
- ⏭️ Carry-over → v0.2 sprint planning: ...
- 📌 Decyzje:
  - Drugi provider wybrany: ___ (cPanel | DirectAdmin | CyberPanel)
  - OAuth Device Flow promote do v0.2 main path: TAK / NIE
  - DAG layout engine: pozostaje deferred do v0.3+
- 🧠 Surprises: ...
- 📊 Metryki:
  - Coverage end-of-sprint: ?%
  - Downloads `v0.1.0` first week: ?
  - Issues zamknięte w okresie RC1 → GA: ?
- 🔒 Security validation:
  - [ ] `govulncheck` na v0.1.0 tag: 0 findings.
  - [ ] OAuth PoC token NIGDY w log / config.json / stack trace.
  - [ ] Schema v3 walidator rzuca `ErrSecretInConfig` dla DB password (powinien być w keyring).
- ➡️ Następny sprint: `sprint-14-v02-foundation.md` (do utworzenia — v0.2 kickoff: drugi provider impl + DB tunnel + env merger).

---

## Retro Link

`docs/retros/<data>-sprint-13.md` (do utworzenia po GA tagu)
