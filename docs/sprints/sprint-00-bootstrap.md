# Sprint 00 — Bootstrap

> **Daty:** TBD → TBD (planowane 1 tydzień solo) · **Czas:** ~20-25h skupienia
>
> **Cel:** Repozytorium jest **buildable, testable, linted, scannable** end-to-end. Po sprincie `make ci` przechodzi zielony lokalnie i w GitHub Actions, a `webox version` drukuje semver z embedded build info.

---

## TL;DR

Po sprincie 00:

- Skompilowany `cmd/webox` skeleton, który drukuje wersję.
- `make ci` (lint + vet + test + vulncheck + build) działa lokalnie i w CI.
- Każdy commit blokowany przez Conventional Commits hook (już mamy w `.cursor/hooks/`).
- `golangci-lint v2`, `govulncheck`, `gofumpt` zainstalowane jako pinned dev tools.
- GitHub repo ma `pull_request_template.md`, `CODEOWNERS`, issue templates, security policy.
- Release pipeline (`goreleaser` config) gotowy do dry-run — bez publikacji jeszcze.

**Nie robimy w tym sprincie:**

- Żadnej logiki domenowej (`config/`, `secrets/`, `providers/` — to Sprint 01+).
- Żadnej integracji SSH ani `small.pl`.
- Żadnej TUI poza `webox version`.

---

## Pre-flight checklist (przed sprintem)

- [ ] Lokalnie zainstalowany `go 1.24+` (`go version`).
- [ ] `gh` CLI zalogowany (`gh auth status`).
- [ ] Repozytorium już na GitHub (origin set).
- [ ] Slot 4h na sprint planning (5 min retro + 25 min dekompozycja).

---

## Taski

### TASK-00.1 — Initialize `go.mod` + project layout

- **Estymata:** S
- **Zależności:** —
- **Acceptance Criteria:**
  - [ ] `go.mod` z `module github.com/<owner>/webox` i `go 1.24`.
  - [ ] Struktura katalogów wg `README.md §Project structure`:
    - `cmd/webox/`, `providers/`, `ssh/`, `wizard/`, `tui/`, `secrets/`, `config/`, `status/`, `env/`, `i18n/`, `internal/log/`, `internal/version/`, `services/`, `testdata/`.
  - [ ] Każdy katalog ma stub `doc.go` z `// Package X ...` (akceptowany przez `golangci-lint`).
  - [ ] `go build ./...` przechodzi bez błędów (nawet jeśli puste pakiety).
- **Pliki:**
  - `go.mod` (new)
  - `<package>/doc.go` × ~12 (new)
