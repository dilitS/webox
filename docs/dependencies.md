# Webox — Dependencies & Toolchain Catalog

> Status: Stable for v0.1 · Ostatnia aktualizacja: 2026-05-25 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [DESIGN.md](./DESIGN.md), [adr/](./adr/), [CONTRIBUTING.md](./CONTRIBUTING.md), [SECURITY.md](./SECURITY.md).
>
> Ten plik jest **autorytetywnym katalogiem zależności** projektu. Każda biblioteka w `go.mod` powinna być tu uzasadniona. Nowa zależność wymaga sign-off maintainera i adnotacji w PR.

---

## 1. Toolchain

| Element | Wersja / Wybór | Uzasadnienie |
|---|---|---|
| Język | **Go 1.25+** | `go 1.25.0` w `go.mod`. Podbite w Sprint 02, bo `golang.org/x/crypto/ssh` fixy z `govulncheck` wymagają `x/crypto v0.52.0`, a ten deklaruje Go 1.25. |
| Module system | Go modules | `go.mod` + `go.sum`; `vendor/` nie commitujemy. |
| Build tool | `go build` + Makefile | `make build`, `make test`, `make lint`. |
| Linter | `golangci-lint v2.x+` | Config: `.golangci.yml` z `version: "2"`. Nazwy v2: `gas→gosec`, `goerr113→err113`, `gomnd→mnd`. |
| Formatter | `gofumpt` + `goimports` | `make fmt`. Ostro: nic poza nimi. |
| Vulnerability scan | `govulncheck` | CI gate (`make vulncheck`). |
| Release | `GoReleaser 2.x` | `make snapshot` / `make release-dry-run`. |
| Signing | `cosign` (keyless OIDC) + SLSA | Obowiązkowe od v0.1. |
| Coverage | `go test -coverprofile=` + `make cover-check` | Minimum 70% (MVP), 80% (v0.2). |

### 1.1 Dev tools pinned via `tools/go.mod`

Webox używa **Go 1.24+ `tool` directive** w **osobnym `tools/go.mod`** (izolacja od głównego modułu). Wszystkie poniższe są przypinane do konkretnych wersji w `tools/go.mod` i odpalane przez `make` przez `go tool -modfile=tools/go.mod <bin>`.

| Tool | Wersja pinned (May 2026) | Wywołanie |
|---|---|---|
| `golangci-lint` | v2.12.2 | `make lint` |
| `gofumpt` | v0.10.0 | `make fmt` (część) |
| `goimports` | (z `golang.org/x/tools` v0.45.0) | `make fmt` (część) |
| `govulncheck` | v1.3.0 | `make vulncheck` |
| `goreleaser` | v2.15.4 | `make snapshot` / `make release-dry-run` |

**Bumping:** `cd tools && go get -tool <pkg>@<version>` → `make tools-tidy` → `make ci`.

---

## 2. Runtime dependencies (production code)

