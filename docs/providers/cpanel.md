# Provider: cpanel

> Status: **RESEARCH / PLANNED POST-MVP (v0.2)** · Ostatnia aktualizacja: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne dokumenty: [providers/smallhost.md](./smallhost.md) (wzorzec), [CONTRIBUTING §3](../CONTRIBUTING.md#3-jak-doda%C4%87-nowy-provider), [ROADMAP §4.1](../ROADMAP.md#41-wyb%C3%B3r-drugiego-providera).

## STATUS

**RESEARCH — implementacja zaplanowana na v0.2.** Ten dokument zawiera **wyłącznie notatki badawcze** i **otwarte pytania**. Wszystkie hipotezy oznaczone `[TO BE VERIFIED]` wymagają potwierdzenia na realnym koncie cPanel przed implementacją. Adapter nie istnieje w kodzie — w MVP profile typu `cpanel` są ukryte za `WEBOX_EXPERIMENTAL=1` i nie mają implementacji.

> Oryginalny PRD zawierał implementację adaptera `cpanel.go` napisaną „z głowy", bez konfrontacji z UAPI. Została usunięta jako fałszywa implementacja (patrz [CHANGES.md poprawka 6.9](../../CHANGES.md#1-poprawki-merytoryczne-z-tabeli-6-briefu)).

## TL;DR

cPanel to dominujący panel hostingowy w USA i częściowo w Europie. Komunikacja możliwa przez **UAPI** (per-user API, dostępne z SSH lub HTTP) lub **WHM API** (administratorskie). Webox będzie korzystać z **UAPI per-user** wywoływanego przez SSH (jak `devil` dla small.pl). Architektonicznie zgodne ze wzorcem `smallhost`, ale parsowanie odpowiedzi i mechanika SSL/Node są zauważalnie inne (Phusion Passenger restart, EasyApache).

## Spis treści

1. [Źródła i oficjalna dokumentacja](#1-%C5%BAr%C3%B3d%C5%82a-i-oficjalna-dokumentacja)
2. [Charakterystyka panelu](#2-charakterystyka-panelu)
3. [Otwarte pytania przed implementacją](#3-otwarte-pytania-przed-implementacj%C4%85)
4. [Wstępne mapowanie metod interfejsu](#4-wst%C4%99pne-mapowanie-metod-interfejsu)
5. [Specyficzne properties](#5-specyficzne-properties)
6. [Ścieżki plików — hipoteza](#6-%C5%9Bcie%C5%BCki-plik%C3%B3w--hipoteza)
7. [Edge cases znane z dokumentacji](#7-edge-cases-znane-z-dokumentacji)
8. [Plan testów przed implementacją](#8-plan-test%C3%B3w-przed-implementacj%C4%85)

---

## 1. Źródła i oficjalna dokumentacja

| Zasób | Link | Uwagi |
|---|---|---|
| cPanel UAPI overview | https://api.docs.cpanel.net/specifications/cpanel.openapi | Pełna specyfikacja OpenAPI. |
| Domain Add (UAPI) | (`Domain::adddomain` w UAPI) | Tworzenie dodatkowych domen / subdomen. |
| SSL Install (UAPI) | https://api.docs.cpanel.net/specifications/cpanel.openapi/ssl-certificate-management/install_ssl | `GET /SSL/install_ssl` |
| SSL Account Management | https://api.docs.cpanel.net/specifications/cpanel.openapi/cpanel-account-ssl-management | `SSL::fetch_best_for_domain`, etc. |
| MySQL Create Database | https://api.docs.cpanel.net/specifications/cpanel.openapi/database-management/create_database | `GET /Mysql/create_database` |
| DNS | https://api.docs.cpanel.net/specifications/cpanel.openapi/dns.md | Zarządzanie zone'em. |
| cPanel Node.js Hosting | Oficjalna dokumentacja "Application Manager" | `[TO BE VERIFIED]` — czy każda wersja cPanel wspiera Node.js out of the box. |

## 2. Charakterystyka panelu

- **Producent:** cPanel L.L.C., własność WebPros.
- **Model:** każdy user ma własne konto w shared environment, dostęp przez WHM (admin) i cPanel (user).
- **UAPI** — REST-like, ale faktycznie GET-based z query params. Wywołania z SSH wyglądają jak:

  ```
  uapi --user=<USER> Domain adddomain domain=<DOMAIN> documentroot=<PATH>
  ```

  Z poziomu HTTP wymagają session token (port 2083) lub WHM API token.
- **Authentication:** dla wywołań SSH wystarczy bycie zalogowanym jako user (cPanel session); webox planuje używać tej ścieżki.
- **Restart Node aplikacji:** **Phusion Passenger**. `[TO BE VERIFIED]` — czy `restart_app` UAPI istnieje, czy zostaje `touch tmp/restart.txt` w katalogu aplikacji.
- **Wersje Node:** zarządzane przez "Application Manager" lub `cPanel Node.js Selector` (CloudLinux). `[TO BE VERIFIED]` — jakie wersje dostępne, jak konfigurowane.
- **SSL:** AutoSSL (wbudowany Let's Encrypt) lub manual install. `SSL::install_lets_encrypt` jest dostępne na większości instalacji.
- **Bazy:** MySQL standard, PostgreSQL opcjonalne (per host).

## 3. Otwarte pytania przed implementacją

> Każde wymaga zweryfikowania na realnym koncie cPanel.

- [ ] **Czy `--user=<USER>` w wywołaniu `uapi` z SSH wystarczy** do autentykacji w kontekście tego usera, czy potrzebny dodatkowy token sesji (`access_hash`)?
- [ ] **Jak działa Node.js Application Manager** — czy webox musi explicit-nie utworzyć "application" przed deploymentem, czy `Domain::adddomain` wystarczy?
- [ ] **Restart node app** — czy `Passenger::restart_application` jest dostępne, czy konwencja `touch tmp/restart.txt` w katalogu Passenger.
- [ ] **AutoSSL vs ręczny `install_lets_encrypt`** — czy musimy najpierw wyłączyć AutoSSL żeby `install_ssl` zadziałało, czy odwrotnie?
- [ ] **Format response UAPI** — JSON jest standardem (`format=json` query param), ale `[TO BE VERIFIED]`: czy domyślnie zwraca JSON gdy wywołane przez `uapi` CLI.
- [ ] **Rate limits UAPI** — czy są na poziomie session, hosta, czy konta.
- [ ] **PostgreSQL** — czy `Postgresql::create_database` UAPI jest dostępne (zależne od konfiguracji hosta).
- [ ] **Idempotentność** — czy `Domain::adddomain` na istniejącą domenę = error czy no-op?
- [ ] **Reverse: usuwanie zasobów (`Domain::deldomain`)** — czy istnieje? Jak rollback?
- [ ] **Subdomeny vs addon domains vs parked domains** — różne API endpointy. `[TO BE VERIFIED]`: webox traktuje wszystko jako subdomeny — czy `Subdomain::addsubdomain` jest właściwy zamiast `Domain::adddomain`?
- [ ] **Wersjonowanie API** — UAPI vs cPanel API 2 (legacy). Czy webox musi wspierać oba, czy UAPI wystarczy.
- [ ] **Restart Node przez "Application Manager"** — UAPI: `Passenger::restart_application name=<APP>` (`[TO BE VERIFIED]`).

## 4. Wstępne mapowanie metod interfejsu

> Wszystkie wpisy oznaczone **TO BE VERIFIED**. Po weryfikacji ta tabela stanie się normatywna.

| Metoda interfejsu | Wstępna komenda / endpoint | Status |
|---|---|---|
| `CreateSubdomain(ctx, domain, nodeVersion)` | `uapi --user=$USER Subdomain addsubdomain domain=$SUB rootdomain=$ROOT dir=...` **+** osobno "Application Manager" rejestracja Node | **TO BE VERIFIED** |
| `SetupSSL(ctx, domain)` | `uapi --user=$USER SSL install_lets_encrypt domains[]=$DOMAIN` (lub Market::request_ssl_certificates) | **TO BE VERIFIED** |
| `CreateDatabase(ctx, "mysql", dbName)` | `uapi --user=$USER Mysql create_database name=$DB` + `Mysql create_user` + `Mysql set_privileges_on_database` (3 calls) | **TO BE VERIFIED** |
| `CreateDatabase(ctx, "postgresql", dbName)` | analogicznie `Postgresql::create_database` | **TO BE VERIFIED** — może nie istnieć |
| `RestartNodeApp(ctx, domain)` | `uapi --user=$USER Passenger restart_application name=$APP` lub `touch ~/<app>/tmp/restart.txt` przez SFTP | **TO BE VERIFIED** |
| `GetDeployPath(domain)` | `/home/$USER/$DOMAIN/public_html` lub `/home/$USER/<app>/public/` (Passenger) | **TO BE VERIFIED** |
| `GetLogPath(domain)` | `/home/$USER/logs/$DOMAIN` lub `/home/$USER/.cpanel/logs/` | **TO BE VERIFIED** |
| `CheckStatus(ctx)` | `uapi --version` lub `uapi User get_user_information` | **TO BE VERIFIED** |
| `ListSubdomains(ctx)` | `uapi --user=$USER Subdomain list_subdomains` | **TO BE VERIFIED** |
| `RemoveSubdomain(ctx, domain)` | `uapi --user=$USER Subdomain delsubdomain domain=$DOMAIN` | **TO BE VERIFIED** |
| `RemoveDatabase(ctx, dbName)` | `uapi --user=$USER Mysql delete_database name=$DB` | **TO BE VERIFIED** |
| `RemoveSSL(ctx, domain)` | `uapi --user=$USER SSL delete_ssl host=$DOMAIN` | **TO BE VERIFIED** |

## 5. Specyficzne properties

Planowane:

| Klucz | Wartości | Cel |
|---|---|---|
| `restart_method` | `"passenger"` (default dla cpanel) / `"app_manager"` | Adapter wybiera mechanizm restart. |
| `app_entrypoint` | `"app.js"` / `"index.js"` / `"server.js"` | Passenger wymaga znania punktu wejścia. |
| `node_selector` | `"cloudlinux"` / `"app_manager"` / `"none"` | Mechanizm wyboru wersji Node. |
| `ssl_provider` | `"autossl"` / `"letsencrypt"` / `"market"` | `[TO BE VERIFIED]`. |

## 6. Ścieżki plików — hipoteza

Standardowy layout cPanel:

```
/home/<USER>/
├── public_html/              # główna domena
├── <subdomain>.<domain>/     # subdomeny
├── logs/                     # logi domen
├── tmp/                      # restart triggers
└── .cpanel/                  # metadata
```

`[TO BE VERIFIED]` — czy Application Manager używa innego layoutu (`/home/<USER>/<app-name>/`).

## 7. Edge cases znane z dokumentacji

- **AutoSSL kolizja:** `SSL::install_ssl` może być blokowane jeśli AutoSSL ma już cert na tej samej domenie. Wymaga albo wyłączenia AutoSSL dla tego host, albo użycia `letsencrypt` path.
- **Database user prefix:** cPanel prependuje `<USER>_` do nazwy bazy i usera. `[TO BE VERIFIED]` — czy w UAPI należy podać już z prefiksem czy panel sam doda.
- **Quota limity:** `[TO BE VERIFIED]` — jak wykryć przekroczenie quoty disk / bandwidth.
- **PassengerSpawnerError:** restart aplikacji Node może rzucić `Passenger spawning error` jeśli `app.js` ma syntax error — webox musi parse'ować ten error i pokazać go w UI zamiast surowego stacktrace.
- **Reseller account hierarchy:** jeśli user jest dystrybutorem (reseller), `--user=` może wymagać dodatkowych uprawnień.

## 8. Plan testów przed implementacją

> Każdy adapter w webox musi mieć fixture'y outputu **z realnego serwera** zanim trafi do main.

1. Pozyskać dostęp do testowego konta cPanel (sandbox lub trial).
2. Wykonać manualnie każdą z planowanych komend UAPI z SSH.
3. Zapisać output (JSON / text) jako fixture w `testing/fixtures/cpanel/`.
4. Sanityzacja: realny login → `testuser`, realne IP → `203.0.113.10`.
5. Każdy fixture ma `*.fixture.md` z opisem pochodzenia.
6. Iteracja `Open question` (§3) → coroczna aktualizacja tego dokumentu.
7. Po weryfikacji wszystkich pozycji §3 — promocja statusu z **RESEARCH** na **PLANNED** + zatwierdzenie tabeli §4.

Dopóki nie ma kompletu fixture'ów — adapter nie wchodzi do `main`. Patrz [CONTRIBUTING §3.4](../CONTRIBUTING.md#34-krok-4--testy).
