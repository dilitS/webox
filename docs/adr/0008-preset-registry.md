# ADR-0008: Preset Registry — embedded, schema-validated, no dynamic loading

> Status: Accepted · Data: 2026-05-25 · Właściciel: @maintainer · Reviewers: @maintainer
>
> Pokrewne ADR: [ADR-0003 Provider Pattern](./0003-provider-pattern.md), [ADR-0004 Sekrety w keyringu](./0004-przechowywanie-sekretow-keyring.md), [ADR-0007 Bento Ultra eskalacja](./0007-bento-ultra-eskalacja-mvp.md). Dokumenty: [providers/preconfiguration-vision.md](../providers/preconfiguration-vision.md), [ROADMAP §4](../ROADMAP.md#4-v02--drugi-provider--doko%C5%84czenie-palette), [SECURITY.md](../SECURITY.md).

## Kontekst

[`docs/providers/preconfiguration-vision.md`](../providers/preconfiguration-vision.md) wprowadza pojęcie **presetu hostera** — niezbędnej warstwy konfiguracyjnej między panelem (`cPanel`, `DirectAdmin`) a *konkretnym hostingiem* operatora. Preset zna runtime Node.js, ścieżki, restart, SSL, znane ryzyka i probe commands dla danego wariantu infrastruktury.

Sprint 19 dostarcza **fundament** preset registry. Trzy decyzje techniczne wymagają jawnego ADR przed implementacją, bo każda z nich ma długoterminowe konsekwencje dla supply chain, wektora ataku i ścieżki community contribution:

1. **Skąd preset registry pobiera dane?** (embedded vs filesystem vs URL).
2. **Jak waliduje strukturę presetu?** (luźny `json.Unmarshal` vs strict JSON Schema).
3. **Jak ekspozycja runtime API wygląda dla TUI i CLI?** (instancja DI vs singleton globalny).

Możliwe odpowiedzi na pytanie 1 i ich trade-offy:

| Wariant | Plus | Minus |
|---|---|---|
| **A. `//go:embed assets/provider-presets/*.json`** | Deterministic build; preset registry = artefakt binarki; brak supply-chain wektora; `make ci` pokrywa walidację każdego presetu; każda wersja Webox ma znany manifest presetów; `webox doctor preset list` jest offline. | Każdy nowy preset wymaga rebuilda binarki. Operator-side hot-reload niemożliwy. |
| **B. Filesystem (`~/.config/webox/presets/`)** | Operator dodaje preset bez czekania na release. Community capture kit pisze tu lokalnie. | Schema drift między binarką a folderem jest możliwy; konieczna runtime walidacja przy każdym starcie; otwiera to wektor: złośliwy preset `.json` z `probes` zawierającymi shell injection. |
| **C. URL-based (registry mirror, np. `https://webox.dev/registry/v1/`)** | Marketingowo silny katalog; presety aktualizowane bez release. | Wymaga TLS pinning + signature verification + offline cache + GDPR/telemetry implications + stałego maintenance backendu — sprzeczne z `Brak telemetrii / phone-home` z `.cursor/rules/00-charter.mdc`. |
| **D. Plugin marketplace (dynamiczne `.so`)** | Maksymalna elastyczność dla community. | Automatic reject po `AGENTS.md §4` — supply chain risk, wymaga RFC + maintainer sign-off na v1.0+. |

Możliwe odpowiedzi na pytanie 2:

| Wariant | Plus | Minus |
|---|---|---|
| **a. Luźny `json.Unmarshal`** | Brak nowej zależności; szybki start. | Drift schema → cichy bug w runtime; brak gwarancji że `status` jest jednym z enum values; trudna walidacja w community capture kit; brak ścieżki dla "preset linter" jako developer tool. |
| **b. JSON Schema 2020-12 (santhosh-tekuri/jsonschema)** | Już używamy w `config/`, recyclujemy infrastrukturę; explicit enum, regex, format constraints; jeden source of truth dla Go types + dokumentacji + community capture kit + `webox doctor preset --validate`. | Schema musi być utrzymywana razem z Go types. Wymaga policzkowania wymyślania dwóch reprezentacji tej samej struktury. |

Możliwe odpowiedzi na pytanie 3:

| Wariant | Plus | Minus |
|---|---|---|
| **i. DI: każdy konsument dostaje `*Registry` w konstruktorze** | Testowalne; brak globalnego stanu; pasuje do TUI surface DI z [ADR-0007](./0007-bento-ultra-eskalacja-mvp.md). | Wszystkie call siteʼy muszą propagować zależność. Boilerplate w `cmd/webox/`. |
| **ii. Singleton + DI seam** | `presets.Default()` daje zerową ceremonię w call site; DI seam umożliwia testowanie. Hybryda. | Żadnych — pod warunkiem że singleton jest **lazy-initialized via `sync.Once`** i **read-only** po inicjalizacji. |

## Decyzja

Sprint 19 implementuje preset registry w wariancie **A + b + ii**:

1. **Źródło: `//go:embed assets/provider-presets/*.json`.** Wbudowane w binarkę, deterministic, zero supply-chain wektora. Każdy nowy preset to PR z fixturą + commit + rebuild. Operator nie ma sposobu wstawić własnego presetu w runtime — to jest **świadoma decyzja bezpieczeństwa**, nie ograniczenie. Filesystem-loadable presets wracają jako oddzielny RFC po v1.0 GA jeśli pojawi się realny pull.
2. **Walidacja: JSON Schema Draft 2020-12 z `github.com/santhosh-tekuri/jsonschema/v6`** — recycling infrastruktury z `config/schema.go`. Schema wbudowana jako `assets/provider-presets/schema.json`. Walidator enforce'uje: required fields, enum values (`status`, `node_runtime`, `restart_method`, `ssl_provider`), ID regex, secret tripwire identyczny z `config.walkStrings` (zero tokens / passwords / private keys / openai-style secrets w danych presetu).
3. **API: singleton + DI seam.** `presets.Default() *Registry` (lazy via `sync.Once`) dla call site convenience; `presets.NewRegistry(loader Loader) *Registry` dla testów. Registry jest read-only po `Load()` — operacje czytające chronione `sync.RWMutex`, ale w praktyce po init nie ma writeʼów.

## Konsekwencje

### Co zyskujemy

- **Zero supply chain risk.** Webox v0.2.0 z preset registry to ten sam threat model co v0.1.0: tylko binarka + `config.json` + keyring/secrets.enc.
- **Build-time gwarancje.** Każdy preset w `assets/provider-presets/*.json` przechodzi przez schema validator w `make ci`. Niemożliwe wytaguje wersji ze sprzecznym presetem.
- **Deterministic `make bench-check`.** Liczba presetów znana w build-time; `presets.Default().List()` jest O(1) lookup do mapy.
- **Bezpieczna ścieżka community contribution.** Contribution flow: `git clone` → edytuj `assets/provider-presets/<your-host>.json` → uruchom `webox doctor preset --validate <path>` lokalnie → PR. Brak runtime registration; brak shell injection w probe commands.
- **Recycling toolchain.** Już dostarczona infrastruktura (`santhosh-tekuri/jsonschema/v6`, `walkStrings`, secret tripwire) — zero nowych dependencies.

### Co tracimy

- **Brak hot-reload presetu.** Operator dodający własny preset dla niszowego hostera musi zrobić PR + czekać na release. Mitigacja: `webox provider new <name>` daje już szybki adapter scaffold; preset PR to ~50 linii JSON + opcjonalny screenshot probe outputu — szacowany cykl < 7 dni.
- **Brak runtime-loadable community packs.** Jeśli ktoś chce "preset bundle dla Hetzner/OVH self-hosted", musi to zrobić jako fork. Mitigacja: dokumentacja w `docs/contributing/PRESET.md` jest jasnym kanałem; registry growth to mierzalna metryka (`# presets in mainline ≥ 10` to v0.3 cel).
- **Schema musi być utrzymywana razem z Go types.** Mitigacja: round-trip test (Go struct → JSON → Schema validate → Go struct) w `presets/schema_test.go` łapie drift natychmiast.

### Operacyjne

- **Lokalizacja artefaktów:**
  - `presets/` — pakiet Go (types, loader, registry, validator).
  - `assets/provider-presets/schema.json` — JSON Schema 2020-12.
  - `assets/provider-presets/<id>.json` — kanoniczne presety (smallhost-devil, cpanel-generic, directadmin-generic, cyberpanel-generic w v0.2 baseline).
- **CLI surface:** `webox doctor preset` (list), `webox doctor preset --id <id>` (show, optional `--json`). `--probe` pozostaje stub-em (`"requires real account, see Sprint 20+"`) bo execution probe wymaga adapter integracji — ten task wraca w cPanel adapter scope (Sprint 17/18).
- **TUI surface:** `tui/surface/providercatalog/` (read-only browsing, capability badges, region grouping). Wizard integration zostaje jako Sprint 20+ — w v0.2 baseline preset registry to **discovery layer**, nie wymuszający flow.
- **Backward compatibility:** Sprint 19 nie zmienia `config.json` schema. Aktualne profile dalej działają. `provider_type` w `config.json` jest dalej źródłem prawdy dla adaptera; preset to opcjonalna warstwa pozycjonowania UX.

### Wektory ataku rozważone

| Wektor | Mitigacja |
|---|---|
| Złośliwy preset `.json` w `assets/provider-presets/` (PR-attack). | Schema validator + secret tripwire + manual review (preset PR wymaga sanityzacji fixture'ów); schema enforce'uje `pattern` na `id`, `panel.name`, `paths.*_template`. |
| Shell injection przez `probes` (`uapi --output=json Version get_version`). | Probes są **dokumentacją**, nie wykonywanymi w v0.2 baseline. Future probe execution (Sprint 17/18+) używa allowlist + `exec.Command` z osobnymi argumentami, nie `sh -c`. |
| Path traversal przez `paths.deploy_path_template`. | Templates używają `{{user}}` / `{{app_root}}` placeholderów; wykonanie templatów (Sprint 17/18+) walidowanymi przez `wizard.ValidateWorkflowField` (już istniejący guard). |
| Secret leak (token / password / key) w preset JSON. | `presets/validator.go` wywołuje identyczny `walkStrings` jak `config/schema.go` — odrzuca `ghp_…`, `ghs_…`, `github_pat_…`, `sk-…`, `BEGIN … PRIVATE KEY` na poziomie loadera. |

## Alternatywy odrzucone

### B (filesystem)

Odrzucone z dwóch powodów: (a) złośliwy preset z shell-injection w `probes` byłby wektorem ataku gdy probe execution ląduje w Sprint 17/18; (b) schema drift między binarką (Webox v0.2.0) a folderem usera wymagałby runtime warning + degradacji UX, czego nie chcemy w pierwszym dostarczeniu.

### C (URL-based)

Sprzeczne z `Brak telemetrii / phone-home` z [`charter`](../../.cursor/rules/00-charter.mdc). Wymagałoby TLS pinning + signature verification + offline cache + maintenance backendu. Wymaga RFC + maintainer sign-off + dyskusji z community po v1.0.

### D (plugin marketplace)

Automatic reject po [`AGENTS.md §4`](../../AGENTS.md). Plugin marketplace jako mechanizm wraca jako odrębny RFC po v1.0+ jeśli pojawi się realny popyt.

### a (luźny `json.Unmarshal`)

Drift schema → ciche bugi w runtime. Brak ścieżki dla `webox doctor preset --validate <path>` jako developer tool. Recycling istniejącej infrastruktury z `config/schema.go` pokrywa zysk wariantu **a** bez minusów.

### i (DI bez singletonu)

Boilerplate w `cmd/webox/run.go` rośnie liniowo z każdym subsystemem konsumującym preset registry (TUI, CLI, doctor, future adaptery). Singleton + DI seam to standardowy wzorzec w Webox (`telemetry.Disabled`, `services/github.NewClient`).

## Reviewers

- @maintainer — accept (2026-05-25)

## References

- [docs/providers/preconfiguration-vision.md §8 — Format preset registry](../providers/preconfiguration-vision.md)
- [docs/providers/preconfiguration-vision.md §9 — UX wybor providera jako powod adopcji](../providers/preconfiguration-vision.md)
- [config/schema.go](../../config/schema.go) — wzorzec schema validator (recycled by `presets/`).
- [.cursor/rules/00-charter.mdc](../../.cursor/rules/00-charter.mdc) — guardrails (no telemetry, no plugin marketplace).
- [AGENTS.md §4 — Scope discipline](../../AGENTS.md#4-scope-discipline)
