# Webox — Security

> Status: Draft · Ostatnia aktualizacja: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [DESIGN.md](./DESIGN.md), [adr/0004](./adr/0004-przechowywanie-sekretow-keyring.md), [CONTRIBUTING.md](./CONTRIBUTING.md).

## TL;DR

Webox jest narzędziem operatorskim z bezpośrednim dostępem do hostingu i GitHuba — błąd w sekrecie albo MITM SSH ma realny koszt (przejęcie konta, defacement strony). Polityka: **sekrety wyłącznie w systemowym keyringu** (fallback AES-GCM z Argon2id master-password tylko dla środowisk headless), **TOFU dla host keys** ze strict mismatch handling, **zero zdalnej telemetrii**, defensywne parsowanie outputu serwera (`devil`/`uapi`) i GitHub API. Threat model bazuje na STRIDE-light. Audyty stanu — przez `webox doctor security`.

Dodatkowa zasada dotycząca sekretów aplikacji: **Webox jest orchestratorem, nie pełnym vaultem**. GitHub Secrets są write-only targetem dla CI, lokalny secure store trzyma tylko wartości świadomie zarządzane przez Webox, a serwerowy `.env` pozostaje runtime representation. To rozróżnienie jest krytyczne, bo GitHub nie pozwala odczytać plaintextu sekretu po zapisie.

## Spis treści

