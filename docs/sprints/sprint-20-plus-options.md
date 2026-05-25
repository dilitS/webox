# Sprint 20+ — Decision Matrix (post-v0.2 path selection)

> **Status:** **Decision document, not a sprint plan** · **Trigger:** Sprint 19 retro
>
> Po Sprint 19 mamy v0.2.0 (lub rc2) z dwoma adapterami + preset registry. Sprint 20 nie jest jeden z góry zaplanowany sprint — to **jedna z trzech ścieżek**, wybierana na podstawie sygnałów z community po Sprint 18-19. Ten dokument opisuje opcje i kryteria wyboru, żeby decyzja była eksplicytna, nie domyślna.

---

## Sygnały do oceny (zebrane z Sprint 16-19 retro)

| Sygnał | Źródło | Próg dla ścieżki A | Próg dla ścieżki B | Próg dla ścieżki C |
|---|---|---|---|---|
| GitHub stars total | `gh api repos/dilitS/webox` | ≥ 500 | 100-500 | < 100 |
| Otwarte issue'y z external contributors | GitHub Issues | ≥ 5 | 2-5 | < 2 |
| External PRs (merged + pending) | GitHub PRs | ≥ 2 merged | 1 merged or pending | 0 |
| Partnership traction (small.pl/lh.pl/H88) | `.local/partnerships/` | Pozytywna odpowiedź | Otwarta rozmowa | Cisza / odmowa |
| Realne usage feedback (issue'y z konkretami) | Issues z `bug` label | ≥ 3 | 1-2 | 0 |
| External press / blog posts | Twitter mentions, Google Alerts | ≥ 1 niezależny | 0 | 0 |

---

## Ścieżka A — DirectAdmin adapter (community-driven)

**Kiedy:** ≥ 1 z poniższych:

- Pojawia się community-contributor zainteresowany DirectAdmin (issue #2 z Sprint 15-task-15.6).
- Co najmniej 2 user feedback'i typu „mam DirectAdmin, kiedy będzie wsparcie?".
- Partnership z hosterem DirectAdmin (HITME / DotRoll / NODEA) jest na stole.

**Scope sprint 20-21:**

- `providers/directadmin/` adapter — analogicznie do cPanel.
- Decyzja: legacy CMD API vs nowe `/api/*` (capability detection).
- 1-2 inicjalne presety (CloudLinux Selector wariant + Nginx Unit wariant).
- **Parowanie z kontrybutorem jeśli się zgłosił** — Twoja rola: review + spec, kontrybutor: kod. Sprint 22+: integracja + verified status.

**Plus:**

- Tempo: ~2-3 tygodnie z kontrybutorem, ~5-6 tygodni solo.
- Walidacja, że Provider Pattern wytrzymuje trzeci adapter.
- Trzeci verified provider = mocniejszy claim w README.

**Minus:**

- DirectAdmin jest trudniejszy niż cPanel (różne API generacje + CloudLinux selector wymaga UI complexity).
- Może rozszerzyć scope za szybko (zostawiamy bugfix queue z v0.2).

---

## Ścieżka B — OAuth GitHub Device Flow + Quality Polish

**Kiedy:**

- Stars 100-500, ale **pierwszy user feedback wskazuje friction w setup** (PAT confusion, `gh` CLI dependency).
- Brak community contributors, ale są realni users.
- Partnership negocjacje aktywne — chcemy „polish the rough edges" zanim partner zobaczy.

**Scope sprint 20-21:**

- **OAuth Device Flow** dla GitHub auth — eliminujesz PAT/gh CLI dependency (większa wartość user-friction reduction niż drugi adapter).
- `webox auth login github` interactive flow.
- `services/github/oauth.go` z `golang.org/x/oauth2`.
- Migracja config v3 (`auth.github.method = oauth | pat | gh_cli`).
- Bug fixes z v0.2 i community feedback.
- README polish + screenshots refresh + asciinema update.

**Plus:**

- Quality compounds — łatwiejszy onboarding = więcej conversions + adoptions.
- Mniejszy scope risk niż adapter (znamy GitHub API).
- Daje czas community na pierwsze adaptery DirectAdmin / CyberPanel (issue'y otwarte).

**Minus:**

- Nie zwiększa „provider count" w README (mniej spektakularne marketing).
- Może być nudne — łatwo zostawić w środku.

---

## Ścieżka C — Repositioning + Content Marketing

**Kiedy:**

- Stars < 100 po Sprint 19, niska traction, brak contributors.
- HN i Reddit posty były ignored albo dismissed.
- Partnership outreach nieproduktywny.

**Scope sprint 20-21 (głównie nie-kod):**

- **Reflection sprint:** retro całego launchu (Sprint 15-19), analiza komentarzy HN/Reddit.
- Identify positioning issue: czy „shared hosting TUI" jest niedostatecznie precyzyjne? Czy „Bubble Tea cockpit" przyciąga niewłaściwą publikę?
- **Long-form dev.to / Medium articles** (2-3 sztuki):
  - „Why I built a TUI for shared hosting (instead of using Coolify)"
  - „Building a Provider Pattern in Go for a hosting tool"
  - „Lessons from a failed Show HN (and what I'm doing next)"
- **Twitter/X content series** — 1 post/tydzień, screencasts, architecture mini-deep-dives.
- **Re-engagement community PL** — sprawdzić jeśli post-v0.2 daje nową kartę.
- **Bug bash + UX polish** — najlepszy sygnał life dla projektu, którego nikt nie używa.

**Plus:**

- Brutal honesty z sobą — nie planujesz na puste pole.
- Content marketing buduje search traffic długoterminowo.
- Daje czas na drugi launch (v0.3) z lepszym pitch'em.

**Minus:**

- Łatwo zsunąć w „I'll fix the marketing" przez 3 miesiące i nie pisać kodu.
- Wymaga akceptacji że pierwszy launch nie poszedł.

---

## Kryterium decyzji (eksplicyt)

W retro Sprint 19:

1. Wypełnij metryki z tabeli „Sygnały do oceny".
2. Sprawdź który próg jest spełniony (ścieżka A / B / C).
3. **Jeśli nie ma jednoznacznej ścieżki** — wybierz **B (OAuth + Polish)** jako default. To ścieżka średniego ryzyka, najbardziej kumulatywna.
4. Zapisz decyzję w `.local/notes/<data>-sprint-20-decision.md` + odpowiedni sprint-20 detailed plan.

---

## Co po Sprint 20-21? (rough horizon)

- **Sprint 22-23:** Trzeci adapter (DirectAdmin albo CyberPanel — drugi z par kandydatów, wybór jeszcze nie zamknięty).
- **Sprint 24:** Community capture kit (`webox doctor provider-capture`) — `.zip` z sanitized probe outputs do GH Issue jako contribution flow.
- **Sprint 25:** v1.0 readiness review — czy mamy 3+ verified providery, 1+ community-maintained, stabilne API.

Te są **horizon, nie plan**. Sprint 19 retro odswieża.

---

## Jak NIE używać tego dokumentu

- **Nie traktuj jak roadmap.** To branching point, nie linia.
- **Nie blokuj sprint 20 startu czekając aż „znajdziesz idealne kryterium"**. Wybierz w 1 godzinę po retro, później iteruj.
- **Nie ukrywaj ścieżki C przed sobą.** Jeśli liczby mówią C — idź C. Brutalna uczciwość = projekt przetrwa.
