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
  - [ ] `ssh/errors.go` z sentinels: `ErrPoolBusy`, `ErrHostKeyMismatch`, `ErrHostKeyUnknown`, `ErrReconnectExhausted`.
  - [ ] `ssh/types.go` definiuje minimalny kontrakt połączenia (`Target`, `ExecResult`, `Clock` / `Dialer` seam).
  - [ ] Builder `ssh.ClientConfig` eksplicitnie deklaruje algorytmy z `SECURITY.md §5.5`.
  - [ ] Host-key callback zwraca rozróżnialne błędy: unknown vs mismatch.
  - [ ] Testy jednostkowe potwierdzają:
    - `ssh-rsa` nie wchodzi bez explicit compat flag,
    - unknown host key → `ErrHostKeyUnknown`,
    - mismatch → `ErrHostKeyMismatch`.
- **Pliki:**
  - `ssh/errors.go` (new)
  - `ssh/types.go` (new)
  - `ssh/client_config.go` (new)
  - `ssh/client_config_test.go` (new)
- **Docs:** [`DESIGN.md §5.1, §5.2`](../DESIGN.md#5-warstwa-ssh--sftp-connection-pooling), [`SECURITY.md §5.2–§5.5`](../SECURITY.md#5-host-keys-i-ssh)
- **Notatki:**
  - Brak auto-accept. TOFU / phrase-confirm flow zostaje po stronie przyszłej TUI.
  - Zero provider-specific logiki w `ssh/`.

---

### TASK-02.2 — `testing/sshmock` in-process SSH server

- **Estymata:** M
- **Zależności:** TASK-02.1
- **Acceptance Criteria:**
  - [ ] `testing/sshmock/` uruchamia lokalny serwer SSH na losowym porcie.
  - [ ] Obsługuje mapowanie `command → stdout/stderr/exit code`.
  - [ ] Akceptuje ephemeral key auth generowany per test.
  - [ ] Smoke test: `echo hello` → `stdout == "hello\n"`, exit code 0.
  - [ ] Support dla injected failures (disconnect / timeout / exit code != 0).
- **Pliki:**
  - `testing/sshmock/doc.go` (new)
  - `testing/sshmock/server.go` (new)
  - `testing/sshmock/server_test.go` (new)
- **Docs:** [`TESTING.md §3`](../TESTING.md#3-mockowanie-ssh), [`RISKS.md R-002`](../RISKS.md#r-002--smallpl-panel-niestabilny--api-zmiany)
- **Notatki:**
  - To infrastruktura testowa dla sprintów 02–06.
  - Fixture capture z realnego `devil` zostaje na Sprint 03.

---

### TASK-02.3 — `ssh.Pool` (`Acquire` / `Release` / idle timeout)

- **Estymata:** L
- **Zależności:** TASK-02.1, TASK-02.2
- **Acceptance Criteria:**
  - [ ] `ssh/pool.go` implementuje pool z limitem `max=3` per host.
  - [ ] `Acquire(ctx, target)` respektuje timeout i zwraca `ErrPoolBusy`.
  - [ ] `Release(target, client)` zwraca klienta do puli; double-release nie korumpuje stanu.
  - [ ] Idle timeout 60 s zamyka bezczynne połączenia.
  - [ ] Tabela testów:
    - happy path reuse,
    - limit 3 + 4th waiter timeout,
    - idle cleanup,
    - cancelled context,
    - concurrent `Acquire/Release` pod `-race`.
- **Pliki:**
  - `ssh/pool.go` (new)
  - `ssh/pool_test.go` (new)
- **Docs:** [`DESIGN.md §5`](../DESIGN.md#5-warstwa-ssh--sftp-connection-pooling), [`RISKS.md R-005`](../RISKS.md#r-005--bubble-tea-mvu-rozje%C5%BCd%C5%BCa-si%C4%99-w-prawdziwym-%C5%BCyciu)
- **Notatki:**
  - Najpierw testy black-box, potem ewentualne white-box branch tests dla cleanup loop.

---

### TASK-02.4 — `ssh` keepalive + exec path + reconnect classification

- **Estymata:** M
- **Zależności:** TASK-02.3
- **Acceptance Criteria:**
  - [ ] Keepalive ticker co 15 s (`keepalive@openssh.com`) na aktywnych klientach.
  - [ ] `Exec(ctx, target, command)` zwraca `ExecResult{Stdout, Stderr, ExitCode}`.
  - [ ] Zerwane połączenie klasyfikowane pod retry policy z `DESIGN §9` (3 próby, `3s/6s/12s` + jitter seam).
  - [ ] Komenda nie jest ślepo ponawiana; API zwraca enough context, by provider mógł wykonać state check.
  - [ ] Testy:
    - keepalive loop stops on close,
    - one reconnect path succeeds,
    - retries exhausted → `ErrReconnectExhausted`.
- **Pliki:**
  - `ssh/exec.go` (new)
  - `ssh/keepalive.go` (new)
  - `ssh/exec_test.go` (new)
- **Docs:** [`DESIGN.md §5`, `§9`](../DESIGN.md#5-warstwa-ssh--sftp-connection-pooling), [`TESTING.md §3`](../TESTING.md#3-mockowanie-ssh)
- **Notatki:**
  - Retry logic musi mieć injectable clock / sleeper. Żadnego `time.Sleep` hardcoded w testach.

---

### TASK-02.5 — `status.Cache` core SWR + `singleflight`

- **Estymata:** L
- **Zależności:** —
- **Acceptance Criteria:**
  - [ ] `status/cache.go` z `GetOrFetch[T]` jako funkcją pakietową (nie method).
  - [ ] Semantyka:
    - cache hit fresh → natychmiast,
    - cache stale → zwrot stale + refresh w tle,
    - cache miss → blokujący fetch.
  - [ ] `singleflight` zapewnia 1 inflight fetch per key.
  - [ ] Czas (`now`) injectable.
  - [ ] Testy:
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

---

### TASK-02.6 — `status` invalidation + stale age metadata

- **Estymata:** M
- **Zależności:** TASK-02.5
- **Acceptance Criteria:**
  - [ ] `Invalidate(prefix string)` czyści wszystkie matching keys.
  - [ ] Cache entries niosą `buffered age` / `isStale` metadata pod UI.
  - [ ] TTL table z `ADR-0005` odwzorowana w kodzie helperami / stałymi.
  - [ ] Testy invalidacji eventowej (`Restart`, `Deploy`, `SetupSSL` prefixy).
- **Pliki:**
  - `status/invalidate.go` (new)
  - `status/invalidate_test.go` (new)
- **Docs:** [`ADR-0005 §Parametry cache`](../adr/0005-cache-statusow-projektow.md#parametry-cache), [`DESIGN.md §8.2, §8.3`](../DESIGN.md#8-tr%C3%B3jpoziomowy-status-cache-stale-while-revalidate)
- **Notatki:**
  - Nie implementujemy jeszcze UI badge'a; tylko dane i kontrakt.

---

### TASK-02.7 — `services` probes: HTTP status + TLS cert info

- **Estymata:** M
- **Zależności:** TASK-02.5
- **Acceptance Criteria:**
  - [ ] `services/httpcheck/` (lub `services/probe/`) z probe HTTP 200/3xx/5xx + latency.
  - [ ] TLS probe zwraca `not_after` + `days_left`.
  - [ ] Timeouty injectable, default 1 s dla HTTP / TLS handshake.
  - [ ] Testy przez `httptest.NewServer` i lokalny TLS server.
- **Pliki:**
  - `services/httpcheck/doc.go` (new)
  - `services/httpcheck/http.go` (new)
  - `services/httpcheck/tls.go` (new)
  - `services/httpcheck/http_test.go` (new)
  - `services/httpcheck/tls_test.go` (new)
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

## Outcome (wypełnij po sprincie)

- ✅ Done: TASK-02.X, ...
- ⏭️ Carry-over: TASK-02.X → Sprint 03
- 📌 Decyzje: <ADR jeśli powstał>
- 🧠 Surprises: ...
- 📊 Metryki:
  - Coverage `ssh/`: %
  - Coverage `status/`: %
  - Coverage `services/httpcheck/`: %
  - Czas faktyczny vs estymata: ratio
- 🔒 Security validation:
  - [ ] `go test -race ./ssh ./status ./services/...` green
  - [ ] Host-key mismatch nadal strict-block (brak auto-accept)
  - [ ] No secrets in any SSH / cache logs
- ➡️ Następny sprint: `sprint-03-provider-smallhost.md`

---

## Retro link (po sprincie)

`docs/retros/YYYY-MM-DD-sprint-02.md`
