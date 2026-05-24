# Sprint 12 — Polish, Standard fallback & v0.1 Release Candidate

> **Daty:** 2026-05-25 → 2026-06-07 (planowane 2 tygodnie solo) · **Czas:** ~30–40h skupienia
>
> **Cel:** zamknąć MVP scope. Sprint 12 to ostatni sprint przed Release Candidate 1: usuwamy braki z poprzednich sprintów (Standard Cockpit topology fallback, animation toggle), robimy bug bash całego cockpitu, regenerujemy snapshoty, podpisujemy artefakty release'a, i tagujemy `v0.1.0-rc1`. **Żadnych nowych ficzerów** — wszystko, co tu robimy, służy „polish to ship".

---

## TL;DR

Po sprincie 12:

- Standard Cockpit (`100×30`) ma sekcję `Connections:` w Overview (tabelaryczny fallback topologii — TASK-11.4 z poprzedniego sprintu).
- Wszystkie ekrany (Init Wizard, Project Wizard, Resume Wizard, Import Preview, Project Detail) używają wspólnej chrome (status bar + footer hints) — sprawdzone snapshotami.
- Bug bash: 10 scenariuszy z `docs/UX.md §11` przejdzie ręcznie + zostanie nagrany jako asciinema (`docs/demos/`).
- Performance budget: cockpit re-render < 16ms na M-series Mac (60fps), CPU < 8 % przy 3 aktywnych projektach.
- `make release-dry-run` produkuje SLSA-signed artefakty (Linux/macOS/Windows) z cosign keyless OIDC.
- CHANGELOG.md ma sekcję `[v0.1.0-rc1]` z pełnym opisem zmian od bootstrapu.
- `goreleaser snapshot` zamyka się czysto; manualnie zweryfikowane: smoke-run binarki na każdej z trzech platform.
- Tag `v0.1.0-rc1` na `main` po pełnym CI green.

**Nie robimy w tym sprincie:**

- Drugi provider (cPanel / DirectAdmin) — sprint 13.
- Webox OAuth Device Flow — sprint 13.
- DB wiring w topologii (`graph.DB`) — czeka na `config.Project.DB*` w v0.2.
- Stretch ficzery z `AGENTS.md §3.2`.

---

## Pre-flight Checklist

