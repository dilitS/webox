# Webox — Risk Register

> **Status:** Living document · **Owner:** Project owner · **Last review:** 2026-05-22
>
> _„Risk is what happens when the variance has a name."_

Lista ryzyk dla MVP (`v0.1`) i ścieżki do `v1.0`. Każde ryzyko ma **prawdopodobieństwo × impact = score (1-25)**, **mitigation** (proaktywne) i **contingency** (gdy się ziści).

## Skala

| | Likelihood | Impact |
|-|------------|--------|
| 1 | < 10% | Cosmetic |
| 2 | 10-30% | Slowdown < 1 tyg |
| 3 | 30-60% | Slowdown 1-2 tyg |
| 4 | 60-85% | Slowdown 3-4 tyg lub scope cut |
| 5 | > 85% | Project blocker |

**Score** = L × I. **Threshold do active monitoring:** ≥ 9. **Threshold do escalation (pauza sprintu):** ≥ 16.

---

## Aktywne ryzyka (score ≥ 9)

### R-001 — Solo maintainer burnout

- **Kategoria:** People/Process
- **Likelihood:** 4 · **Impact:** 5 · **Score:** 20 🔴
- **Opis:** Webox to znacznie większy projekt niż przeciętny side-project (16-20+ tyg estymata solo). Burnout jest najbardziej prawdopodobnym powodem porażki, nie kod.
- **Wczesne sygnały:**
  - Velocity spada o > 30% w 2 kolejnych sprintach.
  - Coverage spada (TDD discipline tracona).
  - Retra robione „bo trzeba", nie z refleksji.
  - 0 commitów > 7 dni bez planowanej pauzy.
- **Mitigation:**
  - **Hard limit: 20h/tydzień skupienia.** Nie 40.
  - Sprint planning zawsze ≤ 5 storyek M lub 3 L.
  - **Zaplanowane pauzy** co 4 sprinty (1 tydzień bez kodu).
  - Retro pytanie: „czy mam ochotę na następny sprint?" — odpowiedź „nie" = pauza.
- **Contingency:**
  - **Plan B-1:** Scope cut — wycinamy GitHub Actions workflow (zostaje SFTP), GA criteria łagodzimy.
  - **Plan B-2:** Open-source wcześnie (po sprincie 04 — TUI shell). Zaproś 1-2 trusted devs do crit ścieżki.
  - **Plan B-3:** Pauza > 4 tyg → ADR „project hibernacja, restart w Q3/Q4".
- **Status:** Open, monitorowane co retro.

---

### R-002 — `small.pl` panel niestabilny / API zmiany

- **Kategoria:** External dependency
- **Likelihood:** 3 · **Impact:** 5 · **Score:** 15 🟠
- **Opis:** `small.pl` (Devil panel) nie ma kontraktu API. Cała integracja opiera się na SSH + parsowaniu CLI output. Operator może zmienić output format między release'ami.
- **Wczesne sygnały:**
  - Fixture'y w `testdata/devil/` aging > 30 dni bez recapture.
  - User reports „dziwne błędy parsera".
- **Mitigation:**
  - **Defensive parsing** (skill `audit-trace`, reguła `30-providers.mdc`): każdy parser ma fixture-based testy i waliduje strukturę przed użyciem.
  - **Monthly live fixture recapture** przez maintainera (`AUDIT §8 IMP-16`, `TESTING.md §3.3`).
  - Telemetria parser errorów (opt-in, `v0.3+`).
  - **Defensive defaults** — gdy parser nie umie zinterpretować linii, zwraca `ErrUnknownFormat` z hintem do issue + raw output do logu (redacted).
- **Contingency:**
  - **Plan B-1:** Provider abstraction (już zaplanowana) pozwala wstrzymać `small.pl` i rozwijać `cyberpanel`/`directadmin` jako primary.
  - **Plan B-2:** Negocjuj z `small.pl` o stabilny CLI contract (mały hosting, możliwe).