1. [Cel dokumentu](#1-cel-dokumentu)
2. [Model zaufania](#2-model-zaufania)
3. [Threat model](#3-threat-model)
4. [Przechowywanie sekretów](#4-przechowywanie-sekret%C3%B3w)
5. [Host keys i SSH](#5-host-keys-i-ssh)
6. [GitHub Token](#6-github-token)
7. [Audyt sekretów i tryb doctor](#7-audyt-sekret%C3%B3w-i-tryb-doctor)
8. [Reportowanie podatności i supply chain](#8-reportowanie-podatno%C5%9Bci-i-supply-chain)
9. [Logging i wycieki](#9-logging-i-wycieki)
10. [Zarządzanie `.env` i sekrety aplikacji](#10-zarz%C4%85dzanie-env-i-sekrety-aplikacji)

---

## 1. Cel dokumentu

SECURITY.md ujmuje to, co w innych narzędziach często wpada do README jako jeden akapit. Webox dotyka:

- klucza prywatnego SSH użytkownika,
- tokena GitHuba z `repo` i `workflow`,
- hasła do bazy danych klienta,
- treści `.env` produkcyjnego.

Każdy z tych zasobów ma swój threat model i swoją politykę. Ten dokument je opisuje. Dokument nie zawiera kodu Go — tylko zasady i parametry.

## 2. Model zaufania

| Zasób | Zaufany? | Uzasadnienie |
|---|---|---|
| `~/.ssh/id_ed25519_webox` (klucz prywatny) | TAK | Pod kontrolą OS, plik z `0600` perms. |
| `~/.config/webox/config.json` | TAK (metadane), NIE (nigdy sekrety) | Plik niezaszyfrowany, ale nie zawiera sekretów. |
| Systemowy keyring (Keychain / Secret Service / Credential Manager) | TAK | Standard branżowy, zarządzany przez OS. |
| `pending_cleanups.json` | TAK (zawiera tylko nazwy zasobów, nie sekrety) | Sanity check przy migracji. |
| Wartości pól (subdomena, ścieżki) wprowadzone przez usera | TAK (z walidacją) | Walidator regex per pole + sanityzacja przed wstawieniem do komendy SSH. |
| Output komend serwera (`devil`, `uapi`) | NIE | Defensywne parsowanie, fail-soft. |
| Response GitHub API | NIE | Walidacja schema response, timeout, retry. |
| Plik `.env` na serwerze | NIE | Czytany binarnie, traktowany jako blob. |
| Host key serwera SSH (przy pierwszym połączeniu) | NIE (TOFU) | Pierwsze widzenie zapisujemy, kolejne porównujemy. Patrz [§5](#5-host-keys-i-ssh). |
| Binarka webox dystrybuowana z GH Releases | TAK (po weryfikacji cosign + checksum) | Patrz [§8](#8-reportowanie-podatno%C5%9Bci-i-supply-chain). |
| Pluginy / third-party providery (post-MVP) | NIE | Wymagane code review + signed release w core. W v1 brak dynamicznego ładowania pluginów. |

## 3. Threat model

Stosujemy STRIDE-light: identyfikujemy aktorów i ich możliwości.

### 3.1 Aktorzy

| Aktor | Możliwości | Motywacja |
|---|---|---|
| **A1 — Atakujący z dostępem do nieodblokowanego laptopa** | Może próbować brute-force hasła OS, ale nie ma keyringa. | Kradzież danych. |
| **A2 — Atakujący z dostępem do odblokowanej sesji OS** | Pełny dostęp do procesu webox, keyringa, schowka. | Kradzież credentiali. |
| **A3 — MITM na Wi-Fi w kawiarni** | Może próbować zmodyfikować pierwsze TCP do SSH. | Przejęcie sesji. |
| **A4 — Skompromitowany serwer hostingowy** (np. przejęte konto innego usera + eskalacja) | Może podać webox „złośliwy" output `devil`. | Wstrzyknięcie komend, kradzież danych przy parsowaniu. |
| **A5 — Skompromitowany serwer GitHub API** (hipotetyczny lub MITM po stronie GH proxy) | Może zwracać złośliwe response. | Wstrzyknięcie do scaffoldingu. |
| **A6 — Supply chain attacker** (kompromis releasy webox) | Podpisany inny artefakt. | Masowa kradzież. |

### 3.2 Macierz zagrożeń

| Zagrożenie | Aktor | Wektor | Mitygacja |
|---|---|---|---|
| **Kradzież `~/.config/webox/config.json`** | A1, A2 | Skopiowanie pliku. | Brak sekretów w pliku → atakujący zna **strukturę** projektów, ale nie ma tokenów ani haseł DB. Patrz [§4](#4-przechowywanie-sekret%C3%B3w). |
| **Kradzież keyringa** | A2 | Odblokowany Keychain. | Hard problem — jeśli sesja OS jest odblokowana, OS daje keyring. Mitygacja: keyring sam wymaga unlock po idle (`security set-keychain-settings -l`). Webox dodatkowo: minimal exposure (sekret pobierany tylko on-demand, nie cache'owany w pamięci dłużej niż 60 s). |
| **MITM przy pierwszym połączeniu SSH** | A3 | Active attacker podstawia swój klucz. | TOFU — pierwsza akceptacja zostawia user'a z odpowiedzialnością, ale **ostrzegamy explicite**, że to pierwsze połączenie. Patrz [§5.3](#53-pierwsze-po%C5%82%C4%85czenie-tofu). |
| **MITM przy kolejnym połączeniu (zmiana host key)** | A3 | Atak po przejęciu konta hostingowego. | **Strict block** — nigdy auto-accept, wymaga manualnej weryfikacji + edycji known_hosts. Patrz [§5.4](#54-zmiana-host-key). |
| **Złośliwy output `devil` (shell injection w outpucie)** | A4 | Server zwraca wartości z `\r\n` / cudzysłowami / ANSI escape. | Parsujemy w trybie strict: regex zamiast `eval`, output `devil` traktowany jako bajty, ścieżki walidowane przed użyciem. Patrz [§3.3](#33-defensywne-parsowanie-outputu). |
| **Command injection przez nazwę subdomeny** | wprowadzony przez user / A4 | Subdomena `"; rm -rf /; #` w UI. | Walidacja regex `^[a-z0-9-]{1,63}$` przed wstawieniem do komendy. SSH command builder używa whitelisting'u, nie escapowania. |
| **Złośliwa response GitHub API** | A5 | Manipulacja repo URL / treści workflow. | Walidacja schema (`org/name` regex, status code check), żadnych `eval` na response. |
| **Compromise binarki webox** | A6 | Atak na pipeline release. | Cosign signatures + SLSA provenance + reproducible builds (Go z `-trimpath` + `-buildvcs=true`). Patrz [§8](#8-reportowanie-podatno%C5%9Bci-i-supply-chain). |
| **Wyciek sekretu do logów** | bug developerski | Niedopilnowane formatowanie. | Wycinarka sekretów w warstwie loggingu — patrz [§9](#9-logging-i-wycieki). |
| **Reveal `.env` widoczny dla osoby obok** | shoulder surfing | User pokazuje ekran. | Maskowanie + confirm na reveal — patrz [UX.md §9](./UX.md#9-maskowanie-sekret%C3%B3w-w-ui). |
| **Wyciek tokena GitHub przez `git config`** | bug | Token wepchnięty do `.git/config` zamiast keyringa. | Zakaz w kodzie: helper `git-credential-webox` (post-MVP) zwraca z keyringa bez zapisu. W MVP `gh` CLI zarządza tokenem swoim mechanizmem. |

### 3.3 Defensywne parsowanie outputu

Każdy parser komendy serwera **musi**:

1. Strip ANSI escape sequences.
2. Walidować rozmiar (max 1 MB per komenda; powyżej — error).
3. Używać named regex groups (np. dla `devil www list`).
4. Nigdy nie wykonywać exec / eval na zawartości outputu.
5. Mieć golden test (patrz [TESTING.md §7](./TESTING.md#7-test-fixtures)) z poprawnymi i złośliwymi fixture'ami.

## 4. Przechowywanie sekretów

### 4.1 Domyślnie — systemowy keyring

Biblioteka: `github.com/zalando/go-keyring`. Mapa kluczy → [DESIGN.md §7](./DESIGN.md#7-sekrety). Webox **nigdy** nie zapisuje sekretów do pliku tekstowego niezaszyfrowanego.

| Platforma | Backend keyringa |
|---|---|
| macOS | Keychain |
| Linux (z D-Bus i Secret Service) | gnome-keyring / KWallet / KeePassXC z secret-service |
| Windows | Credential Manager |
| FreeBSD | brak natywnego — wymaga fallback |

### 4.2 Fallback dla środowisk headless

> **Poprawka 7.10.** W Linux server / Docker / WSL bez D-Bus / FreeBSD keyring nie zadziała. Bez fallback'u webox tam nie działa wcale.

**Decyzja:** wspieramy plik szyfrowany `~/.config/webox/secrets.enc`.

| Parametr | Wartość |
|---|---|
| Algorytm szyfrowania | AES-GCM-256 |
| KDF | Argon2id (parametry: `memory=64MB`, `iterations=3`, `parallelism=2`, sól 16 B losowa per plik) |
| Master password | wpisywany przy starcie webox (lub przekazany przez `WEBOX_MASTER_PASSWORD` env w trybie CI) |
| Cache hasła w sesji | tak, w pamięci procesu; nie persystowane |
| Format pliku | NDJSON encoded, każda linia: `{"key": "...", "ciphertext": "...", "nonce": "..."}` |
| Backup | webox tworzy `.bak` przed każdym zapisem |

**Tryb degraded** wskazywany w UI:

```
╭─ Webox ─────────────────────── ⚠ DEGRADED MODE ───╮
│  Keyring unavailable. Using encrypted file fallback.│
│  Run `webox doctor security` for details.           │
╰─────────────────────────────────────────────────────╯
```

**Ograniczenia trybu fallback** (jasno komunikowane):

- Nie zalecane dla produkcyjnych credentiali na publicznym serwerze.
- Hasło master jest tak silne, jak jest silne — webox wymusza minimum 12 znaków.
- Brute force pliku wymaga dostępu do `secrets.enc`. Permisy `0600`.
- **Brak recovery hasła master.** Jeśli user je zgubi, fallback secrets trzeba odtworzyć z ich źródeł pierwotnych. To celowa decyzja: recovery channel byłby backdoorem.

**Detekcja środowiska** (kolejność prób, jednoznaczne mapowanie błędów):

1. **Probe write+read+delete** sentinela: webox próbuje `keyring.Set("webox-probe", "sentinel", token)`, następnie `keyring.Get`, na końcu `keyring.Delete`. Token to losowych 16 B w hex.
2. Klasyfikacja błędu (sentinel errors z [`go-keyring`](https://pkg.go.dev/github.com/zalando/go-keyring)):

   | Błąd | Decyzja |
   |---|---|
   | `nil` na wszystkich trzech operacjach | keyring **działa** → `fallback=false` |
   | `errors.Is(err, keyring.ErrUnsupportedPlatform)` | platforma bez wsparcia → `fallback=true` |
   | `errors.Is(err, keyring.ErrSetDataTooBig)` na probe Set | konfiguracja problematyczna, ale keyring działa → `fallback=false` + warning |
   | Inny błąd (D-Bus connection error / Secret Service unavailable / timeout) | log warning + `fallback=true` |
   | `keyring.ErrNotFound` na probe Get **po** udanym Set | błąd implementacji backendu → log warning + `fallback=true` |
3. **`keyring.ErrNotFound` przy normalnym pobraniu zapisanego wcześniej sekretu** to **nie** sygnał o braku keyringa, tylko o braku sekretu w keyringu — webox prosi user'a o ponowne wprowadzenie wartości. Probe write+read+delete jest osobnym mechanizmem detekcji.
4. Wynik detekcji cache'owany na czas sesji. Można wymusić tryb przez `WEBOX_SECRETS_BACKEND=keyring|fallback`.
5. Przy pierwszym `Get` w trybie fallback webox prosi o master password.
6. Jeśli `secrets.enc` nie istnieje → webox zakłada nowy z user-provided hasłem (z confirm).

### 4.3 Cykl życia sekretu w pamięci

- Sekret pobierany na żądanie (np. tuż przed `ssh.Dial`).
- Nie cache'owany dłużej niż jedna operacja.
- Po użyciu zmienna `password` jest zerowana (`zerocopy.Wipe(buf)`).
- Brak dump'u stack trace z sekretem — patrz [§9](#9-logging-i-wycieki).

### 4.4 Rotacja sekretów

| Sekret | Rotacja | Trigger |
|---|---|---|
| `webox-gh-token` | Ręczna w `/settings → Rotate` (post-MVP: per-90-dni przypomnienie) | User. |
| `webox-db-...` | Tylko podczas re-create DB (rzadko). | Wizard. |
| `webox-fallback-master` | Komenda `webox doctor security --rotate-master`. | User. |

## 5. Host keys i SSH

### 5.1 Lokalizacja `known_hosts`

**Decyzja:** webox używa **dedykowanego pliku** `~/.config/webox/known_hosts`, **nie** `~/.ssh/known_hosts`.

**Uzasadnienie:**
- Izolacja od ogólnych SSH użytkownika (jeśli atakujący zmodyfikuje webox known_hosts, nie wpływa to na `ssh user@host` ręczne).
- Łatwiejsze czyszczenie przy debug.
- User może zaimportować z `~/.ssh/known_hosts` jednorazowo (`webox doctor security --import-known-hosts`).

### 5.2 Algorytm weryfikacji

1. Próba SSH connect → callback `HostKeyCallback`.
2. Webox czyta `~/.config/webox/known_hosts`.
3. Sprawdza fingerprint vs zapisany.
4. Decyzja:
   - **Match** → continue.
   - **Brak wpisu** → TOFU (§5.3).
   - **Mismatch** → strict block (§5.4).

### 5.3 Pierwsze połączenie (TOFU)

UI:

```
╭─ First connection to s1.small.pl ─────────────────────────────╮
│                                                                │
│  ! No host key on record for s1.small.pl:22.                  │
│                                                                │
│  Fingerprint (SHA256):                                         │
│  4d:1d:e8:0a:...:c2:9f                                         │
│  Type: ssh-ed25519                                             │
│                                                                │
│  Verify out-of-band:                                           │
│  • Check small.pl panel → Server settings → SSH fingerprints  │
│  • Or run: `ssh-keyscan -t ed25519 s1.small.pl`               │
│                                                                │
│  [ Accept and save ]    [ Reject ]                             │
╰────────────────────────────────────────────────────────────────╯
```

Akceptacja zapisuje wpis do `~/.config/webox/known_hosts` (format kompatybilny z OpenSSH).

### 5.4 Zmiana host key

```
╭─ ⚠ HOST KEY CHANGED FOR s1.small.pl ──────────────────────────╮
│                                                                │
│  Stored fingerprint:                                           │
│    4d:1d:e8:0a:...:c2:9f (type: ssh-ed25519)                   │
│  Received fingerprint:                                         │
│    f0:32:7a:51:...:b4:e0 (type: ssh-ed25519)                   │
│                                                                │
│  POSSIBLE CAUSES:                                              │
│  • Server reinstalled / migrated.                              │
│  • Man-in-the-middle attack.                                   │
│                                                                │
│  Webox will NOT auto-accept. Verify out-of-band, then:        │
│                                                                │
│  webox doctor security --update-host-key s1.small.pl          │
│                                                                │
│  [ Abort connection ]                                          │
╰────────────────────────────────────────────────────────────────╯
```

`--update-host-key` jest świadomym, manualnym krokiem; wymaga wpisania `confirm` w trybie interaktywnym lub flagi `--force` w skrypcie CI.

### 5.5 Algorytmy

| Akcja | Zalecane | Akceptowane | Odrzucone |
|---|---|---|---|
| Host key | ed25519 | rsa-sha2-512, rsa-sha2-256, ecdsa-sha2-nistp256 | ssh-rsa (SHA-1), ssh-dss |
| User key | ed25519 | rsa-sha2-512 (≥3072 bit) | ssh-rsa (SHA-1), ssh-dss, rsa <2048 |
| KEX | curve25519-sha256 | ecdh-sha2-nistp256/384 | diffie-hellman-group1-sha1 |
| Cipher | chacha20-poly1305 | aes256-gcm, aes128-gcm | arcfour, 3des-cbc |

Webox eksplicitnie deklaruje listę dozwolonych algorytmów w `ssh.ClientConfig`.

## 6. GitHub Token

### 6.1 Wymagane scope (fine-grained PAT — zalecane)

| Scope | Cel | Wymagane? |
|---|---|---|
| `Contents: Read and write` | Tworzenie i edycja plików w repo (workflow). | Tak. |
| `Workflows: Read and write` | Pisanie `.github/workflows/deploy.yml`. | Tak. |
| `Actions: Read` | Monitoring runów. | Tak. |
| `Secrets: Read and write` | Zapis SSH key + DEPLOY_* do secrets repo. | Tak. |
| `Metadata: Read` | Default required. | Tak. |
| `Administration: Read and write` | Tworzenie repo. | Tak (per organization scope). |

### 6.2 Classic PAT

Akceptowany, ale `repo` + `workflow` to bardzo szerokie uprawnienia. Webox loguje warning przy starcie + w `webox doctor security`.

### 6.3 OAuth Device Flow (zalecany w v0.2+)

W MVP: `gh` CLI lub PAT. W v0.2+: `webox auth login github` uruchamia OAuth Device Flow z webox jako App registration (skonfigurowana na GitHub Apps Marketplace). Token kończy w keyring.

### 6.4 Walidacja tokena

`webox doctor security` wykonuje:

- `GET /user` z tokenem → walidacja działania.
- `GET /repos/:owner/:repo` z każdego skonfigurowanego repo → walidacja dostępu.
- `GET /user/installations` (dla fine-grained) → wykrycie wygaśnięcia.

## 7. Audyt sekretów i tryb doctor

`webox doctor security` (komenda post-MVP — design w MVP, implementacja v0.2):

| Check | Status OK | Status WARN | Status FAIL |
|---|---|---|---|
| Keyring dostępny | tak | n/a | nie (przejdź na fallback) |
| Permisje pliku configu | `0600` | `0644` (group readable) | `0666` (world writable) |
| Permisje `~/.ssh/id_ed25519_webox` | `0600` | inny niż `0600` | brak pliku |
| Permisje `~/.config/webox/known_hosts` | `0600` | inny | brak |
| Permisje `secrets.enc` (jeśli fallback) | `0600` | inny | n/a |
| GitHub token ważny | 200 OK | 401 (wygasł) | brak tokena |
| Zbędne sekrety w keyring (orphan) | brak | są (lista) | n/a |
| Wersja webox | `latest` | starsza minor | starsza major |
| Pending cleanups | brak | są | n/a |

Output `--json` dostępny dla skryptów.

## 8. Reportowanie podatności i supply chain

### 8.1 Reportowanie podatności

W stylu GitHub Security Advisories:

- **Gdzie zgłaszać:** prywatny GitHub Security Advisory na repo (preferowane). Jeśli repo jeszcze nie ma skonfigurowanego kanału advisory, zgłoszenie powinno trafić prywatnie do maintainera poza publicznymi issues/discussions. Dedykowany adres security zostanie dodany przed publicznym launch.
- **Czas odpowiedzi (SLA):** 72 h pierwsza odpowiedź, 30 dni do publicznego disclosure (lub krócej jeśli aktywnie eksploatowane).
- **Bug bounty:** brak w MVP. Honor wall w `SECURITY.md` po pierwszym kwartale po release.
- **PGP key:** maintainer key opublikowany w repo `SECURITY.md` (post-MVP).

Zasady dla zgłaszającego:

1. Nie publikuj problemu publicznie przed koordynacją.
2. Dostarcz reproducible PoC (najlepiej self-contained).
3. Akceptujemy zgłoszenia w EN i PL.

### 8.2 Supply chain — release pipeline

| Mechanizm | Status | Cel |
|---|---|---|
| GoReleaser z `-trimpath` | obowiązkowy od v0.1 | Reproducible paths. |
| `-buildvcs=true` | obowiązkowy | Stempel git commit w binarce. |
| SHA256 checksum file (`webox_v0.1.0_checksums.txt`) | obowiązkowy | Walidacja artefaktu. |
| **Cosign sygnatury** (keyless OIDC GitHub Actions) | obowiązkowy od v0.1 | `cosign verify` możliwa przez konsumentów. |
| **SLSA provenance** | obowiązkowy od v0.1 | Audit trail kto/co/jak. |
| Reproducible builds | best-effort | `goreleaser release --snapshot` + dwa runy = identyczne hashe. |
| SBOM (CycloneDX) | obowiązkowy od v0.2 | Lista zależności w czytelnej formie. |
| Dependency scanning (govulncheck) | obowiązkowy w CI | Brak znanych CVE w pipeline. |
| Pinowane wersje GH Actions w workflow (SHA, nie tag) | obowiązkowe | Ochrona przed compromise tagów. |

### 8.3 Polityka dependencji

- Tylko biblioteki z aktywnym maintain (commit < 12 miesięcy).
- Brak biblioteki z 0 testów lub 1 commit w historii.
- Każda nowa zależność wymaga uzasadnienia w PR description.
- `go mod tidy && govulncheck ./...` jest checkiem w CI.

## 9. Logging i wycieki

### 9.1 Co loggujemy lokalnie (opt-in, patrz [DESIGN.md §15](./DESIGN.md#15-telemetria-i-logi-diagnostyczne))

| Typ | Loggowane? | Forma |
|---|---|---|
| Domena projektu | Tak | plaintext |
| Ścieżki plików | Tak | plaintext |
| Komendy SSH przed wysłaniem | Tak | plaintext (sanityzowane: bez tokenów w env vars) |
| Output SSH | Tak | plaintext, ale truncated do 16 KB |
| Sekret z keyringa | **Nigdy** | redagowane jako `***` |
| Token GitHub | **Nigdy** | redagowane jako `ghp_***` |
| Wartości `.env` | **Nigdy** | redagowane |
| Stack trace przy crash | Tak | bez wartości lokalnych zmiennych zawierających `password`/`token`/`key` |

### 9.2 Redaktor sekretów

Warstwa loggingu używa filtra: regex match na typowe wzorce (`ghp_[A-Za-z0-9]{36}`, `sk-[A-Za-z0-9]{20,}`, `ssh-rsa AAAA...`, `BEGIN PRIVATE KEY`) → zamiana na `[REDACTED]`. Lista wzorców rozszerzalna w `secrets/redactor.go`.

### 9.3 Schowek systemowy

`Ctrl+Y` w `/env` (kopiowanie wartości) — sekret idzie do clipboard. Webox **ostrzega**: clipboard managers mogą zachować historię. Po 30 s webox próbuje wyczyścić clipboard (best-effort — niektóre OS nie pozwalają).

### 9.4 Core dumps

Webox **nie** włącza core dumpów. Goroutyna główna ma `defer recover()` z handlerem crashu zapisującym sanitized stack trace (patrz [DESIGN.md §15.3](./DESIGN.md#153-crash-reports)).

## 10. Zarządzanie `.env` i sekrety aplikacji

### 10.1 Filozofia: Webox orchestruje, ale nie udaje pełnego vaulta

Webox **nie jest vaultem klasy 1Password / Vault / Doppler** dla sekretów aplikacji. Jednocześnie nie może udawać, że problem nie istnieje, bo pracuje z realnymi `DB_PASSWORD`, `API_KEY` i podobnymi wartościami.

W praktyce Webox operuje na trzech warstwach:

| Warstwa | Rola | Czy Webox może odczytać wartość? |
|---|---|---|
| **Lokalny secure store** (`keyring` / `secrets.enc`) | Miejsce na wartości, które user świadomie zarządza przez Webox. | Tak. |
| **GitHub Secrets** | Write-only target dla pipeline CI/CD. | **Nie** — GitHub nie ujawnia plaintextu po zapisie. |
| **Serwerowy `.env`** | Runtime representation na hoście. | Tak, przez SSH/SFTP jeśli user ma dostęp. |

Z tego wynikają dwie twarde reguły:

1. **GitHub Secrets nie są canonical vaultem**, bo ich wartości nie da się pobrać z API po zapisie.
2. **Jeśli Webox ma umieć zsynchronizować wartość "teraz, bez redeployu", musi mieć tę wartość lokalnie w secure store albo dostać ją od usera w bieżącej sesji.**

Tryby życia sekretu aplikacji:

- **managed** — wartość została wprowadzona przez Webox i jest obecna lokalnie w secure store; Webox może ją wypchnąć do GH Secrets i/lub na serwer.
- **server-only** — wartość istnieje tylko w `.env` na serwerze (np. import legacy projektu); Webox zna ją dopiero po odczycie z serwera i nie kopiuje jej lokalnie bez świadomej akcji usera.
- **external** — klucz jest znany, ale wartość żyje poza Webox (np. ustawiona ręcznie w GitHub UI / innym narzędziu). Webox może znać nazwę i metadane, ale nie plaintext.

### 10.2 Konwencja `.env.example`

Każde repo zarządzane przez Webox **musi** mieć zacommitowany plik `.env.example` w katalogu głównym. Zasady:

| Zasada | Uzasadnienie |
|---|---|
| `.env.example` zawiera wszystkie klucze, wartości puste lub opisowe placeholdery (`DB_PASSWORD=<your-password-here>`) | Dokumentacja kontraktu środowiskowego dla nowych developerów i audytu. |
| `.env` jest zawsze w `.gitignore` | Zapobiega przypadkowemu commitowi wartości. |
| `.env.example` nie zawiera żadnych realnych sekretów | Historyczne commity nie mogą nieść realnych wartości. |
| Nowy sekret = najpierw nazwa w `.env.example`, potem wartość przez Webox lub inny świadomy kanał | Najpierw definiujemy kontrakt kluczy, dopiero potem dystrybucję wartości. |

Przy deploy Webox **waliduje kompletność kluczy**, nie wartości:

- diff `.env.example` vs nazwy sekretów dostępnych w repo GitHub,
- diff `.env.example` vs klucze obecne w runtime `.env` na serwerze.

Jeśli brakuje kluczy wymaganych przez `.env.example` → deploy jest blokowany.  
Jeśli są klucze nadmiarowe → warning, ale nie fail.  
**Wartości nie są porównywane z GitHub Secrets**, bo GitHub nie udostępnia plaintextu po zapisie.

### 10.3 GitHub Secrets jako kanał deployu, nie źródło odczytu

Sekrety aplikacji dla projektów CI-managed trafiają do GitHub Secrets, ale trzeba to rozumieć precyzyjnie:

- GitHub Secrets to **publish target** dla workflow,
- Webox może je **ustawić lub nadpisać**,
- Webox może pobrać **nazwy i metadane** sekretów,
- Webox **nie może** pobrać z GitHub plaintext wartości po zapisie.

Przepływ dla projektu zarządzanego przez GitHub Actions:

```text
Developer
  │  webox [Add/Rotate secret]
  ▼
Local secure store (managed secret)
  │
  ├── push --> GitHub Secrets API (write-only)
  ▼
GitHub Actions deploy.yml
  │  materializes .env during deploy
  ▼
Server ~/.env
```

Konsekwencje:

- **Natychmiastowy sync `.env` przez SFTP** jest możliwy tylko wtedy, gdy Webox ma wartość lokalnie (`managed`) albo user poda ją ponownie w tej sesji.
- **GH-only value is unrecoverable by design** — jeśli ktoś ustawił sekret tylko w GitHub UI i nigdzie indziej go nie zachował, Webox nie odtworzy go z API.
- **Server `.env` nie jest gwarantowanie odtwarzalny z samego GH** bez dodatkowego lokalnego źródła lub ponownego wejścia wartości.
- **Klucz deployu jest projektowy, nie operatorski** — `SSH_PRIVATE_KEY` w repo secrets pochodzi z osobnej pary kluczy wygenerowanej per projekt.

Sekret `DEPLOY_PATH` używany w `deploy.yml` = wynik `provider.GetDeployPath(domain)` zapisany do repo secrets przez wizard w `stateDeploying`.

### 10.4 Lokalizacja i permisje `.env` na serwerze

| Parametr | Wartość | Weryfikacja przez Webox |
|---|---|---|
| Ścieżka | `<deploy_path>/../.env` — jeden poziom **powyżej** katalogu `dist/` / `public/` | Tak — `ls -la` przez SSH po deploy, porównanie z `provider.GetDeployPath()`. |
| Permisje | `0600` (właściciel: SSH user) | Tak — `stat --format="%a" .env` lub `ls -la`. Warn jeśli `>0600`. Fail deploy jeśli `0644` lub `0666`. |
| Właściciel | SSH user | Tak — `stat --format="%U" .env`. |
| `.env` nigdy w `public/`, `public_html/`, `www/`, `htdocs/` | Dostęp przez HTTP = pełna kompromitacja. | Tak — Webox sprawdza ścieżkę i rzuca `ErrEnvExposedInPublic` jeśli `.env` wylądowałby w web root. |

Wykrywanie web root per provider:

| Provider | Web root |
|---|---|
| `smallhost` | `~/domains/<domain>/public_html/` |
| `cpanel` (post-MVP) | `~/public_html/<subdomain>/` |
| `directadmin` (post-MVP) | `~/domains/<domain>/public_html/` |

Jeśli `GetDeployPath` zwraca ścieżkę **wewnątrz** web root, Webox umieszcza `.env` obok aplikacji, ale poza publicznym katalogiem.

### 10.5 Lokalny `.env` dewelopera

Lokalny deweloper **nie powinien mieć produkcyjnego `.env` w katalogu repo**. Podejście Webox:

**MVP (v0.1):**

1. Developer trzyma lokalne pliki środowiskowe poza repo, np. `~/.config/webox/local-env/<project-id>.env`.
2. Do ładowania używa `direnv`, `dotenvx` albo własnego workflow.
3. Webox w przyszłym `/env` pokazuje:
   - klucze z `.env.example`,
   - czy klucz istnieje w runtime `.env` na serwerze,
   - czy klucz istnieje w repo secrets GitHuba,
   - czy wartość jest `managed` lokalnie przez Webox.

**Post-MVP (v0.2+):**

| Komenda | Działanie |
|---|---|
| `webox env pull` | Pobiera `.env` z serwera do katalogu lokalnego poza repo. |
| `webox env push` | Wypycha **lokalnie dostępne** wartości do runtime `.env` i/lub do GH Secrets. |
| `webox env edit` | Otwiera lokalny plik środowiskowy w `$EDITOR`. |
| `webox env promote` | Kopiuje wybrane klucze z `server-only` do `managed` secure store. |
| `webox env diff` | Diffuje klucze i wartości między lokalnym plikiem a serwerem. |

### 10.6 Rotacja sekretów aplikacji — metadane i warningi

Webox śledzi **metadane** sekretów w `config.json`, nigdy same wartości.

#### Schemat `projects[].secrets_meta[]`

| Pole | Typ | Opis |
|---|---|---|
| `key` | string | Nazwa klucza środowiskowego, np. `DB_PASSWORD`. |
| `created_at` | RFC 3339 | Kiedy klucz pojawił się w projekcie. |
| `last_rotated` | RFC 3339 | Kiedy wartość była ostatnio zmieniona przez Webox lub świadomie potwierdzona. |
| `source` | enum: `managed` / `server_only` / `external` | Skąd Webox realnie zna wartość lub jej nie zna. |
| `last_synced_github` | RFC 3339 | Kiedy wartość była ostatnio wypchnięta do repo secrets. |
| `last_synced_server` | RFC 3339 | Kiedy wartość była ostatnio wypchnięta do runtime `.env`. |
| `rotation_reminder_days` | int (default 90) | Po ilu dniach dashboard ostrzega o rotacji. |

> `secrets_meta` to tylko stan wiedzy i synchronizacji. Plaintext żyje wyłącznie w secure store, aktualnej sesji użytkownika albo na serwerze.

#### Warningi w dashboardzie

| Próg | Komunikat | Ikona |
|---|---|---|
| `last_rotated` > `rotation_reminder_days` | `Secret DB_PASSWORD not rotated in 94 days.` | ⚠ żółta |
| `last_rotated` > `rotation_reminder_days × 2` | `Secret DB_PASSWORD overdue for rotation (180 days).` | 🔴 czerwona |
| `source = server_only` | `Secret API_KEY exists only on server. Promote to managed for safer sync.` | ℹ szara |
| `source = external` | `Secret STRIPE_KEY is managed outside Webox. Value drift cannot be verified.` | ℹ szara |

Przykładowy output `webox doctor security`:

```text
SECRET            SOURCE       LAST_ROTATED   GH_SYNC        SERVER_SYNC    STATUS
DB_PASSWORD       managed      2025-12-01     2026-05-01     2026-05-01     ⚠ 172 days ago
STRIPE_KEY        external     2026-04-15     2026-04-15     (unknown)       ℹ external
OLD_TOKEN         server_only  (unknown)      (never)        2026-05-12      ℹ promote recommended
```

### 10.7 Edge case: small.pl / Devil bez natywnego secret storage

**Problem:** Devil nie ma natywnego `config:set`. Runtime aplikacji opiera się o plik `.env`.

**Decyzja:** Webox obsługuje dwa tryby:

| Tryb | Trigger | Jak powstaje `.env` |
|---|---|---|
| **GH Actions** (domyślny) | Projekt ma repo + `deploy.yml` | Actions job materializuje `.env` z repo secrets podczas deployu. |
| **Direct SFTP** (fallback) | Projekt bez repo, stack statyczny lub legacy import | Webox składa `.env` z lokalnie dostępnych wartości (`managed`) i/lub aktualnej kopii serwerowej (`server-only`), a brakujące pola pyta usera. |

W trybie Direct SFTP:

1. Webox pobiera aktualny `.env` z serwera, jeśli istnieje.
2. Nakłada na niego wartości lokalnie `managed`, które user chce zsynchronizować.
3. Jeśli wymagany klucz nie istnieje ani lokalnie, ani na serwerze, prosi usera o wartość.
4. Kompiluje wynik w pamięci procesu (`bytes.Buffer`) — bez zapisu plaintextu na lokalny dysk.
5. Wysyła `.env` przez SFTP do `<deploy_path>/../.env`, ustawia `0600`, weryfikuje `stat`, zeruje bufor.

**Ograniczenie:** Devil nie przeładowuje środowiska bez restartu aplikacji. Po zmianie `.env` w trybie Direct SFTP Webox **zawsze** wykonuje `RestartNodeApp`.
