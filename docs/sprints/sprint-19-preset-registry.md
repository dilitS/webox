# Sprint 19 — Preset Registry Foundation

> **Daty:** TBD (po Sprint 18 + v0.2.0-rc1) → +10 dni · **Czas:** ~20-25h
>
> **Cel:** dostarczyć **Preset Registry** — kluczową warstwę pozycjonującą Webox jako narzędzie znające *konkretne hostingi*, nie tylko panele w abstrakcji. Implementuje wizję z `docs/providers/preconfiguration-vision.md §8` (preset format) + §9 (UX Provider Catalog).

---

## TL;DR

Po Sprint 18 mamy dwa adaptery (smallhost, cPanel), ale każdy hoster wymaga manualnej konfiguracji ścieżek, runtime, SSL. Preset Registry odwraca relację: użytkownik wybiera **"Krystal cPanel + CloudLinux Node.js Selector"** z listy, dostaje preset zoptymalizowany pod tego hostera, Webox uruchamia capability probe i albo potwierdza, albo prosi o ręczną korektę.

To jest **najmocniejszy marketing-differentiator** projektu po Sprint 18:

> „Webox zna Twój hosting, nie tylko Twój panel."

Scope sprint 19:

- `assets/provider-presets/*.json` — JSON schema z `preconfiguration-vision.md §8`.
- `presets/loader.go` — embed.FS-based, walidacja schema v1.
- `presets/registry.go` — runtime lookup + capability matching.
- TUI: **Provider Catalog** screen (zastępuje obecny manual provider type input).
- `webox doctor preset` — capability probe + match level (`verified` / `detected` / `partial` / `unsupported` / `dangerous`).
- 3 initial presets z fixture'ami: `smallhost-devil`, `cpanel-generic`, `cpanel-<hoster-from-sprint-16>`.
- ADR-0008: Preset Registry.

**Nie robimy w Sprint 19:**

- Plugin marketplace — automatic reject (charter rule).
- Dynamic preset loading from URL — security/blast radius issue. Embed.FS only.
- DirectAdmin / CyberPanel adapters — Sprint 20+.
- Community capture kit (`webox doctor provider-capture`) — Sprint 21.

---

## Pre-flight Checklist

- [ ] Sprint 18 zamknięty, v0.2.0-rc1 cut.
- [ ] `docs/providers/preconfiguration-vision.md` review — żadne kluczowe decyzje (status enum, schema) nie zmienione.
- [ ] cPanel test account dostępny dla regression testów Provider Catalog flow.

---

## Taski (outline)

### TASK-19.1 — JSON Schema dla preset format

- **Estymata:** S
- **AC outline:** `assets/provider-presets/schema.json` w JSON Schema Draft 2020-12. Walidator w `presets/schema_test.go`. Schema mirror `preconfiguration-vision.md §8`.

### TASK-19.2 — `presets/loader.go` + `embed.FS` integration

- **Estymata:** M
- **AC outline:** `//go:embed assets/provider-presets/*.json`. Load on startup. Strict schema validation. Failed preset → log + skip, never panic. Coverage ≥ 85%.

### TASK-19.3 — `presets/registry.go` runtime API

- **Estymata:** M
- **AC outline:** `Registry.List() []Preset`, `Registry.Get(id) (*Preset, error)`, `Registry.Match(capabilities) []PresetMatch`. Thread-safe (sync.RWMutex). Singleton init via `sync.Once`.

### TASK-19.4 — TUI: Provider Catalog screen

- **Estymata:** L
- **AC outline:** New surface `tui/surface/providercatalog/`. Grouped by region (Poland / Europe / Global / Advanced). Capability badges (SSH / API / Node / SSL / DB / Logs / Safe Restart / Fixtures). Each preset clickable → detail panel z probe preview, paths, secrets storage. Confirm → switches wizard to that preset.

### TASK-19.5 — `webox doctor preset --id <preset-id>` capability probe

- **Estymata:** M
- **AC outline:** Loads preset → runs all probes (UAPI version, SSH cmds, port reachability) → outputs match level. `--json` machine-readable. Cache result 1h.

