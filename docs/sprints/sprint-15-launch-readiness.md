# Sprint 15 — Launch Readiness (post-MVP, pre-public)

> **Daty:** TBD (po zamknięciu Sprint 14) → +10 dni roboczych · **Czas:** ~25-30h skupienia
>
> **Cel:** zamienić technicznie zielone v0.1 GA w **prezentowalny produkt OSS**, gotowy do publicznego launchu na Reddicie / r/golang / r/selfhosted / Hacker News / promocji partnerskich. Sprint **głównie nie-kod** (60% docs/marketing, 40% kod scaffoldingowy obniżający próg wejścia kontrybutora).

---

## TL;DR

Sprint 14 zamknął architecture hardening i daje stabilną bazę pod v0.1 GA. Sprint 15 odpowiada na pytanie:

> *„Repo jest zielone — i co teraz?"*

Bez tego sprintu można wystrzelić puste pole (Show HN przy 0 starsów, README po polsku, brak demo, brak ścieżki dla kontrybutora). Z tym sprintem mamy:

- **Nowy README EN** w strukturze konwertującej (hero/GIF → why → 30s try → 4h add provider → architecture → status → contributors call).
- **Asciinema z `--mock`** (45 s) jako pojedynczy najmocniejszy asset marketingowy + jeden statyczny PNG snapshot.
- **`webox provider new <name>`** generator szkieletu adaptera — dramatycznie obniża próg wejścia kontrybutora.
- **`docs/contributing/PROVIDER.md`** — walkthrough EN „add a provider in 4 hours" oparty na `smallhost` jako wzorcu.
- **`landing/en/`** statyczna kopia obecnego polskiego landingu, manualne tłumaczenie + Apache-2.0 reflected wszędzie.
- **5 issue'ów `good-first-issue`** z konkretnymi tematami (cPanel skeleton, DA skeleton, CyberPanel research, scaffolding template, EN→DE translation).
- **Docs refactor** — `AGENTS.md` z 619 linii do ~140 (mapa dokumentacji), wydzielenie `docs/dependencies.md`, `docs/conventions.md`, `docs/gotchas.md`.
- **Repo polish** — Apache-2.0 license consistency, `FUNDING.yml`, sprawdzenie publicznych docs pod kątem private notes / wewnętrznych ścieżek.

**Nie robimy w tym sprincie:**

- Drugi provider (cPanel/DirectAdmin) — research dopiero w Sprint 16.
- Public launch — Sprint 16 (potrzebujemy 1 tydzień soft-launch w PL community przed HN).
- Preset registry (`assets/provider-presets/`) — Sprint 19.
- Migracja landing PL → CMS / dynamic site — landing pozostaje static, EN to manualna kopia.
- AI / ML anomaly detection. Plugin marketplace. Telemetria.

---

## Pre-flight Checklist

- [ ] Sprint 14 zakończony, `v0.1.0` GA wytagowane LUB `v0.1.0-rc2` z świadomą decyzją.
- [ ] `make ci` + `make bench-check` zielone na `main`.
- [ ] `--mock` mode działa w pełnym Bento Ultra (Sprint 08) i można nim nawigować (dashboard → detail → wizard → import → CI/CD modal → logs).
- [ ] `landing/index.html` PL otwiera się lokalnie i renderuje demo poprawnie.
- [ ] `LICENSE` to Apache-2.0 (poprawione 2026-05-25). Wszystkie referencje `MIT` w docs zaktualizowane.

---

## Taski

### TASK-15.1 — README.md (root) — przepisanie EN według struktury konwertującej