- **Status:** Open. Pierwszy capture fixtures w Sprint 02 lub 03.

---

### R-003 — Crypto code bug (AES-GCM / Argon2id)

- **Kategoria:** Security
- **Likelihood:** 2 · **Impact:** 5 · **Score:** 10 🟠
- **Opis:** `secrets/fallback.go` (Sprint 01) jest **handmade crypto** — nawet biblioteczny (`crypto/aes`, `crypto/cipher`, `argon2id`), użycie wymaga precyzji. Bug nonce reuse = total break.
- **Wczesne sygnały:**
  - Code review przesunięty „na później".
  - `gosec` warnings ignorowane.
  - Brak testu na CSPRNG failure.
- **Mitigation:**
  - **TDD twarde** w TASK-01.7 (test na nonce uniqueness, CSPRNG panic, round-trip).
  - **Code review przez 2-gie oczy** — w solo modzie: 7-dniowy cooldown przed mergem (świeże oczy).
  - `gosec ./secrets/...` na każdym PR.
  - Wymagana referencja do `docs/SECURITY.md §4.2.1` w komentarzu kodu.
  - **External crypto audit przed v1.0** (zlecony lub trusted reviewer).
- **Contingency:**
  - **Plan B-1:** Jeśli audit wykryje issue → CVE process, security advisory, force update binary, password rotation flow w-app.
  - **Plan B-2:** Replace handmade z `age` (golang.design/age) — straci się fallback portability na rzecz dojrzałej biblioteki.
- **Status:** Open, blocker dla Sprint 01 retro.

---

### R-004 — Estymata 16 tyg jest nierealistyczna (rzeczywiście 24+)

- **Kategoria:** Scope/timeline
- **Likelihood:** 4 · **Impact:** 3 · **Score:** 12 🟠
- **Opis:** Historyczna dokładność estymat solo-devów: P50 = 1.5×, P90 = 2.5× pierwotnej estymaty. 16 tyg może oznaczać 24-40.
- **Wczesne sygnały:**
  - Każdy z pierwszych 3 sprintów ma > 25% carry-over.
  - Spike taski rosną w ROADMAP.
- **Mitigation:**
  - **Sprint capacity = 20h/tydzień** (already accounted).
  - **Honest velocity tracking** — w retro zapisujemy faktyczny vs estymata.
  - Po sprincie 03 (mamy 3 punkty danych) — **re-baseline ROADMAP** z policzonym mnożnikiem.
- **Contingency:**
  - **Plan B-1:** Scope cuts (in priority order):
    1. GitHub Actions workflow → manual SFTP (akceptowalne dla MVP).
    2. Drift detection cosmetic (zostaje tylko domains).
    3. i18n → tylko EN.
    4. `webox doctor` ograniczony do 3 checków.
  - **Plan B-2:** Public beta wcześniej (po sprincie 04), zewnętrzne feedback przyspiesza priorytetyzację.
- **Status:** Open, re-baseline po Sprint 03.

---

### R-005 — Bubble Tea MVU rozjeżdża się w prawdziwym życiu

- **Kategoria:** Technical/Architecture
- **Likelihood:** 3 · **Impact:** 3 · **Score:** 9 🟡
- **Opis:** MVU pattern jest piękny w teorii. Real-world TUI z asynchronicznym I/O (SSH, GitHub API) wymaga ostrożnego zarządzania `tea.Cmd` i obsługi cancellation. Łatwo o leaky goroutines lub deadlocks.
- **Wczesne sygnały:**
  - Goroutine leak w `go test -race`.
  - „Spinner kręci się wiecznie" w demo.
- **Mitigation:**
  - Reguła `20-bubbletea-mvu.mdc` (cancellation, no globals, `tea.Tick`).
  - Wszystkie `tea.Cmd` z `context.Context` (propagowany z model).
  - `teatest` snapshoty na critical screens (skill `tdd-loop`).
  - **Spike przed sprintem 04** (4h) — proof-of-concept screen z SSH integration.
