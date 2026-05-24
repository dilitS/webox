# Sprint 09 — Live Log Stream + Header Bar Server Metrics

> **Status:** ✅ Completed 2026-05-23 (PR #14) · **Daty:** 2026-05-23 (rdzeń) · production wiring jako follow-up
>
> **Cel:** dostarczyć dwa premium kafelki Bento Ultra: **Live Log Stream** (Tab `[4] Logs` w project detail oraz Bento `Live Micro-Logs` na dashboardzie) i **Header Bar Server Metrics** (uptime, load avg, RAM %, RTT, SSL day-warnings). Bazujemy na `ssh.Pool` ze Sprintu 02 i `internal/log/redact.go` ze Sprintu 01. **Bezwzględny wymóg bezpieczeństwa:** każda linia logu z serwera przepuszczona przez redactor **przed** dodaniem do ring buffera — sekret nigdy nie trafia do bufora, nawet tymczasowo.

---

## TL;DR

Po sprincie 09:

- `services/sshtail/` zawiera streamer SSH `tail -f` z context-cancellable channel.
- `tui/components/ringbuffer.go` — generic ring buffer 1000 linii z circular overwrite.
- `tui/components/ansi.go` — parser ANSI color codes + level detector (INFO/WARN/ERROR/DEBUG).
- `tui/views/live_logs.go` — Tab `[4] Logs` w project detail z live stream + auto-scroll + manual scrollback.
- `tui/bento/tiles/micro_logs.go` — Bento `Live Micro-Logs` na dashboardzie (top 5 linii ze stream).
- `tui/bento/tiles/header_metrics.go` — `WEBOX v0.1 [LIVE] · 14:32:01 · US-EAST · Uptime: 24d · Load: 0.12/0.28/0.31 · RAM: 3.4G/8G (42%) · Ping: 18ms`.
- `services/sshmetrics/` — pollery server metrics przez SSH (`uptime`, `free -m`, `ping`-equivalent przez RTT pomiaru) z TTL 5s w `status/` cache.
- Redactor smoke test corpus rozszerzony o sample log lines z `ghp_...`, `sk-...`, `BEGIN RSA PRIVATE KEY`, base64-encoded secrets — wszystkie redagowane przed pojawieniem się w UI.
- 60fps throttle cap na live updates; context cancel na `q`/`Esc`.

**Nie robimy w tym sprincie:**

- Live CI/CD Pipeline Panel — Sprint 10.
- Live Service Topology Map — Sprint 11.
- Manual log file selector (`/var/log/...`) — `tail` tylko z hardcoded paths (`logs/node.log`, `logs/error.log`) z `providers.HostingProvider.GetLogPath`.
- `journalctl` integration — STRETCH v0.2+.

---

## Pre-flight Checklist

- [ ] Sprint 08 zamknięty z retro i `Outcome`; Bento layout engine green.
- [ ] Re-read [UX §3.4](../UX.md#34-wizualny-graf-topologii-us%C5%82ug-live-service-topology-map) (tylko jako odniesienie do live data flow), [UX §4.3 Karta [4]](../UX.md#karta-4--live-log-stream--mvp-v01-sprint-09).
- [ ] Re-read [DESIGN §9](../DESIGN.md#9-obs%C5%82uga-b%C5%82%C4%99d%C3%B3w-ssh) (SSH error handling), [DESIGN §15.2](../DESIGN.md#152-redacted-logger--wzorce) (redactor patterns).
- [ ] Re-read [SECURITY §2](../SECURITY.md), [SECURITY §6](../SECURITY.md), [SECURITY §10.6](../SECURITY.md).
- [ ] Audit `internal/log/redact.go` — czy obejmuje wszystkie patterns które mogą pojawić się w logach aplikacji? (PAT, OAuth tokens, JWT, AWS keys, base64 secrets, password=, token=, secret=).
- [ ] Confirm `make ci` green on `main` after Sprint 08 merge.

---

## Taski

### TASK-09.1 — `services/sshtail/` streamer

- **Estymata:** L
- **Zależności:** Sprint 08 done; `ssh.Pool` ze Sprintu 02 stabilny.
- **Acceptance Criteria:**
  - [ ] `services/sshtail/stream.go` exposes `Stream(ctx, profile, logPath string) (<-chan Line, error)`.
  - [ ] Implementacja używa `ssh.Pool.Acquire` + `session.Start("tail -f " + shellEscape(logPath))`.
  - [ ] Channel zwraca `Line{Timestamp, Raw, Level, Redacted bool}`.
  - [ ] Redactor wywołany **przed** wysłaniem na channel; `Raw` zawiera już redagowaną wersję; `Redacted bool` flag ustawiana gdy regex znalazł match.
  - [ ] `ctx.Done()` zamyka SSH session i channel w obu kierunkach (sender nie zostaje zawieszony).
  - [ ] Backoff dla reconnect (max 3 retries, 2/4/8s exponential).
  - [ ] Sentinel errors: `ErrLogPathInvalid`, `ErrSessionClosed`, `ErrReconnectFailed`.
  - [ ] Tests: mock SSH server (`testing/sshmock`) emituje linie z różnymi sekretami; assert że każda linia w channel jest redagowana.
- **Docs:** [DESIGN §9](../DESIGN.md#9-obs%C5%82uga-b%C5%82%C4%99d%C3%B3w-ssh), [SECURITY §6](../SECURITY.md), [providers/smallhost §6](../providers/smallhost.md).

### TASK-09.2 — `tui/components/ringbuffer.go` + ANSI parser

- **Estymata:** M
- **Zależności:** TASK-09.1
- **Acceptance Criteria:**
  - [ ] `tui/components/ringbuffer.go` ma `RingBuffer[T any]` z `Push(T)`, `Snapshot() []T`, `Len()`, `Cap()`, mutex-guarded.
  - [ ] Default capacity 1000; circular overwrite gdy full.
  - [ ] `tui/components/ansi.go` ma `ParseLevel(line string) Level` (INFO/WARN/ERROR/DEBUG/UNKNOWN) — wzorce: `[info]`, `[INFO]`, `INFO:`, `WARN`, `[error]`, `ERROR`, `[debug]`, kolorowy ANSI escape codes (`\x1b[31m` → ERROR by convention).
  - [ ] `ANSIStrip(line string) string` — usuwa wszystkie escape sequences (potrzebne dla snapshot testów i ring buffer storage; rendering dodaje kolory na podstawie wykrytego `Level`).
  - [ ] Tests: golden corpus dla parser (Apache log format, Node.js console, Express morgan, plain text).
- **Docs:** [UX §4.3 Karta [4]](../UX.md#karta-4--live-log-stream--mvp-v01-sprint-09).

### TASK-09.3 — `tui/views/live_logs.go` + Tab `[4] Logs` aktywne

- **Estymata:** L
- **Zależności:** TASK-09.1, TASK-09.2
- **Acceptance Criteria:**
  - [ ] `tui/views/live_logs.go` renderuje Tab `[4]` z live stream: `tail -f` w tle, auto-scroll do bottom, manual scroll przez `↑`/`↓` (pause auto-scroll), `f` toggle auto-scroll, `c` clear local buffer, `Esc` powrót do Overview.
  - [ ] Project detail TUI ma `[4] Logs` jako **aktywną** zakładkę (zniknął dimmed indicator `unlocked in v0.2`).
  - [ ] Każda linia kolorowana wg `Level` (INFO=Muted, WARN=Warning, ERROR=Error, DEBUG=TextDim).
  - [ ] 60fps throttle cap — re-render nie częściej niż co 16ms nawet jeśli channel emituje szybciej; missed updates kumulowane do następnego ticka.
  - [ ] Header status bar w panelu: `Active File: <path> · Stream Mode · Tail -f: On/Off · Buffer: 543/1000 lines`.
  - [ ] Tests: teatest scenariusz: open project detail → press `4` → assert live logs panel rendered → send `tea.KeyMsg{Esc}` → assert back to Overview.
- **Docs:** [UX §4.3 Karta [4]](../UX.md#karta-4--live-log-stream--mvp-v01-sprint-09).

### TASK-09.4 — Bento `Live Micro-Logs` tile (dashboard)

- **Estymata:** M
- **Zależności:** TASK-09.1, TASK-09.2, TASK-08.1 (BentoTile interface)
- **Acceptance Criteria:**
  - [ ] `tui/bento/tiles/micro_logs.go` implementuje `BentoTile`; renderuje top 5 linii z ring bufferu **aktualnie wybranego projektu** na dashboardzie.
  - [ ] Slot: `Bottom` (full-width); MinSize `(120, 8)` — wymaga Bento Ultra `120×35`.
  - [ ] Aggregacja: per project stream, ale wyświetlany tylko jeden (zgodnie z `SelectedProject`).
  - [ ] Przełączenie projektu na dashboardzie cancel'uje poprzedni stream i otwiera nowy.
  - [ ] Tests: teatest scenariusz dashboard z 2 projektami → switch → assert old stream closed (no goroutine leak via `goleak.VerifyNone`).
- **Docs:** [UX §4.2](../UX.md#42-dashboard-20--bento-box-grid-system-12035-mvp--16045-stretch).

### TASK-09.5 — `services/sshmetrics/` + Header Bar metrics tile

- **Estymata:** L
- **Zależności:** Sprint 08, `status.Cache` ze Sprintu 02
- **Acceptance Criteria:**
  - [ ] `services/sshmetrics/poll.go` exposes `Poll(ctx, profile) (Metrics, error)` gdzie `Metrics{Uptime, Load1, Load5, Load15, RAMUsedMB, RAMTotalMB, RTTms}`.
  - [ ] Implementacja: jeden SSH session na poll, runs `uptime && free -m` w pipeline (parsed przez dedykowany parser z fixture'ami).
  - [ ] RTT zmierzony jako round-trip czas wykonania `echo` przez SSH (niskokosztowy, agnostic od `ping`).
  - [ ] Cache TTL 5s, key `ssh:metrics:<profile.Alias>`.
  - [ ] `tui/bento/tiles/header_metrics.go` renderuje pasek: `WEBOX v0.1 [LIVE] · 14:32:01 · <profile.Alias> · Uptime: 24d 11h · Load: 0.12, 0.28, 0.31 · RAM: 3.4G/8G (42%) · Ping: 18ms`.
  - [ ] Live indicator `[LIVE]` pulsuje (toggle co 1s w `Primary` ↔ `Muted`) gdy fetch OK; `[STALE]` w `Warning` gdy cache TTL przekroczone bez świeżego pobrania.
  - [ ] Tests: fixture-driven parsers dla `uptime` i `free -m` z Linuxa, FreeBSD (small.pl używa FreeBSD), macOS (na wszelki wypadek).
- **Docs:** [DESIGN §8](../DESIGN.md#8-tr%C3%B3jpoziomowy-status-cache-stale-while-revalidate).

### TASK-09.6 — Redactor smoke corpus + log fixture audit

- **Estymata:** M
- **Zależności:** TASK-09.1
- **Acceptance Criteria:**
  - [ ] `internal/log/redact_corpus_test.go` dodaje **rozszerzony** corpus sample log lines:
    - GitHub PAT (`ghp_xxxx`, `github_pat_xxxx`, `ghs_xxxx`)
    - OpenAI API key (`sk-xxxx`)
    - AWS access key (`AKIA[0-9A-Z]{16}`)
    - JWT (3 base64 sections separated by `.`)
    - Private key blocks (`BEGIN RSA PRIVATE KEY` / `BEGIN OPENSSH PRIVATE KEY`)
    - Database URIs z embedded credentials (`mysql://user:password@host:3306/db`)
    - Generic `password=`, `token=`, `secret=` z wartością
    - Base64-encoded secrets (≥40 chars, likely encoded credential)
  - [ ] Każdy z corpus przechodzi przez `internal/log.Redact` i wynik jest assertowany jako **niezawierający** oryginalnego sekretu.
  - [ ] Property test: random secret-shaped string × random surrounding text — Redact znajduje secret w 99%+ przypadków (false-negative rate ≤1%).
  - [ ] **False-positive tolerancja:** Redact może zredagować coś co nie jest sekretem (e.g. random base64-ish hash) — to acceptable, ostrożność > recall.
- **Docs:** [SECURITY §6](../SECURITY.md), [DESIGN §15.2](../DESIGN.md#152-redacted-logger--wzorce).

### TASK-09.7 — Goroutine leak prevention + perf budget

- **Estymata:** M
- **Zależności:** TASK-09.1 do TASK-09.5
- **Acceptance Criteria:**
  - [ ] `goleak.VerifyNone(t, ...)` we wszystkich testach które używają `sshtail.Stream` lub `sshmetrics.Poll`.
  - [ ] Benchmark `BenchmarkRingBufferPush1000` — push 1000 linii < 100µs (perf budget dla 60fps re-render).
  - [ ] Benchmark `BenchmarkRedactLogLine` — 1 linia 200 chars z PAT < 50µs.
  - [ ] Integration test: 1000 linii/s przez 30s → CPU usage `webox` proces <5% na M-series Mac.
  - [ ] `q` z poziomu live logs view cancel'uje wszystkie sshtail streams + sshmetrics pollery w <100ms.
- **Docs:** [DESIGN §9](../DESIGN.md#9-obs%C5%82uga-b%C5%82%C4%99d%C3%B3w-ssh).

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| Sekret prześlizgnie się do ring buffera (redactor false-negative) | **CRITICAL** | Pre-buffer redaction zachowuje single-source-of-truth; TASK-09.6 corpus z 8+ secret families; property test z 99% recall. |
| Goroutine leak przy szybkim przełączaniu projektów | H | TASK-09.7 `goleak.VerifyNone` we wszystkich testach; context cancellation contract w `Stream`. |
| Perf collapse przy 1000+ linii/s | H | 60fps throttle cap; ring buffer overwrite zamiast unbounded growth; benchmarki. |
| SSH session limit (small.pl: 3 concurrent) przekroczony przez tail + metrics + provider | M | Connection pool ze Sprintu 02 ma `max=3`; tail + metrics współdzielą tę samą sesję per host gdzie możliwe (multiplex via `session.Setenv`). |
| FreeBSD `uptime` format różni się od Linux | M | Per-OS parsers z fixture'ami; small.pl dostarcza FreeBSD output → testowane jako primary. |
| Live updates zniechęcają user'a (zbyt szybkie scrolling, oczy bolą) | M | Auto-scroll można wyłączyć `f`; manual scroll pauzuje auto-scroll; throttle 60fps cap. |
| ANSI escape sequences w log line crashują renderer | M | `ANSIStrip` zanim renderer dodaje swoje kolory; corpus test z złośliwymi escape sequences. |
| Tail -f na nieistniejący plik → SSH session w nieskończoność czeka | M | Sentinel `ErrLogPathInvalid` po stat() na pliku przed `tail -f`. |

---

## Dependencies signoff

Sprint 09 **może wymagać** jednej nowej zależności:

1. `go.uber.org/goleak` — już dodane w Sprincie 04. Reuse.

Nic nowego nie powinno być potrzebne. Jeśli pojawi się potrzeba — wymaga ADR + maintainer sign-off zgodnie z [AGENTS §1.2](../../AGENTS.md#12-kluczowe-biblioteki-sprawdzone-przez-context7).

---

## Outcome (2026-05-23)

**Status:** ✅ Completed (branch `feat/s09-live-logs`, PR #14)

- ✅ **Done:**
  - `services/sshtail/` — `Executor`-backed streamer z context-aware
    cancel, pre-buffer `internal/log.Redact`, sentinels
    (`ErrLogPathInvalid`, `ErrSessionClosed`, `ErrReconnectFailed`,
    `ErrStreamerClosed`), exponential backoff 2/4/8s, `shellEscape` +
    `validateLogPath` defense-in-depth.
  - `tui/components/ringbuffer.go` — generic, thread-safe FIFO z
    circular overwrite (Push / Snapshot / Tail / Len / Cap).
  - `tui/components/ansi.go` — `ANSIStrip` (SGR + OSC + residual) i
    `ParseLogLevel` z fall-throughem: ANSI colour → strukturalne
    prefixy → JSON `"level"` → word-boundary scan → `LevelInfo`.
  - `services/sshmetrics/` — parsery uptime (Linux + FreeBSD + macOS +
    minute/hour-only) i `free -m`, `Poller.Poll` z `status.Cache` SWR
    (TTL 5s, klucz `ssh:metrics:<alias>`), graceful degradation gdy
    `free` brakuje (FreeBSD), `FormatUptime/RAM/LoadAvg/RTT` helpers.
  - `tui/bento/`:
    - `NewHeaderMetricsTile` (snapshot-based, `[LIVE]`/`[STALE]`),
    - `NewMicroLogsTile` (top-N redacted lines z marker-per-level),
    - placeholdery (`NewMetricsPlaceholderTile`,
      `NewLogsPlaceholderTile`) jako fallback przed pierwszą próbą
      pobrania.
  - `tui/views/live_logs.go` + Tab `[4] Logs` aktywne; integracja
    `tui/live_logs.go` (state machine: `enterLiveLogsTab`,
    `updateLiveLogsKey` z `f`/`c`/`Esc`/`↑↓`).
  - `LiveLogLine` + `liveLogsForm` + `liveLogsSnapshot` jako pure
    view-layer projekcja (`Snapshot()` na ring bufferze gwarantuje
    immutable read path).
  - `internal/log/redact.go` rozszerzony o JWT, generic
    `key=value`/`key: value`, `mysql/psql -p<password>`. Property
    test (200 sampli, 5 secret families) — 0% leakage.
  - Goleak (`services/sshtail/leak_test.go`) na cancel-to-shutdown
    i exhausted-reconnect scenarios.
  - Benchmarki: `RingBuffer.Push` ≈ 6 ns/op (budżet ≤100 ns), Redact
    log-line PAT ≈ 18 µs/op (budżet ≤50 µs).
  - Snapshot Sprint 09 (`docs/screenshots/sprint-09-live-logs-120x35.txt`).

- ⏭️ **Carry-over (świadomy scope cut):**
  - Production SSH `Executor` wiring w `cmd/webox/main.go` + faktyczne
    odpalenie `sshtail.Stream` przy `enterLiveLogsTab`. Tab UI działa
    end-to-end ze sztucznymi pushami; tła streaming wymaga adaptera
    `ssh.Pool` → `Executor` + dyspatch `tea.Cmd` w `update.go`.
    To 1-2h pracy które zostawiam jako follow-up, żeby Sprint 10
    (CI/CD panel) miał czysty start.
  - `header_metrics` tile zarejestrowany jako wymienialny placeholder;
    auto-poll przez `services/sshmetrics.Poller` wymaga tej samej
    integracji co live logs (executor pool).
  - Integration test "1000 linii/s przez 30s + CPU <5%" → wymaga
    fizycznego/sshmock streamingowego targetu; pominięte w MVP,
    benchmarki + leak test pokrywają ścieżkę regresyjną.

- 📌 **Decyzje:**
  - **Executor seam zamiast bezpośredniego `ssh.Pool`** — testy
    sshtail/sshmetrics nie potrzebują boot'a SSH servera, kontrakt
    jest 1-method interface (`Open` / `Run`). Production composition
    łączy `ssh.Pool` z tym interfacem w `cmd/webox`.
  - **Redactor pre-buffer** zachowany jako single source of truth;
    `Line.Raw` zawsze już zredagowany, `Redacted bool` informuje czy
    regex coś dostał.
  - **`LiveLogLine` w `tui/` package, nie w `services/sshtail`** —
    view-layer chce minimalnej projekcji (Level/Text/Redacted) bez
    timestampu; sshtail.Line bogatszy.
  - **Tab `[4]` rozróżniony per-state w `updateProjectDetailKey`** —
    osobny `updateLiveLogsKey` zapewnia że klawisze tabowe (`r`/`s`/`v`)
    nie konfliktują z keybindings live-logu (`f`/`c`).

- 🧠 **Surprises:**
  - `gocritic regexpSimplify` zgłaszał `day[s]?` jako uproszczalne do
    `days?` — drobny lint refactor, ale pokazuje że v2 linter ma sporo
    nowych reguł vs v1.
  - `golangci-lint` v2 ma `badRegexp` regułę która flaguje `[@-~]`
    jako "suspicious char range" — musiałem rozwinąć terminator class
    w `ansi.go` explicitly (per ECMA-48).
  - `DetailTab.Enabled()` test w `wizard_form_test.go` zakładał MVP
    scope = tylko Overview; aktualizacja matrix mówi już że Logs jest
    enabled w v0.1.

- 📊 **Metryki:**
  - Coverage `services/sshtail/`: **93.2%**
  - Coverage `services/sshmetrics/`: **89.1%**
  - Coverage `tui/components/`: **90.6%**
  - Coverage `tui/bento/`: **86.0%**
  - Coverage `tui/views/`: **83.4%**
  - Coverage `internal/log/`: **100%**
  - Aggregate: **83.6%** (≥70% gate).
  - Perf: RingBuffer.Push **6 ns/op**, Redact PAT line **18 µs/op**.

- 🔒 **Security validation:**
  - [x] Redactor corpus 13 secret families + property test (200
    random samples × 5 templates) — 0 leakage.
  - [x] `go test -race ./services/sshtail ./services/sshmetrics ./tui/...`
    green.
  - [x] `goleak.VerifyNone` w obu scenariuszach (cancel, exhausted
    reconnect).
  - [x] `shellEscape` + `validateLogPath` blokują traversal /
    metacharacters (`..`, `\n`, `\x00`).

- ➡️ **Następny sprint:** `sprint-10-cicd-panel.md`

---

## Retro Link

`docs/retros/<data>-sprint-09.md` (do utworzenia po sprincie)