### TASK-19.6 — Initial preset content (3 presets w v0.2)

- **Estymata:** M
- **AC outline:**
  - `smallhost-devil.json` — `status: verified`, fixtures from Sprint 03.
  - `cpanel-generic.json` — `status: candidate`, fixtures from Sprint 16-17, defaults z Application Manager + Passenger.
  - `cpanel-<hoster>.json` — `status: verified`, fixtures + hoster-specific paths/limits.

### TASK-19.7 — ADR-0008: Preset Registry

- **Estymata:** S
- **AC outline:** Decision: embedded vs filesystem vs URL-based. Trade-offs (supply chain, update cadence, contributor surface).

### TASK-19.8 — Documentation: How to add a preset (vs adapter)

- **Estymata:** S
- **AC outline:** New `docs/contributing/PRESET.md` — walkthrough „contribute a preset in 1 hour" (no Go knowledge required, just fill JSON + capture fixtures).
  - Distinct from PROVIDER.md (which is for adapters = Go code).
  - Acceptance: known target persona (web-dev using shared hosting, willing to contribute).

### TASK-19.9 — Sprint 19 retro + v0.2.0 GA decision

- **Estymata:** S
- **AC outline:** Decision: v0.2.0-rc2 z preset registry, czy v0.3.0 z nowym minor? Recommendation: rc2 jeśli registry zachowuje wstecz-kompatybilne API; nowy minor jeśli config schema się rusza.

---

## Risk Watch

| Ryzyko | Impact | Mitygacja |
|---|---|---|
| Schema v1 zaprojektowana ciasno → szybko trzeba v2 i migracja | M | Aggressive review + Context7 lookup czy podobne preset registries (Coolify, Casa OS, Yunohost) — kradnij dobre wzorce. |
| Provider Catalog UX over-engineered (`research` presets pokazują się userom) | M | Default filter: tylko `verified` + `detected`. `research` za `WEBOX_EXPERIMENTAL=1`. |
| Capability probe sieciowo wolny — Provider Catalog feels slow | M | Probe wykonywany async po wybraniu presetu, nie podczas listingu. Cache 1h. |
| Cross-preset name collisions (kilka hosterów ma cPanel + CloudLinux + Passenger) | L | Unique ID format: `<panel>-<runtime>-<hoster|generic>`. |

---

## Outcome — 2026-05-25 (out-of-cadence early delivery)

> Sprint 19 wykonany **przed** Sprintem 16/17/18, jako autonomiczna sesja (operator śpi). Decyzja: kod-heavy fundament Preset Registry można w pełni dostarczyć bez żywego konta cPanel — żywe probes są stub'owane do Sprint 17/18. Cała powierzchnia non-TUI gotowa, **TUI Provider Catalog (TASK-19.4) świadomie odroczone do Sprint 20+** żeby nie ryzykować regresji w `teatest` goldenach poza godzinami operatora.

- ✅ **Done:**
  - **ADR-0008** — embedded `go:embed` (nie URL, nie filesystem), strict JSON Schema (Draft 2020-12), singleton API z DI seam.
  - **JSON Schema v1** (`assets/provider-presets/schema.json`) — `$id: https://webox.dev/schema/provider-presets/v1.json`. `additionalProperties: false`, regex pattern dla `id` / `provider_type` / paths / probes (zakaz shell metacharacters w probach).
  - **`presets/` package** — `Preset`, `Capabilities`, `Paths`, `Verified` types z deterministycznymi helperami (`Region()`, `CapabilityBadges()`); validator + secret tripwire (GitHub tokens, OpenAI keys, AWS access keys, PEM PRIVATE KEY blocks); `embed.FS` loader z skip-on-error semantics; runtime `Registry` z `sync.RWMutex` + singleton via `sync.Once`. Coverage **88.1 %** (≥ 85 % AC). Race tests + concurrency goroutine smoke pass.
  - **6 initial presets** (zamiast planowanych 3): `smallhost-devil` (verified), `mock` (verified), `cpanel-generic` (research), `cpanel-cloudlinux-selector` (research), `directadmin-generic` (research), `cyberpanel-generic` (research).
  - **`webox doctor preset` CLI** — list (text + `--json`), show (`--id=ID`), `--probe` jako stub z explicit komunikatem o Sprint 17/18 jako live-probe milestone. 11 nowych testów w `cmd/webox/preset_test.go`.
  - **`docs/contributing/PRESET.md`** — 1-godzinny walkthrough dla nie-Go-developerów (capture host facts → fill JSON → validate → optional fixtures → PR).
  - **CHANGELOG `[Unreleased]`** — szczegółowe wpisy w sekcji Added.
