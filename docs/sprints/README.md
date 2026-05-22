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
| 00 | Bootstrap (repo, CI, tooling) | 📅 Planned | [sprint-00-bootstrap.md](sprint-00-bootstrap.md) | — |
| 01 | Foundations (config + secrets) | 📅 Planned | [sprint-01-foundations.md](sprint-01-foundations.md) | — |
| 02 | SSH + status cache | 📝 Outlined | — | — |
| 03 | Provider abstraction + `small.pl` skeleton | 📝 Outlined | — | — |
| 04 | TUI shell (MVU, navigation, dashboard) | 📝 Outlined | — | — |
| 05 | Wizard tworzenia projektu (LIFO rollback) | 📝 Outlined | — | — |
| 06 | GitHub deploy workflow | 📝 Outlined | — | — |
| 07 | Doctor + diagnostics + i18n | 📝 Outlined | — | — |
| 08 | Polish, beta release, RC1 | 📝 Outlined | — | — |

Sprinty 02-08 dostaną pełne plany w trybie rolling-wave, po zakończeniu poprzedniego.

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

_Last reviewed: 2026-05-22. Reviewer: project owner._
