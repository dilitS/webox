# Webox — Roadmap

> Status: Stable scope, evolving estimates · Ostatnia aktualizacja: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [PRD.md](./PRD.md) (priorytety ficzerów), [DESIGN.md](./DESIGN.md), [adr/](./adr/), [sprints/](./sprints/) (rolling-wave taktyka), [RISKS.md](./RISKS.md).

## TL;DR

MVP (v0.1) ogranicza się **wyłącznie do small.pl/Devil** i tylko do wybranego wycinka P0 z [PRD §6](./PRD.md#6-ficzery--z-priorytetami). v0.2 dorzuca **jednego, świadomie wybranego** drugiego providera (kandydaci: cPanel, DirectAdmin — wybór po review notatek badawczych), public contributor surface po angielsku, live log stream, GHA monitor i pełny Command Palette. v0.3+ — multi-provider, in-app updater, scaffolding kolejnych stacków, eksport/import konfiguracji. Wersja **v1.0** wymaga 3 mies. stabilności + ścieżki dla community providerów potwierdzonej realnym PR albo jawnie odroczonej w GA review.

**Estymata solo (P50): ~22 tygodnie**. Pełne uzasadnienie w [§3.5](#35-estymata). Sprint plan i taktyka w [`sprints/`](./sprints/). Aktywne ryzyka w [`RISKS.md`](./RISKS.md).

## Spis treści

1. [Cel dokumentu](#1-cel-dokumentu)
2. [Filozofia wersjonowania](#2-filozofia-wersjonowania)
3. [MVP (v0.1) — wyłącznie small.pl](#3-mvp-v01--wy%C5%82%C4%85cznie-smallpl)
4. [v0.2 — drugi provider + dokończenie palette](#4-v02--drugi-provider--doko%C5%84czenie-palette)
5. [v0.3+ — multi-provider, scaffolding, auto-update](#5-v03--multi-provider-scaffolding-auto-update)
6. [Czego nie robimy nigdy](#6-czego-nie-robimy-nigdy)
7. [Kryteria decyzji o dodaniu providera](#7-kryteria-decyzji-o-dodaniu-providera)
8. [Definition of Done per release](#8-definition-of-done-per-release)

---

## 1. Cel dokumentu

ROADMAP organizuje co i kiedy. PRD mówi *jakie* ficzery istnieją, DESIGN *jak* są zbudowane, a ROADMAP *kiedy* się pojawiają i jakie kryteria muszą spełnić.

## 2. Filozofia wersjonowania

### 2.1 Semver

Webox przyjmuje **Semantic Versioning 2.0** od dnia 1:

- `MAJOR` — breaking change w schemacie `config.json`, interfejsie `HostingProvider`, lub kontrakcie CLI (`webox doctor`).
- `MINOR` — nowy ficzer kompatybilny wstecz (np. dodanie `/db` w v0.2).
- `PATCH` — bugfix.

`v0.x.y` traktujemy jako "early development" — minor może zawierać małe breaking change w UI (skróty), ale **nigdy** w `config.json` ani interfejsie providera.

### 2.2 Kanały release

| Kanał | Cel | Częstotliwość |
|---|---|---|
| `latest` (GH Releases tag) | Stabilna wersja dla użytkowników. | Co ~4–6 tygodni. |
| `pre-release` (GH Releases beta tag) | Release-candidate. | Każdy kandydat przed `latest`. |
| `main` branch | Continuous integration. | Bez gwarancji stabilności. |

### 2.3 Kryteria GA (v1.0)

GA = `v1.0.0`. Wymagania (wszystkie naraz):

1. ≥ 3 miesiące w produkcji bez breaking change.
2. Co najmniej **1 community-provided provider** zewnętrzny albo formalny GA review dokumentujący, dlaczego K6 z [PRD §8](./PRD.md#8-kryteria-sukcesu--mierzalne) nie zostało spełnione mimo dostępnej dokumentacji EN. Brak providera nie blokuje automatycznie GA, ale przesuwa review o 6 miesięcy jeśli nie ma mocnego uzasadnienia.
3. Coverage ≥ 80 %, govulncheck zielony.
4. CHANGELOG + dokumentacja kompletna.
5. SECURITY.md publicznie z aktywnym kanałem disclosure (≥ 1 zamknięte zgłoszenie).
6. **Bez `experimental` flag** na żadnym providerze włączonym domyślnie.

## 3. MVP (v0.1) — wyłącznie small.pl

### 3.0 Mapowanie sprint → release

| Sprint | Temat | Co dostarcza |
|--------|-------|--------------|
| 00 | Bootstrap | Repo, CI, tooling. Brak ficzerów user-visible. |
| 01 | Foundations | `config/`, `secrets/`, redactor, `webox doctor` minimum. |
| 02 | SSH + status cache | Connection pool, SWR cache, redacted SSH logging. |
| 03 | Provider abstraction + small.pl | `HostingProvider` interface + adapter `smallhost` (status, list, restart). |
| 04 | TUI shell | MVU, navigation, dashboard (read-only). |
| 05 | Wizard tworzenia projektu | 5-step wizard + LIFO rollback + `pending_cleanups.json`. |
| 06 | GitHub deploy workflow | Deploy keys, fine-grained PAT, embed.FS templates, SSL Let's Encrypt. |
| 07 | Doctor + diagnostics + i18n | `webox doctor` rozszerzony, i18n PL/EN core screens. |
| 08 | Polish + release hardening | Bug bash, RC1 → v0.1. |

Pełna dekompozycja per sprint: [`sprints/`](./sprints/).

### 3.1 Zakres ficzerowy

Lista P0 z [PRD §6](./PRD.md#6-ficzery--z-priorytetami) (skrócone IDs):

- **F1** Init wizard.
- **F2** Provider management — CRUD profili, **w UI tylko `type=smallhost`**, inne typy schowane za env `WEBOX_EXPERIMENTAL=1`.
- **F3** Wizard nowego projektu (5 kroków, **smart skip DB** — [poprawka 6.10](../CHANGES.md#1-poprawki-merytoryczne-z-tabeli-6-briefu)).
- **F4** Dashboard (lista + szczegóły, status HTTP + SSL + Node).
- **F5** Status check.
- **F6** Restart aplikacji.
- **F7** SSL Let's Encrypt — issue + renew.
- **F8** Podgląd logów (tail ostatnich N linii — **nie** live stream).
- **F9** Import istniejących projektów (`/import`).
- **F10** Rollback transakcyjny kreatora + `pending_cleanups.json`.
- **F11** Sekrety w keyringu + fallback AES-GCM.
- **F12 (minimalne)** Command Palette z `/create`, `/provider`, `/import`, `/settings`.
- **F21 (częściowe)** Scaffolding `Vite + React`, `Node.js (Express)`, `Static site`. Next/Nuxt → v0.2.
- **F23** Stale projects detection.
- **F24 (zewnętrzny)** Update przez `brew` / `go install` / GH Releases — brak in-app updatera.

### 3.2 Zakres techniczny

- Pełny interfejs `HostingProvider` w kodzie.
- Adapter `smallhost` (Devil).
- Adapter `mock` dostępny w trybie testowym.
- Connection pool SSH, status cache, rollback stack, redaktor sekretów.
- Osobny deploy key pair per projekt/repo; brak reuse globalnego klucza operatorskiego w GitHub Secrets.
- Maszyna stanów TUI (wszystkie stany z [DESIGN.md §12](./DESIGN.md#12-maszyna-stan%C3%B3w-tui) **poza** `stateDBCreator` / `stateENVManager` / `stateStorageManager` / `stateDomainManager`).
- Pakiety językowe `en` + `pl` dla ekranów core (`dashboard`, wizard, settings, dialogi), z `en` jako default.
- Contributor-facing docs gate: przed publicznym launch `v0.1` README, CONTRIBUTING, Provider Pattern quickstart i provider template muszą mieć wersję EN wystarczającą do napisania zewnętrznego adaptera bez znajomości polskiego.
- CI: `lint`, `test`, `vulncheck`, `build` na macOS + Linux.

### 3.3 Czego NIE ma w MVP

- **Brak** `/db`, `/env`, `/storage`, `/domain` jako interaktywne sub-widoki. Po wybraniu z palette → komunikat `coming in v0.2`.
- **Brak** live log stream.
- **Brak** GitHub Actions Monitor (status workflow runs widoczny w `last_deploy.status`, ale bez logów workflow).
- **Brak** drugiego providera (mock nie liczy się).
- **Brak** in-app updatera.
- **Brak** CLI flags do skryptowania (poza `webox doctor`).
- **Brak** długiego ogona lokalizacji (DE/ES/FR itd.) i pełnego coverage tłumaczeń we wszystkich przyszłych sub-widokach. `v0.1` utrzymuje tylko `en` i `pl`, z priorytetem dla ekranów core.

### 3.4 Kryteria wypuszczenia v0.1

- Wszystkie ficzery z §3.1 zaimplementowane.
- Coverage ≥ 70 %.
- Manualny checklist [TESTING.md §8.1](./TESTING.md#81-pre-mvp--v01) zaznaczony przez 2 maintainerów (lub maintainera + 1 zewnętrznego testera).
- Brak open `critical` lub `high` security issue.
- Release notes + instrukcja instalacji w README repo (poza tym katalogiem).

### 3.5 Estymata

**Solo maintainer (20h/tydzień focus time):**

| Percentyl | Czas (tyg) | Komentarz |
|-----------|------------|-----------|
| P50 | **22** | Median scenario, brak większych blokerów. |
| P70 | 26 | Jeden większy spike (np. crypto rework, parser overhaul). |
| P90 | 32 | Wystąpi 1-2 ryzyka z [`RISKS.md`](./RISKS.md) (R-001, R-002, R-004). |

**Dwuosobowy zespół (każdy 25h/tydzień):** 10-14 tygodni (P50: 12).

**Ścieżka krytyczna** (`~60% czasu`):

1. SSH layer + connection pool (Sprint 02).
2. Provider Pattern + `small.pl` adapter (Sprint 03).
3. Wizard z LIFO rollback (Sprint 05).
4. GitHub Actions integracja + workflow templates (Sprint 06).

**Reszta** (`~40%`): TUI polish, testy live z `small.pl`, fixture capture, release hardening, contributor-facing docs EN, dogłębne security review.

**Sprint cadence:** ~9 sprintów (00-08) × 1-2 tyg, planowane rolling-wave. Pełna metodologia w [`sprints/README.md`](./sprints/README.md).

> ⚠️ **Honest disclaimer:** historyczna dokładność estymat solo-devów to 1.5×-2.5× pierwotnej wartości. Re-baseline ROADMAP planowany po sprincie 03 (mamy wtedy 3 punkty rzeczywistej velocity).

## 4. v0.2 — drugi provider + dokończenie palette

### 4.1 Wybór drugiego providera

Decyzja **po review** notatek badawczych ([providers/cpanel.md](./providers/cpanel.md) i [providers/directadmin.md](./providers/directadmin.md)). Kryteria (patrz [§7](#7-kryteria-decyzji-o-dodaniu-providera)):

| Kryterium | cPanel | DirectAdmin | Wynik |
|---|---|---|---|
| Stabilne API / CLI | Tak (UAPI) | Tak (Legacy + nowe JSON API) | porównywalne |
| Dokumentacja oficjalna | Bardzo dobra (OpenAPI) | Dobra (Swagger + Legacy docs) | **cPanel marginalnie lepiej** |
| Udział rynkowy w PL | Duży | Średni | **cPanel** |
| Trudność uwierzytelnienia | Token UAPI, czasem dwa-stop | API key, prostsze | **DirectAdmin** |
| Operacje destrukcyjne (rollback) | Wymaga osobnych endpointów | Wymaga osobnych endpointów | porównywalne |

**Wstępny wybór:** **cPanel** (większa baza użytkowników w PL, lepsza dokumentacja). Decyzja ostateczna w momencie zamknięcia v0.1.

### 4.2 Zakres v0.2

- **Drugi provider** (cPanel **lub** DirectAdmin — jeden, **porządnie**).
- **F12 pełne Command Palette** — `/db`, `/env`, `/storage`, `/domain`.
- **F14** Live log stream (`tail -f` via SSH, ring buffer w UI).
- **F15** GitHub Actions monitor: progress workflow runs + logi job-ów.
- **F17** Cert expiry monitoring + OS notifications (opt-in).
- **F18** Multi-provider — wybór profilu na dashboardzie, agregacja projektów z różnych providerów.
- **F21 pełne** scaffolding — Next.js + Nuxt + dodatkowe stack'i.
- Rozszerzenie coverage tłumaczeń poza ekrany core + pierwsze community packi (`de`, `es`, `fr`, ...).
- **Coverage** ≥ 75 %.

### 4.3 Czego nie ma w v0.2

- In-app updater (v0.3+).
- Trzeci provider.
- CLI flags do skryptowania (v0.3+).

## 5. v0.3+ — multi-provider, scaffolding, auto-update

### 5.1 v0.3

- Trzeci provider (DirectAdmin lub CyberPanel — drugi z kandydatów).
- **F24** In-app updater (cosign signed, atomic replace).
- **F22** CLI flags do skryptowania (`webox restart <project> --json`).
- **F19** SSH key health check.
- **F20** Export/Import konfiguracji.
- Coverage ≥ 80 %.

### 5.2 v0.4

- Czwarty provider (CyberPanel jeśli nie w v0.3, lub community-provided).
- SBOM, SLSA provenance dla każdego release.
- `webox doctor security` jako pełna komenda.

### 5.3 v1.0 — GA

Kryteria w [§2.3](#23-kryteria-ga-v10).

## 6. Czego nie robimy nigdy

Lista z [PRD §9 Non-goals](./PRD.md#9-non-goals) skrócona do roadmapy:

| Nie | Powód |
|---|---|
| Webox jako panel hostingowy. | Panel zostaje u providera. |
| Webox jako zamiennik GitHuba. | Repo i CI/CD u GH. |
| Webox jako CI/CD platforma. | GH Actions. |
| Wsparcie VPS / Docker / Kubernetes. | Inna nisza. |
| Generyczny SSH klient. | `ssh` w systemie. |
| Edytor kodu. | Edytor systemowy. |
| Zdalna telemetria. | Decyzja produktowa, [PRD §12.4](./PRD.md#124-telemetria--analytics). |
| Pluginy dynamiczne (loadable `.so`). | Risk supply chain. Adaptery providerów są kompilowane in-tree. |

## 7. Kryteria decyzji o dodaniu providera

Każdy nowy provider w core webox musi spełnić:

| # | Kryterium | Wymóg |
|---|---|---|
| C1 | Stabilne API / CLI panelu | Vendor publicznie dokumentuje, max 1 breaking change per rok. |
| C2 | Dostępna dokumentacja | OpenAPI / Swagger / oficjalne docs (nie tylko forum). |
| C3 | SSH dostęp dla usera | Webox wymaga SSH — panel bez tego niewspierany. |
| C4 | Idempotentne operacje destrukcyjne | `Remove*` zwraca sukces na brak zasobu. Inaczej rollback jest fragile. |
| C5 | Maintainer ochotnik | Zewnętrzny provider wymaga ochotnika, który *commitnie się* do utrzymania ≥ 6 miesięcy. |
| C6 | Testy fixture'owe | Mock SSH fixture'y + golden files dla parserów. |
| C7 | Dokumentacja providera | `docs/providers/<nazwa>.md` zgodnie ze [smallhost.md](./providers/smallhost.md) jako wzorcem. |

Provider który nie spełnia C1–C4 trafia do **community-maintained external plugins** (post-v1.0 mechanizm — gdy będziemy mieli stabilne ABI).

## 8. Definition of Done per release

Każda wersja jest DONE tylko gdy:

- [ ] Wszystkie ficzery z zakresu zaimplementowane.
- [ ] Coverage osiąga próg dla wersji (70 %/75 %/80 %).
- [ ] `golangci-lint`, `govulncheck` zielone.
- [ ] Manualny checklist [TESTING.md §8](./TESTING.md#8-manualny-checklist-pre-release) zaznaczony.
- [ ] Brak open `critical` / `high` security issue.
- [ ] CHANGELOG zaktualizowany.
- [ ] Release notes opublikowane.
- [ ] Tłumaczenia EN/PL synchroniczne dla ekranów objętych zakresem wersji (`v0.1` = core screens, `v0.2+` = cały aktywny surface).
- [ ] [SECURITY.md](./SECURITY.md) zweryfikowane (szczególnie macierz threat model — czy nowe ficzery nie wprowadzają nowych wektorów).
- [ ] [ROADMAP.md](./ROADMAP.md) zaktualizowane (przesunięcie ficzerów do gotowych).
