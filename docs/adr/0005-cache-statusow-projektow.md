# ADR-0005: Cache statusów projektów na dashboardzie

> Status: Accepted · Data: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne ADR: [ADR-0001 TUI](./0001-tui-zamiast-cli.md), [ADR-0003 Provider Pattern](./0003-provider-pattern.md). Dokumenty: [DESIGN §8](../DESIGN.md#8-tr%C3%B3jpoziomowy-status-cache-stale-while-revalidate), [UX §4.2](../UX.md#42-dashboard-20--bento-box-grid-system-12035--stretch).

## Kontekst

Dashboard pokazuje per projekt:

- HTTP status (200/3xx/5xx).
- Wersja Node z serwera.
- Days left dla cert SSL.
- Last deploy timestamp + status (z GH Actions).

Pomiar: 20 projektów × (1 HTTP ping + 1 SSH `node --version` + 1 SSL probe + 1 GH API call) = ~80 operacji I/O. Sumarycznie 20–40 s bez optymalizacji. To **nieakceptowalne dla TUI**, który musi reagować w <100 ms na navigację.

Możliwe strategie:

1. **Brak cache** — fetch każdy refresh, dashboard zamarznięty.
2. **Pełen TTL cache w pamięci** — szybko, ale dane mogą być przestarzałe bez sygnału użytkownikowi.
3. **Stale-while-revalidate (SWR)** — pokaż stare, refresh w tle, oznacz "stale".
4. **Cache na dysku** (persystencja) — szybkie cold-start, ale dane wrażliwe (SSL info → krótkoterminowo).
5. **Subskrypcja przez webhook / SSE** — niemożliwe (small.pl nie wystawia webhooków).

## Decyzja

**Stale-while-revalidate cache w pamięci**, trójpoziomowy (HTTP / SSH / SSL / deploy):

- Każdy typ ma własny TTL (30 s / 60 s / 300 s / 60 s).
- Po wygaśnięciu TTL UI wciąż pokazuje stare dane z badge'em `(buffered Xs ago)`, w tle uruchamia fetch.
- Cache trzymany **wyłącznie w RAM procesu webox** — nie persystowany do dysku.
- Invalidacja eventowa: po `Restart`, `Deploy`, `SetupSSL` odpowiednia kategoria cache jest czyszczona.
- **Wzorzec funkcyjny** zamiast generycznej metody na strukturze (Go tego nie wspiera) — patrz [DESIGN §8.4](../DESIGN.md#84-wzorzec-funkcyjny-zamiast-generycznej-metody).

## Dlaczego SWR a nie inne

### Brak cache

Każdy ↓↑ na liście projektów = 30 s spinning. Niemożliwe użytkowanie.

### Pełen TTL cache

Tablica TTL'i z hard-cutoffem:

- TTL 30 s: po 30 s user dostaje fresh dane, ale na sekundę 31 czeka 2 s na fetch — wciąż gorsze niż SWR.
- Bez wskaźnika świeżości — user nie wie czy patrzy na zbuforowane czy aktualne.

### Cache na dysku

- **Wycieki**: SSL days_left wycieka informacje o twoim hostingu jeśli ktoś weźmie laptop. Akceptowalne, ale po co.
- **Stale on restart**: po `webox` start, cold cache = świeży fetch, więc dysk pomógłby tylko przy bardzo krótkich relaunchach. Niewart dodatkowej kompleksowości.
- **Race conditions**: webox uruchomiony w dwóch terminalach jednocześnie ma dwa cache na tym samym dysku.

### Webhooki / subskrypcje

- Small.pl, cPanel itp. nie wystawiają webhooków.
- GitHub ma webhooki, ale wymagają public endpoint — webox jest lokalny.
- Brak realnej opcji dla shared hostingu.

## Parametry cache

| Typ | TTL | Powód TTL'a | Invalidacja eventowa |
|---|---|---|---|
| HTTP status (200/3xx/5xx) | 30 s | Status HTTP może się zmienić podczas deploymentu. | Po `Restart` i `Deploy` (manualny). |
| SSH Node version | 60 s | Wersja Node zmienia się rzadko (przez wizard / `/settings`). | Po `ChangeNodeVersion`. |
| SSL cert info | 300 s (5 min) | Cert zmienia się raz na 60–90 dni. | Po `SetupSSL` / `RenewSSL`. |
| Last deploy (GH Actions) | 60 s | Workflow runs aktualizują się co kilkadziesiąt s. | Po `Deploy` (push) + w przyszłości po webhook. |

## Konsekwencje

### Pozytywne

- Pierwsze renderowanie dashboardu po starcie: <200 ms dzięki renderowi z pustego / stale cache.
- Cold cache nie blokuje na wszystkie projekty naraz. HTTP i GH fetch'e mogą startować równolegle, ale SSH-heavy checki są limitowane przez pool (`max_connections=3` per host). Dla 20 projektów realistyczny pełny warm-up to ~30-40 s; UI pokazuje statusy stopniowo, projekt po projekcie, bez zamrożenia interakcji.
- Każdy kolejny render: instant (cache hit).
- SWR daje wrażenie szybkiego UI bez maskowania problemów (offline → stary stan z badge'em `OFFLINE`).

### Negatywne

- **Świeżość:** dane mogą być przestarzałe do 300 s (SSL). Mitygacja: badge `(buffered 287s ago)` + `Ctrl+R` force refresh.
- **RAM**: 20 projektów × ~200 B per cache entry × 4 kategorie = ~16 KB. Pomijalne.
- **Race conditions**: dwie goroutyny próbują fetch tego samego key. Mitygacja: singleflight (`golang.org/x/sync/singleflight`) — jeden fetch in-flight per key.
- **Stale cache misleading**: w bardzo rzadkim przypadku (cert wygasł między TTL invalidate i fetch) user widzi stare "27 days left". Mitygacja: kolor badge'a zmienia się na żółty dla `(buffered > 60s)`, na pomarańczowy dla `(buffered > 180s)`.

### Neutralne

- Cache wyłączany w trybie `--no-cache` (debug).
- Sentry-style metryki cache hit rate dostępne w `webox doctor` (post-MVP).

## Concurrency

- `*Cache` wewnętrznie używa `sync.RWMutex`.
- Każdy fetch w osobnej goroutynie (przez `tea.Cmd`).
- Singleflight zabezpiecza przed thundering herd.
- Cancellation: gdy user wychodzi z dashboardu, kontekst fetch'a jest cancelled (`context.WithCancel`).

## Alternatywy rozważane

Wszystkie omówione powyżej. SWR jest najlepszym balansem responsywności i świeżości.

## Implikacje dla testów

Patrz [TESTING §2.1](../TESTING.md#21-unit--60--wszystkich-test%C3%B3w). Cache musi mieć:

- Test TTL (`time.Now` injectable).
- Test SWR (cache miss → fetch + fill, cache stale → return + bg refresh).
- Test invalidacji eventowej.
- Test race (`go test -race`).
- Test singleflight (dwie równoległe operacje na tym samym key → jeden fetch).