| Pakiet | Cel | Uwagi krytyczne |
|---|---|---|
| `github.com/charmbracelet/bubbletea` | TUI MVU framework | `tea.Cmd`, `Update()`, `View()` — patrz [DESIGN §2.3](./DESIGN.md#23-zasady-przep%C5%82ywu-danych-mvu). |
| `github.com/charmbracelet/lipgloss` | Terminal styling | Deklaratywny, OKLCH colors. **Nie** używać do 60fps renderingu. |
| `github.com/charmbracelet/bubbles` | Common TUI components | Spinner, textinput, table — używamy gdy pasują. |
| `golang.org/x/crypto/ssh` | SSH client | Native Go, **bez** zależności od systemowego `ssh`. |
| `github.com/pkg/sftp` | SFTP nad SSH | Atomic put: `<file>.tmp` + `Rename`. |
| `github.com/zalando/go-keyring` | Keyring | Sentinel errors: `ErrUnsupportedPlatform`, `ErrNotFound`. **Detekcja przez probe write/read/delete** (patrz [SECURITY §4.2](./SECURITY.md#42-fallback-dla-środowisk-headless)). |
| `github.com/gofrs/flock` | Cross-platform file locks | Dla `config.json` lock. **NIE polegamy na PID check** (patrz [DESIGN §6](./DESIGN.md#6-model-danych-i-atomowość-zapisu-configjson)). |
| `github.com/awnumar/memguard` | Locked secret buffers | `LockedBuffer.Destroy()` zamiast wymyślonego `zerocopy.Wipe`. |
| `golang.org/x/sync/singleflight` | Deduplikacja równoległych fetch'ów | Dla SWR cache (patrz [DESIGN §8](./DESIGN.md#8-trójpoziomowy-status-cache-stale-while-revalidate)). |
| `golang.org/x/sync/semaphore` | In-flight SSH semaphore | Sprint 14 task 14.3. Budget `max(8, len(profiles)/2)`. |
| `crypto/rand` | CSPRNG | **Jedyne** dopuszczalne źródło nonce dla AES-GCM (patrz [SECURITY §4.2.1](./SECURITY.md#421-generowanie-nonce-krytyczne-dla-aes-gcm)). |
| `golang.org/x/crypto/argon2` | KDF | Parametry: `memory=64MB, iterations=3, parallelism=2`. |
| `gopkg.in/yaml.v3` | YAML parsing | Dla walidacji `deploy.yml` przed commit do repo. |
| `gopkg.in/natefinch/lumberjack.v2` | Log rotation | `webox.log` rotuje przez tę bibliotekę. |
| `github.com/google/uuid` | UUID v4 dla `projects[].id` | Nigdy nie sekwencyjny ID. |

---

## 3. Test-only dependencies

| Pakiet | Cel | Uwagi krytyczne |
|---|---|---|
| `github.com/charmbracelet/x/exp/teatest` | TUI testing harness | **Eksperymentalna** ścieżka — pinujemy commit hash w `go.mod`, nie `latest`. |
| `testing/sshmock` (in-tree) | SSH server mock dla integration tests | `testing/sshmock/`. Replays captured fixtures. |
| `net/http/httptest` (stdlib) | HTTP server mock (cPanel UAPI w Sprint 17+) | Standard library. |

---

## 4. CI / Action dependencies

| Action / Image | Wersja (full SHA pinned) | Cel |
|---|---|---|
| `actions/checkout` | `@<full-40-char-SHA>` (NIE `@v4`) | Source checkout. Pinned SHA dla supply chain. |
| `actions/setup-go` | `@<full-40-char-SHA>` | Go install. |
| `actions/cache` | `@<full-40-char-SHA>` | Go modules cache. |
| `actions/labeler` | `v5` (config: `.github/labeler.yml`) | Path-based labels per PR. |
| `dependabot/fetch-metadata` | `v2` | Auto-merge dla patch/minor non-prod deps. |
| `goreleaser/goreleaser-action` | `@<full-40-char-SHA>` | Release automation. |
| `sigstore/cosign-installer` | `@<full-40-char-SHA>` | Signing tooling. |

**Reguła:** GitHub Actions używamy **TYLKO** z pinned full SHA (40 chars). Tagi można przepisać przez supply-chain attack, SHA nie.

---

## 5. Polityka dodawania nowej zależności

1. **Sign-off maintainera w PR description** — bez tego PR nie merge.
2. **Uzasadnienie:** dlaczego biblioteka, nie własna implementacja? Jaka alternatywa rozważana?
3. **Licencja kompatybilna:** Apache-2.0, MIT, BSD-2/3, MPL-2.0 OK. GPL/AGPL — automatyczna rejekcja.
4. **Coverage upstream:** sprawdź ostatni release (≥ 6 miesięcy temu = czerwona flaga), # of stars, # of contributors.
5. **`govulncheck` clean** dla nowej zależności i transitive deps.
6. **Po dodaniu:** **przeczytaj `go.mod`**, sprawdź transitive `go.mod` nowych modułów, **pinuj ostatnią wersję kompatybilną z aktualnym Go floor** (`go 1.25.0`).
7. **Wyjątek:** jeśli `govulncheck` wskazuje realnie wywoływany kod i fix wymaga wyższego flooru, podbij floor świadomie, opisz dlaczego w PR + CHANGELOG i zaktualizuj ten dokument.

---

## 6. Antypatterns

**Czego NIE używamy** (i dlaczego):

| Zabronione | Dlaczego | Co używamy zamiast |
|---|---|---|
| `github.com/sirupsen/logrus` | Outdated, Go ma `log/slog` od 1.21 | `log/slog` (stdlib) + `internal/log/redact.go` wrapper. |
| `github.com/spf13/viper` | Magic config loading, łatwo wycieka sekrety | Direct JSON decode + `config/schema.go`. |
| `github.com/spf13/cobra` poza `cmd/webox/` | Większość ścieżek w Webox to TUI, nie CLI | Minimal command parsing w `cmd/webox/run.go`. |
| `github.com/stretchr/testify/assert` | Magic asserts ukrywają intencję | `t.Errorf` / `t.Fatalf` z explicit message. |
| `database/sql` driver (any) | MVP scope nie potrzebuje lokalnej DB | DB management u providera (UAPI Mysql). |
| HTTP client framework (`gorilla`, `gin`, `echo`) | Webox jest klientem, nie serwerem | `net/http.Client` standardowy. |
| Plugin loading (`plugin` stdlib, `.so`) | Supply chain risk | Adaptery providerów kompilowane in-tree. Patrz [ROADMAP §6](./ROADMAP.md#6-czego-nie-robimy-nigdy). |

---

> _Last reviewed: 2026-05-25. Wydzielone z `AGENTS.md §1` jako część Sprint 15 docs refactor._
