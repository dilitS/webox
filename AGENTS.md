# AGENTS.md — Webox

> Operator handbook dla AI coding agents pracujących nad Webox.
>
> Status: Stable for v0.1+ · Ostatnia aktualizacja: 2026-05-25 · Właściciel: @maintainer
>
> Ten plik jest **kontraktem** między człowiekiem-maintainerem a agentem. Agent czyta go **przed** każdym task'em. Jeśli zalecenie z `AGENTS.md` koliduje z user request, **agent zatrzymuje się i pyta**.

---

## TL;DR

Webox to **monolit w Go 1.25+** z TUI opartym o Bubble Tea/Lipgloss. **MVP scope = small.pl/Devil only**, v0.2 dorzuca cPanel. Implementacja jest **docs-first** — kod sprzeczny z `docs/` zostaje odrzucony w review. **TDD jest obowiązkowe** dla logiki krytycznej (parsery, walidatory, redactor, status cache, state machine TUI, keyring detekcja, config migracje). Sekrety **nigdy** nie wchodzą do logów ani plików tekstowych poza `keyring`/`secrets.enc`. Commity są **Conventional Commits 1.0.0** bez gitmoji. Każda znacząca zmiana → wpis w `CHANGELOG.md [Unreleased]`.

Pełne uzasadnienia, tabele i przykłady są w **wydzielonych dokumentach** — patrz `Documentation map` poniżej.

---

## 1. Guardrails (Non-negotiables)

Pełna lista i wymuszanie: [.cursor/rules/00-charter.mdc](.cursor/rules/00-charter.mdc).

Skrót, bez którego PR jest **automatic reject**:

- **Sekret nigdy w `config.json`, logach, error message, stack trace.** Tylko keyring lub `secrets.enc`.
- **AES-GCM nonce TYLKO z `crypto/rand.Read(12 bytes)`.** Brak `time.Now()`, brak licznika.
- **SSH host key mismatch = strict block.** Brak auto-accept poza explicit user TUI confirmation z out-of-band phrase.
- **Brak telemetrii / phone-home.** `--debug-trace` (Sprint 14) zapisuje wyłącznie lokalnie do `~/.cache/webox/trace.jsonl` z redactorem.
- **Hardcoded provider name w business logic = automatic reject.** Logika biznesowa nigdy nie zna `smallhost` po nazwie — wszystko przez `providers.HostingProvider`.
- **`Update()` i `View()` są pure functions.** Brak `os.*`, `net.*`, channels w `tui/update.go`. Wszystko I/O w `tea.Cmd`.
- **`config.json` save = atomic.** `flock(2)` → write tmp → `fsync` → `rename` → `fsync(dir)`. Nigdy direct write.
- **Idempotentne `Remove*` w providerach.** Brak zasobu = `nil` error.
- **Multi-tick TUI flows** muszą mieć ≥ 1 scenariusz w `internal/e2e/` (`teatest` × `sshmock`).
- **`make bench-check` (perf gate) zielony** przed merge'em każdej zmiany w `tui/bento`. Próg `BENCH_MAX_NS = 5 ms`.
- **GitHub Actions tylko z pinned full 40-char SHA.** Nigdy `@v4`.

---

## 2. Documentation map (gdzie szukać czego)

Webox ma świadomie 3-warstwową dokumentację. Entry points (krótkie) → specialized docs (długie, na żądanie) → rules (auto-loaded by agent).

