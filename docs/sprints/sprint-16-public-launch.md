# Sprint 16 — Public Launch + cPanel Research

> **Daty:** TBD (po Sprint 15) → +12-14 dni · **Czas:** ~20-25h (kod) + 8-10h launch-day attendance
>
> **Cel:** wystrzelić projekt publicznie *raz, dobrze* (Reddit + r/golang + r/selfhosted + Hacker News) **i** zacząć cPanel od strony researchowej **przed** zakupem hostingu testowego. Po tym sprincie albo mamy pierwszego kontrybutora i sygnał życia, albo wiemy dokładnie, że pozycjonowanie wymaga zmiany.

---

## TL;DR

Sprint 15 zostawia repo gotowe do publikacji: nowy README EN, asciinema, generator szkieletu adaptera, walkthrough kontrybutora, slim AGENTS.md. Sprint 16 to **tygodniowa kampania launchowa** plus realny zakup test-accountu cPanel i pierwsze ślady kodu cPanel adapter (research-only, **bez** implementacji metod HostingProvider).

Klucze:

- **Pre-warming PL community (tydzień 1):** Discord small.pl, r/Polska, r/programming.pl, 4programmers, wykop, grupy FB. Target: 15-30 stars, 1-3 zgłoszone bugi, 1 realna konwersacja.
- **Show HN + r/golang + r/selfhosted tego samego dnia (tydzień 2):** środa rano US-East. 6h attendance pod komputerem.
- **Partnerships outreach (równolegle):** mail do CTO/community small.pl/lh.pl, follow-up po 7 dniach.
- **cPanel test account purchase:** HostArmada lub Krystal (rekomendowane — cheap cPanel + CloudLinux Node.js Selector + 30-day refund). Inwestycja ~10-15 USD/mc.
- **`docs/providers/cpanel.md` research expansion:** real-world findings z test account (uapi outputs, paths, Application Manager state). Bez kodu cPanel adapter w `providers/cpanel/`.

**Nie robimy w tym sprincie:**

- Implementacja cPanel HostingProvider methods — Sprint 17.
- Drugi launch po HN (jeśli pierwszy słaby) — retrospekcja + iteracja w Sprint 17.
- Email marketing / newsletter — post-Sprint 18 jeśli sensowny.
- Discord server uruchamianie — tylko jeśli ≥ 20 osób eksplicytnie poprosi.

---

## Pre-flight Checklist

- [x] Sprint 15 zakończony, TASK-15.1, 15.3, 15.4, 15.6, 15.7, 15.9 ✅ (closed 2026-05-25, [retro](../retros/2026-05-25-sprint-15.md)).
- [x] `README.md` EN merged (136 linii, all-absolute links).
- [x] `webox provider new` generator end-to-end działa (verified: `go build ./...` po `webox provider new test_provider --preset cpanel-uapi`).
- [ ] **Carry-over z Sprint 15 TASK-15.2** — operator nagrywa `assets/demo/demo.cast` przez `bash scripts/record-demo.sh` (wymaga 120×35 terminal + asciinema + expect). Tu dopiero asciinema URL może trafić do README.
- [ ] **Carry-over z Sprint 15 TASK-15.2** — operator robi capture `assets/screenshots/dashboard.png` przez `bash scripts/capture-screenshot.sh` (wymaga `agg` + `ffmpeg` lub manual screenshot).
- [ ] **Carry-over z Sprint 15 TASK-15.5** — native-speaker / Grammarly Premium pass na `landing/en/index.html` body. Decyzja kiedy flipnąć `https://webox.dev/en/` deployment — patrz Sprint 16 retro.
- [ ] `docs/contributing/PROVIDER.md` przeczytany przez znajomego dev (poza Twoim wzrokiem) — feedback udokumentowany.
- [ ] `.local/strategy/go-to-market.md` zaktualizowany konkretnymi datami launch days.
- [ ] `.local/partnerships/small-lh-devilweb-outreach.md` — szablon maila gotowy.
- [ ] `.local/strategy/reddit-launches.md` + `hn-show-draft.md` — drafty post-by-post po review.
- [ ] **Decyzja do Sprint 16 retro:** czy `01-cpanel-skeleton.md` issue zostanie zostawiony do community, czy maintainer sam dostarcza skeleton w Sprint 17 żeby seed-ować ekosystem? Patrz [Sprint 15 retro Open questions](../retros/2026-05-25-sprint-15.md).

---

## Taski

### TASK-16.1 — Soft launch w polskiej społeczności (Week 1)

