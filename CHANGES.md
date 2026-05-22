# CHANGES — restrukturyzacja dokumentacji webox

> Data: 2026-05-22 · Właściciel: @maintainer
>
> Lista konkretnych zmian wprowadzonych przez restrukturyzację monolitu `archive/PRD_v0_monolith.md` na profesjonalną strukturę `docs/`. Każda pozycja linkuje do docelowego pliku, w którym można review'ować zmianę.

## 1. Poprawki merytoryczne (z tabeli §6 brief'u)

| ID | Problem w oryginale | Działanie | Docelowy plik |
|----|---------------------|-----------|---------------|
| 6.1 | Generyki na metodzie struktury (Go tego nie wspiera) — `StatusCache.GetOrFetch[T any]`. | Usunięto kod, opisano wzorzec funkcyjny jako funkcja pakietowa + tabela TTL i invalidacji eventowej. | [docs/DESIGN.md §8](docs/DESIGN.md#8-tr%C3%B3jpoziomowy-status-cache-stale-while-revalidate) |
| 6.2 | Błąd składni JSON: `profiles: [` bez cudzysłowów. | Schema config przedstawiona jako **tabela pól** (pole / typ / wymagane / przykład), pełny przykład JSON odrębnie + zwalidowany ręcznie. | [docs/DESIGN.md §6](docs/DESIGN.md#6-model-danych-i-atomowo%C5%9B%C4%87-zapisu-configjson) |
| 6.3 | Maszyna stanów zdefiniowana dwa razy z różnymi listami. | **Jedna**, scalona lista stanów + diagram przejść. | [docs/DESIGN.md §12](docs/DESIGN.md#12-maszyna-stan%C3%B3w-tui-tabbed-cockpit-spec) |
| 6.4 | Niespójność timeout SSH (15 s vs 30 s). | Jednoznacznie: dial = 15 s, retry × 2, łączny worst case 51 s; komunikaty UI odzwierciedlają łączny czas. | [docs/DESIGN.md §9](docs/DESIGN.md#9-obs%C5%82uga-b%C5%82%C4%99d%C3%B3w-sieciowych-i-reconnect) |
| 6.5 | Konflikt `Ctrl+R` (refresh vs reveal `.env`). | `Ctrl+R` = kontekstowy refresh. Reveal w `/env` przeniesiony na lokalny klawisz `v` (poza globalnym mappingiem) + jednorazowe potwierdzenie. | [docs/UX.md §6](docs/UX.md#6-key-bindings), [docs/UX.md §9](docs/UX.md#9-maskowanie-sekret%C3%B3w-w-ui) |
| 6.6 | Mockupy 80×24 nierealistyczne (~100×30 jest faktycznie potrzebne). | Określono **dwa progi**: minimalny 88×28 (z fallbackami: single-pane, ukryty help bar) i zalecany 100×30. Poniżej 88×28 — bramka startowa. | [docs/UX.md §5](docs/UX.md#5-wymagania-terminala) |
| 6.7 | Sprzeczność „deploy tylko przez GH Actions" vs SFTP w `/storage` i `/env`. | Doprecyzowane: **deploy kodu** = GH Actions, **operacje administracyjne** (env, storage, restart) = SSH/SFTP. ADR-0002 wprost. | [docs/adr/0002-deploy-tylko-przez-github-actions.md](docs/adr/0002-deploy-tylko-przez-github-actions.md) |
| 6.8 | `DA_API_KEY` jako string literal w kodzie. | Usunięto cały kod adaptera DA; w `providers/directadmin.md` opisano: API key zawsze w keyring, ścieżka pobrania, nigdy w configu. | [docs/providers/directadmin.md](docs/providers/directadmin.md) |
| 6.9 | Adaptery cPanel/DA/CyberPanel pisane „z głowy". | Usunięto wszystkie ciała funkcji; zostały notatki badawcze z linkami do oficjalnej dokumentacji i listą otwartych pytań. | [docs/providers/cpanel.md](docs/providers/cpanel.md), [directadmin.md](docs/providers/directadmin.md), [cyberpanel.md](docs/providers/cyberpanel.md) |
| 6.10 | Kreator wymusza krok DB nawet dla statycznych SPA. | Smart skip: dla stacku statycznego krok ukryty, domyślnie „No DB"; widoczny tylko gdy user świadomie zażąda. | [docs/UX.md §4](docs/UX.md#4-layouty-ekran%C3%B3w) (Step 3 wizard) |
| 6.11 | Confirm dialogi po każdej akcji — zmęczą power-userów. | Sekcja **Expert mode** w `/settings`: globalny i per-akcja „don't ask again this session" + per-akcja persystencja. | [docs/UX.md §7](docs/UX.md#7-confirm-dialogs) |
| 6.12 | Mieszanka EN/PL — claim „default EN", mockupy PL. | Wszystkie mockupy przepisane na **EN**. Polski jako opt-in `translations/pl.json`. | [docs/UX.md §10](docs/UX.md#10-internacjonalizacja), [docs/adr/0006-jezyk-interfejsu-en-domyslny.md](docs/adr/0006-jezyk-interfejsu-en-domyslny.md) |
| 6.13 | Kryteria sukcesu aspiracyjne, nie mierzalne. | Każde kryterium ma sposób pomiaru (`webox doctor` raport opt-in, ankieta po N projektach, log lokalny). | [docs/PRD.md §8](docs/PRD.md#8-kryteria-sukcesu--mierzalne) |

## 2. Uzupełnienia (z tabeli §7 brief'u)

| ID | Brakujący temat | Docelowy plik |
|----|-----------------|---------------|
| 7.1 | Threat model + STRIDE-light | [docs/SECURITY.md §3](docs/SECURITY.md#3-threat-model) |
| 7.2 | Migracje schematu config | [docs/DESIGN.md §6.4](docs/DESIGN.md#64-migracje-schematu) |
| 7.3 | Strategia testów (unit / integration / e2e / CI) | [docs/TESTING.md](docs/TESTING.md) |
| 7.4 | Telemetria / error reporting (opt-in, lokalny log) | [docs/DESIGN.md §15](docs/DESIGN.md#15-diagnostyka-doctor--redacted-logger) + [docs/SECURITY.md §7](docs/SECURITY.md#7-audyt-sekret%C3%B3w-i-tryb-doctor) |
| 7.5 | Import istniejących projektów | [docs/PRD.md §7](docs/PRD.md#7-import-istniej%C4%85cych-projekt%C3%B3w) + [docs/UX.md §11.4](docs/UX.md#114-flow-d-import-istniej%C4%85cego-projektu) |
| 7.6 | Konflikt z ręcznymi zmianami w panelu | [docs/DESIGN.md §11](docs/DESIGN.md#11-detekcja-rozbie%C5%BCno%C5%9Bci-konfiguracji-drift--stale-detection) |
| 7.7 | Auto-update binarki | [docs/DESIGN.md §14](docs/DESIGN.md#14-dystrybucja-i-mechanizm-sprawdzania-wersji) |
| 7.8 | Competitive landscape | [docs/PRD.md §5](docs/PRD.md#5-konkurencja-i-landscape) |
| 7.9 | Non-goals | [docs/PRD.md §9](docs/PRD.md#9-non-goals) |
| 7.10 | Środowiska bez keyringu (Linux headless, WSL bez D-Bus) | [docs/SECURITY.md §4.2](docs/SECURITY.md#42-fallback-dla-%C5%9Brodowisk-headless) |

## 3. ADR-y utworzone

- [ADR-0001 — TUI zamiast CLI](docs/adr/0001-tui-zamiast-cli.md)
- [ADR-0002 — Deploy tylko przez GitHub Actions](docs/adr/0002-deploy-tylko-przez-github-actions.md)
- [ADR-0003 — Provider Pattern](docs/adr/0003-provider-pattern.md)
- [ADR-0004 — Przechowywanie sekretów w systemowym keyringu](docs/adr/0004-przechowywanie-sekretow-keyring.md)
- [ADR-0005 — Cache statusów projektów](docs/adr/0005-cache-statusow-projektow.md)
- [ADR-0006 — Język interfejsu: angielski domyślnie](docs/adr/0006-jezyk-interfejsu-en-domyslny.md)

## 4. Zacieśnienie zakresu MVP

- MVP (v0.1) ogranicza się **wyłącznie do providera `smallhost`**.
- Interfejs `HostingProvider` istnieje w kodzie od dnia 1; w `providers/` żyje `mock.go` używany w testach (patrz [docs/TESTING.md §3](docs/TESTING.md#3-mockowanie-ssh)).
- Command Palette w MVP zawiera **tylko** `/create`, `/provider`, `/import` i `/settings` — reszta (`/db`, `/env`, `/storage`, `/domain`) → v0.2+. Patrz [docs/ROADMAP.md §3](docs/ROADMAP.md#3-mvp-v01--wy%C5%82%C4%85cznie-smallpl).

## 5. Archiwum

- Oryginalny monolit nietknięty: [archive/PRD_v0_monolith.md](archive/PRD_v0_monolith.md).
- Mapowanie sekcji oryginału → nowych plików: [docs/MIGRATION_NOTES.md](docs/MIGRATION_NOTES.md).

## 6. Dodatkowe doprecyzowania po pełnym rereadzie

- Dodano publiczny, repozytoryjny [README.md](README.md), który opisuje projekt z perspektywy GitHubowego wejścia do repo.
- Ujednolicono MVP scope między `PRD.md`, `UX.md`, `ROADMAP.md` i `TESTING.md`:
  - `/import` jest częścią minimalnej palette w `v0.1`,
  - logi w MVP to **tail**, nie live stream,
  - manualny checklist testuje `Node.js backend + MySQL`, nie `Next.js + MySQL`.
- Dopisano politykę:
  - atomowego zapisu i blokady `config.json`,
  - osobnego deploy key per projekt,
  - bogatszego modelu `pending_cleanups.json`,
  - first-class traktowania zmian dokumentacyjnych w `CONTRIBUTING.md`.
