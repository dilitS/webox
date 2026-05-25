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

## Outcome (wypełnij po sprincie)

- ✅ Done: ...
- 📌 Decyzje: ADR-0008 ratified: TAK / NIE. v0.2.0 final czy rc2: ...
- 🧠 Surprises: ...
- 📊 Metryki:
  - Preset registry coverage: ?
  - Provider Catalog UX testing (3rd party): ?
  - Probe latency P95: ?
- ➡️ Następny sprint: `sprint-20-plus-options.md` (decyzja)

---

## Retro Link

`docs/retros/<data>-sprint-19.md`
