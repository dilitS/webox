# MIGRATION NOTES — z monolitu do nowej struktury

> Status: Draft · Ostatnia aktualizacja: 2026-05-22 · Właściciel: @maintainer
>
> **Cel dokumentu:** ułatwić review restrukturyzacji. Pokazuje, dokąd trafiła każda sekcja oryginalnego `archive/PRD_v0_monolith.md`. Po zatwierdzeniu zmian plik można usunąć.

## TL;DR

Oryginalny monolit (sekcje §1–§11) został rozbity na **rdzeń dokumentacji + ADR-y + docs providerów**. Treść produktowa została w `PRD.md`, techniczna trafiła do `DESIGN.md`, mockupy i flowy do `UX.md`. Sekcje, których w oryginale nie było (bezpieczeństwo, testy, contributing, ADR-y, notatki providerów post-MVP), powstały od zera i są oznaczone jako `Draft`.

## Mapowanie sekcji oryginału → docelowe pliki

| Oryginalna sekcja (`archive/PRD_v0_monolith.md`) | Docelowa lokalizacja | Zmiana / poprawka |
|---|---|---|
| §1 Vision | `PRD.md §2 Wizja i motto` | Skrócone, bez zmian merytorycznych. |
| §2 Target User | `PRD.md §4 Persony` | **Rozszerzone** do dwóch person (freelancer / agencja). |
| §3.1 MVP Features F1–F8 | `PRD.md §6 Ficzery` (kolumna P0) + `ROADMAP.md §3` | Dodano kolumnę Priorytet i Provider-agnostic. |
| §3.2 Full Features F9–F19 | `PRD.md §6 Ficzery` (kolumny P1/P2) + `ROADMAP.md §4–5` | Posortowane wg priorytetu. |
| §4.1 Stack technologiczny | `DESIGN.md §2 Wysokopoziomowa architektura` (tabela bibliotek) | Bez bloków kodu Go. |
| §4.2 Provider Pattern + kod interfejsu | `DESIGN.md §3 Provider Pattern` | Interfejs jako tabela kontraktu + dozwolone podpisy w `providers/smallhost.md`. |
| §4.3 Struktura katalogów | `DESIGN.md §2.1 Layout repo` | Skrócone do drzewa wysokiego poziomu. |
| §4.4 Przepływ danych (ASCII) | `DESIGN.md §2.2 Przepływ danych` | Diagram zostaje. |
| §4.5 SSH Error Handling | `DESIGN.md §9 Obsługa błędów SSH` | **Poprawka 6.4**: doszlifowana macierz timeoutów (51 s realnie). |
| §4.6 Status Cache + kod generyczny | `DESIGN.md §8 Status cache` | **Poprawka 6.1**: usunięto niekompilujący się kod, opis wzorca funkcyjnego. |
| §5.1 Paleta | `UX.md §2 Design system` | Bez Go, sama tabela HEX. |
| §5.2 Globalne style (kod Go) | usunięte | Tylko opis prozą. |
| §5.3 Komponenty wizualne | `UX.md §3 Komponenty wizualne` | Mockupy ASCII zachowane. |
| §5.4 Layouty wizardów | `UX.md §4 Layouty ekranów` | Mockupy przepisane na EN (**Poprawka 6.12**). |
| §5.4 Rollback transakcyjny | `DESIGN.md §10 Rollback transakcyjny` + `UX.md §11 Flow rollback` | Część koncepcyjna → DESIGN, mockup → UX. |
| §5.5 Maszyna stanów | `DESIGN.md §12 Maszyna stanów TUI` | **Poprawka 6.3**: jedno źródło prawdy, scalone z §5.8. |
| §5.6 Key bindings | `UX.md §6 Key bindings` | **Poprawka 6.5**: konflikt `Ctrl+R` rozwiązany. |
| §5.7 Confirm Dialogs | `UX.md §7 Confirm dialogs` + nowy podrozdział `Expert mode` (**Poprawka 6.11**). |
| §5.8 Command Palette + sub-widoki | `UX.md §8 Command Palette` | `/create`, `/provider`, `/import` i `/settings` zostają w MVP, reszta → v0.2+ (ROADMAP). |
| §5.8.4 `/env` reveal | `UX.md §9 Maskowanie sekretów w UI` | **Poprawka 6.5**: reveal nie używa już Ctrl+R. |
| §6.1 Schemat config.json | `DESIGN.md §6 Model danych` | **Poprawka 6.2**: błędny JSON zastąpiony tabelą pól. |
| §6.2 Modele Go | usunięte | Mapowanie typów w tabeli. |
| §6.3 Model stanu TUI (kod Go) | `DESIGN.md §12.2 Stan w pamięci` | Tabela, bez kodu. |
| §7 Roadmap (etapy 1–4) | `ROADMAP.md` | Zacieśnienie MVP wyłącznie do small.pl. |
| §8.1 Rejestr providerów (kod Go) | `DESIGN.md §4 Rejestr providerów` | Opis tekstowy + diagram. |
| §8.2 Adapter small.pl (kod Go) | `providers/smallhost.md` | Tylko podpisy + tabela mapowania komend (**Poprawka 6.9**). |
| §8.3 Dodawanie providera | `CONTRIBUTING.md §3` | Krok po kroku. |
| §8.4.1–8.4.3 cPanel/DA/CyberPanel (kod Go) | `providers/cpanel.md`, `directadmin.md`, `cyberpanel.md` | **Poprawki 6.8 + 6.9**: kod usunięty, zostały notatki badawcze z linkami do oficjalnej dokumentacji. |
| §9 Flow użytkownika | `UX.md §11 Flowy użytkownika` | Dodano flow `import`. |
| §10 Wymagania systemowe | `PRD.md §10 Wymagania i ograniczenia` + `UX.md §5 Wymagania terminala` | **Poprawka 6.6**: realistyczny rozmiar terminala. |
| §11 Kryteria sukcesu | `PRD.md §8 Kryteria sukcesu — mierzalne` | **Poprawka 6.13**: każde kryterium ma sposób pomiaru. |

