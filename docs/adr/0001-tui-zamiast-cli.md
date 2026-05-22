# ADR-0001: TUI zamiast klasycznego CLI

> Status: Accepted · Data: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne ADR: [ADR-0003 Provider Pattern](./0003-provider-pattern.md). Dokumenty: [PRD.md §3](../PRD.md#3-problem-kt%C3%B3ry-rozwi%C4%85zujemy), [UX.md](../UX.md), [DESIGN.md §2](../DESIGN.md#2-wysokopoziomowa-architektura).

## Kontekst

Webox musi obsłużyć operatora hostingu współdzielonego, który dziś przeskakuje między panelem webowym, terminalem SSH, edytorem `.env` i UI GitHuba. Można zaadresować ten problem na trzy sposoby:

1. **Klasyczne CLI** — komendy `webox create my-app`, `webox restart my-app`. Każda akcja niezależnie.
2. **TUI (interaktywny terminal UI)** — jeden ekran z dashbordem, lista projektów, key bindings, persistent state.
3. **Aplikacja webowa lokalna** — `webox` startuje serwer na `localhost:7878`, user otwiera przeglądarkę.

Cele produktu (z [PRD §3](../PRD.md#3-problem-kt%C3%B3ry-rozwi%C4%85zujemy)) wymagają:

- Pokazania **listy projektów z statusami live** (nie pojedyncza akcja, ale ciągły dashboard).
- Szybkiego przełączania kontekstu między operacjami (restart → logi → SSL → wizard).
- Minimalnego context-switchu — operator już siedzi w terminalu.

## Decyzja

Wybieramy **TUI w Go + Bubble Tea + Lipgloss**. W MVP nie ma CLI flag dla zewnętrznych skryptów (poza `webox doctor`). Tryb non-interactive CLI → P2, v0.3+.

Dlaczego nie aplikacja webowa lokalna:

- Wymaga przeglądarki + portu + autostartu serwera — większa powierzchnia ataku i więcej do utrzymania.
- Czas startu „od `webox` do interakcji" rośnie z 200 ms (TUI) do 2–4 s (web).
- Browser context znów wciąga usera do przeskakiwania okien.

Dlaczego nie czyste CLI:

- Operator nie pamięta wszystkich slug'ów projektów; dashboard z listą jest tańszy poznawczo niż `webox list`.
- Akcje seryjne (wizard) wymagają stanu między pytaniami — w czystym CLI to seria flag, niepatchowalnych w trakcie.
- Refresh statusów wymaga persistent process — w CLI to byłaby albo cronowa pętla, albo `watch webox status`.

Bubble Tea wybrane spośród alternatyw:

| Framework | Język | Powód odrzucenia (lub akceptacji) |
|---|---|---|
| **Bubble Tea (Charm)** | Go | **Wybrany.** MVU, aktywny ecosystem (Bubbles, Lipgloss), tests via `teatest`, deklaratywny. |
| `tview` | Go | OOP, mniej deklaratywne, mniej aktywny dev. |
| `gocui` | Go | Niski poziom, dużo boilerplate'u. |
| `textual` (Python) | Python | Wymaga runtime'u Pythona — pakowanie binarne pain. |
| `ink` (Node) | JS | Wymaga Node — pakowanie binarne pain. |
| `ratatui` (Rust) | Rust | Świetny, ale Go ma lepszy ecosystem dla `ssh`/`sftp`/`gh CLI` integracji. |

## Konsekwencje

### Pozytywne

- Pojedyncza binarka, brak zewnętrznych runtime'ów.
- Tests `teatest` dają deterministyczne snapshot testing.
- Idiomatic Go = łatwiej znaleźć contributorów do shared-hosting tooling (Go dominuje w devops).
- MVU model pozwala traktować `update()` jako czystą funkcję — testowalność.

### Negatywne

- TUI wyklucza usera, który chce skryptować webox z bash. Workaround: `webox doctor --json` od dnia 1, pełen CLI flag surface w v0.3+.
- Terminal jest ograniczony rozmiarem — wymaga adresowania w [UX §5](../UX.md#5-wymagania-terminala).
- Renderowanie kolorów / emoji zależy od terminala — fallbacki.
- Brak natywnego copy-paste w niektórych terminalach (Windows cmd) — `Ctrl+Y` dla `/env` może nie działać.

### Neutralne

- Bubble Tea ma stabilne API, ale **major bump** w przeszłości się zdarzył (v1 → v2). Wymaga monitorowania.
- Lipgloss jest opinionated co do stylów — ułatwia spójność, utrudnia dziwactwa.

## Alternatywy rozważane

Wszystkie powyżej w tabeli. Decyzja: Bubble Tea jako optymalny balans dev velocity, testowalności i ecosystem'u.