- [x] Sprint 11 zamknięty z Outcome (2026-05-24).
- [ ] `git log --oneline main..HEAD` po Sprint 11 zawiera commit z topology map + UI refresh.
- [ ] Re-read [PRD §6 priorytety](../PRD.md), [SECURITY §10](../SECURITY.md), [TESTING §3 piramida](../TESTING.md).
- [ ] `make ci` green na `main`.
- [ ] `.golangci.yml` v2 — bez przerwy 0 issues po `make lint`.
- [ ] Re-read [docs/UX.md §11](../UX.md#11-flowy-end-to-end-mvp) — bug bash scenariusze.

---

## Taski

### TASK-12.1 — Standard Cockpit `Connections:` strip (carry-over z Sprint 11)

- **Estymata:** S
- **Zależności:** none
- **Acceptance Criteria:**
  - [ ] Dla `width<120 || height<35` (Standard Cockpit) Overview tile dostaje dodatkową sekcję `Connections:` z 3-4 wierszami:
    ```
    Connections:
      GitHub → Server : ✓ Active (2h ago, success)
      Server → App    : ✓ Online (200 OK, 88ms)
      Server → MySQL  : (no DB linked)
    ```
  - [ ] Producer reuses `buildTopologySnapshot` z Sprint 11 — fold edges w 1-linijkowy fallback (`renderTopologyTextLine(edge)`).
  - [ ] Snapshot test `100×30` zawiera `Connections:`.
- **Docs:** [UX §3.4](../UX.md#34-wizualny-graf-topologii-us%C5%82ug-live-service-topology-map), Sprint 11 TASK-11.4.

### TASK-12.2 — Chrome consistency audit (init wizard, project wizard, import preview)

- **Estymata:** M
- **Zależności:** TASK-12.1
- **Acceptance Criteria:**
  - [ ] Snapshot test per surface (`init_wizard_100x30.txt`, `project_wizard_120x35.txt`, `import_preview_120x35.txt`, `resume_wizard_120x35.txt`, `project_detail_140x40.txt`) — wszystkie pokazują:
    - Brand `WEBOX vX.Y.Z` w status barze
    - Breadcrumb cell w status barze (`Init Wizard`, `Project Wizard`, …)
    - Footer hint strip `[q] quit · [?] help · [/] palette · [Tab] cycle`
  - [ ] Każdy panel używa `lipgloss.ThickBorder()` przez `theme.Styles.Panel`; ActivePanel używa `lipgloss.DoubleBorder()`.
  - [ ] Wizardy mają ASCII WEBOX logo na pierwszym ekranie (TASK-11.* już dodał na init; rozszerzyć na project wizard intro).
- **Docs:** [UX §4](../UX.md#4-uk%C5%82ad-ekran%C3%B3w), nowy ADR-0008 jeśli format chrome wymaga formalnej decyzji.

### TASK-12.3 — Bug bash scenariusze + asciinema demos

- **Estymata:** L
- **Zależności:** TASK-12.2
- **Acceptance Criteria:**
  - [ ] 10 scenariuszy z `docs/UX.md §11` zarchiwizowane w `docs/demos/<scenario>.cast`:
    1. `init-wizard-happy-path.cast`
    2. `project-wizard-create-no-db.cast`
    3. `project-detail-restart-app.cast`
    4. `project-detail-logs-redacted-secret.cast`
    5. `dashboard-ci-pipeline-failure.cast`
    6. `dashboard-ssl-near-expiry-degraded.cast`
    7. `topology-offline-cascade.cast`
    8. `doctor-json.cast`
    9. `import-preview-detects-gaps.cast`
    10. `resume-wizard-after-crash.cast`
  - [ ] `make demo` target generuje pojedynczą asciinemę z `--mock`.
  - [ ] Każdy plik `.cast` ma siostrzany `.md` z 3-5 zdaniami "co tu się dzieje" — żeby reviewer widział kontekst.
- **Docs:** new `docs/demos/README.md` z indeksem.

### TASK-12.4 — Performance budget enforcement

- **Estymata:** M
- **Zależności:** TASK-12.2
- **Acceptance Criteria:**
  - [ ] Nowy benchmark `tui/cockpit_bench_test.go::BenchmarkCockpitRender` — 60fps target (16ms/frame) na M-series Mac.
  - [ ] `make bench` target uruchamia bench i fail'uje jeśli regression > 20% względem snapshot baseline (`docs/perf/baseline.json`).
  - [ ] Dodatkowo `goleak.VerifyNone` w teście quit transition cockpitu — żaden timer nie wycieka po `q`.
- **Docs:** ADR-0009 (TBD) — performance budget formalizacja.

### TASK-12.5 — Release tooling smoke-test

- **Estymata:** M
- **Zależności:** TASK-12.4
- **Acceptance Criteria:**
  - [ ] `make release-dry-run` produkuje 6 artefaktów (linux-amd64/arm64, darwin-amd64/arm64, windows-amd64, source-tarball) podpisanych przez cosign keyless OIDC.
  - [ ] SLSA provenance attached do każdego artefaktu.
  - [ ] Manualny smoke-test każdej platformy: rozpakuj binarkę → `webox --version` → `webox doctor --json` → `webox --mock`.
  - [ ] `goreleaser check` zero warnings.
- **Docs:** [DESIGN §14 Release pipeline](../DESIGN.md), CONTRIBUTING.md sekcja release.

### TASK-12.6 — CHANGELOG release notes + tag `v0.1.0-rc1`

- **Estymata:** S
- **Zależności:** TASK-12.5
- **Acceptance Criteria:**
  - [ ] Sekcja `[v0.1.0-rc1] - 2026-06-07` w `CHANGELOG.md` z kategoriami: Added / Changed / Fixed / Security / Performance.
  - [ ] `[Unreleased]` zostaje pusty.
  - [ ] Annotated git tag `v0.1.0-rc1` na zielonym CI.
  - [ ] GitHub Release draft (NIE published) z release notes wygenerowanymi przez goreleaser.
- **Docs:** [ROADMAP §3.1](../ROADMAP.md), CHANGELOG zasady.

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| Asciinema demos nagrane na małym terminalu — Ultra+ tile nie renderuje się czytelnie w 80×24 | M | `make demo` ustawia jawnie `tput cols 140; tput lines 40` przed nagraniem; reviewer ma jasny rozmiar. |
| Goreleaser SLSA + cosign wymaga zmiany GitHub Actions workflow → potencjalny supply-chain ryzyko | M | Pinujemy actions przez SHA (AGENTS.md §2.1). Manualnie weryfikujemy każdy nowy step przed merge'em. |
| Performance budget regression niewykryta lokalnie (M-series Mac), wykryta dopiero w CI na ARM Linux | M | `make bench` jest osobnym CI jobem na 3 runnerach (Ubuntu, macOS, Windows). Baseline per runner. |
| Bug bash znajdzie blocker → przesunięcie RC1 → ryzyko zsuwania v0.1 GA | H | Bug bash zaczynamy na 4 dzień sprintu; bufor 5 dni na fix + retest. |
| Mock mode rozjedzie się z realnym kodem (mock fetcher zwraca dane niezgodne z `ProjectStatus` v2) | M | `make ci` uruchamia mock cockpit jako snapshot test; rozjazd lapie się na PR. |

---

## Dependencies signoff

Sprint 12 **nie dodaje** nowych zewnętrznych zależności. Jeśli SLSA tooling wymaga nowego helpera, idzie przez ADR + maintainer sign-off.

---

## Outcome (wypełnij po sprincie)

- ✅ Done: ...
- ⏭️ Carry-over → Sprint 13: ...
- 📌 Decyzje: ...
- 🧠 Surprises: ...
- 📊 Metryki:
  - Coverage post-sprint: ?%
  - Cockpit render time (M-series): ?ms
  - All-tiles CPU%: ?
  - Release artefacts size: ? MB total
- 🔒 Security validation:
  - [ ] `govulncheck` zero findings na release tag.
  - [ ] Cosign verify każdej platformy zielony.
  - [ ] SLSA provenance present + matches commit SHA.
- ➡️ Następny sprint: `sprint-13-second-provider-research.md` (post-MVP — second provider research + OAuth Device Flow PoC).

---

## Retro Link

`docs/retros/<data>-sprint-12.md` (do utworzenia po RC1 tagu)
