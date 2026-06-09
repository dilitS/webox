# Webox — Sprint Planning Operating Model

> **Status:** Active · **Audience:** Solo maintainer + future contributors · **Cadence:** 1-2 tyg.
>
> _„Sprint nie istnieje, dopóki ma więcej niż jedną otwartą interpretację. Planujemy aż backlog stanie się nudny."_

Ten dokument opisuje **jak** prowadzimy implementację po fazie docs-first. Sprint plany pojedyncze (`sprint-XX-*.md`) są generowane **rolling-wave** — zawsze jeden bieżący w pełni rozpisany, następny w zarysie, dalsze tylko jako tematyka (z `ROADMAP.md`).

---

## 1. Filozofia

1. **Plan jest narzędziem, nie umową.** Sprint plan to instrukcja na _ten tydzień_, nie kontrakt. Jeśli rzeczywistość nie pasuje, modyfikujemy plan, nie rzeczywistość.
2. **Małe, niezależne PR-y > monolityczne sprinty.** Cel: każdy task = jeden PR mergowalny w izolacji.
3. **TDD jest twarde dla logiki krytycznej** (`config/`, `secrets/`, parser `devil`, `status/cache`, state machine TUI, redactor). Reszta — pragmatyczna pokrywa testami (table-driven + golden files).
4. **Sprint kończy się retrospektywą i CHANGELOG entry.** Bez wyjątku. Skill: `.cursor/skills/retro/SKILL.md`.
5. **Estymata to sygnał ryzyka, nie obietnica.** Jeśli task ma estymatę „L (1-2 dni)" i jedziemy 3-ci dzień — to znaczy, że źle rozumiemy problem. STOP, zrobimy spike.

---

## 2. Definitions

### 2.1 Definition of Ready (DoR) — task wchodzi do sprintu, gdy

- [ ] Ma jasny **acceptance criteria** (lista testowalnych warunków).
- [ ] Ma **estymatę** (S / M / L / XL — patrz §4).
- [ ] Linkuje do **autoritatywnego dokumentu** (PRD/DESIGN/SECURITY/ADR) — jedno źródło prawdy.
- [ ] Zidentyfikowane **zależności** (blokuje / zablokowane przez).
- [ ] Brak dwuznaczności techniczych — jeśli są, najpierw spike.

### 2.2 Definition of Done (DoD) — task jest skończony, gdy

- [ ] **Kod + testy** w tym samym PR-ze (TDD: test napisany _przed_ implementacją dla logiki krytycznej).
- [ ] `make ci` zielony lokalnie (`lint`, `test`, `vulncheck`, `build`).
- [ ] **Coverage >= 80%** dla pakietu (jeśli pakiet ma logikę; pure plumbing zwolnione).
- [ ] Brak `TODO` / `FIXME` bez linku do issue.
- [ ] Brak `nolint` bez uzasadnienia w komentarzu (patrz `CONTRIBUTING.md §2.1`).
- [ ] **Conventional Commit** (skill: `commit-policy`).
- [ ] **CHANGELOG entry** w sekcji `Unreleased` (jeśli zmiana user-visible lub security).
- [ ] **Dokumentacja zaktualizowana**, jeśli zmienia kontrakt / scope (skill: `audit-trace`).
- [ ] PR review checklist przejdzie (`.github/pull_request_template.md`).

### 2.3 Definition of Sprint Done — sprint zamknięty, gdy