- **Estymata:** L (10-15h rozłożone na 5 dni)
- **Zależności:** Sprint 15 closed.
- **Acceptance Criteria:**
  - [ ] **Day 1 (Poniedziałek):** Discord small.pl + lh.pl (jeśli dostępne) — krótki post w `#showcase` / `#projekty`. Bez agresji, „Stawiałem na small.pl projekty i zrobiłem narzędzie, które mi pomaga — może komuś z was też się przyda. Feedback mile widziany."
  - [ ] **Day 1:** Grupa FB **JavaScript Polska** / **Frontend Polska** / **DevOps Polska** — pojedynczy post z asciinema.
  - [ ] **Day 2 (Wtorek):** `r/Polska` (lub `r/Polska_Programowanie` jeśli istnieje) — wieczorny post, tytuł „Stawiasz aplikacje na shared hostingu? Zbudowałem terminalowy cockpit (open source)."
  - [ ] **Day 3 (Środa):** `4programmers.net` — wątek w „Projekty użytkowników".
  - [ ] **Day 3:** Wykop — link + 2-3 zdania (skromnie, bez hype).
  - [ ] **Day 4-5:** monitorowanie + odpowiadanie na każdy komentarz **w ciągu 2h**.
  - [ ] **Day 5:** Outreach DM do **3 polskich tech-influencerów Twitter/X** (programowanie + DevOps) z prośbą o feedback (nie share, tylko feedback).
  - [ ] Metryki Day 7:
    - Stars: cel 15-30 (P50: 20).
    - Issue'y: cel 1-3 bugi lub feature request.
    - Forks: cel 2-5.
    - Discord/X realne konwersacje: ≥ 1.
- **Notatki:**
  - **NIE** publikuj na HN w pierwszym tygodniu. To zmarnowanie najmocniejszego strzału.
  - Jeśli post na r/Polska zbierze < 10 upvotów po 24h — to nie znaczy że produkt jest zły. Sub jest trudny dla self-promo, próbuj r/Polska_Programowanie albo r/learnprogramming PL.
  - Nie cross-postuj identyczego contentu. Każdy kanał ma własny pitch.

### TASK-16.2 — Public launch: Show HN + r/golang + r/selfhosted (Week 2, jeden dzień)

- **Estymata:** L (full-day attendance: 6-8h)
- **Zależności:** TASK-16.1 zamknięty (mamy social proof: starsy, issue'y, forks).
- **Acceptance Criteria:**
  - [ ] **Day chosen:** **Środa, 9:00-11:00 EST** (15:00-17:00 PL). Statystycznie najlepszy slot HN front-page. Czwartek 2nd best.
  - [ ] **T-2h (13:00 PL):** Sanity check: repo accessible, README renders, asciinema plays, all links work.
  - [ ] **T-0 (15:00 PL):** Post na Show HN — tytuł exact z draftu `.local/strategy/hn-show-draft.md`. Body ≤ 1500 chars. Pierwszy komentarz od Ciebie (3 min po publikacji) — explanation **what it is + why I built it + ask for feedback**.
  - [ ] **T+15min:** Reddit r/golang post. Inny tytuł, inny angle (technical: „Show: Webox — Provider Pattern + Bubble Tea TUI for shared hosting").
  - [ ] **T+30min:** Reddit r/selfhosted post. Jeszcze inny: „Built a TUI to manage shared hosting deployments (Apache-2.0, open-source)."
  - [ ] **T+45min:** Lobste.rs post (tylko jeśli masz invite).
  - [ ] **T+1h:** Soft outreach do 2-3 osób z pre-warming community (TASK-16.1) z prośbą o **konkretny komentarz** (nie upvote — komentarz z pytaniem / bug report / user testimonial). HN algorithm waży aktywność w komentarzach.
  - [ ] **Sześć następnych godzin:** odpowiadasz na **każdy** komentarz, nawet hostile, w ciągu 20 min. Bez wyjątku. **To często decyduje czy projekt łapie.**
  - [ ] Metryki T+24h:
    - HN: cel front page (przynajmniej 30 min). P50: 30-80 upvotów, 5-20 komentarzy.
    - r/golang: cel 50+ upvotów, 10+ komentarzy.
    - r/selfhosted: cel 30+ upvotów (sub mniejszy ale lojalny).
    - GitHub: cel +50-150 stars combined.
  - [ ] Metryki T+72h:
    - GitHub stars combined: cel 100-300 (P50: 150).
    - New issues / PR: cel 3-8.
    - Email/DM od potencjalnych kontrybutorów: cel 1-3.
- **Notatki:**
  - **Nie publikuj jeśli masz < 25 stars po Week 1.** Show HN przy 5 starsach to spalanie najmocniejszego shotu. Raczej przesuń o tydzień + zrób więcej outreach.
  - **Hostile komentarze:** „This already exists" → odpowiedź z konkretną różnicą (Provider Pattern + small.pl scope). „Why not just use Coolify?" → odpowiedź „Coolify jest dla VPS / Docker. Webox jest dla shared hostingu — ta nisza nie ma dobrych narzędzi." Nie obraź się, nie tłumacz długo, **be specific**.

### TASK-16.3 — Partnership outreach: small.pl/lh.pl/DevilWeb (równolegle z TASK-16.2)

- **Estymata:** S (4-6h)
- **Zależności:** TASK-16.2 partial (potrzebujemy mieć już social proof do pokazania — minimum 50 stars).
- **Acceptance Criteria:**
  - [ ] Research: zidentyfikuj CTO / Community Manager / partner contact w **H88 S.A.** (owner small.pl/lh.pl/devilweb). LinkedIn + oficjalny `contact@` + Twitter/X handle.
  - [ ] Draft maila gotowy w `.local/partnerships/small-lh-devilweb-outreach.md`. Treść:
    - Otwarcie: „Hej, zbudowałem open-source TUI dla developerów używających waszego hostingu."
    - Social proof: GitHub URL + asciinema + 100+ stars (jeśli HN poszło dobrze).
    - Konkret value: „Wasi power-userzy (devs + freelancerzy + agencje) dostają cockpit-replacement dla 5 paneli."
    - Ask: „Czy bylibyście zainteresowani: a) wzmianką w docs/blog o webox jako partnerze, b) test account lh.pl, żebym dodał cPanel preset dla was, c) wstępną rozmową o partnerships?"
    - Nieagresywne CTA. Brak płatności w komunikacji.
  - [ ] Wyślij **najpóźniej Piątek tydzień 2** (4-5 dni po Show HN, jak masz peak social proof).
  - [ ] Follow-up po 7 dniach jeśli brak odpowiedzi.
  - [ ] Outcome zapisany w `.local/partnerships/small-lh-devilweb-outreach.md` z timestamps.
