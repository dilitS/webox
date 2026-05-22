# Webox — Contributing Guide

> Status: Draft · Ostatnia aktualizacja: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [DESIGN.md](./DESIGN.md), [TESTING.md](./TESTING.md), [SECURITY.md](./SECURITY.md), [providers/smallhost.md](./providers/smallhost.md).

## TL;DR

Webox jest open-source (MIT). Każdy PR przechodzi przez `golangci-lint`, testy z coverage threshold ≥ 70 % i govulncheck. Dodanie nowego providera = implementacja interfejsu z [DESIGN.md §3](./DESIGN.md#3-provider-pattern) + testy z mock SSH + doc w `docs/providers/<nazwa>.md`. Dodanie tłumaczenia = jeden plik `translations/<lang>.json` + skrypt walidacji kluczy. Maintainer review w ciągu 7 dni roboczych.

## Spis treści

1. [Setup deweloperski](#1-setup-deweloperski)
2. [Style kodu i konwencje](#2-style-kodu-i-konwencje)
3. [Jak dodać nowy provider](#3-jak-doda%C4%87-nowy-provider)
4. [Jak dodać tłumaczenie](#4-jak-doda%C4%87-t%C5%82umaczenie)
5. [Proces review i merge](#5-proces-review-i-merge)
6. [Kanał komunikacji](#6-kana%C5%82-komunikacji)
7. [Code of Conduct](#7-code-of-conduct)

---

## 1. Setup deweloperski

### 1.1 Wymagania

| Narzędzie | Wersja minimalna | Cel |
|---|---|---|
| Go | 1.24+ (target: 1.24 LTS-style; CI matrix testuje też 1.25 RC gdy dostępne) | Build. `CGO_ENABLED=0` dla release. |
| `golangci-lint` | **2.x+** (uwaga: zmiana mappingu nazw względem v1 — patrz §2.1) | Linter. |
| `govulncheck` | latest | Skan CVE. |
| `goreleaser` | 2.x | Lokalne snapshot builds. |
| `git` | 2.30+ | Oczywiste. |
| `gh` CLI | 2.30+ | Praca z PR-ami z konsoli. |
| `make` | dowolny | Skróty zadań (`make test`, `make lint`). |

### 1.2 Pierwsze uruchomienie

```bash
git clone https://github.com/<org>/webox.git
cd webox
go mod download
make build
./webox --help
```

### 1.3 Testy lokalne

```bash
make lint           # golangci-lint run
make test           # go test -race -coverprofile=coverage.out ./...
make test-tui       # go test ./tui/... -update (regen golden files)
make vulncheck      # govulncheck ./...
```

### 1.4 Konto testowe small.pl (opcjonalne)

Do uruchomienia integration tests z realnym kontem hostingowym ustaw:

```bash
export WEBOX_TEST_HOST=s1.small.pl
export WEBOX_TEST_USER=<twój login testowy>
export WEBOX_TEST_KEY=~/.ssh/id_ed25519_webox_test
```

Testy z `// +build integration` uruchamiają się **tylko** z tymi zmiennymi.

### 1.5 Debug

- `webox --debug` włącza `webox.log` z poziomem `debug`.
- `WEBOX_LOG_LEVEL=trace ./webox` jeszcze gadatliwszy.
- `dlv debug ./cmd/webox` — Delve dla step-through.

## 2. Style kodu i konwencje

### 2.1 Linter

`golangci-lint v2.x+` z konfiguracją w `.golangci.yml`. Konfiguracja **wymaga** deklaracji `version: "2"` na wierzchu pliku.

Włączone (nazwy v2):

`gofmt`, `goimports`, `govet`, `staticcheck`, `errcheck`, `gocritic`, `revive`, `gocyclo` (**max 20**, motywacja patrz [IMPROVEMENT_PLAN §IMP-19](./IMPROVEMENT_PLAN.md#imp-19-contributingmd-21--gocyclo-max-15-dla-metod-providera)), `gosec`, `misspell`, `unconvert`, `prealloc`, `whitespace`, `unused`, `err113` (post-v1 nazwa `goerr113`), `mnd` (post-v1 nazwa `gomnd`), `loggercheck` (post-v1 nazwa `logrlint`).

Mapowanie nazw v1 → v2 (do uwzględnienia przy migracji starych config'ów):

| v1 | v2 |
|---|---|
| `gas` | `gosec` |
| `goerr113` | `err113` |
| `gomnd` | `mnd` |
| `logrlint` | `loggercheck` |
| `megacheck` | `staticcheck` |

Override per funkcja przez `//nolint:<lint-name>` z **wymaganym** komentarzem uzasadniającym, np. `//nolint:gocyclo // SetupSSL: 7 explicit error paths, splitting would reduce readability`. Każdy `//nolint` review'owany w PR.

### 2.2 Konwencje nazewnicze

| Element | Konwencja | Przykład |
|---|---|---|
| Pakiety | krótkie, lowercase, bez `_` | `providers`, `ssh`, `secrets` |
| Eksportowane typy / funkcje | `PascalCase` | `HostingProvider`, `RegisterProvider` |
| Wewnętrzne typy / funkcje | `camelCase` | `parseDevilOutput` |
| Stałe | `PascalCase` lub `ALL_CAPS` jeśli enum-like | `StateDashboard`, `MaxRetries` |
| Sentinel errors | `ErrFoo` | `ErrSubdomainExists` |
| Test files | `*_test.go` w tym samym pakiecie; `*_export_test.go` dla wewnętrznych asercji | `provider_test.go` |
| Interface naming | rzeczownik bez `I` prefix | `HostingProvider`, nie `IHostingProvider` |

### 2.3 Konwencje commit messages

[Conventional Commits 1.0.0](https://www.conventionalcommits.org/):

```
<type>(<scope>): <opis>

<body opcjonalny>

<footer opcjonalny — np. BREAKING CHANGE, refs>
```

`type`: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `perf`, `ci`, `build`, `revert`.
`scope`: jeden z pakietów top-level (`providers`, `tui`, `ssh`, `config`, `docs`, `ci`, …) lub feature area.

Przykłady:

- `feat(providers): add cyberpanel adapter (issue SSL, basic DB ops)`
- `fix(tui): handle resize below 70x22 without panic`
- `docs(security): clarify TOFU flow for first SSH connection`

### 2.4 Struktura PR

- **Tytuł** = pierwsza linia commit message (Conventional).
- **Body**: motywacja, zmiana, jak przetestować.
- **Linki**: GH Issue, ADR jeśli zmienia architekturę.
- **Checklist** (template w `.github/pull_request_template.md`):

```
- [ ] Tests pass locally (`make test`).
- [ ] Lint clean (`make lint`).
- [ ] Coverage doesn't drop below threshold.
- [ ] Docs updated (if behavior change).
- [ ] CHANGELOG entry added.
- [ ] If security-relevant: SECURITY.md threat model reviewed.
```

### 2.5 Architectural choices

- **Brak globalnych mutowalnych zmiennych.** Stan w `model` lub w explicit struct.
- **Wszystkie efekty I/O w `tea.Cmd`.** Patrz [DESIGN.md §2.3](./DESIGN.md#23-zasady-przep%C5%82ywu-danych-mvu).
- **Errors wrap'owane** przez `fmt.Errorf("%w: ...", err)`. Sentinel errors w `errors.go` pakietu.
- **Brak `panic()`** poza `init()` i fatal startup.

### 2.6 Zmiany dokumentacji są first-class

W Webox dokumentacja nie jest dodatkiem "po kodzie". Jeśli PR zmienia zachowanie, zakres, flow, security posture albo ergonomię operatora, reviewer ma prawo zatrzymać merge do czasu aktualizacji docs.

Minimalna zasada:

- zmiana zakresu → zaktualizuj [PRD.md](./PRD.md) lub [ROADMAP.md](./ROADMAP.md),
- zmiana kontraktu / modelu danych / sekretnych założeń → [DESIGN.md](./DESIGN.md),
- zmiana interakcji użytkownika / skrótów / ekranów → [UX.md](./UX.md),
- zmiana SSH / tokenów / `.env` / release chain → [SECURITY.md](./SECURITY.md),
- zmiana sposobu testowania lub release gate → [TESTING.md](./TESTING.md).

## 3. Jak dodać nowy provider

To **kluczowy** dokument dla społeczności. Krok po kroku.

### 3.1 Krok 1 — Research

Przed pisaniem kodu — `docs/providers/<nazwa>.md` z:

- Linkami do oficjalnej dokumentacji panelu.
- Mapowaniem metod `HostingProvider` na komendy / endpointy.
- Listą otwartych pytań (`TO BE VERIFIED`).
- Edge cases (limity, dziwactwa parsowania).

**Wzór** = `docs/providers/smallhost.md`. Nie pomijaj sekcji.

### 3.2 Krok 2 — Implementacja interfejsu

Plik `providers/<nazwa>.go`:

1. Importuj `providers` (pakiet z interfejsem).
2. Zarejestruj fabrykę w `init()`:

```text
func init() {
    providers.Register("<nazwa>", func(cfg providers.ProviderConfig) (providers.HostingProvider, error) {
        // build the adapter
    })
}
```

3. Implementuj **wszystkie** metody `HostingProvider` z [DESIGN.md §3.2](./DESIGN.md#32-kontrakt--hostingprovider).
4. Użyj `cfg.Properties` do różnic między panelami tego samego rodzaju (np. WHM vs cPanel reseller).

### 3.3 Krok 3 — Parsery output'u

Output panelu (CLI / API) parsuj **defensywnie** ([SECURITY.md §3.3](./SECURITY.md#33-defensywne-parsowanie-outputu)):

- Strip ANSI.
- Named regex groups.
- Fail-soft: nieparsujący się output → konkretny błąd `ErrUnknownOutputFormat` + log dla diagnostyki.

### 3.4 Krok 4 — Testy

1. **Fixture'y outputu** w `testing/fixtures/<panel>/`:
   - Co najmniej: success + failure + edge case (już istnieje, brak, błąd auth).
   - Każdy fixture ma `*.fixture.md` z opisem pochodzenia.
2. **Unit testy parserów** w `providers/<nazwa>_test.go`.
3. **Integration testy** z mock SSH server (`testing/sshmock`) — pełne sekwencje wizard'a.
4. **Coverage** dla nowego adaptera ≥ 75 %.

### 3.5 Krok 5 — Dokumentacja

W `docs/providers/<nazwa>.md` (uzupełnij dokument z §3.1):

- Status: `Stable` / `Experimental` / `Research`.
- Charakterystyka panelu.
- Tabela mapowania metod → komendy.
- Ścieżki plików (deploy / logs).
- Specyficzne `properties`.
- Edge cases i known issues.
- TODO / Open questions.

### 3.6 Krok 6 — Dodanie do rejestru

`main.go` lub `providers/imports.go`:

```text
import (
    _ "github.com/<org>/webox/providers/smallhost"
    _ "github.com/<org>/webox/providers/<nazwa>"
)
```

Pakiet **musi** mieć `init()` rejestrujący się w globalnym registry.

### 3.7 Krok 7 — Flagi

Pierwsza wersja providera = `experimental` flag:

- W kodzie: `Register("<nazwa>", factory, providers.Experimental)`.
- W UI: profil typu `<nazwa>` widoczny tylko gdy `WEBOX_EXPERIMENTAL=1`.
- Po 1 minor release bez critical issues + manualny sign-off maintainera → `Stable`.

### 3.8 Krok 8 — PR

Template PR (`.github/pull_request_template.md` — provider section):

```
## Provider: <nazwa>

- [ ] `docs/providers/<nazwa>.md` complete.
- [ ] All HostingProvider methods implemented.
- [ ] Fixtures in `testing/fixtures/<nazwa>/`.
- [ ] Coverage ≥ 75 % for `providers/<nazwa>.go`.
- [ ] Integration tests against mock SSH pass.
- [ ] At least one manual test against real account (described in PR body).
- [ ] Marked as `experimental`.
- [ ] Linked open questions / future work in PR body.
```

## 4. Jak dodać tłumaczenie

### 4.1 Struktura

`translations/<lang>.json` — patrz [UX.md §10](./UX.md#10-internacjonalizacja). Klucze pochodzą z `translations/en.json` (źródło prawdy).

### 4.2 Procedura

1. Skopiuj `translations/en.json` → `translations/<lang>.json`.
2. Przetłumacz wartości. Klucze zostawiamy w spokoju.
3. Zaktualizuj `_meta.code` i `_meta.name`.
4. Uruchom `make i18n-check` — wykrywa brakujące i nadmiarowe klucze.
5. Test wizualny: `WEBOX_LANG=<lang> ./webox`.

### 4.3 Konwencje

- Klucze hierarchiczne: `wizard.step1.title`, nie `wizard_step1_title`.
- Plurals: dwie warianty (`{n} project` / `{n} projects`) jako osobne klucze: `dashboard.project_count.one`, `dashboard.project_count.other`.
- Nie zmieniaj długości tekstu drastycznie — terminal ma ograniczoną szerokość. Jeśli polski tekst jest 30 % dłuższy → szukaj kompromisu (skrót, alternatywne sformułowanie).

### 4.4 PR template (translation)

```
## Translation: <lang>

- [ ] `translations/<lang>.json` complete.
- [ ] `make i18n-check` passes.
- [ ] Manual test of dashboard, wizard, settings.
- [ ] Native speaker review (or self-review if maintainer is native).
```

## 5. Proces review i merge

### 5.1 Kto reviewuje

- **Bug fixes / docs:** 1 maintainer.
- **Nowy ficzer / nowy provider:** 1 maintainer **+** 1 contributor lub maintainer drugi.
- **Zmiana architektury (ADR):** 2 maintainerów + 7 dni okienka feedback (issue / discussion).
- **Security-sensitive (sekrety, SSH, GH token):** wymagane review przez maintainera + sign-off w PR description.

### 5.2 SLA review

- Pierwsza odpowiedź: 7 dni roboczych.
- Pełny review: 14 dni roboczych.
- Re-review po addressed comments: 3 dni robocze.

Jeśli przekroczone — można ping'nąć w GH Discussions kanale `meta` lub e-mailem maintainera.

### 5.3 Merge strategy

- **Squash and merge** dla feature branches.
- **Rebase and merge** dla małych fixów (≤2 commitów, czyste).
- **Merge commit** — nigdy w main.

### 5.4 CI gate

PR nie zostaje merge'owany dopóki:

- ✅ `lint` zielony.
- ✅ `test-linux` i `test-macos` zielone.
- ✅ `vulncheck` zielony.
- ✅ `build` zielony.
- ✅ Wszystkie review comments addressed (resolved) lub explicit acceptance maintainera.

## 6. Kanał komunikacji

- **GitHub Issues:** bug reports, feature requests.
- **GitHub Discussions:** otwarte pytania, design talk, RFC dla ADR.
- **Discord (opcjonalnie, post-MVP):** chat realtime. Jeśli powstanie — link w README repo.
- **Prywatny kanał maintainera:** tylko sprawy prywatne i security disclosure. Dedykowany adres kontaktowy powinien zostać dodany przed publicznym launch repo.
- **GitHub Security Advisories:** **wyłączny** kanał dla podatności bezpieczeństwa. Nie zgłaszaj CVE w issues.

## 7. Code of Conduct

Webox przyjmuje **Contributor Covenant v2.1**. Pełny tekst w `CODE_OF_CONDUCT.md` w root repo. W skrócie:

- Bądź uprzejmy.
- Nie atakuj osób, krytykuj kod / idee.
- Akceptujemy prywatne zgłoszenia o naruszeniach do maintainera; publiczny adres kontaktowy zostanie dodany przed publicznym launch repo.
- Maintainer może zablokować na 24 h / 7 dni / permanentnie, transparentnie z uzasadnieniem.

**Brak tolerancji dla:** harassment, doxxing, dyskryminacja, spam, ad hominem.