- ⏭️ **Świadomie odroczone:**
  - **TASK-19.4 TUI Provider Catalog** — wymaga `tui/surface/providercatalog/` + golden refresh dla 4-5 `teatest` scenariuszy + integracja z `wizard`. Sprint 20+ kiedy operator dostępny do akceptacji wizualnej regresji.
  - **TASK-19.5 live capability probes** — czeka na cPanel adapter (Sprint 17/18) który ma już metody do wykonania `uapi --output=json Version get_version`. Stub z grzeczną wiadomością wystarczy do ekspozycji powierzchni CLI.
  - **TASK-19.9 v0.2.0 GA decision** — bez Sprintów 17/18 (cPanel adapter), v0.2.0 nie jest cuttable, więc decyzja przenosi się tam.
- 📌 **Decyzje:** ADR-0008 ratified: **TAK**. v0.2.0 final czy rc2: **decyzja przesunięta do Sprint 18 retro** (cPanel adapter musi być najpierw kompletny). Preset registry shippuje w ramach v0.2.0 single-cut.
- 🧠 **Surprises:**
  - **`assets/` package już istniał** dla landing assets — czyste rozdzielenie `embed.FS` na poziomie `assets/provider_presets.go` (a nie `presets/embed.go`) pojawiło się naturalnie i utrzymuje wszystkie `go:embed` w jednym miejscu pakietowym. Mała wygrana architekturalna.
  - **`golangci-lint` mnd alarmował na `statusRank` switch** zwracający `0..5`. Naprawione przez `const rankVerified = 0; rankCandidate = 1; ...` — efekt uboczny: czytelniejszy `Sort` w testach (`statusRank(StatusVerified) == rankVerified`).
  - **Konwencja `--key=value` jest twarda** — pierwszy smoke test próbował `--id smallhost-devil` (z spacją) i to nie działa, bo parser `webox` używa `strings.CutPrefix("--id=")`. Zgodne z istniejącymi `--debug-trace=PATH` i `--preset=ID`. Documented w `webox --help` przykładach.
- 📊 **Metryki:**
  - **Preset registry coverage:** **88.1 %** (cel 85 %). Brakujące 12 % to error branches w `loader.go` które trafiają fs-edge cases (np. plik `.json` który nie jest plikiem) — tryb defensywny zachowany, ale cost/benefit testowania symlink hell zbyt wysoki.
  - **Provider Catalog UX testing (3rd party):** N/A (TASK-19.4 odroczony).
  - **Probe latency P95:** N/A (live probes odroczone do Sprint 17/18).
  - **Liczba initial presets:** 6 (planowane 3 → +100 % przez dodanie `mock`, `cpanel-cloudlinux-selector`, `directadmin-generic`, `cyberpanel-generic` jako research-tier seedów).
  - **`make ci`:** zielony (lint 0 issues, race-tests 100 %, coverage 80.8 % global, govulncheck 0 vuln, build OK).
  - **`make bench-check`:** zielony (worst 202 290 ns/op ≤ 5 000 000 budżetu).
- ➡️ **Następny sprint:** **Sprint 16** (Public Launch, operator-time) lub **Sprint 17** (cPanel Adapter MVP part 1) — wybór operatora po przebudzeniu. TUI Provider Catalog (TASK-19.4) zostaje w `sprint-20-plus-options.md` jako wariant **D: Provider Catalog Polish** post-cPanel.

---

## Retro Link

[`docs/retros/2026-05-25-sprint-19.md`](../retros/2026-05-25-sprint-19.md)