- **Notatki:** **To jest Twój jeden najmocniejszy partnership-asset.** lh.pl ma cPanel — pozytywna odpowiedź daje Ci free cPanel test environment + marketing reach.

### TASK-16.4 — cPanel test account purchase + initial setup

- **Estymata:** S (2-3h)
- **Zależności:** TASK-16.2 done (potrzebujesz social proof żeby uzasadnić wydatek).
- **Acceptance Criteria:**
  - [ ] Wybór hostera (recommendation tier):
    - **HostArmada Start Dock** (~3-5 USD/mc) — cPanel, CloudLinux, Node.js Selector, 45-day refund.
    - **Krystal Lite** (~7 USD/mc) — cPanel, AutoSSL, UK-based, fast support.
    - **A2 Hosting Lite** (~2 USD/mc) — cPanel, ale slow Node.js Selector wsparcie.
  - [ ] Zakup minimum 3-miesięcznego okresu (żeby się nie spieszyć z fixturami).
  - [ ] SSH access enabled + key uploaded.
  - [ ] cPanel API token wygenerowany (manualnie w panelu) — **NIE** zapisany w repo.
  - [ ] Node.js Selector / Application Manager — sprawdzić co jest dostępne.
  - [ ] Test deploy: stwórz dummy subdomenę `test.<user>.<host>`, deploy „Hello world" Express app, SSL via AutoSSL.
  - [ ] Notatki dostępu (poza repo): `.local/notes/2026-XX-XX-cpanel-test-account.md` z URL + user (bez hasła / tokena).
- **Notatki:** **Refund policy:** wszystkie 3 mają 30+ dni money-back. Jeśli okazuje się że hoster wycina UAPI access dla cheap planów — refund + spróbuj innego. Test API access **PRZED** wyjściem z refund window.

### TASK-16.5 — `docs/providers/cpanel.md` expansion z real-world findings

