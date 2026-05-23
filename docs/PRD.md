# Webox — Product Requirements Document

> Status: Draft · Ostatnia aktualizacja: 2026-05-23 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [DESIGN.md](./DESIGN.md) (jak to zbudować), [UX.md](./UX.md) (jak to wygląda), [ROADMAP.md](./ROADMAP.md) (co i kiedy), [SECURITY.md](./SECURITY.md) (model zaufania).

## TL;DR

Webox to operatorskie TUI w Go dla deweloperów hostujących projekty na **hostingu współdzielonym**. Skraca pełny lifecycle projektu — od założenia subdomeny, przez konfigurację SSL, repo, CI/CD, po codzienne restarty i podgląd logów — z ~30 minut klikania w 4–5 zakładkach przeglądarki do <5 minut w jednym terminalu. MVP (v0.1) celuje **wyłącznie w small.pl/Devil**, dostarcza **pełen Bento Ultra cockpit** z live log stream, GitHub Actions live panelem i Service Topology Map (eskalowane do MVP przez [ADR-0007](./adr/0007-bento-ultra-eskalacja-mvp.md)), a architektura (Provider Pattern) jest gotowa na cPanel, DirectAdmin i CyberPanel od dnia 1. Webox nie jest panelem hostingowym, nie zastępuje GitHuba i nie wspiera VPS/Dockera — to świadoma decyzja, patrz [§9 Non-goals](#9-non-goals).

## Spis treści

1. [Cel dokumentu](#1-cel-dokumentu)
2. [Wizja i motto](#2-wizja-i-motto)
3. [Problem, który rozwiązujemy](#3-problem-kt%C3%B3ry-rozwi%C4%85zujemy)
4. [Persony](#4-persony)
5. [Konkurencja i landscape](#5-konkurencja-i-landscape)
6. [Ficzery — z priorytetami](#6-ficzery--z-priorytetami)
7. [Import istniejących projektów](#7-import-istniej%C4%85cych-projekt%C3%B3w)
8. [Kryteria sukcesu — mierzalne](#8-kryteria-sukcesu--mierzalne)
9. [Non-goals](#9-non-goals)
10. [Wymagania i ograniczenia](#10-wymagania-i-ograniczenia)
11. [Założenia i ryzyka produktowe](#11-za%C5%82o%C5%BCenia-i-ryzyka-produktowe)
12. [Decyzje otwarte](#12-decyzje-otwarte)

---

## 1. Cel dokumentu

PRD odpowiada na pytania **CO** i **DLACZEGO**. Nie definiuje kodu ani architektury (to robi [DESIGN.md](./DESIGN.md)), nie definiuje też wyglądu ekranów (to robi [UX.md](./UX.md)). Adresatami są: product owner, contributorzy, użytkownicy mający zrozumieć granice produktu, oraz osoby decydujące czy w ogóle z webox warto się bawić.

## 2. Wizja i motto

<!-- z PRD §1 -->

Webox to terminalowe narzędzie operatorskie (TUI) dla deweloperów hostujących projekty na **hostingach współdzielonych**. Łączy pełen lifecycle projektu — od utworzenia subdomeny, przez scaffolding, konfigurację CI/CD, monitoring, aż po zarządzanie SSL — w jednym, czytelnym interfejsie inspirowanym `lazygit` i `k9s`. Otwartoźródłowy, zaprojektowany wokół Provider Patternu, tak by społeczność mogła dorzucać adaptery do kolejnych paneli.

> **Motto:** *Twój hosting w twoim terminalu.*

### 2.1 Zasady produktu

Webox ma być celowo węższy niż brzmi na pierwszy rzut oka. Pięć reguł prowadzących:

1. **Najpierw realny ból, potem szeroki zasięg.** MVP ma rozwiązać prawdziwy workflow autora i pierwszych użytkowników `small.pl`, a nie symulować wsparcie dla całego rynku.
2. **Jedno narzędzie operatorskie, nie nowy panel hostingowy.** Webox ma skracać pracę dewelopera, nie klonować cały panel WWW w terminalu.
3. **Architektura szeroka, powierzchnia produktu wąska.** Provider Pattern istnieje od dnia 1, ale nie usprawiedliwia rozdymania MVP.
4. **Bezpieczna automatyzacja wygrywa z "magicznością".** Import ma być read-only, rollback jawny, a sekrety i host keys traktowane z ostrożnością.
5. **Dokumentacja ma usuwać niejednoznaczność, nie ją maskować.** Każdy obszar, którego nie wiemy, ma być oznaczony jako decyzja otwarta albo `TO BE VERIFIED`.

## 3. Problem, który rozwiązujemy

Hosting współdzielony jest dla małych projektów wciąż najtańszym i najprostszym wyborem (kilkanaście złotych miesięcznie za nieograniczoną liczbę aplikacji Node, MySQL/PostgreSQL, e-mail, SSL). Cena tej taniości to **rozproszony tooling**: każdy projekt wymaga przeskakiwania między 4–5 kontekstami.

### 3.1 Codzienna rzeczywistość dewelopera na small.pl

Realny czas postawienia nowego projektu Node ręcznie (mierzone, nie aspiracyjne):

| Krok | Tooling | Czas |
|---|---|---|
| Utworzenie subdomeny `my-app.user.smallhost.pl` | panel WWW small.pl (Devil) | 1–2 min |
| Konfiguracja Node 24 dla subdomeny | panel WWW → tab Node | 1 min |
| Generowanie certyfikatu Let's Encrypt | SSH + `devil ssl www add ...` | 2–3 min (czekanie na DNS + Let's Encrypt) |
| Utworzenie bazy danych | panel WWW → MySQL/PgSQL | 1 min |
| Skopiowanie credentials do lokalnego `.env` | edytor + terminal | 1 min |
| `git init` + `gh repo create` + secrets `SSH_PRIVATE_KEY`, `DEPLOY_HOST` | terminal + GitHub web | 3–4 min |
| Napisanie `.github/workflows/deploy.yml` | edytor (copy-paste z poprzedniego projektu) | 3–5 min |
| Pierwszy push + obserwacja Actions | terminal + przeglądarka | 2 min |
| Diagnoza pierwszego błędu (zwykle path/uprawnienia/Node version) | SSH + Actions logs | 5–15 min |
| **Łącznie** | **~5 kontekstów** | **~25–40 minut** |

Codzienny restart aplikacji to przeskok do panelu lub `ssh user@s1.small.pl && devil www restart …`. Podgląd logów — kolejny SSH i `tail`. Po 5 projektach traci się rachubę, w którym profilu/aliasie SSH siedzą czyje klucze. Po 15 projektach człowiek pisze sobie własny skrypt shell — co właśnie zdarzyło się dla autora webox.

### 3.2 Co webox upraszcza

- **Jedno okno terminala zamiast 5 kontekstów.** Dashboard pokazuje wszystkie projekty z statusem HTTP, wersją Node, dniami do wygaśnięcia SSL i ostatnim deploymentem.
- **Wizard nowego projektu robi wszystkie ~25 min ręcznej pracy w <5 min** (subdomena → SSL → DB → repo → secrets → workflow → pierwszy deploy).
- **Operacje administracyjne (restart, logi, edycja `.env`, dodanie domeny)** są jednym klawiszem zamiast łańcucha komend.
- **Transakcyjny rollback**: jeśli wizard padnie w połowie (np. DNS nie zdążył się propagować), webox sprząta po sobie zamiast zostawiać orphan'owaną subdomenę.

Webox nie zastępuje panelu hostingowego ani GitHuba — zastępuje **przełączanie kontekstu** między nimi.

## 4. Persony

### 4.1 Persona A — „Marek, freelancer z 5–20 projektami na small.pl"

| Atrybut | Wartość |
|---|---|
| Środowisko pracy | macOS, terminal iTerm2 + Neovim, `gh` CLI, `git` |
| Konta hostingowe | 1–2 konta na small.pl (własne + brat / klient) |
| Liczba projektów | 5–20 (mix landing page'y SPA + 2–3 backendy Node) |
| Frustracja #1 | Pamiętanie, który projekt na której subdomenie i kiedy mu wygasa SSL |
| Frustracja #2 | Powtarzalność: za każdym razem ten sam workflow Actions + ten sam `.env` boilerplate |
| Budżet czasowy | Chce skrócić „nowy projekt od zera" z ~30 min do <5 min |
| Tolerancja na ryzyko | Niska — single dev, nie ma kogo prosić o pomoc gdy coś padnie |
| Kanał komunikacji | GitHub Issues / Discussions, Discord (jeśli istnieje) |

**Sukces dla Marka:** „W ciągu 5 minut od `webox /create` mam działający link `https://my-app.marek.smallhost.pl` z certyfikatem i pierwszym commitem na GH."

### 4.2 Persona B — „Agencja XYZ, 3 osoby, 30+ stron klientów"

| Atrybut | Wartość |
|---|---|
| Środowisko pracy | macOS + Linux, mix paneli (small.pl, cPanel klienta, DirectAdmin u resellera) |
| Konta hostingowe | 6–10 różnych kont/paneli, każdy z innym user/portem/kluczem |
| Liczba projektów | 30–60 (głównie WordPress + 5–10 Node/Next.js) |
| Frustracja #1 | Każdy panel ma inną logikę i klikanie po 6 panelach to godziny dziennie |
| Frustracja #2 | Trudno przekazać dostępy nowemu pracownikowi — wszystko w głowie i `.ssh/config` |
| Budżet czasowy | Czas zwrotu inwestycji w tool: <1 miesiąca |
| Tolerancja na ryzyko | Średnia — mogą testować nowe narzędzia, ale tylko po godzinach pracy klientów |
| Kanał komunikacji | E-mail wewnętrzny, kanał Slack, klient krzyczy gdy SSL wygaśnie |

**Sukces dla agencji:** „Jeden dashboard, wszystkie projekty wszystkich klientów, ostrzeżenia o wygasającym SSL na 14 dni przed terminem, dostępy w keyringu macOS — onboarding nowego pracownika to skopiowanie configu webox + zaproszenie do organizacji GH."

> Persona B nie jest priorytetem MVP (v0.1 obsługuje wyłącznie small.pl), ale jej istnienie wymusza architekturę multi-provider od dnia 1.
>
> **Ważne:** akceptacja MVP będzie oceniana przede wszystkim na personie A. Persona B działa jako presja architektoniczna, nie jako źródło nowych obowiązków zakresowych dla `v0.1`.

## 5. Konkurencja i landscape

Webox celuje w **niszę hostingu współdzielonego**. Konkurenci dzielą się na trzy obozy:

| Narzędzie | Co robi | Czego nie robi w naszej niszy |
|---|---|---|
| **Coolify** (self-hosted PaaS) | Pełna platforma deployment + DB + monitoring na własnym VPS-ie. | Wymaga VPS-a z Dockerem — nie działa na shared hostingu, gdzie nie ma Dockera. |
| **Dokploy / CapRover / Dokku** | Self-hosted alternatywy Heroku. | Jak wyżej — wymagają VPS. |
| **Panele hostingowe (Devil/cPanel/DirectAdmin)** | UI webowy do tworzenia subdomen, baz, SSL. | Nie integrują się z lokalnym devloopem (Git, edytor, CI) — wymagają klikania. |
| **Hosting CLI (np. `devil` u small.pl, `uapi` w cPanel)** | Pojedyncze komendy na SSH. | Brak dashboardu, brak zarządzania wieloma projektami, brak transakcyjności. |
| **Ręczny workflow + skrypty bash** | To, co większość ludzi robi dziś. | Skrypty są lokalne, niespójne, źle udokumentowane, nie ma rollbacku, dashboard znaczy `cat ~/projects.txt`. |
| **Vercel / Netlify / Cloudflare Pages** | Hosting dedykowany, jeden klik z repo. | Inny model biznesowy (płacisz za zużycie / build minutes), nie pasuje do shared hostingu, na który już zapłaciłeś rocznie. |

### Dlaczego webox ma sens w niszy shared hostingu

1. **Cena.** Shared hosting to ~50–150 zł/rok za nieograniczoną liczbę aplikacji. VPS z Coolify zaczyna się od ~30 zł/miesiąc i wymaga utrzymania OS-a, certyfikatów, backupów.
2. **Dla małych projektów wystarczy.** Landing strona, blog, mała aplikacja klienta — nie potrzebują autoskalowania.
3. **Tooling deweloperski jest zaniedbany.** Panel webowy z 2008 r. wciąż jest jedynym interfejsem na większości shared hostingów. Nikt nie zrobił dla nich „lazygita".

### Czego webox nie chce być

- **Nie chce być Coolify dla shared hostingu** — nie deployuje konteneryzowanych aplikacji, nie ma własnego runtime'u.
- **Nie chce być GUI panelu hostingowego** — panel webowy zostaje do operacji rzadkich (zmiana hasła do panelu, faktury). Webox bierze tylko to, co dev robi codziennie.
- **Nie chce być CI/CD platformą** — od deploymentu jest GitHub Actions, patrz [ADR-0002](./adr/0002-deploy-tylko-przez-github-actions.md).

## 6. Ficzery — z priorytetami

Priorytety: **P0** = MVP (v0.1, wyłącznie small.pl), **P1** = post-MVP (v0.2), **P2** = nice-to-have (v0.3+). Provider-agnostic = czy ficzer da się zaimplementować na poziomie `HostingProvider` bez wycieku specyfiki small.pl.

| ID | Feature | Priorytet | Provider-agnostic? | Opis |
|----|---------|-----------|-------------------|------|
| F1 | Init wizard | P0 | Tak | Pierwsze uruchomienie: konfiguracja profilu hostingowego (typ + host + user + key), SSH key gen, GitHub Token. |
| F2 | Provider management — CRUD profili | P0 | Tak | `/provider` w TUI. **W MVP tylko `type=smallhost`**, inne typy schowane za feature-flagą `experimental`. Brak operatorskich CLI commands typu `webox provider add` w `v0.1`. |
| F3 | Wizard nowego projektu (5 kroków, krok DB skippable) | P0 | Tak | provider → stack → DB (smart skip dla statycznych) → domena → deploy. |
| F4 | Dashboard (lista + szczegóły, status HTTP + SSL + Node) | P0 | Tak | Lewy panel lista, prawy panel szczegóły. Dane przez Provider + HTTP ping. |
| F5 | Status check (HTTP ping + cert info + Node version) | P0 | Tak | Per projekt, cache patrz [DESIGN.md §8](./DESIGN.md#8-tr%C3%B3jpoziomowy-status-cache-stale-while-revalidate). |
| F6 | Restart aplikacji | P0 | Tak (przez `properties.restart_method`) | Provider decyduje *jak* (Devil / Passenger / systemd). |
| F7 | SSL management — Let's Encrypt issue + renew | P0 | Tak | Provider definiuje komendy konkretne dla panelu. |
| F8 | Podgląd logów (tail, niekoniecznie live stream) | P0 | Tak | Tail ostatnich N linii z `GetLogPath(domain)`. |
| F9 | Import istniejących projektów | **P0** | Tak | Patrz [§7](#7-import-istniej%C4%85cych-projekt%C3%B3w). |
| F10 | Rollback transakcyjny kreatora | P0 | Tak | Stos LIFO + `pending_cleanups.json`. Patrz [DESIGN.md §10](./DESIGN.md#10-dag-based-transactional-engine-wznawialny-rollback). |
| F11 | Bezpieczne sekrety (keyring) | P0 | Tak | `go-keyring` + fallback (patrz [SECURITY.md §4](./SECURITY.md#4-przechowywanie-sekret%C3%B3w)). |
| F12 | Command Palette `/` — fuzzy search | P0 (minimalny: `/create`, `/provider`, `/import`, `/settings`) | Tak | Pełne palette → P1. |
| F13 | Live dashboard auto-refresh co N sekund | P1 | Tak | Konfigurowalny interwał, default 10 s. |
| F14 | Live log stream | **P0** | Tak | `tail -f` przez SSH, ring buffer w UI, ANSI level coloring, redactor pre-render. **Eskalowane z P1 do P0 przez [ADR-0007](./adr/0007-bento-ultra-eskalacja-mvp.md).** |
| F15 | Deployment Monitor (GitHub Actions API) | **P0** | Tak (GH-only, ale inne CI to inny temat) | Live status workflow runs + logi jobów. **Eskalowane z P1 do P0 przez [ADR-0007](./adr/0007-bento-ultra-eskalacja-mvp.md).** |
| F16 | Node.js version manager | P1 | Tak (przez Provider) | Zmiana wersji Node per domena. |
| F17 | Cert expiry monitoring + warning <14 dni | P1 | Tak | Pasek na dashboardzie + opt-in notyfikacje OS. |
| F18 | Multi-provider w jednym configu | P1 | Tak | Działa od dnia 1 architektonicznie, w MVP UI ogranicza wybór do `smallhost`. |
| F19 | SSH key health check | P2 | Tak | Weryfikacja klucza i `authorized_keys`. |
| F20 | Export/Import konfiguracji (bez sekretów) | P2 | Tak | Backup + restore. |
| F21 | Stack scaffolding (Vite/Next/Nuxt/Node) | P0 (Vite+React, Node.js backend, Static site) / P1 (Next.js, Nuxt, reszta) | Częściowo | Wybór szablonu w kreatorze. |
| F22 | Non-interactive CLI flags (skryptowanie) | P2 | Tak | `webox restart <project> --json`, patrz [§12.3](#12-decyzje-otwarte). |
| F23 | Manual changes detection ("stale projects") | P0 | Tak | Patrz [DESIGN.md §11](./DESIGN.md#11-detekcja-rozbie%C5%BCno%C5%9Bci-konfiguracji-drift--stale-detection). |
| F24 | Auto-update binarki | P2 (in-app) / P0 (zewnętrznie przez `brew`/`go install`) | n/a | Patrz [DESIGN.md §14](./DESIGN.md#14-auto-update). |

## 7. Import istniejących projektów

**Dlaczego P0:** projekt ma już działający odpowiednik MVP jako skrypt shell, a użytkownik ma istniejące projekty na serwerze. Bez importu adopcja webox = zero.

### 7.1 User story

> *Jako Marek mam 12 projektów już postawionych na small.pl ręcznie. Chcę wszystkie wciągnąć do webox jednym kreatorem, bez recreate'owania subdomen i konfigów.*

### 7.2 Wejście / wyjście

- **Wejście:** użytkownik zna domenę projektu (np. `sui.biuromody.smallhost.pl`) lub wybiera ją z listy auto-wykrytej z `devil www list` w panelu.
- **Wyjście:** wpis w `~/.config/webox/config.json` z polami: `domain`, `profile_alias`, `repo` (opcjonalnie), `local_path` (opcjonalnie), `stack` (best-effort detection), `node_version` (zdetektowane), `imported_at`.

### 7.3 Flow (skrócony, pełny w [UX.md §11.4](./UX.md#114-flow-d-import-istniej%C4%85cego-projektu))

1. `webox` → `/` → `/import`.
2. Krok 1 — webox lista subdomen z serwera (przez provider) + opcja „wpisz ręcznie".
3. Krok 2 — webox SSH-detect: czy `public_nodejs/` istnieje, jaka wersja Node, czy jest `.env`, czy katalog ma `.git/`.
4. Krok 3 — user opcjonalnie wskazuje GitHub repo (URL slugu `org/name`) i lokalną ścieżkę.
5. Krok 4 — webox zapisuje wpis, oznacza projekt etykietą **`imported`** i listą pól, których nie udało się ustalić (`build_command`, `dist_dir`, `deploy_branch`).
6. Projekt pojawia się na dashboardzie z banerem **„Settings incomplete — verify build & deploy fields"**.

### 7.4 Niezmienność po stronie serwera

Import **niczego nie zmienia** na serwerze (zero `devil ... add`). To kluczowe dla zaufania.

## 8. Kryteria sukcesu — mierzalne

Każde kryterium musi mieć metodę pomiaru. Brak pomiaru = kryterium niezdatne i wycięte.

| # | Kryterium | Sposób pomiaru | Cel MVP |
|---|-----------|----------------|---------|
| K1 | Czas instalacji + konfiguracji pierwszego profilu | Czas między pierwszym `webox` a sukcesem init wizard (mierzony lokalnie przez `webox doctor --opt-in-stats`, raport ręczny w ankiecie po N projektach). | < 3 min mediana z ankiety (cel) / mierzalność: porównanie n≥10 użytkowników w pre-release. |
| K2 | Czas „nowy projekt od zera do HTTPS live" przez wizard | Timer od enter na kroku 5 (`Deploying`) do statusu `success` ostatniego kroku rollback-safe. Wartość zapisywana w `~/.config/webox/metrics.local.json` (opt-in). | Mediana < 5 min na konto z istniejącym SSH key i `gh` zalogowanym. |
| K3 | Czas codziennej operacji „restart" | Klawisz `r` → spinner → ✅. Mierzony lokalnie. | Mediana < 8 s, p95 < 20 s (na łączu 50 Mbit, hosting odpowiada do 2 s). |
| K4 | Liczba kontekstów odwiedzanych do utrzymania projektu | Ankieta jakościowa po N=20 projektów: „Ile razy w ostatnim tygodniu otworzyłeś panel WWW small.pl?" / „Ile razy zalogowałeś się ręcznie do SSH?". | Spadek o ≥70% vs baseline pre-webox (deklaracja użytkownika). |
| K5 | Płynność interfejsu na 20 projektach i 1 profilu | Benchmark integracyjny w CI: 20 mock projektów × pełny render dashboard. | Czas renderu pierwszej klatki < 200 ms, refresh tickiem < 100 ms. |
| K6 | Dodanie nowego providera w społeczności | Realny PR z adapterem zewnętrznego panelu, w ramach `CONTRIBUTING.md`. | 1 community-provided provider w ciągu 12 miesięcy od `v0.1` **albo** 6 miesięcy od publikacji EN contributor surface (README + CONTRIBUTING + Provider Pattern + provider template), zależnie co nastąpi później. Niespełnienie K6 nie blokuje `v1.0`, ale przesuwa GA review o kolejne 6 miesięcy. |
| K7 | Crash-free sesje | Lokalny `webox.log` (opt-in) loguje crash'e. Po release `v0.1` próbka 50 sesji → ≤2 crashe niepowtórzone. | ≥96 % crash-free w sample release-candidate. |

> **Telemetria jest opt-in i lokalna** — webox nie wysyła nic na żaden serwer. Szczegóły w [DESIGN.md §15](./DESIGN.md#15-diagnostyka-doctor--redacted-logger) i [SECURITY.md §7](./SECURITY.md#7-audyt-sekret%C3%B3w-i-tryb-doctor).

## 9. Non-goals

Aby uniknąć scope creep, webox **nie jest i nie będzie**:

| Nie-jest | Dlaczego |
|---|---|
| Panelem hostingowym | Panel zostaje u providera (faktury, hasła, support tickets, DNS edytor zaawansowany). |
| Zamiennikiem GitHuba | Repo i CI/CD żyją na GitHubie — webox to klient operatorski, nie host kodu. |
| Platformą CI/CD | Builds, testy i deploy = GitHub Actions ([ADR-0002](./adr/0002-deploy-tylko-przez-github-actions.md)). |
| Narzędziem dla VPS / Dockera / Kubernetes | Inna nisza, inni konkurenci. Patrz [§5](#5-konkurencja-i-landscape). |
| Klientem SSH / SFTP do ogólnego użytku | Webox używa SSH wyłącznie do realizacji konkretnych operacji hostingowych. Do ad-hoc `ssh user@host` użytkownik nadal używa `ssh`. |
| Narzędziem audytu bezpieczeństwa zewnętrznym | Może raportować swój własny health (`webox doctor security`), ale nie skanuje serwera za użytkownika. |
| Narzędziem do migracji hostingu | Nie portuje projektów między panelami. |
| Edytorem kodu | Edycja `.env` to maksimum jakie robimy w UI; do reszty wracasz do swojego edytora. |

## 10. Wymagania i ograniczenia

### 10.1 System operacyjny

- **Główne wsparcie:** macOS (Apple Silicon + Intel), Linux (x86_64 + ARM).
- **WSL** (Windows): wspierane jako podzbiór Linuksa, ale z zastrzeżeniami keyring → [SECURITY.md §4.2](./SECURITY.md#42-fallback-dla-%C5%9Brodowisk-headless).
- **Czysty Windows (cmd/PowerShell)**: best-effort, bez SLA. Wymaga Credential Managera.

### 10.2 Zależności runtime

| Zależność | Wymagane | Wersja minimalna | Komentarz |
|---|---|---|---|
| `git` | Tak | 2.30+ | Hard requirement dla wszystkich projektów. |
| `gh` (GitHub CLI) | Opcjonalne (zalecane) | 2.30+ | Bez `gh` webox używa REST API i prosi user'a o token. |
| Konto small.pl z dostępem SSH | Tak (dla MVP) | n/a | `devil` CLI dostępny dla każdego konta na small.pl. |
| Terminal UTF-8 z 24-bit color | Tak | n/a | Patrz [UX.md §5](./UX.md#5-wymagania-terminala). |

### 10.3 Terminal

- **Zalecane:** 100 × 30 znaków, true-color, czcionka z ligaturami i emoji wsparcia (np. JetBrains Mono Nerd Font).
- **Minimalne komfortowe:** 88 × 28 znaków z klasycznym split-pane.
- **Awaryjne:** 70 × 22 znaków w trybie single-pane focus (bez pełnego help bara).
- **Poniżej 70 × 22:** webox pokazuje pełnoekranowy komunikat „Terminal too small. Recommended ≥ 100×30."

Pełna analiza w [UX.md §5](./UX.md#5-wymagania-terminala).

### 10.4 Sieć i hosting

- Wymagane wyjście SSH na port serwera (zwykle 22, ale konfigurowalne).
- Wymagane wyjście HTTPS na GitHub API (443).
- W przypadku firewallu klienta blokującego bezpośrednie SSH wychodzące — webox `v0.1` nie zadziała. `ProxyJump` / bastion / `ProxyCommand` są świadomie odroczone do `v0.2+`, bo wymagają dodatkowego modelu zaufania i testów host-key dla dwóch hopów.

## 11. Założenia i ryzyka produktowe

| # | Założenie | Ryzyko jeśli błędne | Mitygacja |
|---|---|---|---|
| A1 | Użytkownik MVP używa small.pl | Niska adopcja jeśli najpierw uderzymy w cPanel | Skrypt shell istnieje, baza użytkowników też. |
| A2 | Użytkownik zna podstawy terminala | Wykluczenie nietechnicznych userów | OK — to wprost target persona. |
| A3 | small.pl/Devil nie zmieni nagle składni `devil` CLI | Rozsypanie wszystkich operacji jednym update'em panelu | Properties bag + testy integracyjne (mock SSH server odzwierciedlający output Devila) + warstwa parsowania izoluje zmiany. |
| A4 | GitHub Actions pozostanie dostępny i darmowy dla publicznych repo | Reorganizacja modelu cenowego GHA | Architecturally niezablokowane na inne CI (GitLab CI, Gitea Actions) — patrz [ADR-0002 Konsekwencje](./adr/0002-deploy-tylko-przez-github-actions.md#konsekwencje). |
| A5 | Hosting będzie tolerował SSH session pooling + okazjonalne stałe połączenia | Banowanie kont za „nadużycie" | Webox limituje liczbę jednoczesnych sesji SSH per host (max 3) + reconnect z backoffem (patrz [DESIGN.md §9](./DESIGN.md#9-obs%C5%82uga-b%C5%82%C4%99d%C3%B3w-ssh)). |
| A6 | Społeczność contributorów Go istnieje dla shared-hosting tooling | Adapter cPanel/DA będzie wisieć rok | MVP nie zależy od community contributorów; v0.2 plan zakłada dorobienie cPanela przez maintainera. |

## 12. Decyzje otwarte

> Te decyzje wpływają na SECURITY/ROADMAP/CONTRIBUTING. Wstępne rozstrzygnięcia poniżej — będą skorygowane po MVP w oparciu o feedback.

### 12.1 Model dystrybucji

**Decyzja:** open-source od dnia 1, licencja **MIT**. Powód: ekosystem TUI Charm/Bubble Tea jest open-source i tylko publiczne repo umożliwia community-provided providery.

### 12.2 Monetyzacja

**Decyzja wstępna:** brak monetyzacji w v1. GitHub Sponsors button opcjonalnie. Bez pro-tieru, bez paywalla na ficzery. Powody:
- Najtwardszy dowód użyteczności to adopcja w społeczności shared-hostingowej.
- Pro-tier wymaga infrastruktury (subskrypcje, licencjonowanie) — koszt utrzymania > prawdopodobny przychód w niszy.

Powrót do tematu po osiągnięciu K6 ([§8](#8-kryteria-sukcesu--mierzalne)).

### 12.3 Tryb nieinteraktywny (CLI flags)

**Decyzja wstępna:** w MVP — **tylko TUI**. CLI flags do skryptowania → P2 (v0.3+). Powód: MVP musi być małe; pełen CLI surface (`webox restart <project> --json`, `webox status --quiet`) podwaja powierzchnię testów.

Wyjątek: `webox doctor` (lokalny raport diagnostyczny) działa nieinteraktywnie od dnia 1, bo to debugging tool dla maintainerów.

### 12.4 Telemetria / analytics

**Decyzja wstępna:** żadnej zdalnej telemetrii. **Wyłącznie lokalny** `~/.config/webox/webox.log` (rotacja, max 5 MB) + `metrics.local.json` z opt-in stats. Crash reports — user musi ręcznie odpalić `webox doctor --bundle` (tworzy zip z anonimizowanym logiem) i samodzielnie wkleić do GH Issue.

### 12.5 Fallback bez keyringu

**Decyzja wstępna:** wspieramy **plik szyfrowany AES-GCM** z hasłem master derived przez Argon2id. Tryb jawnie oznaczony jako „degraded" w UI. Tylko dla środowisk gdzie keyring jest niedostępny (Linux headless bez D-Bus, WSL bez Secret Service, CI). Szczegóły [SECURITY.md §4.2](./SECURITY.md#42-fallback-dla-%C5%9Brodowisk-headless).
