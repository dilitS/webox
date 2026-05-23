# Sprint 02 — SSH + Status Cache

> **Daty:** TBD → TBD (planowane 2 tygodnie solo) · **Czas:** ~40-50h skupienia
>
> **Cel:** dostarczamy dwie rzeczy, bez których dashboard i provider adapter nie mają sensu: bezpieczną warstwę SSH z connection poolem oraz pamięć statusów w modelu stale-while-revalidate.

---

## TL;DR

Po sprincie 02:

- `ssh/` ma działający **connection pool** (`Acquire` / `Release`, idle timeout, keepalive, strict host-key callback, sentinel errors).
- `testing/sshmock/` daje deterministyczny in-process serwer SSH do testów integracyjnych bez dotykania realnego hostingu.
- `status/` ma **SWR cache** z TTL, `singleflight`, invalidacją eventową i testami `-race`.
- `services/` ma minimalne **HTTP + TLS probes** potrzebne do dashboardu statusów.
- Powstają dane i kontrakty pod Sprint 03 (`smallhost` adapter) bez jeszcze implementowania samego providera.

**Nie robimy w tym sprincie:**

- żadnego `smallhost` adaptera produkcyjnego,
- żadnej TUI poza kontraktami potrzebnymi pod dashboard,
- żadnych operator commands poza już istniejącym `webox doctor`.

---

## Pre-flight checklist

- [ ] Sprint 01 zamknięty z retro i `Outcome`.
- [ ] Read `docs/DESIGN.md §5`, `§8`, `§9` end-to-end.
- [ ] Read `docs/SECURITY.md §5.2–§5.5` (host keys, algorithms).
- [ ] Read `docs/adr/0005-cache-statusow-projektow.md`.

---

## Taski

### TASK-02.1 — `ssh` sentinels + config seam + strict host-key contract

- **Estymata:** M
- **Zależności:** Sprint 01 done
- **Acceptance Criteria:**
  - [x] `ssh/errors.go` z sentinels: `ErrPoolBusy`, `ErrHostKeyMismatch`, `ErrHostKeyUnknown`, `ErrReconnectExhausted` (+ dorzucone `ErrHostKeyDBRequired`, bo bez niego builder by milcząco akceptował `InsecureIgnoreHostKey`).
  - [x] `ssh/types.go` definiuje minimalny kontrakt połączenia (`Target`, `ExecResult`, `Clock` z `SystemClock`, `Dialer`, `HostKeyDB`). `Target.Addr()` używa `net.JoinHostPort` dla IPv6, `Target.Key()` bucketuje pool per `(user, host, port)`.
  - [x] Builder `ssh.ClientConfig` eksplicitnie deklaruje algorytmy z `SECURITY.md §5.5` (ed25519 → rsa-sha2-512 → rsa-sha2-256 → ecdsa-sha2-nistp256, ssh-rsa tylko z `LegacyAlgorithmCompat=true`, ssh-dss nigdy).
  - [x] Host-key callback zwraca rozróżnialne błędy: unknown vs mismatch. Nie-`knownhosts.KeyError` propagują się bez re-mappingu (`errors.Is` przepuszcza je).
  - [x] Testy jednostkowe potwierdzają:
    - `ssh-rsa` nie wchodzi bez explicit compat flag (`TestBuildClientConfig_HostKeyAlgorithms`).
    - `LegacyAlgorithmCompat=true` dorzuca `ssh-rsa` (`TestBuildClientConfig_LegacyCompatAddsSSHRSA`).
    - unknown host key → `ErrHostKeyUnknown` (`TestBuildClientConfig_HostKeyCallback_UnknownVsMismatch/unknown_host_returns_ErrHostKeyUnknown`).
    - mismatch → `ErrHostKeyMismatch` (`TestBuildClientConfig_HostKeyCallback_UnknownVsMismatch/mismatched_host_returns_ErrHostKeyMismatch`).
    - dopasowany klucz → `nil` (`TestBuildClientConfig_HostKeyCallback_NilOnMatch`).
    - nil `HostKeyDB` → `ErrHostKeyDBRequired` (`TestBuildClientConfig_RequiresHostKeyDB`).
    - non-KeyError błędy DB nie są re-mapowane (`TestBuildClientConfig_HostKeyCallback_PreservesNonKeyError`).