- **Estymata:** M (5-8h)
- **Zależności:** TASK-16.4 (potrzebujemy test account).
- **Acceptance Criteria:**
  - [ ] Recapture UAPI outputs dla wszystkich potrzebnych endpointów (manualne, przez SSH `uapi ... --output=json`):
    - `Version get_version`
    - `PassengerApps list_applications`
    - `PassengerApps create_application` (subdomain + Node version)
    - `Tokens list`
    - `Mysql list_databases`
    - `Mysql create_database`
    - `SSL installed_hosts`
    - `Domains get_domains`
    - `SubDomain addsubdomain`
  - [ ] Sanityzacja outputów (usuń user-specific data, paths zostają z `<user>` placeholder).
  - [ ] Captures w `testing/fixtures/cpanel/` jako `.txt` + `.fixture.md` z metadata: hoster, plan, cPanel version, data capture, sanitization steps.
  - [ ] Update `docs/providers/cpanel.md`:
    - Sekcja **Verified findings** (vs `TO BE VERIFIED`) — co potwierdzone real-world.
    - Tabela mapowania metod `HostingProvider` → konkretne UAPI calls.
    - Edge cases zaobserwowane: empty response shape, error response shape, rate limit (cPanel ma `429 Too Many Requests`?).
    - Restart workflow: `passenger-config restart-app <path>` vs `touch ~/<app>/tmp/restart.txt` — który działa na tym konkretnym hosterze.
  - [ ] Update `docs/providers/preconfiguration-vision.md §5.2` — preset `cpanel-<hoster>` z `status: candidate` (mamy fixture'y).
- **Notatki:** **Brak kodu** w `providers/cpanel/` w tym sprincie. To research-only. Implementacja w Sprint 17.

### TASK-16.6 — Launch retrospekcja + sprint 17 re-planning

- **Estymata:** S (2-3h)
- **Zależności:** TASK-16.1-16.5 closed.
- **Acceptance Criteria:**
  - [ ] `docs/retros/<data>-sprint-16-launch.md` — szczegółowa retro launch'u, z metrykami i wnioskami:
    - HN front-page achieved: TAK / NIE
    - Pierwszy zewnętrzny PR: TAK / NIE
    - Outreach replies: count
    - Partnership response (small.pl/lh.pl/H88): pending / positive / negative
    - Topowe pytania/objections w komentarzach (insighty do FAQ na README/landing)
  - [ ] Decyzja w retro: scenariusz **A (success, 100+ stars)** → Sprint 17 cPanel implementation z full focus. Scenariusz **B (mid, 30-80 stars)** → Sprint 17 mix cPanel + content marketing (dev.to artykuł). Scenariusz **C (low, < 30 stars)** → reflection: positioning issue albo timing → 2 tygodnie content + iteracja README.
  - [ ] Update `docs/ROADMAP.md` ETA — jeśli launch dał kontrybutorów dla cPanel, ETA na cPanel skraca się.
- **Notatki:** **Bądź uczciwy w retro.** Nie zaokrąglaj „świetnie poszło" jeśli było średnio. To Ty będziesz planował na podstawie tych liczb.

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| Show HN flop (< 30 upvotów, brak front page). | H | Mamy soft launch PL tydzień 1 jako bezpiecznik. Jeśli flop — retro analiza + przygotowanie do iteracji w Sprint 17. |
| Hostile / dismissive komentarze pierwsze 30 min HN → snowball negatywności. | M | **Bądź pod kompem.** Każdy komentarz < 20 min. Nie tłumacz się długo, bądź konkretny. Mamy `hn-show-draft.md` z Q&A prep. |
| Partnership outreach milcz całkowicie (H88 nie odpowiada). | M | Nie blokujemy roadmap'y. Sprint 17 cPanel idzie niezależnie. Follow-up Sprint 18. |
| cPanel test account ma UAPI disabled → research blocked. | M | Refund window. Spróbuj alternatywnego hostera (lista 3 opcji). |
| Pierwszy zewnętrzny kontrybutor zaczyna cPanel PR i blokuje moją iterację. | M | Pair-review commitment z TASK-15.3 — paruj się eksplicytnie. Plus opcja: kontrybutor robi DirectAdmin (innego), ja robię cPanel — równolegle. |
| Burnout po 8h launch day. | M | Następny dzień **off**. Sprint 16 ma świadomie zarezerwowane 8-10h non-coding launch day. |

---

## Dependencies signoff

- Asciinema już jest (Sprint 15).
- HostArmada / Krystal / A2 Hosting — ~10-15 USD wydatek własny (cPanel test account).
- Brak nowych Go dependencies.
- Brak nowych GitHub Actions.

---

## Outcome (wypełnij po sprincie)

- ✅ Done: ...
- ⏭️ Carry-over → Sprint 17: ...
- 📌 Decyzje:
  - HN front-page achieved: TAK / NIE (z metrykami)
  - Partnership traction: ...
  - Sprint 17 scenario: A / B / C (patrz TASK-16.6)
- 🧠 Surprises:
  - Comments / objections topowe: ...
  - Unexpected positive feedback: ...
- 📊 Metryki:
  - GitHub stars T+72h: ?
  - GitHub forks: ?
  - New issues / PRs: ?
  - Email/DM from interested contributors: ?
  - HN points / comments: ?
  - Reddit r/golang upvotes / comments: ?
  - Reddit r/selfhosted upvotes / comments: ?
- 🔒 Security validation:
  - [ ] cPanel test account token **nigdzie** w repo, **nigdzie** w `.local/`, **nigdzie** w fixture'ach.
  - [ ] Test account user / host **nigdzie** w repo poza placeholder.
- ➡️ Następny sprint: `sprint-17-cpanel-adapter.md`

---

## Retro Link

`docs/retros/<data>-sprint-16-launch.md` (to jest **najważniejsza** retro projektu)
