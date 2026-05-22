# ADR-0004: Przechowywanie sekretów w systemowym keyringu

> Status: Accepted · Data: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne ADR: [ADR-0001 TUI](./0001-tui-zamiast-cli.md). Dokumenty: [SECURITY §4](../SECURITY.md#4-przechowywanie-sekret%C3%B3w), [DESIGN §7](../DESIGN.md#7-sekrety).

## Kontekst

Webox musi trzymać kilka **sekretów narzędziowych** per user (sekrety potrzebne webox do działania, nie sekrety aplikacji Node):

- GitHub Personal Access Token (`repo`, `workflow`, `secrets:write`).
- Hasło do bazy danych każdego projektu z DB — **tylko w celu wyświetlenia credentiali userowi** i ewentualnych health checków przez webox. To nie to samo co `DB_PASSWORD` w GitHub Secrets / `.env` na serwerze (patrz [SECURITY.md §10](../SECURITY.md#10-zarz%C4%85dzanie-env-i-sekrety-aplikacji)).
- (Post-MVP) Token API panelu (DirectAdmin, CyberPanel).

Klucz prywatny SSH leży w `~/.ssh/` zarządzany przez OS — webox nie dotyka.

> **Zakres ADR:** ten ADR dotyczy **sekretów narzędziowych webox** (gh-token, db-credential do wyświetlenia, master fallback). Sekrety aplikacji (zmienne środowiskowe idące do `.env` na serwerze) są osobną warstwą: GitHub Secrets pełnią tam rolę write-only targetu dla CI, a nie odczytywalnego source of truth. Patrz [SECURITY.md §10](../SECURITY.md#10-zarz%C4%85dzanie-env-i-sekrety-aplikacji).

Możliwe miejsca przechowywania sekretów:

1. **Plaintext w `config.json`** — najprostsze, najgorsze.
2. **Plaintext w osobnym pliku `secrets.json` z `0600`** — lepiej, ale wciąż czytelne dla każdego procesu usera.
3. **Systemowy keyring** (Keychain / Secret Service / Credential Manager).
4. **Plik szyfrowany AES-GCM z master password** (jak `pass`, `age`).
5. **HSM / hardware key** (YubiKey, sekretarka SE w macOS).
6. **Cloud secret manager** (1Password CLI, Bitwarden CLI, Doppler).

## Decyzja

**Domyślnie:** **systemowy keyring** przez `github.com/zalando/go-keyring`.

**Fallback (gdy keyring niedostępny):** plik szyfrowany **AES-GCM-256** z master password derived przez **Argon2id**, lokalizacja `~/.config/webox/secrets.enc`.

**Nigdy:** plaintext w żadnym pliku w `~/.config/webox/`.

Klucze w keyring używają prefixu `webox-` (`webox-gh-token`, `webox-db-<profile>-<project>`, `webox-fallback-master`).

## Dlaczego keyring + fallback, a nie tylko jedno

### Tylko keyring — odrzucone

Keyring nie istnieje w środowiskach:

- Linux serwer headless bez D-Bus / Secret Service (np. CI, Docker container).
- WSL bez konfiguracji `keyring-with-dbus` (większość WSL2 default).
- FreeBSD bez Secret Service.

Webox bez fallback'u byłby tam niedostępny — co wycina realną grupę userów (Marek z agencji testujący na maszynie deweloperskiej Linux z minimalistycznym DE).

### Tylko szyfrowany plik — odrzucone

Plik szyfrowany wymaga:

- User wpisuje master password przy każdym starcie webox (gorsze UX niż keyring).
- Master password sam wymaga miejsca trzymania — koło Möbiusa.
- Brak integracji z OS (Touch ID na macOS, biometria) — keyring tu wygrywa naturalnie.

### Cloud secret manager (1Password / Bitwarden CLI) — odrzucone w MVP

- Wymaga, by user już go miał (założenie ryzykowne).
- Integracja w pierwszej wersji = dodatkowy depend ABI (zewnętrzne CLI, którego wersjonowania nie kontrolujemy).
- W v0.3+ rozważymy `webox secrets backend=1password` jako opcję.

### HSM / YubiKey — odrzucone

- Niska adopcja w docelowej grupie (freelancer + mała agencja).
- Wymaga FIDO2 setup'u poza scope'em narzędzia operatorskiego.
- Macroescape do biometric (TouchID) jest częścią keyringa macOS automatycznie.

## Parametry kryptograficzne (fallback)

| Parametr | Wartość |
|---|---|
| Cipher | AES-GCM-256 |
| KDF | Argon2id |
| `memory` | 64 MB |
| `iterations` | 3 |
| `parallelism` | 2 |
| `saltLen` | 16 bajtów (losowa per plik) |
| Master password minimum length | 12 znaków |
| Pamięć cache hasła w sesji | tak (in-memory, nie persystowane) |
| Pamięć wyzerowana po użyciu | tak, przez `awnumar/memguard.LockedBuffer.Destroy()` + unikanie konwersji sekretów do `string` |

Te parametry są **konserwatywne** — Argon2id z 64 MB memory + 3 iterations daje ~250 ms derivation czasu na nowoczesnym CPU, co skutecznie utrudnia brute force offline.

Uczciwe ograniczenie: w Go nie da się zagwarantować absolutnego wymazania każdego bajtu, jeśli sekret wcześniej przeszedł przez heap jako `string`/`[]byte`. `memguard` ogranicza ryzyko przez locked buffer i explicit destroy, ale polityka implementacyjna pozostaje ważniejsza: sekret pobieramy on-demand, trzymamy krótko, nie logujemy, nie formatujemy i nie kopiujemy bez potrzeby.

## Konsekwencje

### Pozytywne

- **W większości środowisk (macOS, Linux z DE, Windows) zero friction** — webox prosi OS o sekret, OS odblokowuje (lub pyta TouchID).
- **Brak plaintextu** w plikach configu.
- **Standard branżowy** — IDE, `git`, `gh` używają tej samej infrastruktury.
- **Fallback szyfrowany** rozszerza zasięg na headless środowiska bez kompromisu na bezpieczeństwo.
- **Audyt** przez `webox doctor security` — patrz [SECURITY §7](../SECURITY.md#7-audyt-sekret%C3%B3w-i-tryb-doctor).

### Negatywne

- **Keyring na Linuksie jest fragmentaryczny** — gnome-keyring, KWallet, KeePassXC z secret-service. Webox wykrywa go-keyring API, ale problemy specyficzne per implementacja mogą się zdarzyć.
- **Fallback wymaga master password** — gdy user się pomyli, webox nie ma jak odzyskać. **Brak recovery** — to świadoma decyzja (recovery = backdoor).
- **Plik fallback może wyciec** (np. backup do chmury) — szyfrowanie chroni, ale jeśli master jest słaby (12 znaków `password1234!`), brute force jest możliwy.
- **Migracja keyring ↔ fallback** wymaga `webox doctor security --migrate-to-fallback` / `--migrate-to-keyring`. Niezbyt wygodne, ale rzadkie.

### Neutralne

- Każda nowa instalacja webox na nowym hoście wymaga **re-input** sekretów. Mitygacja: w v0.2+ `/settings → Export config (without secrets)` daje stencyl, ale tokeny user musi wprowadzić ponownie. To akceptowalne.
- Hasło DB ma **podwójne życie**: `webox-db-<profile>-<project>` w keyring (dla webox UI) i `DB_PASSWORD` w GitHub Secrets (dla Actions → `.env`). To celowe — dwie warstwy mają różne przeznaczenie. Muszą być synchronizowane przy rotacji (webox `[Rotate DB password]` aktualizuje oba).

## Środowiska — macierz wsparcia

| Środowisko | Keyring działa? | Decyzja webox |
|---|---|---|
| macOS (Apple Silicon + Intel) | ✓ Keychain | keyring |
| Linux desktop (GNOME / KDE / XFCE z DE) | ✓ Secret Service | keyring |
| Linux desktop bez DE (i3, sway bez setup'u) | częściowo | keyring jeśli D-Bus + Secret Service, inaczej fallback |
| Linux serwer headless / Docker container | ✗ | fallback |
| WSL2 default | częściowo | fallback (zwykle) |
| WSL2 z `keyring-with-dbus` setup | ✓ | keyring |
| Windows (natywny) | ✓ Credential Manager | keyring |
| FreeBSD | ✗ | fallback |
| OpenBSD | ✗ | fallback |

Decyzja detekcji jest **per-start**, cache'owana per sesja, można wymusić `WEBOX_SECRETS_BACKEND=fallback` (na potrzeby testów / specjalnych przypadków).

## Alternatywy rozważane

Wszystkie powyżej w sekcji "Dlaczego". Decyzja keyring + AES-GCM fallback jest balansem między UX, security i zasięgiem platform.