- **Contingency:**
  - **Plan B-1:** Hybrid model — pure MVU dla UI state, dedicated state machine (`looplab/fsm`) dla flow.
  - **Plan B-2:** Wrap heavy async w `services/` z pub/sub channelem, MVU tylko subscribe-and-render.
- **Status:** Open, spike planowany w Sprint 03 retro lub Sprint 04 planning.

---

### R-006 — Polish-only docs blokuje community contributors

- **Kategoria:** Scope/process
- **Likelihood:** 5 · **Impact:** 2 · **Score:** 10 🟠
- **Opis:** `ADR-0006` zatwierdził PL dla docs strategicznych, ale „contributor surface" musi być EN. Tłumaczenie jest drogie czasowo (~1-2 tyg). Bez EN — żadnego community provider w K6.
- **Wczesne sygnały:**
  - 0 external PR-ów po publicznym ogłoszeniu.
  - Issue „can you provide english docs?" pojawia się > 3 razy.
- **Mitigation:**
  - **Contributor surface** musi być EN przed public launch:
    - `README.md` (już EN-ish).
    - `CONTRIBUTING.md` (translate w Sprint 07).
    - `docs/providers/<template>.md` (translate w Sprint 03).
    - `AGENTS.md` (zostaje PL — internal).
- **Contingency:**
  - **Plan B-1:** K6 (community provider) → relaksacja: ocena po 3 mies. zamiast hard gate na v1.0.
  - **Plan B-2:** Auto-translate przez DeepL z DISCLAIMER „machine translated, verify".
- **Status:** Open, action w Sprint 07.

---

### R-007 — GitHub fine-grained PAT scope flaki

- **Kategoria:** Security / External
- **Likelihood:** 3 · **Impact:** 3 · **Score:** 9 🟡
- **Opis:** Fine-grained PAT-y są w GitHub w stałym rozwoju. Scope „Administration: write" w przyszłości może wymagać dodatkowych pól, lub format tokenów się zmieni.
- **Wczesne sygnały:**
  - GitHub API zwraca 403 z message „insufficient scopes" przy operacjach, które działały.
- **Mitigation:**
  - Wrapper `services/github/auth.go` z explicit scope check przed każdą operacją.
  - Sentinel error `ErrInsufficientScopes` z linkiem do docs.
  - **Test integration live** (manual, off-CI) — sprawdza scope każdego PAT-a przed głównymi release'ami.
- **Contingency:**
  - **Plan B-1:** Fallback do classic PAT (ze ścisłą instrukcją bezpieczeństwa).
  - **Plan B-2:** GitHub App zamiast PAT — `v0.3+` migracja.
- **Status:** Open, action w Sprint 06.

---

### R-008 — SSH host key TOFU UX is confusing

- **Kategoria:** UX / Security
- **Likelihood:** 4 · **Impact:** 2 · **Score:** 8 🟡 (sub-threshold, ale active)
- **Opis:** Trust-On-First-Use jest standardem, ale mismatch flow w v0.1 (TUI phrase-confirm) może być błędnie interpretowany jako security FUD i odstraszać.
- **Wczesne sygnały:**
  - User reports „nie mogę dodać projektu, mówi że host key changed".
  - Discord/discussion: „why is this scary?".
- **Mitigation:**
  - Bardzo jasny copy w `docs/SECURITY.md §5.4` + link w-app.
  - Phrase-confirm message tłumaczy DLACZEGO (nie tylko CO).
  - `docs/UX.md` ma section dedykowaną.
- **Contingency:**
  - **Plan B-1:** Dodaj `--accept-host-key-fingerprint <fingerprint>` flag dla power users — `v0.2`.
  - **Plan B-2:** Quick-rotate UX: jeden ekran z fingerprint compare side-by-side.
- **Status:** Open, monitoring w beta.

---

