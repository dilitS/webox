# Sprint 23 — DirectAdmin Adapter (Path A selected)

> **Daty:** 2026-05-27 → 2026-06-09 (2 tygodnie) · **Cel:** Drugi hosting provider — DirectAdmin Live API adapter (read-only + diagnostic CLI). Powtarzamy wzorzec Sprint 21 dla DirectAdmin. Skupienie na **code-only**: 0 zależności od operatora-side credential rotation / live account procurement w pierwszych 3 taskach.
>
> **Status:** 🚧 In progress · **Predecessor:** [Sprint 22](sprint-22-cpanel-adapter-mutations.md) (cPanel adapter mutations) · **Decision:** [§Path selection](#path-selection-decision-2026-05-27).

## Kontekst

Po Sprincie 22 Webox ma:
- Pełny cPanel adapter (read + mutate ops, env-var guarded).
- Wizard × cpanel integration tests + E2E suite (real HTTPS transport).
- Workflow integration via `providers/cpanel/workflow.go`.
- **`v0.2.0-rc1` BLOCKED** — TASK-22.0 (live cPanel fixture capture) deferred bo operator opóźnił credential rotation. Nie znamy daty unblock'a.
- Preset registry z 6 entries: smallhost-devil + cpanel-generic verified (po fixturach), reszta candidate / research.

**Trzy drogi rozpatrywane na decision gate:**

## Path selection decision (2026-05-27)

| Kryterium | A. DirectAdmin | B. CyberPanel | C. Public Launch |
|---|---|---|---|
| Test account ready? | ❌ (later) | ❌ | n/a |
| **Bloker startu sprintu?** | ❌ (3 taski code-only) | ❌ (research only) | ✅ (v0.2.0 GA) |
| cPanel adapter stable? | ✅ (Sprint 22) | ✅ | ✅ |
| Operator demand signal | 🟢 2nd-biggest panel PL (VPS/dedi) | 🟡 niche, open-source | 🟡 audience-first, OK ale po GA |
| Effort confidence (1-5) | 4/5 (wzorzec z Sprint 21) | 3/5 (root + threat model unknown) | 3/5 (timing-dependent) |
| Risk to v0.2.0 GA timeline | 🟢 niski (równoległy track) | 🟡 średni (threat model RFC) | 🔴 GATED |
| Decyzja matchowa | ✅ wybór | ❌ defer to v0.4+ | ❌ defer until v0.2.0 GA |

**Decision: Path A — DirectAdmin Adapter.**

Uzasadnienie:
1. **Path C odpada bez warunków** — `v0.2.0-rc1` jest dziś blocked. Launch przed GA tag z research-derived fixtures = chłodne pierwsze wrażenie + niska wiarygodność preset registry. Launch po GA, nie wcześniej.
2. **Path B odkładamy do v0.4+** — CyberPanel wymaga roota na hoście, threat model rośnie, ADR-grade decision wymagany. Plus brak signalu demand od operatorów.
3. **Path A jest poprawnym następnym ruchem** — wzorzec dokładnie copy-paste z Sprint 21, `directadmin-generic` preset już w registry (`research`), DA Live API ma publiczny Swagger spec (`https://<host>:2222/static/swagger.json` — bundled with every install), pierwsze 3 taski NIE wymagają live accountu. Można jechać równolegle do operator-side rotation cPanela.

**Out-of-sprint:** TASK-22.0 (live cPanel fixtures) + TASK-22.6 (`v0.2.0-rc1` tag) — operator-side; gdy operator zrotuje credentials, wracam i domknę te dwa taski w izolacji.

## Cel sprintu

Po Sprincie 23:

1. **`providers/directadmin/api/`** ma read-only client dla DirectAdmin Live API (`/api/*` Swagger-defined endpoints), z mirror'em z Sprintu 21 cpanel/uapi: `Reader` interface, HTTPS transport, retry policy, typed sentinels, table-driven shape decoders, golden fixtures z research.
2. **`providers/directadmin/api/`** ma SSH fallback dla `da-cli` / `directadmin` binary (na każdej DA installce w `/usr/local/directadmin/`), z `SSHRunner` + `SSHFallback` w identycznym shape jak cpanel.
3. **`providers/directadmin/api/`** ma `Composite{Primary, Secondary}` z preferencją HTTPS, fall-over na `ErrTransportUnavailable`.
4. **`webox doctor directadmin`** CLI ([`cmd/webox/directadmin.go`](../../cmd/webox/directadmin.go)) — diagnostic command analogiczny do `doctor cpanel`: per-section status taxonomy, JSON output, exit-code policy.
5. **`directadmin-generic` preset graduate z `research` → `candidate`** w [`assets/provider-presets/directadmin-generic.json`](../../assets/provider-presets/directadmin-generic.json) — `verified` audit trail czeka na live account (jak cpanel-generic).
6. **`docs/providers/directadmin.md`** aktualizowany z mapping decyzjami: które endpoints used z Live API, które z SSH fallback, status hipotez przed implementacją.

**Out of scope (carry-over do Sprint 24+):**
- Live test account procurement + fixture replacement (operator-side; mirrors TASK-22.0 dla cpanel).
- Mutating client + `MutatingClient` interface (Sprint 24).
- Adapter implementation (`providers/directadmin/directadmin.go` — Sprint 24).
- TUI wizard integration z `directadmin-generic` preset (Sprint 24).
- `v0.2.1` tag (po live fixture replacement).

## Taski

### TASK-23.1 — DirectAdmin Live API read-only client + transport

- **Estymata:** L (1.5-2 dni) · **Zależności:** Sprint 22 zakończony, `docs/providers/directadmin.md` research baseline.
- **Acceptance Criteria:**
  - [ ] `providers/directadmin/api/transport.go` — HTTPS transport z `Authorization: Basic <user:loginkey>` headerem (loginKey, NOT password), `User-Agent: webox/<v>`, retry policy (500 ms × 2ⁿ × 3 attempts), 4 MiB body cap, 30s default timeout.
  - [ ] `providers/directadmin/api/client.go` — typed methods: `ListDomains`, `ListSubdomains`, `ListDatabases`, `ListSSLCertificates`. Każda mapuje do dokumentowanego endpointu z Swagger spec.
  - [ ] `providers/directadmin/api/errors.go` — typed sentinels: `ErrInvalidEndpoint`, `ErrMissingCredentials`, `ErrAuthenticationFailed`, `ErrRateLimited`, `ErrServerError`, `ErrMalformedResponse`, `ErrTransportUnavailable`, `ErrAPIDisabled` (some legacy installs).
  - [ ] **HTTPS-only constructor** — rejects `http://` (DA loginkey travels here, TLS mandatory).
  - [ ] **Legacy API graceful detection** — jeśli `/api/v1/...` zwraca 404, `client.go` zwraca `ErrAPIDisabled` (operator wie że musi włączyć Live API lub przejść na legacy adapter w v0.4+).
  - [ ] Test coverage ≥ 80%, fixtures w `providers/directadmin/api/testdata/` (research-derived z Swagger spec do live account exchange).

### TASK-23.2 — SSH fallback + Composite layer

- **Estymata:** M (1 dzień) · **Zależności:** TASK-23.1.
- **Acceptance Criteria:**
  - [ ] `providers/directadmin/api/ssh.go` — `SSHFallback` shells out to `da-cli` binary (path: `/usr/local/directadmin/scripts/da-cli` na każdej DA installce). Args quoted via `shellQuote` z Sprint 21.
  - [ ] `providers/directadmin/api/sshpool.go` — `SSHPoolRunner` adapter integruje z projektowym `ssh.Pool`.
  - [ ] `providers/directadmin/api/composite.go` — generic `Composite[T]` z preferencją HTTPS, fall-over na `errors.Is(err, ErrTransportUnavailable)`. Surfaces auth / rate-limit / API-disabled verbatim.
  - [ ] Closed `Reader` interface satysfakcjonowana przez wszystkie trzy: `Client`, `SSHFallback`, `Composite`.
  - [ ] Compile-time assertions: `var _ Reader = (*Client)(nil)` × 3.
  - [ ] Tests: composite fall-over scenarios, no fall-over on auth errors (RFC 5737 unreachable IP for transport-unavailable mapping).

### TASK-23.3 — `webox doctor directadmin` CLI

- **Estymata:** M (1 dzień) · **Zależności:** TASK-23.2.
- **Acceptance Criteria:**
  - [ ] `cmd/webox/directadmin.go` — pure validation + rollup logic. Funkcje: `validateDirectAdminOptions`, `runDirectAdminDoctor`, `directAdminRollup`, `formatDirectAdminText`, `formatDirectAdminJSON`.
  - [ ] `cmd/webox/directadmin_runner.go` — shells out to native `ssh` binary z `BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10` profile (same as `doctor preset --probe`).
  - [ ] CLI surface: `webox doctor directadmin --host=<host> --user=<user> [--loginkey=<key>] [--api-port=2222] [--ssh-port=22] [--timeout=30s] [--no-ssh] [--no-api] [--json]`.
  - [ ] Section status taxonomy: `OK`, `DISABLED` (legacy install / endpoint not supported), `AUTH_FAILED`, `UNREACHABLE`, `FAILED`.
  - [ ] Rollup verdict: `OK` / `BLOCKED` / `DEGRADED` — same shape as `doctor cpanel`.
  - [ ] Exit codes: 0 (OK / DEGRADED), 1 (BLOCKED), 2 (flag misuse).
  - [ ] Parser integration: new flags scoped to `doctor directadmin` context (state-aware `simpleFlagHandled`, like cpanel).
  - [ ] Help text in `cmd/webox/run.go` documents every flag z przykładem.

### TASK-23.4 — `directadmin-generic` preset graduate `research` → `candidate`

- **Estymata:** S (< 4h) · **Zależności:** TASK-23.3.
- **Acceptance Criteria:**
  - [ ] [`assets/provider-presets/directadmin-generic.json`](../../assets/provider-presets/directadmin-generic.json) — status field changes from `research` to `candidate`.
  - [ ] `verified` block left empty/null (czeka na live fixture capture; mirrors cpanel-generic post-Sprint-21).
  - [ ] `probes[]` filled with read-only DA Live API calls plus SSH-based `da-cli` invocations — every probe passes schema regex (no shell metacharacters).
  - [ ] `paths.app_root_template`, `paths.deploy_path_template`, `paths.log_path_template` populated for DA's standard `/home/<user>/domains/<domain>/` layout.
  - [ ] Loader test (`presets/loader_test.go`) confirms the preset loads without `LoadErrors`.

### TASK-23.5 — docs/providers/directadmin.md research → implementation status

- **Estymata:** S (< 2h) · **Zależności:** TASK-23.1 + TASK-23.4.
- **Acceptance Criteria:**
  - [ ] Status field changes from "RESEARCH / PLANNED POST-MVP (v0.3)" to "READ-ONLY CLIENT SHIPPED (v0.2 path)".
  - [ ] §5 mapping table updated: which endpoint resolved to Live API, which to SSH fallback, which still `[TO BE VERIFIED]`.
  - [ ] §9 plan-testów section linked to TASK-23.4 preset graduation + Sprint 24 live capture.
  - [ ] New §10 "Implementation notes" — what landed, what didn't, why.

### TASK-23.6 — Sprint review + retro

- **Estymata:** S (< 2h) · **Zależności:** All above.
- **Acceptance Criteria:**
  - [ ] Retro in `docs/retros/YYYY-MM-DD-sprint-23.md`.
  - [ ] Sprint outcome filled.
  - [ ] Sprint 24 path drafted: full DirectAdmin adapter + mutating client + wizard integration + live fixture capture (mirror of Sprint 22 dla cpanel).

## Risk watch

| Ryzyko | Mitygacja |
|---|---|
| DA Live API ma 2 inkarnacje (Legacy text + Live JSON); preset musi wybierać per-host. | TASK-23.1 implements `ErrAPIDisabled` so the client surfaces "this install needs legacy" at first call; Sprint 24+ decides whether to ship a legacy adapter or leave operators on a cpanel-style SSH-only path. |
| Operators ask for mutating ops w tym samym sprincie. | Hard reject — sprint scope is read-only client + diagnostic CLI; mutating + adapter ship in Sprint 24. Mirrors Sprint 21/22 split that worked for cpanel. |
| DA `/api/*` endpoints hidden behind feature flag na ekonomicznych hosterach. | `ErrAPIDisabled` surfaced via `doctor directadmin` z BLOCKED verdict + actionable note ("contact host to enable Live API or use SSH-only profile"). |
| Login-key leak risk during initial setup. | Token reads/writes go through keyring (`webox-api-<alias>`); doctor CLI accepts `--loginkey=...` but **NEVER** logs it. Live capture script (Sprint 24) follows the cpanel pattern: keyring-only, never plaintext. |
| Coverage drift jeśli adapter w Sprint 24 zostanie odkryty jako wymagający 2nd-pass refactoru transportu. | Sprint 23 acceptance includes `make ci` zielony z ≥ 70% project coverage; jeśli transport requires rewrite w Sprint 24, refactor lands jako separate PR przed mutating client. |

## Outcome (wypełnij po sprincie)

- 📌 Path selected: **A — DirectAdmin Adapter** (read-only foundation).
- ✅ Done: <fill as tasks close>
- ⏭️ Carry-over: live fixture capture → Sprint 24; mutating client → Sprint 24; adapter implementation → Sprint 24; wizard integration → Sprint 24; `v0.2.1` tag → Sprint 24.
- 📌 Decyzje: Path A wybrane na podstawie decision matrix (§ Path selection decision); Path B (CyberPanel) deferred to v0.4+; Path C (Public Launch) deferred until `v0.2.0` GA.
- 🧠 Surprises: <co się okazało inne niż w docs>
- 📊 Metrics: DirectAdmin read-only client coverage, doctor directadmin pass-rate na docs-fixture base.

## Outcome (wypełnij po sprincie)

- 📌 Path selected: <A / B / C + dlaczego>
- ✅ Done: <fill as tasks close>
- ⏭️ Carry-over: <task → Sprint 24 + reason>
- 📌 Decyzje: <ADR jeśli powstał>
- 🧠 Surprises: <co się okazało inne niż w docs>
- 📊 Metrics: zależne od ścieżki — adapter coverage / launch GitHub stars / partner replies.