- **Estymata:** L
- **Zależności:** TASK-15.2 (asciinema URL), TASK-15.3 (CONTRIBUTING EN link), TASK-15.5 (landing EN URL).
- **Acceptance Criteria:**
  - [ ] `README.md` jest **wyłącznie EN**. PL wersja: opcjonalna `README.pl.md` (nie wymagana w Sprint 15).
  - [ ] Struktura sekcji w tej kolejności:
    1. **Hero** — jedna linia tagline + asciinema badge + statyczny PNG fallback w `assets/screenshots/dashboard.png` (z TASK-15.2).
    2. **Why** — 1 akapit, 3-4 zdania, słowa-magnesy: `cPanel`, `DirectAdmin`, `shared hosting`, `Node.js`, `terminal`.
    3. **Try it in 30 seconds** — `git clone ... && make build && ./bin/webox --mock`. Bez configu, bez SSH.
    4. **What you can do today** — bullet list ficzerów v0.1 (smallhost only, z eksplicyt note „one verified provider, more coming").
    5. **Add your hosting in 4 hours** — link do `docs/contributing/PROVIDER.md`, `webox provider new <name>` quick demo, **linia „Pair-review available: open an issue or DM"**.
    6. **Architecture highlights** — 5 bulletów (Provider Pattern, MVU Bubble Tea, SSH pool + SWR cache, keyring + AES-GCM fallback, atomic config + JSON Schema + 86%+ coverage).
    7. **Status & roadmap** — explicit: „v0.1: small.pl. v0.2: cPanel. v0.3: DirectAdmin." z linkiem do `docs/ROADMAP.md`.
    8. **Contributing** — link do `CONTRIBUTING.md` + `docs/contributing/PROVIDER.md` + `good-first-issue` filter URL.
    9. **License & credits** — Apache-2.0 + ack dla Bubble Tea / charmbracelet / small.pl.
  - [ ] Bez sekcji „Testimonials" (nie mamy), bez `go install` instrukcji (lokalny build z release w v0.1 GA), bez listy feature'ów >10.
  - [ ] Wszystkie linki są **absolute** (działają na GitHub i w fork'ach).
  - [ ] Spell-check + grammar pass (Grammarly / LanguageTool). Pre-merge: native EN speaker review jeśli dostępny, inaczej self-review po 24h.
- **Docs:** `landing/why-webox.md` jako źródło punktów do **Why** sekcji.
- **Notatki:** zostaw obecny PL README jako `README.pl.md` z notką „kept for Polish community, English README is canonical."

### TASK-15.2 — Asciinema z `--mock` + statyczny PNG snapshot

- **Estymata:** M
- **Zależności:** none (mock mode istnieje od Sprint 04).
- **Acceptance Criteria:**
  - [ ] Skrypt `scripts/record-demo.sh` z `asciinema rec --idle-time-limit=1.5 demo.cast` — recorduje 45-60 s **scripted scenariusza**:
    1. Start dashboard (`--mock` mode).
    2. Tab przez kafelki (5 sek).
    3. Wybór projektu (`shop-ease`) → detail panel.
    4. Otwarcie CI/CD modal → przewinięcie pipeline steps.
    5. `tail -f` logów (Live Log Stream tile, 3 zmieniające się linie).
    6. Esc → Topology Map.
    7. `q` quit.
  - [ ] Plik `demo.cast` w `assets/demo/demo.cast` (LFS jeśli > 100kB, inaczej commit prosto).
  - [ ] Hosting publiczny: upload do asciinema.org → embed badge URL w README.
  - [ ] **Statyczny PNG fallback** w `assets/screenshots/dashboard.png` (1280×800, capture frame 8 sek demo). Render przez `asciinema-svg` + screenshot tool **albo** real terminal screenshot (preferred — bardziej autentyczny).
  - [ ] Drugi PNG `assets/screenshots/wizard.png` — capture wizard step 3 (subdomain + Node version).
  - [ ] OG image dla landingu: `landing/og-image.png` (1200×630) — zaprojektowany z PNG + nazwą produktu (Figma / lokalnie w Inkscape).
- **Notatki:**
  - **Nie kopiuj demo z `landing/index.html`** — landing HTML mock jest CSS, nie autentyczny terminal. Asciinema musi być z **prawdziwej `./bin/webox --mock`**.
  - Jeśli mock mode nie pokazuje topology/CICD płynnie — najpierw mikro-fix `--mock` (timer rotuje statusy co 3 s, deterministycznie).
  - Asciinema 3.0+ (jeśli dostępne) ma bezpieczniejszy format. Stary 2.0 też OK.

### TASK-15.3 — CONTRIBUTING.md EN translation + dedicated PROVIDER walkthrough

- **Estymata:** M
- **Zależności:** TASK-15.4 (generator scaffold — żeby walkthrough referował realne komendy).
- **Acceptance Criteria:**
  - [ ] `CONTRIBUTING.md` (root level) — **NOWY** plik EN, ≤ 200 linii, struktura:
    1. Welcome + Code of Conduct link.
    2. Setup (5 komend, max).
    3. Branching + commit convention (Conventional Commits).
    4. PR checklist (link do `.github/pull_request_template.md`).
    5. „Three ways to contribute":
       - Provider adapter (link do `docs/contributing/PROVIDER.md`).
       - Translation (link do `docs/contributing/TRANSLATIONS.md` — przyszła wersja).
       - Bug fix / small feature (link do `good-first-issue`).
    6. Maintainer SLA + response time.
  - [ ] `docs/CONTRIBUTING.md` (existing PL/mixed) — pozostaje jako szczegółowy reference dla PL community + dodać note na górze „This is the legacy detailed guide. For onboarding see ../CONTRIBUTING.md."
  - [ ] `docs/contributing/PROVIDER.md` — **NOWY** plik (osobny task 15.4 lub 15.3-piggyback). Walkthrough 4-godzinny:
    - Step 0: Decide preset vs adapter (link do `docs/providers/preconfiguration-vision.md §3`).
    - Step 1: `webox provider new <name>` (generator z 15.4).
    - Step 2: Walk through `providers/<name>/skeleton.go` — 9 methods, 3 najważniejsze pierwsze.
    - Step 3: Fixtures (`testing/fixtures/<panel>/`) — kopia ze smallhost.
    - Step 4: Parsers (TDD: malicious fixture first).
    - Step 5: Integration test (sshmock).
    - Step 6: PR + maintainer pair-review request.
    - „Difficulty levels" badge: 🟢 cPanel UAPI (mainstream), 🟡 DirectAdmin (mixed API), 🔴 CyberPanel (root concerns).
- **Notatki:** Pisz pod osobę, która **nie zna Go** w ogóle, ale potrafi czytać dokumentację. To jest target persona: web-dev pisząca w PHP/Node, która chce dodać adapter.

### TASK-15.4 — `webox provider new <name>` generator szkieletu

- **Estymata:** M
- **Zależności:** none (rozszerza obecne `cmd/webox/`).
- **Acceptance Criteria:**
  - [ ] Nowa subkomenda `webox provider new <name> [--preset cpanel-uapi|directadmin|cyberpanel|blank]`.
  - [ ] Template w `assets/provider-template/` (z `//go:embed`):
    - `provider.go` — skeleton z `init()` + factory + 9 metod **z TODO komentarzami** odsyłającymi do `docs/contributing/PROVIDER.md` sekcje.
    - `provider_test.go` — table-driven test z 1 failing test case (TDD red).
    - `parsers.go` — pusty z TODO.
    - `parsers_test.go` — table-driven test + 2 fixture entries (TODO).
    - `doc.go` — pakiet doc.
    - `testing/fixtures/<name>/.fixture.md` — instrukcja pochodzenia + sanityzacji.
  - [ ] Komenda generuje pliki w `providers/<name>/`, edytuje `providers/imports.go` (sortowane imports), printuje walkthrough next steps z linkiem do `docs/contributing/PROVIDER.md`.
  - [ ] Sanity check: po `webox provider new test_provider && go build ./...` → buduje się.
  - [ ] **Unit tests** dla generatora (`cmd/webox/provider_new_test.go`): nazwa walid, conflict z istniejącym, błędny preset.
- **Notatki:** to jest **najmocniejszy pojedynczy magnes na kontrybutorów**, ważniejszy niż README marketing. Większość OSS nie ma takiego scaffoldingu — to widoczna różnica.

### TASK-15.5 — Landing EN translation (statyczna kopia)

- **Estymata:** M
- **Zależności:** none.
- **Acceptance Criteria:**
  - [ ] `landing/en/index.html` — manualne tłumaczenie pliku PL.
  - [ ] Wszystkie meta tagi z `og:locale="en_US"`, `lang="en"`, `hreflang` linki cross-pointują.
  - [ ] Schema.org JSON-LD: `"license": "https://www.apache.org/licenses/LICENSE-2.0"` (poprawić też w PL wersji — obecnie mówi MIT).
  - [ ] Brak `landing/en/` w `.gitignore` (chcemy commit-ować wersję EN, ale całe `/landing` jest gitignorowane — **zmiana .gitignore**: tylko `landing/dist/` jeśli ma build process). **Decyzja:** zostawić cały `/landing` w `.gitignore`, deployment landingu od-coupled od repo (Cloudflare Pages / Vercel pulluje osobne repo lub git submodule). Wystarczy upewnić się że EN landing istnieje **lokalnie** + zostaje wgrany do hostingu landingu.
  - [ ] Translation acceptance: native speaker (lub Grammarly Premium) review na 100 % długości. Polish idioms („Czekam na v0.1") → EN equivalents („Waiting for v0.1").
  - [ ] Hreflang switcher (PL / EN) w nav działa obustronnie.
- **Notatki:**
  - Jeśli landing pivotuje do EN-only post-launch (Twoja deklaracja) — w Sprint 15 robimy obie wersje, a w Sprint 16+ wyłączamy PL z deployment, ale plik zostaje.
  - **Nie pakuj** landing demo HTML do README — demo HTML jest interactive, React-by-CSS, nie da się embed w GitHub Markdown. Demo z TASK-15.2 (asciinema) ma inny target.

### TASK-15.6 — Good-first-issue list (5 issue'ów)

- **Estymata:** S
- **Zależności:** TASK-15.3 (PROVIDER walkthrough musi być dostępny — issue do niego referuje).
- **Acceptance Criteria:**
  - [ ] Issue 1: **Add cPanel adapter skeleton (Provider Pattern reference)**. Label: `good-first-issue`, `provider`, `help wanted`. Body: link do PROVIDER walkthrough + difficulty badge + lista konkretnych metod do zaimplementowania + „Reach out before starting — I'll pair on first PR."
  - [ ] Issue 2: **Add DirectAdmin adapter skeleton**. Identyczny szablon, badge 🟡.
  - [ ] Issue 3: **Research CyberPanel API for Phase 1 capabilities**. Badge 🔴, body: link do `docs/providers/cyberpanel.md`, lista pytań z `TO BE VERIFIED`.
  - [ ] Issue 4: **Add scaffolding template: Next.js**. Body: gdzie żyją templates (`assets/scaffolding/`), wzór z Vite-React, sample acceptance test.
  - [ ] Issue 5: **Translate dialog labels to DE (or ES, FR — pick one)**. Body: link do `translations/`, `make i18n-check`, sample diff.
  - [ ] Wszystkie issue'y EN. Każdy ma „Estimated time" header (cPanel: 4-8h, DA: 4-6h, CyberPanel research: 2-3h, Next.js: 1-2h, translation: 1h).
  - [ ] Repo settings: label `good-first-issue` (zielony), `help wanted` (purpurowy), `provider` (orange), `research` (yellow), `documentation` (blue).
- **Notatki:** Te 5 issue'ów są Twoim public scoreboard. Zamknięte issue → social proof. Nie otwieraj 30 issue'ów — 5 fokus konwertuje lepiej.

### TASK-15.7 — Repo hygiene & public-readiness audit

- **Estymata:** S
- **Zależności:** none.
- **Acceptance Criteria:**
  - [ ] Audit `docs/AUDIT.md` — sprawdź czy nie ma open findings z frazą „attacker can exploit X by Y" bez referencji do mitigacji. Zamknij lub przenieś do `.local/notes/`.
  - [ ] Audit `docs/retros/*.md` — usunąć osobiste wzmianki (kłótnie, frustracje, godziny pracy). Przenieść takie wersje do `.local/notes/` z timestamp.
  - [ ] Audit `docs/MIGRATION_NOTES.md` — usunąć absolute paths (`/Users/seba/...`).
  - [ ] `.github/FUNDING.yml` — dodać GitHub Sponsors / Ko-fi placeholder (puste linki OK, pokazuje commitment).
  - [ ] `.github/ISSUE_TEMPLATE/provider_request.yml` — **NOWY** template (form-based): Provider name / API docs URL / SSH availability / Node runtime / target market / contributor commitment level (just suggesting / want to implement myself / want to be co-maintainer).
  - [ ] Sprawdź wszystkie pliki w `docs/` pod kątem `localhost:` / `127.0.0.1` / `s1.small.pl` z hardcoded credentials → zamień na placeholder.
  - [ ] `grep -ri "MIT" docs/ README.md .github/ landing/` → wszystkie occurence → Apache-2.0 (wyjątek: gdy cytujemy bibliotekę używającą MIT).
- **Notatki:** Repo jest publiczne, więc to robimy w mode „assume zero leaks ever occurred, then verify".

### TASK-15.8 — v0.2 backlog freeze + Sprint 16+ pre-planning

- **Estymata:** S
- **Zależności:** TASK-15.1-15.6 done.
- **Acceptance Criteria:**
  - [ ] `docs/sprints/sprint-16-public-launch.md` zaktualizowane jeśli okazało się że Sprint 15 zostawia carry-over.
  - [ ] `docs/ROADMAP.md` — odświeżona estymata v0.2 z uwzględnieniem 1-2 tygodni launch readiness (sprint 15) + 1-2 tygodni public launch (sprint 16).
  - [ ] `docs/sprints/README.md` — tabela sprintów dodaje 15, 16, 17, 18, 19, 20+ z linkami.
  - [ ] CHANGELOG `[Unreleased]` entry — sekcja `Added` z launch readiness scaffolding.
  - [ ] Decision check-in: czy `webox provider new` (TASK-15.4) zostaje w mainline binary, czy idzie do `webox-dev` osobnej komendy (dev-only)? Decyzja w sprint outcome.

### TASK-15.9 — Docs refactor: `AGENTS.md` slim + 3 nowe wydzielenia

- **Estymata:** M
- **Zależności:** none (czysty docs refactor).
- **Acceptance Criteria:**
  - [ ] `docs/dependencies.md` — **NOWY**, zawiera tabelę z `AGENTS.md §1.2` (kluczowe biblioteki). Header: „This is the authoritative library catalog. For decision rationale see linked ADRs."
  - [ ] `docs/conventions.md` — **NOWY**, zawiera `AGENTS.md §5` (Go naming, error handling, context, generics, logging) + `§6` (commits, PR structure). Header: link z `AGENTS.md` jako primary entry.
  - [ ] `docs/gotchas.md` — **NOWY**, zawiera `AGENTS.md §7` (Top 12 pułapek). Wersja EN dla kontrybutorów + PL fragment dla nas. Każdy entry: anti-pattern code snippet + correct code snippet.
  - [ ] `AGENTS.md` skrócony do **≤ 150 linii**. Nowa struktura:
    1. TL;DR (10 linii).
    2. Non-negotiables (15 linii — link do `.cursor/rules/00-charter.mdc`).
    3. Documentation map — tabela „question → doc" (50 linii).
    4. Workflow (15 linii — link do `docs/conventions.md`).
    5. Scope discipline (10 linii — link do ROADMAP §3.3).
    6. Decision policy (15 linii).
    7. Skills reference (10 linii).
  - [ ] Wszystkie linki w nowym `AGENTS.md` działają (relative paths).
  - [ ] Bez utraty informacji — każdy guardrail z obecnego `AGENTS.md` ma odpowiednik w nowych plikach lub w `.cursor/rules/`.
- **Notatki:** **To jest jedyna implementacja-zmiana procesowa w Sprint 15.** Agent kodowania działa szybciej i precyzyjniej z krótkim `AGENTS.md` (≤150 linii) niż z 619-liniowym dokumentem.

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| README pierwsze wrażenie nie przekonuje → 0 stars na HN. | H | Asciinema (TASK-15.2) musi być **rzeczywiście dobry** — nagraj 3 takes, wybierz najlepszy. Test na 3 osobach (kolega z r/programming.pl + 1 dev EN + 1 niedeveloper) przed merge. |
| Generator (TASK-15.4) rozjeżdża się z faktyczną strukturą `providers/smallhost/` po Sprint 16 (kiedy zaczynamy cPanel). | M | Wersjonowanie templates (`assets/provider-template/v1/`). Sprint 17 audit czy template wymaga updateu. |
| Audit (TASK-15.7) ujawnia coś co już wycieklo publicznie. | M | Repo było publiczne od Sprint 06+, więc nie ma sekretów (sprawdziłem). Ale `docs/retros/` mogą mieć subiektywne wzmianki — przegląd manualny. |
| Apache-2.0 vs MIT change nie jest zgłoszony wystarczająco głośno → community confusion. | L | CHANGELOG `Changed` entry + commit message + landing site footer + `LICENSE` header — cztery miejsca. |
| Native speaker EN review nie znaleziony przed merge. | M | LanguageTool Premium 30-day trial + 2× self-review po 24h. Acceptable trade-off w v0.1 — w v0.2 można podszlifować. |

---

## Dependencies signoff

Sprint 15 może wymagać:

- `asciinema` (system tool, nie Go dep) — przyjmujemy że dev maintainer ma go zainstalowany lokalnie.
- Brak nowych zależności Go.
- Brak nowych GitHub Actions.

**Nowe zależności:** zero.

---

## Outcome (wypełnij po sprincie)

- ✅ Done: ...
- ⏭️ Carry-over → Sprint 16: ...
- 📌 Decyzje:
  - README hero converts? TAK / NIE (z liczbą stars w pierwsze 7 dni Sprint 16)
  - Generator scaffold mainline binary vs `webox-dev`: ...
  - EN-only landing post-launch: TAK / NIE
- 🧠 Surprises: ...
- 📊 Metryki:
  - README hero conversion (heat map jeśli landing analytics): ?
  - `webox provider new` generator working end-to-end: ?
  - `docs/contributing/PROVIDER.md` tested by 3rd party (znajomy dev): ?
- 🔒 Security validation:
  - [ ] `govulncheck` zielony (regression — nie powinno być nic).
  - [ ] Repo audit (TASK-15.7) zamknięty.
  - [ ] Brak path absolute w docs.
- ➡️ Następny sprint: `sprint-16-public-launch.md`

---

## Retro Link

`docs/retros/<data>-sprint-15.md` (do utworzenia po zakończeniu sprintu)