| Pytanie | Czytaj |
|---|---|
| **Co i dla kogo budujemy?** | [docs/PRD.md](docs/PRD.md) |
| **Jak to działa technicznie?** | [docs/DESIGN.md](docs/DESIGN.md) |
| **Jak to wygląda dla usera?** | [docs/UX.md](docs/UX.md) |
| **Threat model + secret policy?** | [docs/SECURITY.md](docs/SECURITY.md) |
| **Strategia testowa + piramida?** | [docs/TESTING.md](docs/TESTING.md) |
| **Co kiedy, kryteria GA?** | [docs/ROADMAP.md](docs/ROADMAP.md) |
| **Aktualny sprint + plan?** | [docs/sprints/](docs/sprints/) |
| **Workflow contributora EN entry?** | [CONTRIBUTING.md](CONTRIBUTING.md) (post Sprint 15) |
| **Pełny workflow PL?** | [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) (legacy detailed) |
| **Kod-konwencje, naming, error handling, commits?** | [docs/conventions.md](docs/conventions.md) |
| **Top 15 pułapek + anty-patterns?** | [docs/gotchas.md](docs/gotchas.md) |
| **Lista bibliotek + uzasadnienia?** | [docs/dependencies.md](docs/dependencies.md) |
| **39 pre-implementacyjnych findings + IMP-*?** | [docs/AUDIT.md](docs/AUDIT.md) |
| **Architecture decisions?** | [docs/adr/](docs/adr/) |
| **Wzorzec providera (kanoniczny)?** | [docs/providers/smallhost.md](docs/providers/smallhost.md) |
| **Jak dodać nowego providera (EN walkthrough)?** | [docs/contributing/PROVIDER.md](docs/contributing/PROVIDER.md) |
| **Vision dla preset registry?** | [docs/providers/preconfiguration-vision.md](docs/providers/preconfiguration-vision.md) |
| **Aktywne ryzyka?** | [docs/RISKS.md](docs/RISKS.md) |
| **Historia zmian?** | [CHANGELOG.md](CHANGELOG.md) |
| **Security disclosure?** | [SECURITY.md](SECURITY.md) (root) |

---

## 3. Workflow (TDD loop)

Każdy task ma 5 faz. Pełny szczegół w [docs/conventions.md §6](docs/conventions.md) + skill [tdd-loop](.cursor/skills/tdd-loop/SKILL.md).

```
1. Read    → relevantny PRD/DESIGN/SECURITY fragment.
2. Plan    → 3-5 zdań co zamierzasz zrobić.
3. Red     → failing test FIRST (parser → fixture, validator → table-driven).
4. Green   → minimalny kod żeby test przeszedł.
5. Refactor→ linter, gofumpt, CHANGELOG entry, commit conventional.
```

### TDD jest **twardo obowiązkowe** dla

- Parsery outputu providera (`devil www add`, `uapi PassengerApps list`, etc.).
- Walidatory wejścia użytkownika (regex subdomeny, port, alias).
- Redaktor sekretów (`internal/log/redact.go`).
- Status cache (SWR semantics, TTL, race).
- Maszyna stanów TUI (`tui/update.go`).
- Keyring detekcja (`secrets/keyring.go`).
- `config.json` load/save/migracje.
- `pending_cleanups.json` serialization + resume.

### Uruchomienie lokalnie

```bash
make tidy && make lint && make test && make cover-check && make vulncheck && make ci
```

Jeśli `make ci` przechodzi lokalnie, CI przejdzie. Jeśli CI failuje a lokalnie nie — bug w `Makefile`, nie w teście.

---

## 4. Scope discipline

| Reguła | Konsekwencja |
|---|---|
| **MVP v0.1 = small.pl/Devil only.** Po [ADR-0007](docs/adr/0007-bento-ultra-eskalacja-mvp.md) w MVP są też Bento Ultra (`120×35`), Live Log Stream, CI/CD Live Panel, Topology Map. | Referencje do `cpanel`/`directadmin`/`cyberpanel` w kodzie poza `provider.go` interface = automatic reject. |
| **v0.2 = + cPanel.** Patrz Sprint 17-18. | Implementacja cPanel adapter przed Sprint 17 = automatic reject. |
| **STRETCH v0.2+:** Sound engine, Env Merger, Bento Ultra+ (`≥160×45`), fast-chord bindings, DAG rollback. | Każda implementacja w MVP = reject, chyba że explicit ADR + maintainer sign-off. |
| **Brak operatorskich CLI commands poza `webox doctor`** | Operacje create/restart/import idą przez TUI. Wyjątek: limited startup/debug flags z [ADR-0001](docs/adr/0001-tui-zamiast-cli.md). |
| **Brak plugin marketplace / dynamic code loading** | Automatic reject w MVP i v0.2 — wymaga RFC + maintainer sign-off na v1.0+. |
| **Brak telemetrii / phone-home.** | Lokalny `~/.cache/webox/trace.jsonl` to NIE telemetria. |