- **Docs:** [`README.md §Project structure`](../../README.md#project-structure), [`DESIGN.md §3 Architektura`](../DESIGN.md)
- **Notatki:** Owner module name musi zgadzać się z planowanym home (np. `github.com/webox-tui/webox`). Zmiana później = bolesny refactor.

---

### TASK-00.2 — Pinned dev tools via `tools.go`

- **Estymata:** S
- **Zależności:** TASK-00.1
- **Acceptance Criteria:**
  - [ ] `tools/tools.go` z `//go:build tools` i blank imports dla:
    - `github.com/golangci/golangci-lint/v2/cmd/golangci-lint`
    - `golang.org/x/vuln/cmd/govulncheck`
    - `mvdan.cc/gofumpt`
    - `golang.org/x/tools/cmd/goimports`
    - `github.com/goreleaser/goreleaser/v2`
  - [ ] `go mod tidy` wpisuje wersje do `go.mod`.
  - [ ] `make tools-install` w `Makefile` instaluje wszystkie cztery do `$GOBIN`.
  - [ ] Wersje pinned są w `CONTRIBUTING.md §1.1`.
- **Pliki:**
  - `tools/tools.go` (new)
  - `Makefile` (edit, sekcja `tools-install`)
  - `docs/CONTRIBUTING.md` (edit, wersje)
- **Docs:** [`CONTRIBUTING.md §1.1`](../CONTRIBUTING.md)
- **Notatki:** **Nie używamy `go install` z latest** — zawsze pinned. `tools.go` to standard Go idiom (zob. Go 1.24 `go tool` directive jako alternatywa — dopuszczalne, jeśli wolisz nowsze).

---

### TASK-00.3 — `.golangci.yml` v2 configuration

- **Estymata:** S
- **Zależności:** TASK-00.2
- **Acceptance Criteria:**
  - [ ] `.golangci.yml` z `version: "2"` (v2 format).
  - [ ] Włączone linters (z mapping v1→v2 wg `CONTRIBUTING.md §2.1`):
    - `govet`, `errcheck`, `staticcheck`, `unused`, `ineffassign`, `gosec`, `revive`, `gocyclo` (max 20), `dupl`, `gofumpt`, `goimports`, `misspell`, `unparam`, `bodyclose`, `errorlint`, `gosimple`, `prealloc`, `unconvert`.
  - [ ] `exclude-rules` dla `testdata/` (loose) i `_test.go` (zezwala na `dupl`).
  - [ ] `make lint` przechodzi na pustym kodzie.
- **Pliki:**
  - `.golangci.yml` (new)
- **Docs:** [`CONTRIBUTING.md §2.1`](../CONTRIBUTING.md)
- **Notatki:** `gocyclo` max 20 zgodnie z `AUDIT §8 IMP-19`. Jeśli `gosec` daje false-positives w `tools.go`, dodaj `nolint:gosec` z komentarzem.

---

### TASK-00.4 — GitHub Actions CI workflow

- **Estymata:** M
- **Zależności:** TASK-00.2, TASK-00.3
- **Acceptance Criteria:**
  - [ ] `.github/workflows/ci.yml` z jobami:
    - `lint` (golangci-lint v2 + `go vet`)
    - `test` (matrix: ubuntu-latest, macos-latest; Go 1.24)
    - `vulncheck` (`govulncheck ./...`)
    - `build` (cross-compile: linux/amd64, darwin/arm64, darwin/amd64)
  - [ ] Wszystkie actions **pinned do commit SHA** (nie tag, nie branch) — zgodnie z `.cursor/rules/70-shell-and-workflow.mdc`.
  - [ ] `permissions:` zawężone do `contents: read` na poziomie workflow.
  - [ ] Coverage upload do `codecov` lub `coverage.txt` artifact (do decyzji w sprincie).
  - [ ] Status badge w `README.md` linkuje do tego workflow.
- **Pliki:**
  - `.github/workflows/ci.yml` (new)
  - `README.md` (edit, badge URL)
- **Docs:** [`TESTING.md §6.1`](../TESTING.md)
- **Notatki:**
  - **Pułapka:** GitHub `setup-go` cache invalidation — używaj `cache: true` z explicit `cache-dependency-path`.
  - Pinned SHA: znajduj przez `gh api repos/<owner>/<repo>/commits/<tag>`.

---

### TASK-00.5 — `cmd/webox` skeleton + `internal/version`

- **Estymata:** S
- **Zależności:** TASK-00.1
- **Acceptance Criteria:**
  - [ ] `cmd/webox/main.go` z `func main()` parsuje `--version`, `--help`, `--debug` (tylko te trzy flagi — patrz `ADR-0001`).
  - [ ] `internal/version/version.go` z `var (Version, Commit, BuildDate string)` ustawianymi przez `-ldflags` w `Makefile`.
  - [ ] `webox --version` drukuje `webox v0.0.0-dev (<commit>) built <date>`.
  - [ ] `webox --help` drukuje krótki helper i wskazuje na docs.
  - [ ] `internal/version/version_test.go` weryfikuje format output (tabela).
- **Pliki:**
  - `cmd/webox/main.go` (new)
  - `internal/version/version.go` (new)
  - `internal/version/version_test.go` (new)
  - `Makefile` (edit, target `build` z `-ldflags`)
- **Docs:** [`ADR-0001`](../adr/0001-tui-zamiast-cli.md)
- **Notatki:** Bez TUI jeszcze. Bez `cobra`/`urfave/cli` — manualny parse 3 flag (`os.Args`). Cobra wjedzie później jeśli faktycznie potrzeba.

---

### TASK-00.6 — Package `doc.go` stubs

- **Estymata:** S
- **Zależności:** TASK-00.1
- **Acceptance Criteria:**
  - [ ] Każdy main package (`providers`, `ssh`, `wizard`, `tui`, `secrets`, `config`, `status`, `env`, `i18n`, `services`, `internal/log`, `internal/version`) ma `doc.go` z 3-5 zdaniami opisu w stylu godoc.
  - [ ] Tekst opisu **zgodny z DESIGN.md §3** — używamy tej samej terminologii (np. „LIFO rollback stack", „Stale-While-Revalidate cache").
  - [ ] `golangci-lint run` nie podnosi `revive: package-comments`.
- **Pliki:**
  - `<package>/doc.go` × 12
- **Docs:** [`DESIGN.md §3`](../DESIGN.md)
- **Notatki:** Sześcio-godzinny task jeśli nie skopiujesz, więc skopiuj wzorce z [Go std lib](https://pkg.go.dev/std).

---

### TASK-00.7 — GitHub repo policy files

- **Estymata:** S
- **Zależności:** —
- **Acceptance Criteria:**
  - [ ] `.github/pull_request_template.md` z checklistą DoD.
  - [ ] `.github/ISSUE_TEMPLATE/bug.yml` (form).
  - [ ] `.github/ISSUE_TEMPLATE/feature.yml` (form).
  - [ ] `.github/ISSUE_TEMPLATE/config.yml` (`blank_issues_enabled: false`).
  - [ ] `.github/CODEOWNERS` z minimum: `* @<owner>`.
  - [ ] Root `SECURITY.md` z pointer do `docs/SECURITY.md` (GitHub szuka w root).
  - [ ] `.github/FUNDING.yml` (opcjonalnie, jeśli planujesz sponsor).
- **Pliki:**
  - `.github/...` (new, 5-7 plików)
  - `SECURITY.md` (new, root)
- **Docs:** [`CONTRIBUTING.md`](../CONTRIBUTING.md), [`docs/SECURITY.md`](../SECURITY.md)
- **Notatki:** **TO JEST PRZED FIRST PUBLIC PUSH.** Pierwsze impression matters.

---

### TASK-00.8 — `GoReleaser` config (dry-run only)

- **Estymata:** M
- **Zależności:** TASK-00.1, TASK-00.5
- **Acceptance Criteria:**
  - [ ] `.goreleaser.yml` (v2 format).
  - [ ] Build matrix: `linux/amd64`, `darwin/arm64`, `darwin/amd64`, `linux/arm64`.
  - [ ] `archives:` z `tar.gz`.
  - [ ] `checksum:` z `sha256`.
  - [ ] `signs:` block z `cosign` (placeholder — nie podpisujemy jeszcze).
  - [ ] `make release-dry-run` (`goreleaser release --snapshot --skip=publish --clean`) przechodzi lokalnie.
  - [ ] `CGO_ENABLED=0` dla wszystkich buildów (statyczny binary).
- **Pliki:**
  - `.goreleaser.yml` (new)
  - `Makefile` (edit, target `release-dry-run`)
- **Docs:** [`DESIGN.md §17 Dystrybucja`](../DESIGN.md), [`CONTRIBUTING.md §1.1`](../CONTRIBUTING.md)
- **Notatki:**
  - **NIE publikujemy** żadnego release w tym sprincie — to tylko konfiguracja.
  - Sprawdź `goreleaser check`.

---

### TASK-00.9 — Pre-commit local hook (opcjonalnie)

- **Estymata:** S
- **Zależności:** TASK-00.3
- **Acceptance Criteria:**
  - [ ] `.git/hooks/pre-commit` (lokalny, **nie wersjonowany**) wywołuje `make lint` na zmienionych plikach.
  - [ ] `scripts/install-hooks.sh` ustawia symlink lub kopiuje.
  - [ ] `make setup-hooks` dodane do Makefile.
- **Pliki:**
  - `scripts/install-hooks.sh` (new)
  - `scripts/pre-commit` (new, source for the hook)
  - `Makefile` (edit)
- **Docs:** [`CONTRIBUTING.md`](../CONTRIBUTING.md)
- **Notatki:** To **dodatkowa warstwa** — Cursor hooks już mamy, ale jeśli ktoś nie używa Cursora, ten hook ratuje. Opcjonalny w sprincie 00 — można carry-over.

---

### TASK-00.10 — First green CI run + tag `v0.0.0-bootstrap`

- **Estymata:** S
- **Zależności:** TASK-00.4..TASK-00.8
- **Acceptance Criteria:**
  - [ ] PR z całością Sprint 00 mergowany do `main`.
  - [ ] CI na `main` zielony.
  - [ ] Git tag `v0.0.0-bootstrap` (annotated) bez release notes.
  - [ ] Badge `CI: passing` widoczny w `README.md`.
- **Pliki:** —
- **Docs:** —
- **Notatki:** **To jest** moment, w którym Sprint 00 jest „done". Wszystko poniżej tego nie liczy się.

---

## Risk watch

| Ryzyko | Impact | Mitygacja |
|--------|--------|-----------|
| **Conflict actions SHA** (np. `actions/checkout` updates) | M | Pinned SHA, dependabot na actions enabled. |
| **`golangci-lint v2` ma nieznane lintery** | L | Most pinned w `tools.go`; jeśli wybuchnie — fallback do v1 z TODO ADR. |
| **GoReleaser config edge-case na macOS arm64** | M | Dry-run lokalnie + na CI matrix; bez tag-push. |
| **Owner module name change later** | XL | Wybór nazwy `github.com/<owner>/webox` przed TASK-00.1; nie zmieniamy potem. |
| **TASK-00.6 doc.go nudny → odpuszczę** | S | Skill `commit-policy` blokuje merge bez `chore(docs):` linijek; reguła `60-docs.mdc`. |

---

## Outcome (wypełnij po sprincie)

- ✅ Done: TASK-00.1, ...
- ⏭️ Carry-over: ...
- 📌 Decyzje: ...
- 🧠 Surprises: ...
- 📊 Metryki:
  - PR-ów zmergowanych: N
  - Linijek kodu (prod): ~X
  - Linijek testów: ~Y
  - Coverage: Z%
  - Czas faktyczny vs estymata: ratio
- ➡️ Następny sprint: `sprint-01-foundations.md` (planning slot: …)

---

## Retro link (po sprincie)

`docs/retros/YYYY-MM-DD-sprint-00.md` — wypełnia skill `retro`.
