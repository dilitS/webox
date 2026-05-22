# Webox — Testing Strategy

> Status: Draft · Ostatnia aktualizacja: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [DESIGN.md](./DESIGN.md), [CONTRIBUTING.md](./CONTRIBUTING.md), [adr/0003](./adr/0003-provider-pattern.md).

## TL;DR

Testowanie webox jest piramidą: dużo testów jednostkowych (config, walidatory, parser outputu providerów, redaktor sekretów), warstwa integracyjna z **mock SSH server** (in-process Go SSH) i mock providerem, oraz testy e2e TUI przez **`teatest`** (snapshot testing). Cel coverage: **≥ 70 %** na MVP, **≥ 80 %** w v0.2. CI to GitHub Actions na matrycy macOS+Linux, z `golangci-lint`, `govulncheck` i build artefaktów. Manualne checklisty release zapewniają, że żadne side-effect (DNS propagation, GitHub API) nie zaskoczy w pre-release.

## Spis treści

1. [Cel dokumentu](#1-cel-dokumentu)
2. [Piramida testów](#2-piramida-test%C3%B3w)
3. [Mockowanie SSH](#3-mockowanie-ssh)
4. [Mockowanie GitHuba](#4-mockowanie-githuba)
5. [Testowanie TUI (Bubble Tea + teatest)](#5-testowanie-tui-bubble-tea--teatest)
6. [CI / GitHub Actions](#6-ci--github-actions)
7. [Test fixtures](#7-test-fixtures)
8. [Manualny checklist pre-release](#8-manualny-checklist-pre-release)

---

## 1. Cel dokumentu

Webox manipuluje hostingiem i GitHubem — testy muszą być **deterministyczne**, bez efektów ubocznych na realne zasoby. Ten dokument definiuje, co testujemy, jak izolujemy I/O i co wchodzi w bramkę CI.

## 2. Piramida testów

```
              ┌────────────────────┐
              │     Manual         │
              │   pre-release      │  ← rzadkie, drogie
              │   checklist        │
              └────────────────────┘
           ┌──────────────────────────┐
           │       E2E TUI            │
           │ (teatest snapshot tests) │
           └──────────────────────────┘
       ┌──────────────────────────────────┐
       │         Integration              │
       │ Provider × mock SSH × mock GH    │
       └──────────────────────────────────┘
   ┌────────────────────────────────────────────┐
   │                  Unit                       │
   │ parsery, walidatory, redaktor, cache, conf │
   └────────────────────────────────────────────┘
```

### 2.1 Unit (≥ 60 % wszystkich testów)

Co testujemy:

- **Config:** load/save round-trip, walidacja JSON Schema, migracje (`v1.0 → v1.1`).
- **Config persistence:** atomic save, lockfile takeover policy, odporność na partial write / stale lock.
- **Walidatory:** regex subdomeny, regex alias profilu, walidacja portu, walidacja długości nazwy DB.
- **Parser outputu providerów** (golden files): `devil www list`, `devil mysql add`, `devil ssl www add`, error patterns.
- **Redaktor sekretów:** różne payload'y na wejściu, sprawdzenie że nic nie wycieka.
- **Status cache:** TTL, stale-while-revalidate, invalidacja eventowa, race condition test (`go test -race`).
- **Rollback stack:** LIFO ordering, kontynuacja na błędach, persystencja do `pending_cleanups.json`.
- **Workflow template generation:** `deploy.yml` renderuje poprawne placeholdery, nie gubi quoting i nie wypisuje sekretów do logów.
- **I18n integrity:** `en.json` i `pl.json` mają ten sam zestaw kluczy dla ekranów core, brakujące tłumaczenia failują check.
- **Maszyna stanów:** każde przejście z [DESIGN.md §12](./DESIGN.md#12-maszyna-stan%C3%B3w-tui).
- **Key bindings:** mapowanie klawiszy → akcje per stan.

Cel: **≥ 80 %** pokrycia w pakietach `config/`, `providers/` (parsery), `status/`, `secrets/redactor`, `tui/wizard` (state machine).

### 2.2 Integration (≈ 25 %)

Co testujemy:

- **`SmallHostProvider` × mock SSH server** — pełne sekwencje: `CreateSubdomain → SetupSSL → CreateDatabase → RemoveDatabase` (rollback path).
- **`provider.Registry` z `mock` providerem** — symulacja błędów (timeouty, exit codes).
- **GitHub client × mock HTTP server** — tworzenie repo, secrets, monitoring runs.
- **`.env.example` vs GH Secrets drift checks** — brakujące klucze blokują deploy, nadmiarowe tylko ostrzegają.
- **End-to-end wizard** (bez TUI): logika kreatora deterministycznie odpaliana, oczekiwany side-effect w mockach.

### 2.3 E2E TUI (≈ 10 %)

Co testujemy:

- Renderowanie dashboardu z fixture'em (10 projektów, mix statusów).
- Wizard step-by-step: kluczowe na każdym kroku → oczekiwana strona render.
- Confirm dialog flow (Yes / No / Esc).
- Command Palette fuzzy search.
- Reakcja na resize terminala (88×28 vs 100×30 — różne layouty).

Patrz [§5](#5-testowanie-tui-bubble-tea--teatest).

### 2.4 Manual (~5 %)

Realne side-effecty (DNS, Let's Encrypt, GH Actions runner). Patrz [§8](#8-manualny-checklist-pre-release).

## 3. Mockowanie SSH

### 3.1 Wymagania

- Bez zewnętrznego serwera SSH.
- Bez prawdziwego klucza prywatnego usera (testowy ephemeral).
- Determinizm w outpucie komend.

### 3.2 Implementacja — in-process SSH server

Biblioteka: `github.com/gliderlabs/ssh` (lub `golang.org/x/crypto/ssh` bezpośrednio).

```
testing/sshmock/
├── server.go          # ssh.Server na losowym wolnym porcie
├── handler.go         # SessionHandler: routing po command → fixture
└── fixtures/
    ├── devil_www_add_ok.txt
    ├── devil_www_add_exists.txt
    ├── devil_mysql_add_ok.txt
    └── …
```

Każdy test integracyjny ustawia mapowanie `command pattern → fixture file + exit code`. Mock server:

1. Akceptuje key auth (testowy ephemeral pair generowany per test).
2. Loguje wszystkie odebrane komendy.
3. Zwraca skonfigurowany fixture jako stdout, opcjonalnie stderr i exit code.

Smoke test mocka (zapewnia że sam mock działa):

```text
TestSSHMockBasic:
  given server with mapping "echo hello" -> {"hello\n", 0}
  when client.Exec("echo hello")
  then stdout == "hello\n" and exitCode == 0
```

### 3.3 Fixture'y output `devil`

W `testing/fixtures/devil/` żyją realne outputy zarejestrowane manualnie z konta testowego small.pl (sanityzowane: realny login zamieniony na `testuser`, realne IP zamienione na `203.0.113.10`). Każdy fixture ma towarzyszącą notkę:

```
# fixtures/devil/www_add_ok.fixture.md
captured: 2026-04-12
account: testuser@s1.small.pl
command: devil www add test.testuser.smallhost.pl nodejs 24
sanitized: login -> testuser
```

### 3.4 Czego mock SSH NIE robi

- **Nie emuluje realnej propagacji DNS** — testy SSL pomijają `SetupSSL` rzeczywiste, walidują tylko że adapter wykonał odpowiednią komendę.
- **Nie emuluje rate-limitów Let's Encrypt** — wykrywanie rate limit jest testowane osobno przez golden output.
- **Nie emuluje sieciowych awarii** — mock ma osobny tryb `inject_failure` dla testów timeoutów.

## 4. Mockowanie GitHuba

### 4.1 Strategia: HTTP recorder + replay (vcr-style)

Biblioteka: `gopkg.in/dnaeon/go-vcr.v3` lub własny prosty `httptest.NewServer`.

Każdy test integracyjny GH ma odpowiadającą `cassette/<test_name>.yaml`:

```
# cassette/create_repo_basic.yaml
interactions:
  - request:
      method: POST
      uri: https://api.github.com/user/repos
      body: '{"name":"mockupweb","private":true,…}'
    response:
      status: 201
      body: '{"id":12345,"full_name":"dilitS/mockupweb",…}'
```

Pierwszy run testu w trybie `RECORD` (z realnym tokenem deweloperskim na sandbox account) zapisuje cassette. Kolejne runy w `REPLAY` używają cassette bez kontaktu z internetem.

### 4.2 Sanityzacja cassette

Przed commitem do repo cassette przechodzi przez sanityzer:

- Token z headera `Authorization: Bearer ghp_...` → `Authorization: Bearer [REDACTED]`.
- Login realny → `testuser`.
- Repo name realne → `testrepo-<num>`.

CI sprawdza, że żaden cassette nie zawiera `ghp_` ani `gho_` plaintextem.

### 4.3 Alternatywa: lokalny stub serwer

Dla testów które nie wymagają realistycznych payloadów (np. testy logiki retry) — `httptest.NewServer` z mapowaniem path→response. Szybsze niż go-vcr i nie wymaga cassette.

## 5. Testowanie TUI (Bubble Tea + teatest)

### 5.1 Biblioteka

`github.com/charmbracelet/x/exp/teatest` — oficjalny harness od Charm. Pozwala:

- Uruchomić `tea.Program` z controlled inputem.
- Wysyłać `tea.Msg` i klucze (`tea.KeyEnter`, `tea.KeyRunes('n')`).
- Zrobić snapshot finalnego `View()` lub konkretnej klatki.
- Asercja regex match na rendered string.

Patrz [DESIGN.md §2.3 MVU](./DESIGN.md#23-zasady-przep%C5%82ywu-danych-mvu) — czystość `update()` + `view()` to fundament testowalności.

### 5.2 Wzorzec testu snapshot

```text
TestDashboardRendersTenProjects:
  given config with 10 projects (fixture: 10_projects.json)
  given statusCache pre-filled with deterministic data
  when program.Send(tea.WindowSizeMsg{100, 30})
  when wait until program quiescent
  then snapshot of program.Model().View() == golden(dashboard_10_projects_100x30)

TestDashboardFallbacksTo88x28:
  …
  when program.Send(tea.WindowSizeMsg{88, 28})
  then snapshot == golden(dashboard_10_projects_88x28)
  and rendered does not contain right-detail-pane separator
```

Golden files leżą w `tui/testdata/golden/*.golden.txt` i są aktualizowane przez `go test -update`.

### 5.3 Co testujemy w TUI

| Scenariusz | Forma |
|---|---|
| Render dashboardu — pełen rozmiar | snapshot |
| Render dashboardu — fallback rozmiaru | snapshot |
| Render Init Wizard | snapshot |
| Render każdego kroku 1–5 wizardu nowego projektu | snapshot |
| Smart skip kroku DB dla statycznego stack'u | snapshot + key sequence assertion |
| Confirm dialog Yes/No | sequence test |
| Reveal `.env` (key `v` + confirm) | sequence test + assertion że value plaintext widoczny po confirm |
| Stale project banner | snapshot z mock providerem `ListSubdomains` zwracającym braki |
| Command Palette fuzzy `/cre` → highlight `/create` | snapshot |

### 5.4 Czego TUI testy NIE robią

- Nie testują kolorów (snapshot zapisany jako tekst ze stripped ANSI). Kolory walidowane wizualnie.
- Nie testują interakcji z prawdziwym terminalem (xterm/iTerm specific). To manual.

## 6. CI / GitHub Actions

### 6.1 Workflow `ci.yml`

Trigger: `push` (każdy branch) + `pull_request` (do `main`).

| Job | Runner | Czas docelowy |
|---|---|---|
| `lint` | ubuntu-latest | <2 min |
| `test-linux` | ubuntu-latest | <5 min |
| `test-macos` | macos-latest | <8 min |
| `vulncheck` | ubuntu-latest | <1 min |
| `build` | matrix [ubuntu, macos] × [amd64, arm64] | <5 min |

#### `lint`

- `golangci-lint run` z config `.golangci.yml`.
- Linterzy włączeni: `gofmt`, `goimports`, `govet`, `staticcheck`, `errcheck`, `gocritic`, `revive`, `gocyclo`, `gosec`, `misspell`.
- Whitelist: brak.
- Gdy `translations/` istnieje: `make i18n-check` jako część joba `lint`.

#### `test-linux` / `test-macos`

- `go test -race -coverprofile=coverage.out ./...`
- Próg `--threshold 70` (MVP) w `coverage.yml` — CI **fails** jeśli coverage spada poniżej.
- Upload coverage do Codecov (opcjonalne).

#### `vulncheck`

- `govulncheck ./...` — fail przy known CVE.

#### `build`

- `goreleaser build --snapshot --clean` z `-trimpath`.
- Artefakty: `webox_<os>_<arch>.tar.gz`.
- Build się **udać MUSI** dla wszystkich platform — żaden OS fallback.

### 6.2 Workflow `release.yml`

Trigger: tag `v*`.

- `goreleaser release` z signing cosign (`COSIGN_EXPERIMENTAL=1`).
- Publikacja: GH Releases + Homebrew tap update PR.
- SLSA provenance + checksums + SBOM (post-MVP).

### 6.3 Workflow `nightly.yml` (post-MVP)

Trigger: `schedule: '0 3 * * *'`.

- Pełne integracje vs **lokalny mock providera + lokalny mock GH** (bez sieci).
- Test scenariuszy długoczasowych: stale detector, rotacja cache, auto-update probe.

## 7. Test fixtures

### 7.1 Lokalizacja

```
testing/fixtures/
├── config/
│   ├── valid_v1_0.json
│   ├── malformed_no_profiles_array.json
│   ├── breaking_v0_x.json          (do testowania migracji)
│   ├── projects_10.json            (perf benchmark)
│   └── locked_config_stale_lock.json
├── devil/
│   ├── www_add_ok.txt
│   ├── www_add_exists.txt
│   ├── ssl_add_ok.txt
│   ├── ssl_add_dns_not_ready.txt
│   ├── mysql_add_ok.txt
│   ├── mysql_add_name_taken.txt
│   ├── www_list_5_subdomains.txt
│   └── www_restart_ok.txt
├── github/
│   ├── cassettes/
│   │   ├── create_repo_basic.yaml
│   │   ├── set_secret.yaml
│   │   └── workflow_run_status.yaml
│   └── responses/
│       ├── repo_create_201.json
│       ├── secret_set_204.txt
│       └── rate_limit_403.json
├── workflow/
│   ├── deploy_smallhost_basic.golden.yml
│   ├── deploy_smallhost_static_site.golden.yml
│   └── env_example_missing_keys.txt
└── tui/
    ├── golden/
    │   ├── dashboard_10_projects_100x30.golden.txt
    │   ├── dashboard_10_projects_88x28.golden.txt
    │   ├── wizard_step1.golden.txt
    │   └── …
    └── inputs/
        └── (klawisze do sterowania teatest)
```

### 7.2 Konwencje nazewnicze

- `*.fixture.md` obok każdego fixture'a: opis pochodzenia, sanityzacja, kontekst.
- Golden files: `<scenario>_<terminal_size>.golden.txt`.
- Cassettes: `<test_name>.yaml`.

### 7.3 Aktualizacja fixtures

- `go test -update` regeneruje golden files **lokalnie**. Nigdy nie commitować z `-update` ślepo — review diff przed commit'em.
- Recapture realnych cassettes wymaga sandbox account `gh-webox-test` (tylko maintainer).

## 8. Manualny checklist pre-release

Każdy release-candidate przed promocją do `latest` przechodzi:

### 8.1 Pre-MVP / v0.1

- [ ] Świeża instalacja na czystym macOS i Ubuntu (`go install` + `brew`).
- [ ] Init wizard: nowy klucz SSH + auto-deploy do testowego small.pl.
- [ ] Wizard nowego projektu (Vite + React, bez DB) — sukces end-to-end.
- [ ] Wizard nowego projektu (Node.js backend + MySQL) — sukces end-to-end.
- [ ] Wizard z DNS nie skonfigurowanym — graceful failure + rollback całości.
- [ ] Restart projektu (`r`) — strona po ~5 s wraca z 200.
- [ ] SSL renew na cert <14 dni do wygaśnięcia — sukces.
- [ ] Podgląd logów — tail ostatnich 200 linii.
- [ ] Import istniejącego projektu — projekt pojawia się z `imported: true`, banner widoczny.
- [ ] Stale project: ręcznie usunąć subdomenę z panelu → refresh → STALE badge.
- [ ] Wygenerowany `deploy.yml` używa projektowego deploy key, nie globalnego klucza operatorskiego.
- [ ] `config.json` odporny na podwójne uruchomienie: druga instancja pokazuje lock instead of nadpisywanie.
- [ ] Ekrany core (`dashboard`, wizard, settings, dialogi) nie mają brakujących tłumaczeń EN/PL.
- [ ] Resize terminalu z 100×30 → 80×24 → 60×20 — odpowiednie fallbacki + komunikat.
- [ ] `webox doctor` zwraca zielone na sprawnej konfiguracji.
- [ ] Crash test: zabicie połączenia SSH w trakcie wizard step 3 — `pending_cleanups.json` poprawny, restart webox dokańcza cleanup.
- [ ] Test fallback secrets bez keyringa: WSL w trybie bez Secret Service, master password 12 znaków.

### 8.2 v0.2 dodatki

- [ ] `/db`, `/env`, `/storage`, `/domain` sub-widoki.
- [ ] Live log stream przez 5 minut, brak pamięci leak.
- [ ] GitHub Actions monitor: workflow run w toku → progress widoczny do `success`.
- [ ] Auto-update probe (nie in-app, tylko detekcja).

### 8.3 Sign-off

Release-candidate jest mergowany dopiero gdy wszystkie [ ] zostają zaznaczone w PR description przez maintainera. Brak automation tego checklist'a w v1 — to świadoma decyzja (manualny "ludzki" gate).