## Nowe sekcje / pliki (nie istniały w oryginale)

| Plik / sekcja | Powód powstania |
|---|---|
| `PRD.md §3 Problem` | Wymóg §5.2 instrukcji — rozbudowany opis bólu. |
| `PRD.md §5 Konkurencja` | Wymóg §7.8 — Coolify, Dokploy, panele, ręczny workflow. |
| `PRD.md §7 Import istniejących projektów` | Wymóg §5.2 / §7.5 — feature P0. |
| `PRD.md §9 Non-goals` | Wymóg §5.2 / §7.9 — granica scope. |
| `DESIGN.md §6.4 Migracje schematu` | Wymóg §7.2. |
| `DESIGN.md §11 Konflikty z ręcznymi zmianami w panelu` | Wymóg §7.6. |
| `DESIGN.md §14 Auto-update` | Wymóg §7.7. |
| `DESIGN.md §15 Telemetria i logi diagnostyczne` | Wymóg §7.4. |
| `SECURITY.md` (cały) | Wymóg §5.5 / §7.1 / §7.10. |
| `TESTING.md` (cały) | Wymóg §5.6 / §7.3. |
| `CONTRIBUTING.md` (cały) | Projekt open-source od dnia 1 — patrz [ADR-0006](./adr/0006-jezyk-interfejsu-en-domyslny.md) i [decyzje w PRD §12](./PRD.md#12-decyzje-otwarte). |
| `adr/0001`–`adr/0006` | Wymóg §3 instrukcji. |
| `providers/smallhost.md` | Specyfikacja źródła prawdy dla MVP. |
| `providers/{cpanel,directadmin,cyberpanel}.md` | Notatki badawcze post-MVP zamiast wymyślonego kodu. |
| `UX.md §5 Wymagania terminala` | **Poprawka 6.6** + brakująca sekcja w oryginale. |
| `UX.md §10 Internacjonalizacja` | **Poprawka 6.12** + ADR-0006. |
| `../README.md` | Publiczny entry point repo dodany po restrukturyzacji, żeby projekt miał GitHub-ready opis niezależnie od mapy `docs/`. |

## Co zostało usunięte (i dlaczego)

| Treść z oryginału | Powód usunięcia |
|---|---|
| Implementacje `cpanel.go`, `directadmin.go`, `cyberpanel.go` w §8.4 | Pisane z głowy, niezweryfikowane z dokumentacją vendora (**Poprawki 6.8, 6.9**). |
| `DA_API_KEY` jako string literal w kodzie DirectAdmin | Sekret nigdy w pliku źródłowym — zasada [ADR-0004](./adr/0004-przechowywanie-sekretow-keyring.md). |
| Generyczna metoda `StatusCache.GetOrFetch[T any]` | Go nie wspiera generyków na metodach (**Poprawka 6.1**). |
| Aspiracyjne kryteria sukcesu typu „łatwo" | Niewykonalne pomiar (**Poprawka 6.13**). |
| Sprzeczność „deploy tylko przez GH Actions" vs SFTP | Doprecyzowane w [ADR-0002](./adr/0002-deploy-tylko-przez-github-actions.md) (**Poprawka 6.7**). |

## Po review

Po zatwierdzeniu restrukturyzacji ten plik można skasować — historię ruchu treści zachowuje `archive/PRD_v0_monolith.md` + git log.