- [ ] Wszystkie taski DoD ✅ **lub** explicit carry-over do następnego z notatką „dlaczego".
- [ ] **Retrospektywa** zapisana w `docs/retros/YYYY-MM-DD-sprint-XX.md`.
- [ ] **Sprint review** w `docs/sprints/sprint-XX-*.md` (sekcja „Outcome") wypełniona.
- [ ] Nowe ryzyka zaktualizowane w `docs/RISKS.md`.
- [ ] Następny sprint w pełni zaplanowany (lub explicit decyzja: „pauza N dni, planowanie X.Y.Z").

---

## 3. Rolling-wave Cadence

```
┌──────────────────────────────────────────────────────────────┐
│  Tydzień 1                                                   │
│  ├─ Mon 30 min: Sprint planning (ten dokument szczegółowy)   │
│  ├─ Tue-Sun: implementacja taska po tasku                    │
│  └─ ostatni dzień: retro + zarys następnego sprintu          │
└──────────────────────────────────────────────────────────────┘
                  ↓
┌──────────────────────────────────────────────────────────────┐
│  Tydzień 2                                                   │
│  ├─ Mon 30 min: Sprint planning N+1 (uszczegółowienie)       │
│  └─ ...                                                       │
└──────────────────────────────────────────────────────────────┘
```

- **Bieżący sprint:** w pełni rozpisany task po tasku z estymatami i AC.
- **Sprint N+1:** zarys (5-8 storyek, bez subtasków).
- **Sprint N+2 i dalej:** tylko temat z `ROADMAP.md`.

### 3.1 Sprint planning session (30 min)

1. **5 min** — _przegląd retrospektywy poprzedniego sprintu._ Co przeniesiemy do tego sprintu? Jakie zmiany w procesie?
2. **10 min** — _wybór storyek z `ROADMAP.md` + carry-over._ Zasada: nie więcej niż **5 storyek (M)** lub **3 storyki (L)** w 1-tygodniowym sprincie solo.
3. **10 min** — _dekompozycja na taski 0.5d-2d._ Każdy task: AC, estymata, link do docs.
4. **5 min** — _sanity-check capacity._ Realistycznie: 5 dni × 4h skupienia = **20h** w sprincie solo. Nie 40.

### 3.2 Daily checkpoint (5 min, opcjonalnie w chat z Claude)

- Co skończyłem wczoraj?
- Co robię dzisiaj?
- Co mnie blokuje?

Jeśli blocker > 2h — przerywamy task, robimy spike, odraczamy do retro.

---

## 4. Estymaty

| Tag | Czas | Charakter |
|-----|------|-----------|
| **S** | < 2h | Refaktor, mała funkcja, doc fix |
| **M** | 0.5-1 dzień | Jeden pakiet, ~150 LoC + testy |
| **L** | 1-2 dni | Wiele plików, integracje, edge cases |
| **XL** | > 2 dni | **Czerwona flaga — rozbij na M/L lub spike** |

**Zasada:** jeśli task ma XL — nie startuje w sprincie. Najpierw spike (timebox 4h), potem rozbicie.

---

## 5. Sprint Plan Template

Każdy `sprint-XX-name.md` ma sztywny szablon, żeby było predykcyjnie:

```markdown
# Sprint XX — <Tematyka>

> **Daty:** YYYY-MM-DD → YYYY-MM-DD · **Czas:** N tygodni · **Cel:** <jedno zdanie>

## Cel sprintu
<2-3 zdania — co umiemy zrobić po sprincie, czego nie>

## Taski

### TASK-XX.1 — <nazwa>
- **Estymata:** S/M/L
- **Zależności:** —
- **Acceptance Criteria:**
  - [ ] AC1 (testowalne)
  - [ ] AC2 (mierzalne)
- **Pliki:**
  - `path/to/file.go` (new)
  - `path/to/test.go` (TDD)
- **Docs:** [DESIGN.md §X.Y](../DESIGN.md#…)
- **Notatki:** <pułapki, przykłady, ważne edge cases>

### TASK-XX.2 ...

## Risk watch
- <ryzyko 1 → mitygacja>

## Outcome (wypełnij po sprincie)
- ✅ Done: TASK-XX.1, TASK-XX.2
- ⏭️ Carry-over: TASK-XX.5 → Sprint XX+1 (powód: …)
- 📌 Decyzje: <ADR jeśli powstał>
- 🧠 Surprises: <co się okazało inne niż w docs>
```

---

## 6. Wzorce wykonania

### 6.0 Automatyzacja workflow (zero ceremonii)

Wszystkie powtarzalne kroki są skryptowane. Główne komendy z `Makefile`:

| Komenda | Co robi |
|---------|---------|
| `make bootstrap` | One-shot setup nowego klona: instaluje narzędzia + git hooks + cursor hooks. |
| `make setup-hooks` | Tylko git hooks (jeśli `bootstrap` już był). |
| `make sprint-status` | Postęp bieżącego sprintu (taski done/open, AC checked). |
| `make next-task` | Wypisuje id następnego otwartego taska (parseable). |
| `make sprint-start` | Branch `feat/sNN-TT-slug` + opcjonalnie otwiera plan w `$EDITOR`. |
| `make new-task NAME="..." EST=M` | Dopisuje task do bieżącego sprintu. |
| `make dev PKG=./config/...` | TDD watch loop (`gow` / `fswatch+entr` / `inotifywait` / poll). |
| `make ci-fast` | `lint + vet + test-short` — szybki sanity check. |
| `make ci` | Pełny CI bundle lokalnie (mirror GitHub Actions). |
| `make commit-suggest` | Conventional Commits z diffa (input do `git commit -em "$(...)"`). |
| `make changelog KIND=fixed MSG="..."` | Wpis do `CHANGELOG.md` Unreleased. |
| `make retro` | Skeleton retrospektywy dla bieżącego sprintu. |
| `make pr` | Otwiera draft PR via `gh` z prefilled body (sprint context, DoD checklist). |

**Git hooks** (`make setup-hooks` ustawia `core.hooksPath = .githooks`):

- `pre-commit` — `gofumpt + goimports` auto-fix, fast lint na staged Go, secret tripwire.
- `commit-msg` — Conventional Commits 1.0.0 validation.
- `prepare-commit-msg` — pre-fill CC template gdy wiadomość pusta.
- `pre-push` — `make test-short` (override: `WEBOX_PREPUSH=full git push`).

**Cursor skills** które „same się odpalą" w odpowiednich momentach:

- `task-start` — kompletny pickup nowego taska (branch + read spec + watch + plan).
- `tdd-loop` — Red → Green → Refactor → Commit.
- `auto-changelog` — wpis do `CHANGELOG.md` zaraz po implementacji.
- `commit-policy` — walidacja przed commitem.
- `retro` — generacja + wypełnienie retra na koniec sprintu.

**CI side** automatyzacja:

- `actions/labeler@v5` — path-based labels na każdym PR (`.github/labeler.yml`).
- `dependabot/fetch-metadata@v2` + `gh pr merge --auto` — auto-merge patch/minor non-prod deps.
- `dependabot.yml` — weekly Go + Actions updates.

**Zasada:** jeśli pewien krok robisz ręcznie **3 razy w sprincie**, to jest sygnał do dodania go do `scripts/` / `Makefile` / skill / hook.

### 6.1 TDD loop (skill: `tdd-loop`)

1. Wybierz najmniejsze AC z taska.
2. Napisz failing test.
3. Minimalny kod, żeby przeszedł.
4. Refaktor (zachowując zielone testy).
5. Powtórz dla następnego AC.

### 6.2 PR workflow

1. `git checkout -b feat/<sprint>-<task>-<short-name>` (np. `feat/s01-config-load`).
2. Małe commity z Conventional Commits.
3. Push → otwórz draft PR → przejdź checklistę.
4. Gotowy → Ready for review.
5. Self-review (czytam swój diff świeżym okiem, 5 min).
6. Merge (`squash and merge` dla solo, „merge commit" jeśli > 1 logical change).

### 6.3 Spike (research) task

- Timebox **4h max**.
- Output: krótkie ADR-let w `docs/adr/00XX-...md` _lub_ notatka w sprint planie.
- Nie ma kodu produkcyjnego ze spike'u. Tylko wnioski.

---

## 7. Anty-wzorce, których unikamy

| Anty-wzorzec | Symptom | Lekarstwo |
|--------------|---------|-----------|
| **„Wezmę po prostu wszystko"** | Sprint z 15 storyek | Zostaw 5; reszta wraca do ROADMAP. |
| **Estymata „M" dla wszystkiego** | Niewiarygodne planowanie | Wymuś S/M/L/XL z definicją czasową. |
| **TODO bez issue** | Wieczne fragmenty | Reguła `60-docs.mdc`; CI grep blokuje. |
| **„Zrobię refactor przy okazji"** | PR rośnie do 800 LoC | Wydziel osobny task z linkiem. |
| **„Test napiszę później"** | Nigdy nie powstaje | TDD przy logice krytycznej (skill enforced). |
| **Sprint bez retro** | Powtarzamy te same błędy | Skill `retro` jest blokerem zamknięcia sprintu. |

---

## 8. Lista sprintów (live)

| Sprint | Temat | Status | Plan | Retro |
|--------|-------|--------|------|-------|
| 00 | Bootstrap (repo, CI, tooling) | ✅ Done | [sprint-00-bootstrap.md](sprint-00-bootstrap.md) | [2026-05-22-sprint-00.md](../retros/2026-05-22-sprint-00.md) |
| 01 | Foundations (config + secrets) | ✅ Done | [sprint-01-foundations.md](sprint-01-foundations.md) | [2026-05-23-sprint-01.md](../retros/2026-05-23-sprint-01.md) |
| 02 | SSH + status cache | ✅ Done | [sprint-02-ssh-cache.md](sprint-02-ssh-cache.md) | [2026-05-23-sprint-02.md](../retros/2026-05-23-sprint-02.md) |
| 03 | Provider abstraction + `small.pl` skeleton | ✅ Done | [sprint-03-provider-smallhost.md](sprint-03-provider-smallhost.md) | [2026-05-23-sprint-03.md](../retros/2026-05-23-sprint-03.md) |
| 04 | TUI shell (MVU, navigation, dashboard) | ✅ Done | [sprint-04-tui-shell.md](sprint-04-tui-shell.md) | [2026-05-23-sprint-04.md](../retros/2026-05-23-sprint-04.md) |
| 05 | Wizard tworzenia projektu (LIFO rollback) | ✅ Done | [sprint-05-wizard-project.md](sprint-05-wizard-project.md) | [2026-05-23-sprint-05.md](../retros/2026-05-23-sprint-05.md) |
| 06 | GitHub deploy workflow | ✅ Done (z carry-over do 07) | [sprint-06-github-deploy-workflow.md](sprint-06-github-deploy-workflow.md) | [2026-05-23-sprint-06.md](../retros/2026-05-23-sprint-06.md) |
| 07 | Import + Doctor GitHub + deploy polish | 📅 Planned | [sprint-07-import-doctor-polish.md](sprint-07-import-doctor-polish.md) | — |
| 08 | **Bento Ultra Layout Engine + OKLCH theme + Sprint-leak cleanup** (po [ADR-0007](../adr/0007-bento-ultra-eskalacja-mvp.md)) | 📝 Planned | [sprint-08-bento-ultra.md](sprint-08-bento-ultra.md) | — |
| 09 | **Live Log Stream + Header Bar Server Metrics** | 📝 Planned | [sprint-09-live-log-stream.md](sprint-09-live-log-stream.md) | — |
| 10 | **Live CI/CD Pipeline Panel** | 📝 Planned | [sprint-10-cicd-panel.md](sprint-10-cicd-panel.md) | — |
| 11 | **Live Service Topology Map** | 📝 Planned | [sprint-11-topology-map.md](sprint-11-topology-map.md) | — |
| 12 | Polish, beta release, RC1 → v0.1 | 📝 Planned | [sprint-12-polish-release.md](sprint-12-polish-release.md) | — |
| 13 | RC1 hardening + Surface foundation | 📝 Planned | [sprint-13-v01-ga-and-post-mvp-foundation.md](sprint-13-v01-ga-and-post-mvp-foundation.md) | — |
| 14 | Architecture hardening (post-RC, pre-v0.2) | 📝 Planned | [sprint-14-architecture-hardening.md](sprint-14-architecture-hardening.md) | — |
| 15 | **Launch Readiness** — README EN, asciinema scaffold, generator szkieletu adaptera, walkthrough PROVIDER.md, AGENTS.md slim, repo polish, 5 launch-day issues | ✅ Done (2026-05-25) | [sprint-15-launch-readiness.md](sprint-15-launch-readiness.md) | [2026-05-25-sprint-15.md](../retros/2026-05-25-sprint-15.md) |
| 16 | **Public Launch + cPanel Research** — *event sprint*: launch-day attendance (Reddit / r/golang / r/selfhosted / Show HN). Ops-prep (README EN / asciinema / cPanel test account) zostały zlane do Sprintu 21. | 🔁 Reduced (event-only, post-Sprint 22) | [sprint-16-public-launch.md](sprint-16-public-launch.md) | — |
| 17 | ~~cPanel Adapter MVP (part 1)~~ — superseded przez Sprint 21 po renumeracji post-Sprint-19 | ⛔ Superseded → Sprint 21 | [sprint-17-cpanel-adapter.md](sprint-17-cpanel-adapter.md) (historical) | — |
| 18 | ~~cPanel Adapter (part 2) + v0.2.0-rc1~~ — superseded; treść wraca jako Sprint 22+ | ⛔ Superseded → Sprint 22+ | [sprint-18-cpanel-polish.md](sprint-18-cpanel-polish.md) (historical) | — |
| 19 | **Preset Registry Foundation** — JSON-schema `assets/provider-presets/`, `webox doctor preset` CLI, 6 initial presets, ADR-0008 (TUI Provider Catalog deferred to Sprint 20+) | ✅ Done (out-of-cadence — 2026-05-25) | [sprint-19-preset-registry.md](sprint-19-preset-registry.md) | [retros/2026-05-25-sprint-19.md](../retros/2026-05-25-sprint-19.md) |
| 20 | **TUI Polish & Provider Catalog** — click hit-testing, Provider Catalog screen (carry-over of TASK-19.4), Standard mode redesign, Project Detail tabs 2/3 unstub, Help screen overhaul, chrome-hint cleanup (post-Sprint-19 operator feedback) | ✅ Done (2026-05-25) | [sprint-20-tui-polish-and-catalog.md](sprint-20-tui-polish-and-catalog.md) | [retros/2026-05-25-sprint-20.md](../retros/2026-05-25-sprint-20.md) |
| 21 | **cPanel Adapter (part 1) + Public Launch Prep** — UAPI client (TASK-21.1), SSH fallback (TASK-21.2), `webox doctor cpanel` (TASK-21.3), `webox doctor preset --probe` (TASK-21.4), README EN (TASK-21.5), asciinema cast (TASK-21.6), cPanel test account (TASK-21.7 → deferred). | ✅ Done (2026-05-25, 6/8 tasks; TASK-21.7 → Sprint 22) | [sprint-21-cpanel-adapter-prep.md](sprint-21-cpanel-adapter-prep.md) | [retros/2026-05-25-sprint-21.md](../retros/2026-05-25-sprint-21.md) |
| 22 | **cPanel Adapter (part 2) + v0.2.0-rc1** — mutating ops (`createProject`, `restartApp`, `addSSLDomain`), wizard integration, GHA template, E2E, fixtures from live test account (TASK-21.7 carry-over), release. | 📝 Planned | [sprint-22-cpanel-adapter-mutations.md](sprint-22-cpanel-adapter-mutations.md) | — |
| 23 | **Second Provider — DirectAdmin read-only adapter (Path A chosen)** — Live API client + SSH loopback fallback + Composite, `webox doctor directadmin`, preset graduated research → candidate. Path B (CyberPanel) deferred to v0.4+, Path C (Public Launch) deferred until v0.2.0 GA. | ✅ Done (2026-05-27, 6/6) | [sprint-23-second-provider-or-launch.md](sprint-23-second-provider-or-launch.md) | [2026-05-27-sprint-23.md](../retros/2026-05-27-sprint-23.md) |

**Etapy projektu:**
- **Sprints 00-12** — MVP delivery (v0.1).
- **Sprints 13-14** — Architecture hardening + v0.1 GA.
- **Sprint 15** — Launch readiness (post-MVP, pre-public; głównie nie-kod).
- **Sprint 16** — Public launch (operator-time, blokowane do TUI confidence — patrz Sprint 20).
- **Sprint 19** — Preset registry → product differentiator („Webox zna Twój hosting"). Doszedł out-of-order (2026-05-25).
- **Sprint 20** — TUI Polish & Provider Catalog (carry-over 19.4 + post-Sprint-19 operator feedback fixes).
- **Sprint 21** — cPanel Adapter MVP part 1 + Public Launch Prep (parallel tracks; closed 2026-05-25, 6/8 tasks; TASK-21.7 → Sprint 22).
- **Sprint 22** — cPanel Adapter part 2 + v0.2.0-rc1 (mutating ops, wizard, GHA template, E2E, live-account fixtures, release).
- **Sprint 23** — Path A chosen: DirectAdmin read-only adapter shipped (Live API + SSH loopback + Composite + `doctor directadmin`). Closed 2026-05-27 (6/6); Path B (CyberPanel) deferred v0.4+, Path C (Public Launch) deferred to v0.2.0 GA. Mutating adapter → Sprint 24 (needs live test account).

Sprinty 08–11 dodane do MVP po [ADR-0007](../adr/0007-bento-ultra-eskalacja-mvp.md) (2026-05-23) — eskalacja Bento Ultra + live logs + GHA panel + topology z STRETCH (v0.2+) do MVP (v0.1).
Sprinty 15-20+ dodane 2026-05-25 — launch readiness + cPanel adapter + preset registry, na podstawie strategicznej decyzji post-Sprint 14 review.
Sprint 20 dodany 2026-05-25 — TUI polish odpowiada na operator feedback po Sprint 19 release-candidate ("nawigacja, skalowanie, scrolling, klikanie wszystko niedopracowane"). Renumber 17→21 keeps cPanel adapter logical sequence intact.
Sprint 22 plan dopisany 2026-05-25 po zamknięciu Sprintu 21 — uchwyca carry-over TASK-21.7 + mutating ops planowane jako v0.2.0-rc1.
Sprint 23 plan dopisany 2026-05-25 jako decision-doc — trzy ścieżki (DirectAdmin / CyberPanel / Public Launch) do wyboru na retro Sprintu 22.

---

## 9. Kiedy zatrzymujemy implementację

**Twarde przyczyny stopu (sprint pauza):**

1. Krytyczny security finding (P0/P1) — najpierw fix, potem dalej.
2. > 2 carry-overów z poprzedniego sprintu — zła estymacja, planowanie do naprawy.
3. Brak energii / motywacji — uczciwie zatrzymaj zegar, ustal next checkpoint w retro.
4. Zewnętrzny blocker (zmiana w API `small.pl`, regulacje) — spike + decyzja.

**Miękkie sygnały do reflexji:**

- Velocity spada o > 50% — sprawdź czy nie ma narastającego długu technicznego.
- Coverage spada — TDD discipline się rozjeżdża.
- PR-y rosną — wracamy do małych jednostek.

---

## 10. Linki

- **Skills:**
  - `.cursor/skills/tdd-loop/SKILL.md`
  - `.cursor/skills/commit-policy/SKILL.md`
  - `.cursor/skills/retro/SKILL.md`
  - `.cursor/skills/audit-trace/SKILL.md`
- **Rules:**
  - `.cursor/rules/00-charter.mdc`
  - `.cursor/rules/50-tests.mdc`
- **Docs:**
  - [`ROADMAP.md`](../ROADMAP.md)
  - [`AUDIT.md`](../AUDIT.md) (źródło findingów P0-P3 + IMP-*)
  - [`RISKS.md`](../RISKS.md)
- **GitHub:**
  - `.github/pull_request_template.md`
  - `.github/ISSUE_TEMPLATE/`

---

_Last reviewed: 2026-05-25. Reviewer: project owner. Trigger: Sprint 15 zamknięcie (Launch Readiness) — generator + README EN + repo polish ready dla v0.1.0 GA. Następny rewizor: po v0.1.0 release._