Pełna lista v0.1 vs v0.2 vs nigdy: [docs/ROADMAP.md §3-6](docs/ROADMAP.md).

---

## 5. Decision policy

| Sytuacja | Postępowanie |
|---|---|
| **Decyduj sam:** wybór regex dla walidatora, nazewnictwo wewnętrznych funkcji, table-driven vs plain test, CHANGELOG entry, dodanie testu na świeżo poprawiony bug, refactor w pakiecie po zielonym `go test`. | Nie pytaj, zrób. |
| **Pytaj maintainera:** nowa zależność `go.mod`, zmiana publicznego API, zmiana scope MVP/STRETCH, zmiana struktury katalogów top-level, zmiana strategii rollback, nowy ADR, nowa flaga CLI/env var, zmiana którejkolwiek z guardrails (§1). | Stop, zapytaj z propozycją + alternatywą. |
| **Konflikt między docs.** | Priorytet: PRD > ROADMAP > DESIGN > UX. |
| **Brak ADR dla architektonicznej decyzji.** | Zatrzymaj się, zaproponuj ADR (skill: [adr-create](.cursor/skills/adr-create/SKILL.md)). |
| **Sugestia dodania telemetry / analytics / phone-home.** | **Automatic reject.** Lokalny trace ≠ telemetria. |

### Jak pytać efektywnie

```markdown
**Kontekst:** Pracuję nad <pakiet/plik>. <Doc> §X mówi że <zasada>.
**Pytanie:** Czy <opcja A> czy <opcja B>?
**Moja propozycja:** <A>. Argument: ...
**Alternatywa:** <B>. Trade-off: ...
**Co wybrać?**
```

---

## 6. Skills (auto-discoverable)

W `.cursor/skills/`:

- `tdd-loop` — TDD Red → Green → Refactor → Commit dla logiki krytycznej.
- `secret-flow` — secret handling policy enforcement.
- `commit-policy` — Conventional Commits + CHANGELOG.
- `auto-changelog` — automatyczna aktualizacja CHANGELOG po zmianie.
- `audit-trace` — verify diff vs autoritative docs.
- `adr-create` — szablon ADR dla nowej architektonicznej decyzji.
- `add-provider` — onboard nowego providera (cPanel/DA/CyberPanel).
- `release-check` — pre-release checklist.
- `retro` — retrospektywa po sprincie / PR.
- `task-start` — pickup nowego sprint task'a.
- `requesting-code-review` — check work przed merge.

---

## 7. Retrospektywa per task

Po każdym ukończonym task'u — 3 pytania. Skill: [retro](.cursor/skills/retro/SKILL.md).

```markdown
**Co działało dobrze?** ...
**Co poszło źle / wymagałoby ulepszenia?** ...
**Co zmieniam w workflow / dokumentacji?** ...
```

Po każdym sprincie → update [docs/AUDIT.md](docs/AUDIT.md) jeśli pojawiły się nowe znaleziska, rewizja [docs/gotchas.md](docs/gotchas.md) jeśli pojawiła się nowa pułapka.

---

> **Reguła ostateczna:** *Wątpisz? Pytaj. Pewny? Działaj. Działa? Przetestuj. Przetestowane? Skomituj. Skomitowane? Zaktualizuj `CHANGELOG.md`.*