- **Pliki:**
  - `ssh/errors.go` + `ssh/errors_test.go`
  - `ssh/types.go` + `ssh/types_test.go`
  - `ssh/client_config.go` + `ssh/client_config_test.go`
- **Docs:** [`DESIGN.md §5.1, §5.2`](../DESIGN.md#5-warstwa-ssh--sftp-connection-pooling), [`SECURITY.md §5.2–§5.5`](../SECURITY.md#5-host-keys-i-ssh)
- **Notatki:**
  - Brak auto-accept. TOFU / phrase-confirm flow zostaje po stronie przyszłej TUI.
  - Zero provider-specific logiki w `ssh/`.
  - Coverage `ssh/` = **100%**.
  - `staticcheck` SA1019 dla `cryptossh.KeyAlgoDSA` obejdzony przez stałą `dssWireName = "ssh-dss"` — chcemy w teście jawnie zablokować ten algorytm na poziomie wire format, niezależnie od tego, że upstream go już usunął.

---

### TASK-02.2 — `testing/sshmock` in-process SSH server

- **Estymata:** M
- **Zależności:** TASK-02.1
- **Acceptance Criteria:**
  - [x] `testing/sshmock/` uruchamia lokalny serwer SSH na losowym porcie.
  - [x] Obsługuje mapowanie `command → stdout/stderr/exit code`.
  - [x] Akceptuje ephemeral key auth generowany per test.
  - [x] Smoke test: `echo hello` → `stdout == "hello\n"`, exit code 0.
  - [x] Support dla injected failures (disconnect / timeout / exit code != 0).
- **Pliki:**
  - `testing/sshmock/doc.go`
  - `testing/sshmock/server.go`
  - `testing/sshmock/server_test.go`
- **Docs:** [`TESTING.md §3`](../TESTING.md#3-mockowanie-ssh), [`RISKS.md R-002`](../RISKS.md#r-002--smallpl-panel-niestabilny--api-zmiany)
- **Notatki:**
  - To infrastruktura testowa dla sprintów 02–06.
  - Fixture capture z realnego `devil` zostaje na Sprint 03.
  - Implementacja używa `golang.org/x/crypto/ssh`, bez dodatkowej zależności
    typu `gliderlabs/ssh`.

---

### TASK-02.3 — `ssh.Pool` (`Acquire` / `Release` / idle timeout)

- **Estymata:** L
- **Zależności:** TASK-02.1, TASK-02.2
- **Acceptance Criteria:**
  - [x] `ssh/pool.go` implementuje pool z limitem `max=3` per host (konfigurowalne przez `PoolOptions.MaxPerHost`).
  - [x] `Acquire(ctx, target)` respektuje timeout i zwraca `ErrPoolBusy`.
  - [x] `Release(target, client)` zwraca klienta do puli; double-release nie korumpuje stanu.
  - [x] Idle timeout 60 s zamyka bezczynne połączenia (lazy reap + background cleanup loop).
  - [x] Tabela testów:
    - happy path reuse,
    - limit 3 + 4th waiter timeout,
    - idle cleanup,
    - cancelled context,
    - concurrent `Acquire/Release` pod `-race`.
- **Pliki:**
  - `ssh/dialer.go`
  - `ssh/pool.go`
  - `ssh/pool_test.go`
- **Docs:** [`DESIGN.md §5`](../DESIGN.md#5-warstwa-ssh--sftp-connection-pooling), [`RISKS.md R-005`](../RISKS.md#r-005--bubble-tea-mvu-rozje%C5%BCd%C5%BCa-si%C4%99-w-prawdziwym-%C5%BCyciu)
- **Notatki:**
  - Najpierw testy black-box, potem ewentualne white-box branch tests dla cleanup loop.
  - Testy używają realnego transportu SSH przez `testing/sshmock`, więc reuse / limit /
    idle cleanup ćwiczą prawdziwe `*ssh.Client`.

---

### TASK-02.4 — `ssh` keepalive + exec path + reconnect classification

- **Estymata:** M
- **Zależności:** TASK-02.3
- **Acceptance Criteria:**
  - [x] Keepalive ticker co 15 s (`keepalive@openssh.com`) na aktywnych klientach.
  - [x] `Exec(ctx, target, command)` zwraca `ExecResult{Stdout, Stderr, ExitCode, Duration}`.
  - [x] Zerwane połączenie klasyfikowane pod retry policy z `DESIGN §9` (3 próby, `3s/6s/12s` + injectable sleeper seam; jitter zostaje warstwą policy ponad deterministycznym backoffem).
  - [x] Komenda nie jest ślepo ponawiana; `Exec` kończy się wynikiem/błędem, a `Reconnect` tylko przywraca klienta — provider musi wykonać state check przed replay.
  - [x] Testy:
    - keepalive loop stops on close,
    - one reconnect path succeeds,
    - retries exhausted → `ErrReconnectExhausted`.
- **Pliki:**
  - `ssh/exec.go`
  - `ssh/keepalive.go`
  - `ssh/exec_test.go`
- **Docs:** [`DESIGN.md §5`, `§9`](../DESIGN.md#5-warstwa-ssh--sftp-connection-pooling), [`TESTING.md §3`](../TESTING.md#3-mockowanie-ssh)
- **Notatki:**
  - Retry logic musi mieć injectable clock / sleeper. Żadnego `time.Sleep` hardcoded w testach.

---

### TASK-02.5 — `status.Cache` core SWR + `singleflight`

- **Estymata:** L
- **Zależności:** —
- **Acceptance Criteria:**
  - [x] `status/cache.go` z `GetOrFetch[T]` jako funkcją pakietową (nie method).
  - [x] Semantyka:
    - cache hit fresh → natychmiast,
    - cache stale → zwrot stale + refresh w tle,
    - cache miss → blokujący fetch.
  - [x] `singleflight` zapewnia 1 inflight fetch per key.
  - [x] Czas (`now`) injectable.
  - [x] Testy:
    - hit / stale / miss,
    - singleflight on same key,
    - cancellation,
    - `go test -race`.
- **Pliki:**
  - `status/doc.go` (edit jeśli trzeba doprecyzować SWR contract)
  - `status/cache.go` (new)
  - `status/cache_test.go` (new)
- **Docs:** [`DESIGN.md §8`](../DESIGN.md#8-tr%C3%B3jpoziomowy-status-cache-stale-while-revalidate), [`ADR-0005`](../adr/0005-cache-statusow-projektow.md)
- **Notatki:**
  - TDD twarde. To core logic pod dashboard i drift detection.
  - Dodano `golang.org/x/sync v0.20.0`. Sprint 02 podbił main module do
    `go 1.25.0`, bo nowe wywołania `golang.org/x/crypto/ssh` uruchomiły
    realne `govulncheck` findings, a pełny fix wymaga `x/crypto v0.52.0`
    (Go 1.25).
  - Coverage `status/` po TASK-02.5 = **87.8%**.

---

### TASK-02.6 — `status` invalidation + stale age metadata

- **Estymata:** M
- **Zależności:** TASK-02.5
- **Acceptance Criteria:**
  - [x] `Invalidate(prefix string)` czyści wszystkie matching keys.
  - [x] Cache entries niosą `buffered age` / `isStale` metadata pod UI (`GetOrFetchMeta[T]`).
  - [x] TTL table z `ADR-0005` odwzorowana w kodzie helperami / stałymi (`HTTPStatusTTL`, `SSHNodeTTL`, `SSLCertTTL`, `GitHubLastDeployTTL` + prefixy).
  - [x] Testy invalidacji eventowej (`Restart`, `Deploy`, `SetupSSL` prefixy).
- **Pliki:**
  - `status/cache.go`
  - `status/ttl.go`
  - `status/invalidate_test.go`
- **Docs:** [`ADR-0005 §Parametry cache`](../adr/0005-cache-statusow-projektow.md#parametry-cache), [`DESIGN.md §8.2, §8.3`](../DESIGN.md#8-tr%C3%B3jpoziomowy-status-cache-stale-while-revalidate)
- **Notatki:**
  - Nie implementujemy jeszcze UI badge'a; tylko dane i kontrakt.

---

### TASK-02.7 — `services` probes: HTTP status + TLS cert info

- **Estymata:** M
- **Zależności:** TASK-02.5
- **Acceptance Criteria:**
  - [x] `services/httpcheck/` z probe HTTP 200/3xx/5xx + latency.
  - [x] TLS probe zwraca `not_after` + `days_left`.
  - [x] Timeouty injectable, default 1 s dla HTTP / TLS handshake.
  - [x] Testy przez `httptest.NewServer` i lokalny TLS server.
- **Pliki:**
  - `services/httpcheck/doc.go`
  - `services/httpcheck/errors.go`
  - `services/httpcheck/http.go`
  - `services/httpcheck/tls.go`
  - `services/httpcheck/http_test.go`
  - `services/httpcheck/tls_test.go`
- **Docs:** [`PRD.md F5`](../PRD.md#6-ficzery--z-priorytetami), [`DESIGN.md §8.2`](../DESIGN.md#8-tr%C3%B3jpoziomowy-status-cache-stale-while-revalidate)
- **Notatki:**
  - Brak GitHub deploy-status probe w tym sprincie — to zostaje przy Sprint 06.

---

## Risk watch

| Ryzyko | Impact | Mitygacja |
|--------|--------|-----------|
| **`small.pl` CLI / SSH zachowuje się inaczej niż zakłada mock** | M | `testing/sshmock` + późniejszy live fixture capture w Sprint 03; patrz `RISKS.md R-002`. |
| **Pool / keepalive generuje race albo leak goroutines** | H | wszędzie `context.Context`, `go test -race`, idle cleanup testy, retry clock injectable; patrz `RISKS.md R-005`. |
| **SWR semantics okażą się zbyt złożone jak na MVP** | M | trzymać się dokładnie `ADR-0005`, bez persystencji na dysk i bez webhook fantasies. |
| **SSH layer scope creep** | M | zero ProxyJump / bastion / SFTP upload pathów w tym sprincie; tylko pool + exec + keepalive. |

---

## Outcome

- ✅ Done: TASK-02.1, TASK-02.2, TASK-02.3, TASK-02.4, TASK-02.5, TASK-02.6, TASK-02.7.
- ⏭️ Carry-over: none. Sprint 03 starts from provider contracts and smallhost parser fixtures.
- 📌 Decyzje:
  - `testing/sshmock` uses `golang.org/x/crypto/ssh` directly instead of adding `gliderlabs/ssh`, keeping dependencies smaller and exercising the same transport stack as production code.
  - Main module floor raised to `go 1.25.0` because new reachable `x/crypto/ssh` call paths made `govulncheck` fail on `x/crypto v0.41.0`; the full fix requires `x/crypto v0.52.0`, which declares Go 1.25.
  - `Exec` deliberately does not retry commands. `Reconnect` only restores connectivity; providers must run idempotent state probes before replay.
- 🧠 Surprises:
  - `govulncheck` stayed quiet before Sprint 02 because the repo imported `x/crypto/ssh` but did not call the vulnerable symbols. Adding `sshmock`, `NetDialer`, `Exec`, and keepalive made the vulnerabilities reachable.
  - A race appeared in `Pool.ReapIdle` from reading `len(p.hosts)` before acquiring `p.mu`. The fix moved preallocation under the lock and is covered by `go test -race ./ssh`.
  - Avoiding `gliderlabs/ssh` was straightforward; `x/crypto/ssh.NewServerConn` was enough for deterministic session/exec tests.
- 📊 Metryki:
  - Coverage `ssh/`: 82.7%
  - Coverage `status/`: 83.2%
  - Coverage `services/httpcheck/`: 88.9%
  - Coverage `testing/sshmock/`: 79.2%
  - Global coverage: 85.6%
- 🔒 Security validation:
  - [x] `go test -race ./ssh ./status ./services/...` green.
  - [x] `make ci` green: lint, vet, govulncheck, race tests, coverage gate, build.
  - [x] `govulncheck` reports `No vulnerabilities found`.
  - [x] Host-key mismatch remains strict-block (`ErrHostKeyMismatch`, no auto-accept).
  - [x] No secrets introduced in SSH / cache logs.
- ➡️ Następny sprint: [`sprint-03-provider-smallhost.md`](sprint-03-provider-smallhost.md)

---

## Retro link (po sprincie)

[`docs/retros/2026-05-23-sprint-02.md`](../retros/2026-05-23-sprint-02.md)
