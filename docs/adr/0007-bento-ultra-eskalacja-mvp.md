# ADR-0007: Eskalacja Bento Ultra + Live Logs + GHA Panel + Topology Map do MVP v0.1

> Status: Accepted · Data: 2026-05-23 · Właściciel: @maintainer · Reviewers: @maintainer
>
> Pokrewne ADR: [ADR-0001 TUI zamiast CLI](./0001-tui-zamiast-cli.md), [ADR-0003 Provider Pattern](./0003-provider-pattern.md), [ADR-0005 Cache statusów](./0005-cache-statusow-projektow.md). Dokumenty: [PRD §6](../PRD.md#6-ficzery--z-priorytetami), [UX TL;DR](../UX.md#tldr), [ROADMAP §3](../ROADMAP.md#3-mvp-v01--wy%C5%82%C4%85cznie-smallpl), [AGENTS §2.4](../AGENTS.md#24-scope-discipline).

## Kontekst

PRD §3 i [UX.md TL;DR](../UX.md#tldr) definiują Webox jako **„Terminal Cockpit klasy premium"**. Pierwotne planowanie MVP (v0.1) świadomie ograniczyło warstwę prezentacji do **Standard Cockpit 100×30** i zaklasyfikowało jako 🔶 **STRETCH (v0.2+)** cztery elementy doświadczenia premium:

1. **Bento-Box Grid (`≥ 120×35`)** — wielokafelkowy layout z OKLCH theming i dynamicznym layeringiem.
2. **Live Log Stream** — `tail -f` przez SSH z ring bufferem w UI (F14 w [PRD §6](../PRD.md#6-ficzery--z-priorytetami)).
3. **Deployment Monitor (GitHub Actions live panel)** — live status workflow runs i logów job-ów (F15 w PRD).
4. **Live Service Topology Map** — wizualny graf zależności (graph GitHub → Server → App ← DB).

Decyzja o STRETCH była uzasadniona timeboxem (P50 = 22 tygodni). Po ukończeniu Sprintów 04–06 (read-only TUI shell, wizard + LIFO rollback, GitHub deploy workflow + częściowy `services/github`) i przejrzeniu pierwszych zrzutów ekranu produktu z perspektywy persony A z [PRD §4.1](../PRD.md#41-persona-a--marek-freelancer-z-520-projektami-na-smallpl) maintainer ocenił, że **MVP w obecnym kształcie wizualnym nie spełnia brand promise** z PRD/UX TL;DR i nie udowodni kryterium **K1** (czas pierwszego wow-effectu) z [PRD §8](../PRD.md#8-kryteria-sukcesu--mierzalne).

Kluczowe presje, które zmieniły bilans decyzji:

- Backend wizualnie premium kafelków jest **już dostarczony**: `services/github/` ze Sprintu 06 obsługuje pollowanie GHA runs, SSH connection pool ze Sprintu 02 wspiera long-lived sesje (`tail -f`), `status/` ze Sprintu 02 ma SWR cache z TTL-ami pod metrics. CI/CD Panel i live logs to ~5–10 dni nadbudowy na istniejącej infrastrukturze, nie nowe pakiety od zera.
- Brand promise „Standard Cockpit jako minimum" zostawia early adopters z wrażeniem proof-of-concept, podczas gdy v0.2+ planuje dorzucić premium UX dopiero po pierwszej fali ocen — to **odwrotna kolejność** wobec marketing physics open-source TUI tooli (`lazygit`, `k9s`, `bottom` budowały reputację na premium pierwszej impresji).
- Guardrail [`AGENTS.md §2.4`](../AGENTS.md#24-scope-discipline) jawnie przewidział tę ścieżkę: *„Każda implementacja tych ficzerów w MVP PR-ze = automatic reject. **Wyjątek: explicit ADR + maintainer sign-off.**"* — niniejszy ADR jest tym wyjątkiem.

Możliwe odpowiedzi na obserwację:

1. **Trzymać oryginalny scope** (Bento Ultra w STRETCH v0.2+, MVP minimalistyczny).
2. **Eskalować w pełnej skali** (cztery podsystemy do v0.1).
3. **Eskalować tylko Bento Ultra layout** (visual, bez live logs / topology / CI panel).
4. **Demo-skin w osobnej gałęzi** (proof-of-concept bez merge'a do `main`).

## Decyzja

Webox **v0.1 (MVP)** będzie zawierał wszystkie cztery wymienione podsystemy. Konkretnie:

1. **Bento Ultra adaptive layout engine** — auto-detekcja rozmiaru terminala z trzema progami: `100×30` (Standard Cockpit, fallback), `120×35` (Bento Ultra, default jeśli mieści się), `160×45` (Bento Ultra+, dodatkowe kafelki). Pełna paleta OKLCH z [UX §2.1](../UX.md#21-paleta-kolor%C3%B3w-oklch--hsl-precision), dynamic layering z [UX §2.2](../UX.md#22-system-warstw-i-g%C5%82%C4%99bi-dynamic-layering), premium status badges z [UX §3.1](../UX.md#31-badges-statusu-premium), gradientowy logotyp z [UX §2.4](../UX.md#24-identyfikacja-wizualna-i-branding-logo-g%C5%82%C3%B3wne).
2. **Live Log Stream** — `tail -f` przez istniejący `ssh.Pool`, ring buffer (max 1000 linii, circular overwrite), ANSI parser dla level coloring (INFO/WARN/ERROR/DEBUG), **redactor sekretów pre-render** (każda linia przepuszczona przez `internal/log.Redact` zanim trafi do bufora), 60fps throttle cap, context-cancellable na `q`/`Esc`.
3. **Live CI/CD Pipeline Panel** — nadbudowa na `services/github/.GetLatestRun` z polling TTL 10 s, kolorowe step results, click-through do `gh run view` ostatnich 50 linii.
4. **Live Service Topology Map** — ASCII box-drawing renderer (GitHub Repo → Production Server → App ← MySQL), live edge animations (ONLINE / BUILDING / OFFLINE) sterowane z `status/` cache.

Każdy z czterech podsystemów dostarczany jest w osobnym sprincie (08–11) z własnym retro. Implementacja **Bento Ultra layout engine** wchodzi za feature-flagą `WEBOX_LAYOUT=bento_ultra` domyślnie `off`; flaga zostaje **włączona domyślnie** dopiero po teatest goldens green dla wszystkich trzech rozmiarów (Sprint 08 acceptance criteria).

Zmiany w docs zsynchronizowane z tą decyzją:

- [`docs/ROADMAP.md §3.0`](../ROADMAP.md#30-mapowanie-sprint--release): mapa sprintów dodaje 08–11.
- [`docs/ROADMAP.md §3.1`](../ROADMAP.md#31-zakres-ficzerowy): F14, F15, Live Service Topology Map dodane do listy P0.
- [`docs/ROADMAP.md §3.3`](../ROADMAP.md#33-czego-nie-ma-w-mvp): Bento Ultra, live log stream, GHA monitor, topology map usunięte z listy „NIE w MVP".
- [`docs/ROADMAP.md §3.5`](../ROADMAP.md#35-estymata): re-baseline P50 22 → 27 tygodni, P70 32 → 35.
- [`docs/ROADMAP.md §4.2`](../ROADMAP.md#42-zakres-v02): F14, F15 i Live Topology przeniesione z v0.2 do v0.1; v0.2 nadal dostaje drugiego providera, Env Merger, Sound Engine, fast-chord bindings, multi-provider dashboard agregator.
- [`docs/PRD.md §6`](../PRD.md#6-ficzery--z-priorytetami): F14 (Live log stream), F15 (Deployment Monitor) priorytet zmieniony z **P1 → P0**.
- [`docs/AGENTS.md §3.1`](../AGENTS.md#31-w-mvp-v01-musi-dzia%C5%82a%C4%87-przed-release): dodane cztery ficzery do listy „W MVP".
- [`docs/AGENTS.md §3.2`](../AGENTS.md#32-nie-w-mvp-stretch-v02): usunięte z listy „NIE w MVP" Bento Ultra, live log stream, topology map. Sound Engine, Bento Ultra+ (`≥ 160×45`), Env Merger, fast-chord bindings, multi-provider dashboard agregator **zostają** w STRETCH v0.2+.
- [`docs/UX.md` TL;DR](../UX.md#tldr) + §3.4: markery 🔶 STRETCH usunięte dla Bento Ultra (`120×35`), Live Log Stream, Topology Map. Bento Ultra+ (`160×45`) zachowuje marker 🔶 STRETCH (v0.2+) jako rozszerzenie premium tier-a.

## Dlaczego pełna eskalacja, a nie alternatywy

### Trzymać oryginalny scope (Option 1)

Pros: szybsze v0.1 (P50 22 tyg.), mniejsza powierzchnia testów MVP, ryzyko regression mniejsze.
Cons: pierwsza fala recenzji oceni v0.1 jako proof-of-concept; brand „Terminal Cockpit klasy premium" zostanie obietnicą bez pokrycia; v0.2 musi nadrobić zaufanie utracone w v0.1 (typowo droższe niż zbudowanie go raz w v0.1).
Why rejected: maintainer ocenił, że **timing brand promise** matters more than **timing release**. Open-source TUI tooli oceniane są w pierwszych 30 dniach po launchu na GH Trending; minimalistyczne MVP nie wyląduje tam organicznie.

### Eskalować w pełnej skali (Option 2 — WYBRANA)

Pros: v0.1 dostarcza pełen brand promise; pierwsza recenzja jest sprawiedliwa; v0.2 może wystartować z prawdziwych nowych ficzerów (drugi provider, Env Merger) zamiast doganiania warstwy prezentacji; cztery podsystemy w v0.1 wymuszają wczesne ustabilizowanie kontraktów (layout engine, status cache pod metrics, redactor scope).
Cons: +5 tygodni P50 (22 → 27), +3 tygodnie P70 (32 → 35); cztery nowe podsystemy do utrzymania od dnia 1; większy initial test surface (snapshot tests dla trzech rozmiarów × kilku stanów); zwiększone obciążenie SSH/GH API ze strony live log + topology + CI panel — ryzyko rate limit.

### Eskalować tylko Bento Ultra layout (Option 3)

Pros: lżejszy scope (+2 tyg zamiast +5); zachowanie wizualnego promise.
Cons: niespójność estetyczno-funkcjonalna — Bento Ultra layout został zaprojektowany pod live data; statyczne kafelki w premium ramkach wyglądają jak makieta (anti-pattern „style without substance"); użytkownik widzi gradient i topology placeholder, ale po klatce zorientuje się, że dane są zamrożone; v0.2 musi dodać live logikę → drugie dostarczenie tej samej powierzchni.
Why rejected: dzielenie pakietu wizualnego zostawia v0.1 w stanie „już nie minimalist, jeszcze nie premium" — najgorszy możliwy punkt.

### Demo-skin w osobnej gałęzi (Option 4)

Pros: zero ryzyka dla v0.1 timeline; służy jako wizualny brief do v0.2; eksperyment bez zobowiązania.
Cons: skin bez prawdziwego silnika to teatr; widoczna różnica między demo a v0.1 zniechęca early adopters; podwójne utrzymanie (`main` i demo branch); demo szybko driftuje od `main` i staje się bezużyteczne.
Why rejected: maintainer rozważył tę opcję jawnie podczas dyskusji eskalacji i odrzucił na rzecz opcji 2 — wybór single source of truth.

## Konsekwencje

### Pozytywne

- **v0.1 dostarcza pełen brand promise** z PRD §3 i UX TL;DR bez kompromisu wizualnego — pierwsza recenzja jest sprawiedliwa.
- **v0.2 startuje z prawdziwymi nowymi ficzerami** (drugi provider, Env Merger, Sound Engine, multi-provider dashboard) zamiast doganiania warstwy prezentacji.
- **Kontrakty wymuszone wcześnie**: `BentoTile` interface, ring buffer protocol, status metrics shape — zmienianie ich w v0.2 byłoby breaking change w `tui/`; lepiej zaprojektować raz na starcie.
- **`services/github`** zyskuje drugiego konsumenta (CI/CD Panel obok wizard deploy), co naturalnie testuje stabilność klienta.
- **Redactor scope** zostaje twardo zweryfikowany przez live log stream (każda linia logu z serwera produkcyjnego) — to najbardziej hostile testbed jaki możemy sobie wymyślić.

### Negatywne / Trade-offs

- **+5 tygodni P50 do MVP timeline** (z 22 do 27 tygodni); +3 tygodnie P70 (z 32 do 35).
- **Cztery nowe podsystemy** do utrzymania od dnia 1: layout engine, log streamer, GHA panel, topology renderer — większe initial test surface i większe utrzymanie.
- **Live log stream + topology + CI panel** zwiększają obciążenie SSH/GitHub API — ryzyko rate limits (GitHub: 60/h anon, 5000/h z PAT; SSH: limit sesji per host).
- **Layout engine** dla trzech rozmiarów wymaga snapshot tests × 3 sizes × N stanów = N×3 goldens; coverage `tui/views/` musi wyraźnie wzrosnąć (z obecnego 0% do ≥60% przed v0.1).
- **Bardziej skomplikowane fallback path** dla terminala 80×24 (poniżej Standard Cockpit) — wymaga graceful degradation strategy.

### Mitygacje

- **Każdy sprint kończy się TDD-zielonym retro** przed startem kolejnego; kanał kontroli regression przez snapshot goldens.
- **Live log throttle 60fps cap** chroni przed perf collapse; **redactor pre-render guard** chroni przed secret leak (test corpus z `internal/log/redact_test.go` rozszerzony o sample log lines).
- **GHA API używa istniejącego SWR cache** ze Sprintu 02 (TTL 10–60s dobrane pod 60 req/h anon limit + 5000/h z PAT); SSH metrics używają TTL 5s na server z dedykowanym pool slot.
- **Feature flag `WEBOX_LAYOUT=bento_ultra`** domyślnie `off` w Sprint 08 → włączony domyślnie po teatest goldens green; flaga zostaje **dostępna** do v1.0 jako safety net.
- **Auto-degradacja layoutu**: rozmiar terminala `<100×30` → Standard Cockpit fallback (już zaprojektowany); `<70×22` → pełnoekranowy komunikat z PRD §10.3.

## Implementation notes

Sekwencja sprintów po zaakceptowaniu ADR:

| Sprint | Cel | Estymata P50 (P70) |
|---|---|---|
| **07** (bez zmian, już zaplanowany) | Import + Doctor GitHub + deploy polish (carry-over ze Sprintu 06) | 1 tyg (1.5 tyg) |
| **08** | Bento Ultra Layout Engine + OKLCH theme refresh + adaptive grid + **wyczyszczenie wszystkich wycieków „Sprint NN" z UI runtime** | 1.5 tyg (2.5 tyg) |
| **09** | Live Log Stream + header bar server metrics | 2 tyg (3 tyg) |
| **10** | Live CI/CD Pipeline Panel (nadbudowa na `services/github`) | 1 tyg (1.5 tyg) |
| **11** | Live Service Topology Map | 1.5 tyg (2 tyg) |

**Każdy sprint** musi spełnić:

- TDD-first dla parserów (ANSI level, gh run status, topology edges), redactora rozszerzonego pod live logs i layout engine'u (table-driven sizing).
- Snapshot teatest goldens dla trzech rozmiarów (`100×30`, `120×35`, `160×45`) + 1 fallback (`80×24`).
- Race-free pod `-race` (live log goroutiny, GHA pollery, SSH metrics pool).
- Brak nowych zależności bez sign-offu (zgodnie z [AGENTS §1.2](../AGENTS.md#12-kluczowe-biblioteki-sprawdzone-przez-context7)).
- Wpis w `CHANGELOG.md [Unreleased]` z linkiem do tego ADR.

**Kontrakt `BentoTile` interface** (Sprint 08) jest **publicznym API** w `tui/bento/` — kolejne ADR-y są wymagane dla jego zmiany w v0.2+.

**Fallback strategy** (Sprint 08 dostarcza):

- `width < 100 || height < 30` → render Standard Cockpit (`tui/views/dashboard.go` jako fallback path zachowany).
- `width < 70 || height < 22` → render fullscreen warning z [PRD §10.3](../PRD.md#103-terminal).
- Środowisko bez 24-bit color → graceful palette downgrade do 256-color (Lipgloss obsługuje natywnie).

## Referencje

- [PRD §3](../PRD.md#3-problem-kt%C3%B3ry-rozwi%C4%85zujemy), [§6](../PRD.md#6-ficzery--z-priorytetami), [§8](../PRD.md#8-kryteria-sukcesu--mierzalne)
- [UX TL;DR](../UX.md#tldr), [UX §2](../UX.md#2-design-system-20), [UX §3.4](../UX.md#34-wizualny-graf-topologii-us%C5%82ug-live-service-topology-map), [UX §4](../UX.md#4-layouty-ekran%C3%B3w-20)
- [ROADMAP §3](../ROADMAP.md#3-mvp-v01--wy%C5%82%C4%85cznie-smallpl), [§4.2](../ROADMAP.md#42-zakres-v02)
- [AGENTS §2.4 Scope discipline](../AGENTS.md#24-scope-discipline), [§3 Co JEST i NIE JEST w MVP](../AGENTS.md#3-co-jest-i-nie-jest-w-mvp)
- [ADR-0001 TUI zamiast CLI](./0001-tui-zamiast-cli.md), [ADR-0003 Provider Pattern](./0003-provider-pattern.md), [ADR-0005 Cache statusów](./0005-cache-statusow-projektow.md)
- Sprint 08–11 plans: `docs/sprints/sprint-08-bento-ultra.md`, `docs/sprints/sprint-09-live-log-stream.md`, `docs/sprints/sprint-10-cicd-panel.md`, `docs/sprints/sprint-11-topology-map.md` (do utworzenia razem z tym ADR)

## Historia zmian dokumentu

- 2026-05-23: Status **Proposed → Accepted** w tej samej sesji. Decyzja podjęta przez maintainera (solo-maintainer mode) po explicit consensus o eskalacji ze STRETCH do MVP.
