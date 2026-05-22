# Provider: cyberpanel

> Status: **RESEARCH / PLANNED POST-MVP (v0.3+)** · Ostatnia aktualizacja: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [providers/smallhost.md](./smallhost.md) (wzorzec), [CONTRIBUTING §3](../CONTRIBUTING.md#3-jak-doda%C4%87-nowy-provider).

## STATUS

**RESEARCH — implementacja zaplanowana na v0.3 lub później**, w zależności od dojrzałości oficjalnego API. CyberPanel ma najwięcej luk w dokumentacji spośród czterech rozważanych providerów. Ten dokument zawiera **wyłącznie notatki badawcze**. Wszystkie hipotezy `[TO BE VERIFIED]`.

> Oryginalny PRD zawierał implementację `cyberpanel.go` z command'ami w stylu `python3 /usr/local/CyberPanel/createDomain.py ...` napisanymi z głowy — niezweryfikowane z dokumentacją vendora. Usunięte. Patrz [CHANGES.md poprawka 6.9](../../CHANGES.md#1-poprawki-merytoryczne-z-tabeli-%C2%A76-briefu).

## TL;DR

CyberPanel to open-source panel hostingowy oparty o OpenLiteSpeed (LiteSpeed Web Server community edition). Komunikacja przez **CLI `cyberpanel`** (dla większości operacji) i **HTTP API** (mniej dokumentowane, czasem niekompletne). Specyfika: OpenLiteSpeed restart przez `/usr/local/lsws/bin/lshttpd -r`. Webox będzie używał CLI z SSH, fallback na API gdy CLI brakuje danej funkcjonalności.

## Spis treści

1. [Źródła i oficjalna dokumentacja](#1-%C5%BAr%C3%B3d%C5%82a-i-oficjalna-dokumentacja)
2. [Charakterystyka panelu](#2-charakterystyka-panelu)
3. [Otwarte pytania przed implementacją](#3-otwarte-pytania-przed-implementacj%C4%85)
4. [Wstępne mapowanie metod interfejsu](#4-wst%C4%99pne-mapowanie-metod-interfejsu)
5. [Specyficzne properties](#5-specyficzne-properties)
6. [Ścieżki plików — hipoteza](#6-%C5%9Bcie%C5%BCki-plik%C3%B3w--hipoteza)
7. [Edge cases znane z dokumentacji i community](#7-edge-cases-znane-z-dokumentacji-i-community)
8. [Plan testów przed implementacją](#8-plan-test%C3%B3w-przed-implementacj%C4%85)

---

## 1. Źródła i oficjalna dokumentacja

| Zasób | Link | Uwagi |
|---|---|---|
| CyberPanel CLI Reference | https://community.cyberpanel.net/t/cyberpanel-command-line-interface/30683 | Najbardziej kompletna referencja CLI. |
| CyberPanel Docs (DokuWiki) | https://docs.cyberpanel.net/doku.php?id=cli-create-website | Per-command dokumentacja CLI. |
| Community API discussion | https://community.cyberpanel.net/t/i-have-some-questions-about-the-api-looking-forward-to-help/18224 | Potwierdza, że API nie jest w pełni udokumentowane. |
| Community library (PHP) — BurakBoz/CyberLink | https://github.com/BurakBoz/CyberLink | Reverse-engineered library. Daje wgląd jak community workaround'uje braki dokumentacji. |
| CyberPanel API Plugin / MCP | https://github.com/elwizard33/cyberpanel-mcp | Współczesny wrapper. |

> **Ważne:** community konsekwentnie zauważa, że "CyberPanel doesn't have API functions fully described and implemented" — dlatego priorytetem dla webox będzie **CLI**, nie HTTP API.

## 2. Charakterystyka panelu

- **Producent:** Usman Nasir / CyberPersons. Otwarte źródła (GPL).
- **Web server:** OpenLiteSpeed (community) lub LiteSpeed Enterprise.
- **Komunikacja webox → panel:** SSH + `cyberpanel` CLI (Python-based).
- **Restart aplikacji:** OpenLiteSpeed restart globalnie (`/usr/local/lsws/bin/lshttpd -r`) lub per-vhost. **TO BE VERIFIED** — czy istnieje per-app restart bez restartu całego serwera.
- **Wersje Node:** brak natywnego Node Application Manager. Zwykle przez NVM lub PM2 ręcznie. **TO BE VERIFIED** — czy są builds Node packaged przez CyberPanel.
- **SSL:** komenda `cyberpanel issueSSL --domainName <DOMAIN>` (Let's Encrypt wbudowany).
- **Bazy:** MySQL przez CLI; PostgreSQL — `[TO BE VERIFIED]`.

## 3. Otwarte pytania przed implementacją

- [ ] **Czy webox musi mieć root SSH** na serwerze CyberPanel? CLI często wymaga `sudo` (lub `root`). Hosting współdzielony z CyberPanel rzadko daje root userowi.
- [ ] **Per-app restart vs restart całego LSWS** — `lshttpd -r` reset'uje *wszystkie* vhosts. Restart aplikacji Node powinien być per-app.
- [ ] **Listing API endpointów** — CyberPanel ma `/api/*`, ale dokumentacja jest cząstkowa. Trzeba przeczytać kod źródłowy z https://github.com/usmannasir/cyberpanel.
- [ ] **Restart Node** — brak natywnego. Webox musi (a) używać PM2 jeśli zainstalowany, (b) używać systemd user units, (c) ręcznie kill + spawn przez SSH. Decyzja zależna od konwencji hostingu.
- [ ] **Format response CLI** — czy `--json` flag jest spójna we wszystkich komendach.
- [ ] **CLI error reporting** — `cyberpanel createDatabase` zwraca exit code 0 nawet przy częściowych błędach? Trzeba parse'ować stdout/stderr.
- [ ] **Quota / limits** — `[TO BE VERIFIED]`.
- [ ] **PostgreSQL support** — `[TO BE VERIFIED]`.
- [ ] **Idempotentność** — `cyberpanel createDatabase` na istniejącą bazę = error czy no-op?
- [ ] **DNS management** — czy CyberPanel zarządza DNS dla zarejestrowanej domeny, czy webox musi tylko zakładać że DNS jest skonfigurowany.

## 4. Wstępne mapowanie metod interfejsu

> Bazując na community CLI reference (sekcja [§1](#1-%C5%BAr%C3%B3d%C5%82a-i-oficjalna-dokumentacja)). Wszystkie pozycje **TO BE VERIFIED** na realnym serwerze.

| Metoda interfejsu | Komenda CLI | Status |
|---|---|---|
| `CreateSubdomain(ctx, domain, nodeVersion)` | `cyberpanel createChild --masterDomain <ROOT> --childDomain <SUB> --owner <USER> --php <PHP_VER>` | **TO BE VERIFIED** — `--php` flag mylące dla Node; może wymagać tworzenia "Website" + osobnej konfiguracji Node. |
| Nodejs site specifically | brak w standardowym CLI | **TO BE VERIFIED** — community workaround zwykle przez `createWebsite` + ręczna konfiguracja vhost OpenLiteSpeed. |
| `SetupSSL(ctx, domain)` | `cyberpanel issueSSL --domainName <DOMAIN>` | OK z dokumentacją community |
| `CreateDatabase(ctx, "mysql", dbName)` | `cyberpanel createDatabase --databaseWebsite <DOMAIN> --dbName <DB> --dbUsername <USER> --dbPassword <PASS>` | OK z dokumentacją |
| `CreateDatabase(ctx, "postgresql", dbName)` | **brak natywny** | **TO BE VERIFIED** — może brak wsparcia |
| `RestartNodeApp(ctx, domain)` | brak natywny w CyberPanel CLI | **TO BE VERIFIED** — fallback `kill + spawn`, `pm2 restart`, lub `systemctl --user restart`. Wymaga ustalenia konwencji per-host. |
| `GetDeployPath(domain)` | `/home/<DOMAIN>/public_html` | **TO BE VERIFIED** — czy CyberPanel używa `/home/<USER>/<DOMAIN>/` czy `/home/<DOMAIN>/`. |
| `GetLogPath(domain)` | `/home/<DOMAIN>/logs` lub `/usr/local/lsws/logs/<DOMAIN>` | **TO BE VERIFIED** |
| `CheckStatus(ctx)` | `cyberpanel -h` (exit code 0 = OK) | OK |
| `ListSubdomains(ctx)` | brak dokumentowanej komendy | **TO BE VERIFIED** — może wymagać `cyberpanel listChildDomains` |
| `RemoveSubdomain(ctx, domain)` | `cyberpanel deleteWebsite --domainName <DOMAIN>` (lub `deleteChildDomain`) | **TO BE VERIFIED** |
| `RemoveDatabase(ctx, dbName)` | `cyberpanel deleteDatabase --dbName <DB>` | OK z dokumentacją |
| `RemoveSSL(ctx, domain)` | brak natywnej | **TO BE VERIFIED** — może wymagać manualnego usunięcia plików cert |

## 5. Specyficzne properties

| Klucz | Wartości | Cel |
|---|---|---|
| `restart_method` | `"openlitespeed"` / `"pm2"` / `"systemd_user"` | Adapter wybiera. |
| `app_entrypoint` | `"app.js"` / `"server.js"` | Wymagany dla manual spawn. |
| `node_setup_strategy` | `"manual_pm2"` / `"manual_systemd"` / `"none"` | Konwencja hostingu. |
| `openlitespeed_vhost_path` | `"/usr/local/lsws/conf/vhosts/<DOMAIN>/vhconf.conf"` (default) | Ścieżka do edycji vhost (jeśli webox musi). |
| `cyberpanel_user_root` | `"false"` (default) / `"true"` | Czy `cyberpanel` CLI wymaga `sudo`. |

## 6. Ścieżki plików — hipoteza

```
/home/<DOMAIN>/
├── public_html/             # główna treść
├── logs/                    # logi
├── tmp/                     # może być
└── (Node app folder)/       # jeśli skonfigurowany
```

`[TO BE VERIFIED]` — czy `<USER>` jest częścią ścieżki czy nie. Community sugeruje `<DOMAIN>` jako root, nie `<USER>`.

## 7. Edge cases znane z dokumentacji i community

- **Brak per-app restart** — restart `lshttpd -r` jest globalny dla wszystkich vhosts. Webox musi to komunikować userowi: `Restart will reload OpenLiteSpeed for ALL sites on this server. Continue?`. Mitygacja: wykorzystanie PM2 / systemd jeśli dostępne.
- **CLI wymaga uprawnień:** `cyberpanel` często wymaga `sudo`. Webox sprawdza `sudo -n true` przed pierwszym wywołaniem.
- **Brak DA-style API key** — CyberPanel używa Basic Auth lub API key generowany w panelu webowym. **TO BE VERIFIED**.
- **OpenLiteSpeed config edits są ręczne** — webox **nie powinien** edytować plików konfiguracyjnych OLS bezpośrednio (ryzyko zepsucia całego serwera).
- **Community API plugin** istnieje — może być wymagany dla zaawansowanych funkcji (`https://docs.cyberpanel.net/cyberpanel-api-keys-plugin` jeśli istnieje).

## 8. Plan testów przed implementacją

1. Pozyskać dostęp do testowej instancji CyberPanel (open-source, można postawić na DigitalOcean droplet).
2. Zainstalować webox dev build z eksperymentalnym adapterem.
3. Manualnie wykonać każdą z planowanych komend CLI.
4. Capture stdout, stderr, exit code jako fixture w `testing/fixtures/cyberpanel/`.
5. Sanityzacja: realne IP/domeny → `203.0.113.10` / `test.example.com`.
6. Każdy fixture z `*.fixture.md`.
7. Iteracja §3 → coroczna aktualizacja.
8. Po weryfikacji wszystkich punktów §3 — promocja statusu **RESEARCH → PLANNED**.

**Decyzja blokująca dla CyberPanel:** jeśli okaże się, że per-app restart wymaga edycji konfiguracji OpenLiteSpeed (a nie restart całego LSWS), CyberPanel może wylądować jako **community-maintained** zamiast w core webox. Patrz [ROADMAP §7 Kryteria decyzji](../ROADMAP.md#7-kryteria-decyzji-o-dodaniu-providera).

Bez kompletu fixture'ów — adapter nie wchodzi do `main`.
