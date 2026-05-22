# Provider: smallhost (small.pl / panel Devil)

> Status: **MVP — stable target (v0.1)** · Ostatnia aktualizacja: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [DESIGN.md §3](../DESIGN.md#3-provider-pattern), [CONTRIBUTING §3](../CONTRIBUTING.md#3-jak-doda%C4%87-nowy-provider), [adr/0003](../adr/0003-provider-pattern.md).

## TL;DR

`smallhost` to **referencyjny adapter MVP** dla hostingu small.pl używającego panelu Devil. Komunikacja przez SSH + CLI `devil`. Wszystkie operacje (subdomena, SSL, baza, restart) odbywają się komendami w pojedynczych sesjach SSH. Ścieżki konfiguracji deterministyczne (`/usr/home/$USER/domains/$DOMAIN/`). Adapter używa `properties.restart_method = "devil"`. Ten plik jest **wzorcem** dla wszystkich kolejnych providerów.

## Spis treści

1. [Charakterystyka panelu Devil](#1-charakterystyka-panelu-devil)
2. [Mapowanie metod `HostingProvider` na komendy `devil`](#2-mapowanie-metod-hostingprovider-na-komendy-devil)
3. [Ścieżki plików](#3-%C5%9Bcie%C5%BCki-plik%C3%B3w)
4. [Properties bag](#4-properties-bag)
5. [Edge cases i znane dziwactwa](#5-edge-cases-i-znane-dziwactwa)
6. [Deployment workflow szablon](#6-deployment-workflow-szablon)
7. [Otwarte pytania / TODO](#7-otwarte-pytania--todo)

---

## 1. Charakterystyka panelu Devil

- **Producent:** small.pl (operator hostingu, panel wewnętrzny zwany Devil).
- **Mechanizm:** każdy user dostaje SSH dostęp i komendę `devil` w PATH. Komendy są synchroniczne, zwracają linijki w czytelnym tekstem stdout, czasem z `\033[...m` ANSI escape.
- **Brak REST API.** Wszystko przez SSH + CLI.
- **Brak uprawnień root** — wszystko w kontekście usera. Komendy `devil` operują na zasobach przypisanych do konta.
- **Wersje Node** zarządzane centralnie przez panel; user tylko wybiera. Adapter powinien traktować listę wspieranych wersji jako **dane wykrywane lub fixture'owane**, nie jako wiecznie prawdziwy hardcode. Zaobserwowane wersje w 2026-05: `16`, `18`, `20`, `22`, `23`, `24`. Brak `nvm` po stronie użytkownika — wybór jest stały dla subdomeny.
- **Restart aplikacji** — `devil www restart <domain>` (atomowy z perspektywy usera, ~3–5 s).
- **SSL** — wbudowany Let's Encrypt przez `devil ssl www add <ip> le le <domain>` (gdzie `<ip>` = IP konta; `le le` to typ certyfikatu).
- **Bazy danych** — MySQL (`devil mysql ...`) i PostgreSQL (`devil pgsql ...`). User i baza tworzone razem, hasło generowane przez panel.

## 2. Mapowanie metod `HostingProvider` na komendy `devil`

> Dozwolone podpisy interfejsu w docs — pełna implementacja w `providers/smallhost.go`. Adapter parsuje output defensywnie zgodnie z [SECURITY §3.3](../SECURITY.md#33-defensywne-parsowanie-outputu).

### 2.1 Tabela mapowania

| Metoda interfejsu | Komenda shell | Expected output (sukces) | Known errors / wzorce |
|---|---|---|---|
| `CreateSubdomain(ctx, domain, nodeVersion)` | `devil www add <domain> nodejs <nodeVersion>` | `Added domain <domain> with nodejs <nodeVersion>` (lub podobne — patrz §7 TODO) | `exists: domain already exists` → `ErrSubdomainExists`. `invalid node version` → `ErrNodeVersionUnsupported`. |
| `SetupSSL(ctx, domain)` | krok 1: `devil vhost list` (wyciąga IP konta); krok 2: `devil ssl www add <ip> le le <domain>` | `Certificate installed for <domain>` | `dns not configured` → `ErrDNSNotResolving`. Let's Encrypt rate limit → `ErrRateLimitLetsEncrypt`. |
| `CreateDatabase(ctx, "mysql", dbName)` | `devil mysql add <dbName>` | wieloliniowy output z `user: <login>` i `password: <pass>` | `database exists` → `ErrDBNameTaken`. |
| `CreateDatabase(ctx, "postgresql", dbName)` | `devil pgsql add <dbName>` | analogicznie | analogicznie |
| `RestartNodeApp(ctx, domain)` | `devil www restart <domain>` | `Restarted <domain>` | `not a nodejs domain` → `ErrAppNotNode`. `not found` → `ErrAppNotFound`. |
| `GetDeployPath(domain)` | (czysta funkcja) | `/usr/home/<user>/domains/<domain>/public_nodejs/public` | nigdy |
| `GetLogPath(domain)` | (czysta funkcja) | `/usr/home/<user>/domains/<domain>/logs/` | nigdy |
| `CheckStatus(ctx)` | `node --version` (lub `devil --version`) | numeryczna wersja | exit code 127 → `ErrCLINotFound` |
| `ListSubdomains(ctx)` | `devil www list` (z dodatkowym parserem) | tabela z kolumnami `domain`, `type`, `node_version` | parser fail → `ErrUnknownOutputFormat` |
| `RemoveSubdomain(ctx, domain)` | `devil www del <domain>` | `Deleted <domain>` | `not found` → `nil` (idempotent) |
| `RemoveDatabase(ctx, dbName)` | `devil mysql del <dbName>` lub `devil pgsql del <dbName>` | `Deleted <dbName>` | `not found` → `nil` |
| `RemoveSSL(ctx, domain)` | `devil ssl www del <ip> <domain>` | `Removed SSL for <domain>` | `no cert` → `nil` |

### 2.2 Sygnatury (referencja)

```text
package smallhost

// SmallHostProvider implements providers.HostingProvider for small.pl/Devil.
// Created via the registry: providers.Register("smallhost", New).
type SmallHostProvider struct { /* unexported */ }

func New(cfg providers.ProviderConfig) (providers.HostingProvider, error)

// All methods below match providers.HostingProvider verbatim.
// Implementations live in providers/smallhost.go.
// <!-- TODO: implementacja w kodzie -->
```

## 3. Ścieżki plików

Ścieżki na serwerze są deterministyczne i parametryzowane po `cfg.User`:

| Cel | Wzorzec |
|---|---|
| Katalog domeny | `/usr/home/<USER>/domains/<DOMAIN>/` |
| Kod aplikacji Node | `/usr/home/<USER>/domains/<DOMAIN>/public_nodejs/` |
| Document root statyczny | `/usr/home/<USER>/domains/<DOMAIN>/public_nodejs/public/` |
| Logi aplikacji | `/usr/home/<USER>/domains/<DOMAIN>/logs/` |
| `.env` projektu | `/usr/home/<USER>/domains/<DOMAIN>/public_nodejs/.env` |
| Persistent storage (assets) | `/usr/home/<USER>/domains/<DOMAIN>/public_nodejs/public/uploads/` |

Webox **nie tworzy** żadnego z tych katalogów ręcznie — robi to `devil www add`. Webox tylko **transferuje pliki** przez SFTP do already-existing miejsc.

> `.env` jest celowo poza `public_nodejs/public/`, czyli poza web root. To warunek bezpieczeństwa, nie detal ścieżki. Patrz [SECURITY.md §10.4](../SECURITY.md#104-lokalizacja-i-permisje-env-na-serwerze).

## 4. Properties bag

W `config.json`:

```json
{
  "alias": "main",
  "type": "smallhost",
  "properties": {
    "restart_method": "devil"
  }
}
```

| Klucz | Wartość | Cel |
|---|---|---|
| `restart_method` | `"devil"` (jedyne wsparte dla small.pl) | Adapter wie, jak restartować — bez znaczenia w MVP (tylko devil). |
| (przyszłość) `node_version_strategy` | `"panel_managed"` | Marker dla `CreateSubdomain`. |
| (przyszłość) `ssh_pool_max` | `"3"` | Override default'u 3. |

## 5. Edge cases i znane dziwactwa

### 5.1 Maksymalna długość nazwy subdomeny

- Empiryczna obserwacja: do 63 znaków przed `.`. Walidator webox: regex `^[a-z0-9-]{1,63}$` (nie zaczyna od `-`).
- **TO BE VERIFIED**: czy są ograniczenia panelu (np. zarezerwowane subdomeny `www`, `mail`, `ftp`).

### 5.2 Idempotentność `devil ssl www add`

- **TO BE VERIFIED**: jeśli cert już istnieje dla `<domain>`, czy `devil ssl www add` zwraca błąd (`exists`) czy renew? Nasz adapter zakłada error i przed `SetupSSL` sprawdza istnienie poprzez `devil ssl www show <domain>` (jeśli wspierane). **Sprawdzić w testach z realnym kontem.**

### 5.3 Parsowanie outputu `devil mysql add`

Output zawiera linijki w stylu:

```
Database myapp_prod created.
Username: myapp_prod
Password: aBcD1234EfGh5678
```

Parser webox używa regex:

```
Username:\s+(?P<user>\S+)
Password:\s+(?P<pass>\S+)
```

- **TO BE VERIFIED**: dokładny format. Może zawierać linijki `Database 'myapp_prod' created with user 'myapp_prod'` — sprawdzić na realnym koncie.
- ANSI escape: zwykle brak; gdyby się pojawiły — strip.

### 5.4 SSL — DNS not propagated

Let's Encrypt wymaga, by DNS subdomeny ustawiał się na IP serwera. Adapter rozdziela dwa **fundamentalnie różne** scenariusze:

#### 5.4.a Subdomena small.pl (`*.<user>.smallhost.pl`)

DNS instant (rekord A zarządzany przez small.pl natywnie). Webox **może** wykonać `SetupSSL` natychmiast po `CreateSubdomain`. Flow:

1. `CreateSubdomain`
2. `net.LookupHost(domain)` z timeoutem 5 s (sanity check, nie blokujący)
3. `SetupSSL`

#### 5.4.b Custom domain (np. `app.example.com`)

DNS może propagować do 48 h. Webox **nie czeka** synchronnie. Flow:

1. `CreateSubdomain`
2. Wizard kończy z sukcesem, projekt dostaje status `SSL_PENDING` z opisem: *"Custom domain DNS not yet resolved. SSL will be issued automatically on next status refresh (max 48h)."*.
3. Background ticker w `status/` cache retry'uje `SetupSSL` co 15 min aż do sukcesu lub przekroczenia 48 h.
4. Przy sukcesie status zmienia się na `ONLINE` z banner *"SSL issued. Click to dismiss."*. Przy 48 h timeout — `SSL_FAILED` z linkiem do DNS troubleshooting.

Adapter wykrywa rate limit / propagation error w outpucie `devil ssl` i mapuje na `ErrDNSNotResolving` lub `ErrRateLimitLetsEncrypt`.

Patrz [AUDIT §8 IMP-15](../AUDIT.md#8-uzupe%C5%82niaj%C4%85ce-znaleziska-po-drugim-przebiegu).

### 5.5 `devil www restart` — czas oczekiwania

- Komenda wraca po ~3–5 s.
- HTTP po restarcie odpowiada zwykle w ciągu ~5–10 s.
- Webox po `RestartNodeApp` czeka 8 s przed pierwszym HTTP ping (avoid false negative).

### 5.6 PostgreSQL na small.pl

- **TO BE VERIFIED**: czy small.pl udostępnia PostgreSQL na wszystkich kontach? Niektóre tańsze plany mają tylko MySQL.
- Adapter próbuje `devil pgsql --help` przy pierwszym `CreateDatabase("postgresql", ...)` i zwraca jasny błąd jeśli brak.

### 5.7 Listowanie subdomen

- `devil www list` zwraca tabelę (ASCII). Adapter parsuje.
- **TO BE VERIFIED**: dokładny format (column separators, header). Capture fixture po realnym test'cie.

### 5.8 Concurrent SSH

- Small.pl dopuszcza wiele jednoczesnych sesji per konto, ale unika "abuse" patterns.
- `ssh_pool_max=3` (default) dla `smallhost` jest konserwatywny.
- Przy 20 projektach × 3 fetch (HTTP, SSH, SSL) jednocześnie pool buforuje queue — patrz [DESIGN §5.3](../DESIGN.md#53-connection-pool).

## 6. Deployment workflow szablon

Webox generuje `.github/workflows/deploy.yml` w repo projektu na podstawie szablonu osadzonego w binarce. Parametryzacja:

| Placeholder | Wartość przykładowa |
|---|---|
| `{{NodeVersion}}` | `24` |
| `{{BuildCommand}}` | `npm run build` |
| `{{DistDir}}` | `dist` |
| `{{DeployBranch}}` | `main` |
| `{{DeployHost}}` | `s1.small.pl` |
| `{{DeployUser}}` | `biuromody` |
| `{{DeployPath}}` | `/usr/home/biuromody/domains/sui.biuromody.smallhost.pl/public_nodejs/public` |

Sekrety w repo:

| Secret | Wartość |
|---|---|
| `SSH_PRIVATE_KEY` | Zawartość prywatnego **deploy key wygenerowanego per projekt**. Nigdy nie globalny klucz operatorski użytkownika. |
| `DEPLOY_HOST` | `{{DeployHost}}` (też pole jawne dla audit). |
| `DEPLOY_USER` | `{{DeployUser}}`. |
| `DEPLOY_PATH` | `{{DeployPath}}`. |
| `KNOWN_HOSTS` | Pre-fetched fingerprint z `ssh-keyscan -t ed25519 {{DeployHost}}`. |

Workflow w wysokim poziomie:

```
1. Checkout
2. Setup Node {{NodeVersion}}
3. Cache ~/.npm (cache key: hashFiles(**/package-lock.json))    # IMP-17
4. npm ci
5. {{BuildCommand}}
6. Setup SSH (write key + known_hosts)
7. rsync -avz --delete                                            # C6 — excludes
     --exclude='.env'
     --exclude='node_modules/'
     {{#each PersistentDirs}}--exclude='{{this}}/' {{/each}}
     {{DistDir}}/ {{DeployUser}}@{{DeployHost}}:{{DeployPath}}/
8. Verify .env permissions:                                       # IMP-10
     ssh {{DeployUser}}@{{DeployHost}}
       'stat -c "%a %U" {{DeployPath}}/../.env'
       | grep -q '^600 {{DeployUser}}$'
       || (echo "::error::.env has insecure permissions" && exit 1)
9. ssh {{DeployUser}}@{{DeployHost}} 'devil www restart {{Domain}}'
```

> **Krytyczne — `--exclude` dla persistent dirs:** bez tego `--delete` usuwa pliki na zdalnym, których brak w lokalnym `dist/` — np. `public/uploads/` z assetami klientów lub `.env` materializowany przez GHA. To **destructive operation** którą widzieliśmy zabijać produkcję klientów. Patrz [AUDIT C6](../AUDIT.md#c6-providerssmallhostmd-6--workflow-deployyml-u%C5%BCywa-rsync-z---delete).

`{{PersistentDirs}}` to `properties.persistent_dirs` per profile (default: `["uploads", "tmp", "cache"]`). Webox automatycznie dodaje `.env` jako zawsze excluded.

Pełen szablon żyje w `assets/workflow_deploy_smallhost.tmpl.yml` (build embed). Patrz [DESIGN §13.5](../DESIGN.md#135-szablon-workflow-parametryzowany).

## 7. Otwarte pytania / TODO

> Każde z poniższych wymaga weryfikacji **na realnym koncie small.pl** i capture fixture do `testing/fixtures/devil/`.

- [ ] **Exact output formatów `devil` komend** — capture i sanityzacja dla wszystkich operacji P0.
- [ ] **Idempotentność `devil ssl www add`** — czy duplikat = error czy renew?
- [ ] **Zarezerwowane nazwy subdomen** — czy są (np. `www`, `mail`, `cpanel`)?
- [ ] **PostgreSQL availability** — na których planach small.pl jest dostępny?
- [ ] **Limity ilości subdomen / baz** — czy są dynamicznie sprawdzalne (np. `devil quota`)?
- [ ] **Dokładny format `devil www list`** — jak rozpoznać typ subdomeny (nodejs/php/static).
- [ ] **Co robi `devil www restart` na nie-Node domenie?** — error czy no-op?
- [ ] **Wykrycie IP konta** — czy `devil vhost list` jest zawsze dostępne? Alternatywne źródło IP?
- [ ] **Edge case: domain z myślnikami** — czy są ograniczenia.
- [ ] **Edge case: i18n nazwy domeny (IDN/Punycode)** — czy small.pl supports custom domeny z umlautami?
- [ ] **Path normalization** — czy ścieżka `public_nodejs/public/` bywa inna dla niektórych typów subdomen?
- [ ] **Czy lista wspieranych wersji Node jest dostępna z CLI** czy trzeba ją utrzymywać fixture'owo per capture.

Decyzje testowe (capture na sandbox account `webox-test@small.pl`):

- Fixture'y obowiązkowe przed v0.1: `www_add_ok`, `www_add_exists`, `mysql_add_ok`, `mysql_add_name_taken`, `ssl_add_ok`, `ssl_add_dns_not_ready`, `www_restart_ok`, `www_list_5_subdomains`, `www_del_ok`, `mysql_del_ok`, `ssl_del_ok`.
- Bez tych fixture'ów testy integracyjne nie istnieją — patrz [TESTING §3.3](../TESTING.md#33-fixturey-output-devil).