### R-009 — Single tester syndrome (no external user reports)

- **Kategoria:** Process / QA
- **Likelihood:** 4 · **Impact:** 3 · **Score:** 12 🟠
- **Opis:** Solo dev = solo tester = brak różnorodności środowisk. Możemy nie złapać bugów specyficznych dla:
  - macOS Sonoma vs Sequoia
  - terminal emulatorów (iTerm, Ghostty, Alacritty, Kitty)
  - Polish UTF-8 w nazwach katalogów
  - SSH `~/.ssh/config` z exotycznymi opcjami
- **Wczesne sygnały:**
  - Coverage jest dobre, ale issue queue eksploduje po publicznym ogłoszeniu.
- **Mitigation:**
  - **Public alpha po sprincie 04** (TUI shell działa) — invite 3-5 zaufanych.
  - CI matrix: linux + macOS minimum.
  - **Snapshot tests** dla TUI (`teatest`) — chronią przed regressions.
  - `docs/providers/<template>.md` ma sekcję „known issues per OS".
- **Contingency:**
  - **Plan B-1:** Beta period przed v1.0 wydłużony.
  - **Plan B-2:** Bug bounty (small) na trusted-tester program.
- **Status:** Open, action w Sprint 04.

---

### R-010 — Dependency rot (security CVE w go-keyring, memguard, etc.)

- **Kategoria:** Security
- **Likelihood:** 3 · **Impact:** 3 · **Score:** 9 🟡
- **Opis:** Każda dependency to potential CVE. `memguard`, `go-keyring`, `bubble tea`, `lipgloss`, `crypto/...` — wszystkie wymagają monitoringu.
- **Mitigation:**
  - **Dependabot enabled** na repo (PR weekly).
  - `make vulncheck` w CI.
  - Pinned versions w `tools.go`.
  - **Quarterly dep review** — wpisany w retro skill (skill `release-check` ma punkt na dep audit).
- **Contingency:**
  - **Plan B-1:** Security advisory issued + force update binary.
  - **Plan B-2:** Vendor critical deps do `vendor/` (Go module mirror lock).
- **Status:** Open, recurring.

---

## Monitorowane ryzyka (score 6-8)

### R-011 — Documentation drift after first refactor

- **Likelihood:** 3 · **Impact:** 2 · **Score:** 6 🟢
- Skill `audit-trace` + rule `60-docs.mdc` minimalizują.
- Contingency: monthly „docs sync" sprint.

### R-012 — Cosign signing complications

- **Likelihood:** 2 · **Impact:** 3 · **Score:** 6 🟢
- Mitigation: dry-run w Sprint 00, live signing dopiero w Sprint 08.
- Contingency: SHA256-only checksums + GPG fallback.

### R-013 — `flock(2)` Windows port

- **Likelihood:** 2 · **Impact:** 2 · **Score:** 4 🟢
- Mitigation: Build tags, MVP = unix-only.
- Contingency: Windows wsparcie → v0.3+ (acceptable).

---

## Zamknięte / retired

_Brak (project pre-MVP)._

---

## Process

1. **Każdy sprint planning** zaczyna się od przeglądu RISKS.md — czy są nowe sygnały?
2. **Każdy sprint retro** kończy się aktualizacją (nowe ryzyka, zmiany score, retirements).
3. **Risk score ≥ 16** = wymaga **eskalacji** (pauza sprintu, ADR, scope re-baseline).
4. **Nowe ryzyko** dodawane przez:
   - User report.
   - Incydent (np. parse failure na produkcji).
   - Audit finding.
   - Discovery w retro.

---

## Linki

- [`docs/AUDIT.md`](AUDIT.md) — historia findingów.
- [`docs/retros/`](retros/) — szczegóły z retr.
- [`docs/sprints/README.md`](sprints/README.md) — proces planowania.
- `.cursor/skills/retro/SKILL.md` — aktualizacja RISKS.md jako część retro DoD.
