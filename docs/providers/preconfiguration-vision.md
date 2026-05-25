# Provider Preconfiguration Vision

> Status: **VISION / RESEARCH** · Ostatnia aktualizacja: 2026-05-25 · Wlasciciel: @maintainer
>
> Pokrewne dokumenty: [PRD §2](../PRD.md#2-wizja-i-motto), [ROADMAP §4-7](../ROADMAP.md#4-v02--drugi-provider--doko%C5%84czenie-palette), [ADR-0003](../adr/0003-provider-pattern.md), [smallhost](./smallhost.md), [cpanel](./cpanel.md), [directadmin](./directadmin.md), [cyberpanel](./cyberpanel.md).

## TL;DR

Webox ma wygrywac nie tym, ze "obsluguje cPanel" w abstrakcji, tylko tym, ze operator wybiera z listy **konkretny hosting**, dostaje sensowne domyslne sciezki, strategie Node.js, restart, SSL, logi, limity i ostrzezenia, a potem poprawia najwyzej 1-2 pola.

Provider support powinien stac sie produktem samym w sobie:

- **Panel adapter** odpowiada za protokol: Devil, cPanel UAPI, DirectAdmin API, CyberPanel CLI/API.
- **Provider preset** odpowiada za realny wariant hostera: np. "cPanel + CloudLinux Node.js Selector + Passenger", "DirectAdmin + CloudLinux Node.js Selector", "CyberPanel + OpenLiteSpeed + PM2".
- **Capability detection** weryfikuje przy pierwszym polaczeniu, czy preset pasuje do konta.
- **Community capture kit** pozwala uzytkownikom dostarczac fixture'y i preset bez pisania adaptera.

Najwazniejsza decyzja strategiczna: **nie obiecujemy providerow po nazwie panelu bez profilu hostera**. W 2026 ten sam cPanel lub DirectAdmin moze miec kompletnie inny runtime Node.js, sciezki, SSL i restart.

## 1. Cel dokumentu

Ten dokument opisuje wizje prekonfiguracji providerow dla Webox po MVP. Nie jest specyfikacja implementacyjna adapterow i nie zmienia zakresu `v0.1`, ktory pozostaje `smallhost` only.

Ma odpowiedziec na pytania:

1. Jak sprawic, zeby lista wspieranych providerow byla glownym powodem wyboru Webox?
2. Jakie presety warto przygotowac dla cPanel, DirectAdmin i CyberPanel?
3. Jak odroznic "panel wspierany" od "konkretny hoster przetestowany"?
4. Jak zbierac dane od polskich i zagranicznych uzytkownikow bez psucia bezpieczenstwa?
5. Jak nie utopic projektu w niezweryfikowanych obietnicach?

## 2. Zrodla i sygnaly z researchu

Research wykonany 2026-05-25. Linki ponizej sa punktem startowym, nie dowodem gotowosci presetow. Kazdy preset nadal wymaga testowego konta, fixture'ow i sanityzacji.

### 2.1 Oficjalne API i dokumentacja paneli

| Obszar | Zrodlo | Wniosek dla Webox |
|---|---|---|
| cPanel UAPI | [cPanel UAPI OpenAPI](https://api.docs.cpanel.net/specifications/cpanel.openapi) | UAPI jest najlepszym kandydatem na stabilna warstwe dla domen, baz, SSL, tokenow i Passenger Apps. |
| cPanel API tokens | [Manage API Tokens](https://docs.cpanel.net/cpanel/security/manage-api-tokens-in-cpanel/) | Tokeny moga byc wlaczane/wylaczane przez hostera w WHM Feature Manager. Preset musi wykrywac brak funkcji. |
| cPanel Application Manager | [Application Manager](https://docs.cpanel.net/cpanel/software/application-manager/) | Node.js dziala przez Phusion Passenger, ale hoster musi wlaczyc feature i pakiety `ea-nodejs16/18/20/22`. |
| cPanel Node install | [How to Install a Node.js Application](https://docs.cpanel.net/knowledge-base/web-services/how-to-install-a-node.js-application/) | Startup file i `PassengerStartupFile` sa realnym edge case'em. Webox musi wykrywac entrypoint, nie zgadywac w ciemno. |
| Passenger restart | [Passenger restart-app](https://www.phusionpassenger.com/docs/advanced_guides/troubleshooting/apache/restart_app.html) | Najbezpieczniejszy restart to `passenger-config restart-app <path>` lub `touch tmp/restart.txt`, zalezne od uprawnien. |
| DirectAdmin API | [DirectAdmin API Access](https://docs.directadmin.com/developer/api/) | Nowe API jest Swagger/OpenAPI pod `/static/swagger.json`, ale DirectAdmin sam zaleca fallback do legacy gdy endpointu brakuje. |
| DirectAdmin examples | [DirectAdmin API examples](https://docs-dev.directadmin.com/developer/api/examples.html) | Priorytet: discovery endpointu, potem nowe API, potem legacy, dopiero na koncu reverse-engineering UI. |
| DirectAdmin Nginx Unit | [Nginx Unit](https://docs.directadmin.com/webservices/nginx_unit/) | Node.js moze dzialac przez Nginx Unit na czesci instalacji, co jest innym presetem niz CloudLinux/Passenger. |
| CloudLinux Node.js Selector | [CloudLinux Node.js Selector](https://cloudlinux.com/getting-started-with-cloudlinux-os/42-profitability-and-php-features/959-nodejs-selector/) | Node.js Selector dziala na cPanel/DirectAdmin/Plesk z Apache i jest bardzo waznym wspolnym wariantem. |
| CyberPanel CLI | [CyberPanel CLI reference](https://community.cyberpanel.net/t/cyberpanel-command-line-interface/30683) | CLI daje create website, child domain, DB i SSL, ale nie rozwiazuje natywnie Node.js app lifecycle. |
| CyberPanel source | [CyberPanel GitHub](https://github.com/usmannasir/cyberpanel) | CyberPanel to OpenLiteSpeed panel; integracja Node.js wymaga ostroznosci, czesto root/sudo. |
| NodeCyber | [nodecyber](https://github.com/Witsberry/nodecyber) | W praktyce Node.js na CyberPanel moze wymagac PM2 + modyfikacji OpenLiteSpeed; to raczej preset eksperymentalny/rootowy niz shared-hostingowy. |

### 2.2 Sygnaly rynkowe

| Sygnal | Wniosek |
|---|---|
| cPanel nadal jest mocny w shared hostingu, mimo slabszej wykrywalnosci publicznej. | cPanel powinien byc pierwszym post-MVP providerem, bo ma najwieksza rozpoznawalnosc i najlepsze API. |
| DirectAdmin jest czestym kierunkiem migracji z cPanel przez koszty licencji. | DirectAdmin powinien wejsc szybko po cPanel, ale tylko po rozstrzygnieciu Node.js runtime strategy. |
| CyberPanel jest atrakcyjny dla self-hosted / tanich VPS, ale ma najwiecej ryzyk API i uprawnien. | CyberPanel nie powinien blokowac `v0.2`; nadaje sie na `v0.3+ experimental` albo community-maintained preset. |
| Plesk i hPanel maja istotna obecnosc rynkowa. | Nie sa w zakresie tego dokumentu jako adaptery, ale trzeba je trzymac na radarze, bo moga byc wazniejsze globalnie niz CyberPanel. |

## 3. Model mentalny: adapter vs preset

### 3.1 Adapter panelu

Adapter jest kodem Go implementujacym `providers.HostingProvider`.

Przyklady:

- `smallhost` - Devil CLI przez SSH.
- `cpanel` - UAPI przez HTTPS `:2083` i/lub `uapi` przez SSH.
- `directadmin` - nowe `/api/*` + legacy `/CMD_API_*` fallback.
- `cyberpanel` - CLI `cyberpanel` przez SSH + ostrozny API fallback.

Adapter odpowiada na pytanie: **jak rozmawiac z panelem?**

### 3.2 Preset hostera

Preset to dane konfiguracyjne i oczekiwania dla konkretnego hostera albo wariantu infrastruktury.

Preset odpowiada na pytania:

- Czy hoster daje SSH?
- Czy Node.js jest przez Passenger, CloudLinux Selector, Nginx Unit, PM2, systemd user, czy brak?
- Gdzie trafia deploy?
- Gdzie sa logi?
- Jak restartowac?
- Jak tworzyc SSL?
- Czy API tokeny sa wlaczone?
- Jakie sa limity i znane pulapki?

### 3.3 Capability detection

Preset nigdy nie moze byc slepo zaufany. Webox po wybraniu presetu uruchamia probe:

1. SSH/API connectivity.
2. Panel identity: `uapi --version`, DirectAdmin `/static/swagger.json`, `cyberpanel -h`, `devil --version`.
3. Node runtime:
   - `uapi PassengerApps/list_applications`,
   - `cloudlinux-selector --json --interpreter nodejs` / `selectorctl` / `nodejs_selector` (TO BE VERIFIED per panel),
   - DirectAdmin Nginx Unit API presence,
   - `pm2 --version`,
   - `systemctl --user status`.
4. SSL capability:
   - cPanel AutoSSL / Let's Encrypt availability,
   - DirectAdmin Let's Encrypt endpoint,
   - CyberPanel `issueSSL`.
5. Database capability:
   - MySQL/MariaDB always first,
   - PostgreSQL optional and never assumed.
6. Safe paths:
   - deploy path exists or can be created by panel,
   - `.env` outside web root,
   - log path readable.

Preset status po probe:

| Status | Znaczenie |
|---|---|
| `verified` | Preset potwierdzony fixture'ami na realnym koncie hostera. |
| `detected` | Webox rozpoznal wariant runtime na koncie uzytkownika, ale preset nie ma jeszcze oficjalnych fixture'ow. |
| `partial` | Panel dziala, ale brakuje czesci capabilities, np. brak Node.js Selector. |
| `unsupported` | Brak SSH/API albo brak bezpiecznego restartu/deploy path. |
| `dangerous` | Operacja wymagalaby root/sudo, globalnego restartu lub edycji systemowego configu. Domyslnie blokowane. |

## 4. Priorytety paneli

| Priorytet | Panel | Powod | Warunek wejscia do core |
|---|---|---|---|
| P0 | `smallhost` | Wlasny bol autora i realny MVP. | Juz w zakresie `v0.1`. |
| P1 | `cpanel` | Najlepsze API, globalna rozpoznawalnosc, duzo hosterow z Node.js/Passenger. | Minimum 2 hosterow z fixture'ami: 1 PL/EU + 1 USA/UK. |
| P2 | `directadmin` | Popularny w Europie i wsrod tanszych resellerow. | Rozstrzygnac CloudLinux Selector vs Nginx Unit vs legacy-only. |
| P3 | `cyberpanel` | OpenLiteSpeed, self-hosted, tanie VPS, community. | Tylko jesli per-app restart i deploy bez globalnego restartu OLS sa bezpieczne. |
| Radar | `plesk` | Silna pozycja globalna i Node.js Toolkit. | Osobny research po `v0.2`; moze przeskoczyc CyberPanel. |
| Radar | `hpanel` | Hostinger ma duzy zasieg, ale panel jest proprietary. | Tylko jesli istnieje publiczne API/SSH workflow zgodne z Webox. |

## 5. Presety: cPanel

### 5.1 Warianty runtime cPanel

| Preset bazowy | Runtime Node.js | Restart | Deploy path | Ryzyko |
|---|---|---|---|---|
| `cpanel-passenger-app-manager` | cPanel Application Manager + Passenger | `passenger-config restart-app` lub `touch tmp/restart.txt` | app root z UAPI PassengerApps | Najlepszy core target. |
| `cpanel-cloudlinux-selector` | CloudLinux Node.js Selector | selector/app manager + Passenger | app root selectora | Bardzo czesty u shared hosterow, ale CLI do selectora wymaga weryfikacji. |
| `cpanel-static-only` | Brak runtime Node; tylko static build | brak restartu | `public_html` albo addon domain root | Dobry dla React/Vite/Next static export. |
| `cpanel-ssh-pm2` | Reczny Node + PM2 po SSH | `pm2 restart` | user-defined app dir | Tylko gdy hoster jawnie wspiera dlugie procesy. |

### 5.2 Kandydaci presetow cPanel

| Preset ID | Hoster / rynek | Panel/runtime | Domyslne properties | Status |
|---|---|---|---|---|
| `cpanel-cyberin-pl` | Cyberin, PL/global | cPanel, deklarowane SSH/Git/Node.js | `runtime=cloudlinux_selector`, `restart_method=passenger`, `api_port=2083`, `ssl_provider=autossl` | `TO BE VERIFIED` |
| `cpanel-netcloud24-pl` | NetCloud24, PL | Node.js przez SSH/PM2, LiteSpeed | `runtime=pm2`, `restart_method=pm2`, `webserver=litespeed`, `ssl_provider=letsencrypt` | `TO BE VERIFIED`; panel wymaga potwierdzenia. |
| `cpanel-unlimitedwebhosting-uk` | UK | cPanel Premium + Node.js Selector/Passenger, SSH, Git | `runtime=cloudlinux_selector`, `restart_method=passenger`, `node_versions=host_detected` | `TO BE VERIFIED` |
| `cpanel-ultrawebhosting-us` | USA | cPanel + CloudLinux Node.js Selector + Passenger, SSH, Git Version Control | `runtime=cloudlinux_selector`, `restart_method=passenger`, `ssl_provider=autossl` | `TO BE VERIFIED` |
| `cpanel-040hosting-eu-reseller` | EU / reseller | cPanel/WHM + CloudLinux + NodeJS/Python/Ruby | `runtime=cloudlinux_selector`, `reseller=true`, `restart_method=passenger` | `TO BE VERIFIED` |
| `cpanel-asura-uk` | UK/global | cPanel lub DirectAdmin, CloudLinux Node launcher, SSH | `runtime=cloudlinux_selector`, `panel_choice=cpanel`, `restart_method=passenger` | `TO BE VERIFIED` |

### 5.3 cPanel capability probe

Minimalny probe:

```text
1. GET https://<host>:2083/execute/Version/get_version
2. GET /execute/PassengerApps/list_applications
3. GET /execute/Tokens/list
4. GET /execute/Mysql/list_databases
5. GET /execute/SSL/installed_hosts
```

Fallback SSH probe:

```text
1. command -v uapi
2. uapi --output=json Version get_version
3. uapi --output=json PassengerApps list_applications
4. command -v passenger-config
5. command -v cloudlinux-selector || command -v selectorctl || command -v nodejs_selector
```

Otwarte pytania:

- Czy Webox powinien preferowac HTTPS UAPI z tokenem, czy SSH `uapi` bez tokena?
- Jak ujednolicic Application Manager i CloudLinux Node.js Selector w jednym `CreateSubdomain`?
- Czy tworzenie cPanel API tokena przez Webox jest dopuszczalne, czy user powinien wkleic token wygenerowany w panelu?
- Jak bezpiecznie obslugiwac `PassengerStartupFile`, gdy startup file nie nazywa sie `app.js`?

### 5.4 cPanel product angle

cPanel powinien byc sprzedawany w Webox jako:

> "Masz cPanel z Node.js? Webox zamienia Application Manager, SSL, bazy, GitHub Actions i logi w jeden terminalowy cockpit."

To jest potencjalnie najsilniejszy zagraniczny argument marketingowy. Hosterzy cPanel czesto maja identyczny panel, ale kazdy tutorial deployu Node.js jest inny. Webox moze byc warstwa normalizujaca.

## 6. Presety: DirectAdmin

### 6.1 Warianty runtime DirectAdmin

| Preset bazowy | Runtime Node.js | Restart | Deploy path | Ryzyko |
|---|---|---|---|---|
| `directadmin-cloudlinux-selector` | CloudLinux Node.js Selector | selector/Passenger restart | app root selectora | Najlepszy shared-hosting target. |
| `directadmin-nginx-unit` | DirectAdmin Nginx Unit | Unit API restart/reload | app root Unit | Nowoczesny, ale endpointy do automatyzacji trzeba potwierdzic. |
| `directadmin-passenger-custom` | Apache/LiteSpeed + Passenger custom templates | `touch tmp/restart.txt` | custom app root | Duza wariancja hosterow. |
| `directadmin-static-only` | Brak Node runtime | brak restartu | `domains/<domain>/public_html` | Dobry fallback dla static builds. |

### 6.2 Kandydaci presetow DirectAdmin

| Preset ID | Hoster / rynek | Panel/runtime | Domyslne properties | Status |
|---|---|---|---|---|
| `directadmin-hitme-pl` | HITME.pl, PL | DirectAdmin + "Setup Node.js App" wedlug publicznej instrukcji | `runtime=cloudlinux_selector`, `restart_method=passenger`, `api_port=2222` | `TO BE VERIFIED` |
| `directadmin-nodea-pl` | NODEA, PL | DirectAdmin, SSH deklarowane w ofercie | `runtime=unknown`, `restart_method=detect`, `api_port=2222` | `TO BE VERIFIED`; Node.js wymaga potwierdzenia. |
| `directadmin-dotroll-hu` | DotRoll, HU/EU | DirectAdmin, SSH/Git, starsze wersje Node deklarowane | `runtime=legacy_node`, `restart_method=detect`, `node_version_policy=host_old` | `TO BE VERIFIED`; moze byc static/legacy only. |
| `directadmin-uniquehosting` | International | DirectAdmin + CloudLinux Node.js Selector, SSH | `runtime=cloudlinux_selector`, `restart_method=passenger` | `TO BE VERIFIED` |
| `directadmin-domainindia` | India/global | DirectAdmin + CloudLinux Node.js Selector | `runtime=cloudlinux_selector`, `restart_method=passenger` | `TO BE VERIFIED` |
| `directadmin-asura-uk` | UK/global | DirectAdmin lub cPanel, CloudLinux Node launcher, SSH | `runtime=cloudlinux_selector`, `panel_choice=directadmin` | `TO BE VERIFIED` |

### 6.3 DirectAdmin capability probe

Nowe API:

```text
1. GET https://<host>:2222/static/swagger.json
2. GET /api/login-keys/commands
3. Probe domains/subdomains endpoint from swagger
4. Probe databases endpoint from swagger
5. Probe SSL endpoint from swagger
```

Legacy fallback:

```text
1. GET /CMD_API_SHOW_DOMAINS?json=yes
2. POST /CMD_API_SUBDOMAINS action=create|delete
3. POST /CMD_API_DATABASES action=create|delete
4. POST /CMD_API_SSL action=save&letsencrypt=1
```

SSH runtime probe:

```text
1. command -v nodejs_selector
2. command -v cloudlinux-selector
3. command -v unitc
4. curl --unix-socket /var/run/control.unit.sock http://localhost/config
5. command -v passenger-config
```

Otwarte pytania:

- Czy Login Key moze byc utworzony przez user-level API bez przechowywania glownego hasla?
- Czy nowe API ma wystarczajace endpointy dla subdomen, DB i SSL na przecietnym hostingu, czy trzeba od razu wspierac legacy?
- Czy DirectAdmin Nginx Unit daje userowi wystarczajaca kontrole bez roota?
- Jak rozpoznac hosty z przestarzalymi Node 6/8/10 i czy w ogole je wspierac?

### 6.4 DirectAdmin product angle

DirectAdmin powinien byc pozycjonowany jako:

> "Tansze hosty i resellerzy tez dostaja jeden workflow deployu, nawet jesli kazdy DirectAdmin ma inna warstwe Node.js."

To nie jest tak latwy rynek jak cPanel, ale jest strategicznie wazny w Europie i w PL, bo duzo mniejszych hosterow ucieka od kosztow cPanel.

## 7. Presety: CyberPanel

### 7.1 Warianty runtime CyberPanel

| Preset bazowy | Runtime Node.js | Restart | Deploy path | Ryzyko |
|---|---|---|---|---|
| `cyberpanel-ols-static` | Static/PHP under OpenLiteSpeed | OLS reload niepotrzebny | `/home/<domain>/public_html` | Najbezpieczniejszy, ale nie pelny Node. |
| `cyberpanel-pm2-nodecyber` | NodeCyber + PM2 + OLS reverse proxy | `pm2 restart` | app dir wskazany przez NodeCyber | Wymaga root/sudo i third-party tool. |
| `cyberpanel-manual-pm2` | PM2 recznie + OLS proxy | `pm2 restart` | user-defined | Wymaga stabilnej konwencji hostera. |
| `cyberpanel-root-cli` | CyberPanel CLI + root | `cyberpanel ...` + OLS reload | `/home/<domain>/...` | Dangerous by default; globalne efekty uboczne. |

### 7.2 Kandydaci presetow CyberPanel

| Preset ID | Hoster / rynek | Panel/runtime | Domyslne properties | Status |
|---|---|---|---|---|
| `cyberpanel-selfhosted-hetzner` | Self-hosted VPS EU | CyberPanel + root SSH + OpenLiteSpeed | `runtime=manual_pm2`, `restart_method=pm2`, `requires_root=true` | `EXPERIMENTAL / TO BE VERIFIED` |
| `cyberpanel-selfhosted-ovh` | Self-hosted VPS EU/PL | CyberPanel + root SSH | `runtime=manual_pm2`, `restart_method=pm2`, `requires_root=true` | `EXPERIMENTAL / TO BE VERIFIED` |
| `cyberpanel-nodecyber` | Dowolny CyberPanel z NodeCyber | NodeCyber + PM2 | `runtime=nodecyber`, `restart_method=pm2`, `ols_proxy_managed=true` | `EXPERIMENTAL`; third-party dependency. |
| `cyberpanel-static-shared` | Shared CyberPanel | Static/PHP only | `runtime=static_only`, `restart_method=none` | `TO BE VERIFIED`; ograniczona wartosc dla Webox Node MVP. |
| `cyberpanel-agency-vps` | Agencja z wlasnym VPS | CyberPanel admin/root | `runtime=pm2`, `requires_sudo=true`, `dangerous_ops_confirm=true` | `EXPERIMENTAL`; bardziej v0.4+ niz v0.3. |

### 7.3 CyberPanel capability probe

```text
1. command -v cyberpanel
2. cyberpanel listWebsites --json (TO BE VERIFIED)
3. cyberpanel createDatabase --help
4. cyberpanel issueSSL --help
5. command -v pm2
6. command -v nodecyber
7. test -w /usr/local/lsws/conf || sudo -n true
```

Danger blockers:

- Wymagany root do edycji OpenLiteSpeed configu.
- Restart `/usr/local/lsws/bin/lshttpd -r` dotyczy wielu vhostow.
- CLI moze zwracac `success: 1` mimo czesciowego bledu.
- API `/cloudAPI/` bywa slabo udokumentowane i mialo publiczne problemy bezpieczenstwa.

### 7.4 CyberPanel product angle

CyberPanel nie powinien byc obiecywany jako "shared hosting mainstream". Lepiej:

> "Eksperymentalne wsparcie dla self-hosted CyberPanel/OpenLiteSpeed dla osob, ktore maja root i akceptuja jawne potwierdzenia operacji globalnych."

Jesli Webox ma byc narzedziem dla shared hostingu, CyberPanel jest mniej naturalny niz cPanel, DirectAdmin i prawdopodobnie Plesk.

## 8. Format preset registry

Docelowo presety powinny byc wbudowane w binarke przez `embed.FS`, analogicznie do workflow templates.

Proponowany format `assets/provider-presets/*.json`:

```json
{
  "schema_version": 1,
  "id": "cpanel-ultrawebhosting-us",
  "display_name": "Ultra Web Hosting (cPanel Node.js)",
  "provider_type": "cpanel",
  "status": "research",
  "markets": ["US", "global"],
  "panel": {
    "name": "cPanel",
    "api": "uapi",
    "api_port": 2083,
    "ssh_required": true
  },
  "capabilities": {
    "node_runtime": "cloudlinux_selector",
    "restart_method": "passenger",
    "ssl_provider": "autossl",
    "database_engines": ["mysql"],
    "git_available": true,
    "sftp_available": true
  },
  "paths": {
    "deploy_path_template": "/home/{{user}}/{{app_root}}/public",
    "log_path_template": "/home/{{user}}/{{app_root}}/logs",
    "env_path_template": "/home/{{user}}/{{app_root}}/.env"
  },
  "probes": [
    "uapi --output=json Version get_version",
    "uapi --output=json PassengerApps list_applications",
    "command -v passenger-config"
  ],
  "known_risks": [
    "Application Manager may be disabled in WHM Feature Manager",
    "Only one ea-nodejs package may be installed at a time"
  ],
  "sources": [
    "https://docs.cpanel.net/cpanel/software/application-manager/",
    "https://www.ultrawebhosting.com/nodejs-hosting"
  ],
  "verified": {
    "fixture_dir": "",
    "last_verified_at": "",
    "verified_by": ""
  }
}
```

### 8.1 Statusy presetow

| Status | Kiedy |
|---|---|
| `research` | Mamy publiczne zrodla, brak konta testowego. |
| `candidate` | Mamy konto testowe, probe przechodzi, fixture'y niekompletne. |
| `verified` | Komplet fixture'ow + testy adaptera + manual checklist. |
| `deprecated` | Hoster zmienil panel/runtime albo preset jest niebezpieczny. |
| `community` | Utrzymywany przez zewnetrznego maintainera, core tylko waliduje format. |

### 8.2 Zasada bezpieczenstwa presetow

Preset nie moze zawierac:

- tokenow API,
- hasel,
- prywatnych kluczy SSH,
- realnych domen klientow,
- realnych loginow uzytkownikow,
- fingerprintow host key bez jawnego procesu weryfikacji.

Preset moze zawierac:

- nazwe hostera,
- publiczne host patterny,
- porty panelu,
- typ runtime,
- szablony sciezek,
- liste probe commands,
- linki do dokumentacji.

## 9. UX: wybor providera jako powod zakupu/adopcji

### 9.1 Provider marketplace bez plugin marketplace

Nie robimy dynamicznych pluginow. Ale mozemy zrobic "Provider Catalog" jako UI:

```text
Choose hosting provider

Poland
  small.pl / Devil                         verified
  HITME.pl / DirectAdmin Node.js           research
  NetCloud24 / Node.js SSH+PM2             research
  Cyberin / cPanel Node.js                 research

Europe
  UnlimitedWebHosting UK / cPanel          research
  040Hosting EU / cPanel Reseller          research
  DotRoll HU / DirectAdmin                 research

Global
  Ultra Web Hosting / cPanel               research
  DomainIndia / DirectAdmin CloudLinux     research

Advanced
  Generic cPanel UAPI                      candidate
  Generic DirectAdmin API                  candidate
  Generic CyberPanel CLI                   experimental
```

Po wyborze Webox pokazuje:

- co zostanie wykryte automatycznie,
- czego nie wiemy,
- jakie operacje moga byc destrukcyjne,
- jakie dane beda zapisane w `config.json`,
- jakie sekrety trafia do keyringa.

### 9.2 Komunikat marketingowy

Najmocniejszy claim:

> "Webox zna Twoj hosting, nie tylko Twoj panel."

Lepsze niz:

> "Supports cPanel, DirectAdmin, CyberPanel."

Bo uzytkownik nie mysli "mam implementacje UAPI", tylko "mam hosting X i chce, zeby dzialal".

### 9.3 Capability badges

Na liscie presetow:

| Badge | Znaczenie |
|---|---|
| `SSH` | Webox moze wykonac komendy i tail logow. |
| `API` | Panel ma stabilne API dla zasobow. |
| `Node` | Hoster ma runtime Node.js wspierany przez Webox. |
| `SSL` | Webox moze wystawic/odnowic cert. |
| `DB` | Webox moze utworzyc baze i usera. |
| `Logs` | Webox zna sciezke logow. |
| `Safe Restart` | Restart dotyczy aplikacji, nie calego serwera. |
| `Fixtures` | Mamy testy na realnych outputach. |

## 10. Community capture kit

Provider support skaluje sie tylko wtedy, gdy uzytkownik moze pomoc bez rozumienia calego Go codebase.

Minimalny kit:

```text
webox doctor provider-capture
  --provider cpanel
  --preset cpanel-generic
  --redact
  --output ./webox-provider-capture.zip
```

Zakres capture:

- wersja panelu,
- lista dostepnych endpointow/capabilities,
- output probe commands,
- struktura sciezek bez realnych sekretow,
- status Node runtime,
- status SSL,
- status DB engines,
- host key fingerprint tylko po explicit confirm i tylko jesli potrzebny do debug.

Wazne: capture kit nie moze wysylac nic automatycznie. User sam dolacza zip do GitHub Issue.

## 11. Roadmap provider support

### v0.1

- `smallhost` jako jedyny realny adapter.
- `docs/providers/*.md` zostaja research-only dla innych paneli.
- Provider Catalog moze istniec tylko jako docs/landing material, nie jako obietnica UI.

### v0.2

- `cpanel` jako drugi provider.
- Minimum:
  - generic cPanel UAPI preset,
  - 2 hoster presets z fixture'ami,
  - Application Manager / Passenger flow,
  - static-only fallback.
- DirectAdmin pozostaje research, chyba ze cPanel blokuje sie na tokenach/API dostepnosci.

### v0.3

- `directadmin` jako trzeci provider.
- Minimum:
  - generic DirectAdmin API preset,
  - CloudLinux Selector preset,
  - Nginx Unit spike,
  - legacy API fallback tylko tam, gdzie nowe API nie ma endpointu.

### v0.4+

- CyberPanel experimental albo Plesk research, w zaleznosci od realnego feedbacku.
- Provider Catalog z capability detection.
- Capture kit dla community.
- Publiczna dokumentacja EN: "How to add a preset" osobno od "How to add an adapter".

## 12. Kryteria awansu presetu do verified

Preset moze dostac `verified` tylko gdy:

1. Mamy testowe konto lub instancje hostera.
2. Kazda operacja mapowana do `HostingProvider` ma fixture outputu.
3. Kazdy fixture ma `.fixture.md` z data, komenda, sanityzacja.
4. Parser ma malicious fixture.
5. `Remove*` jest idempotentne albo adapter ma bezpieczny workaround.
6. Restart nie dotyka innych aplikacji na serwerze.
7. `.env` jest poza web root i ma `0600`.
8. Brak sekretow w `config.json`, logach, fixture'ach i trace.
9. Manual checklist potwierdza:
   - create project,
   - SSL,
   - DB,
   - deploy,
   - restart,
   - logs,
   - rollback.
10. Hoster nie wymaga roota dla operacji shared-hostingowych.

## 13. Krytyczne ryzyka

### 13.1 Nazwa panelu nie determinuje runtime

cPanel moze miec Application Manager, CloudLinux Selector, albo brak Node. DirectAdmin moze miec CloudLinux Selector, Nginx Unit, custom Passenger albo nic. CyberPanel moze wymagac root.

Mitigacja: capability detection przed pierwszym zapisem.

### 13.2 API bywa wylaczone przez hostera

cPanel API tokens i Application Manager moga byc disabled w WHM Feature Manager. DirectAdmin Live API docs moga istniec, ale nie wszystkie endpointy beda dostepne na poziomie usera.

Mitigacja: Webox musi miec fallback SSH tam, gdzie to bezpieczne, i jasny komunikat `hoster disabled feature`.

### 13.3 Restart moze byc globalny

CyberPanel/OpenLiteSpeed global reload albo DirectAdmin custom setup moga restartowac wiecej niz jedna aplikacje.

Mitigacja: takie operacje sa `dangerous`, wymagaja explicit confirm i nie sa defaultem.

### 13.4 Stare wersje Node

Niektorzy hosterzy deklaruja Node 6/8/10. Webox nie powinien udawac, ze to dobry runtime dla nowych projektow.

Mitigacja: `node_version_policy`:

- `modern`: 18/20/22/24,
- `legacy`: ponizej 18, tylko import/static,
- `unsupported`: brak bezpiecznego runtime.

### 13.5 Hoster marketing != realne capability

Strona moze mowic "Node.js support", ale w praktyce oznacza reczne procesy przez SSH bez process managera.

Mitigacja: preset status `research` dopoki nie ma fixture'ow i realnego testu.

## 14. Najwazniejsze decyzje produktowe do podjecia

1. Czy `cpanel` w `v0.2` ma uzywac domyslnie HTTPS UAPI z tokenem, czy SSH `uapi`?
2. Czy Webox moze tworzyc API tokeny w panelu, czy tylko przyjmowac token wklejony przez usera?
3. Czy `static-only` presety liczymy jako pelne wsparcie providera, czy jako ograniczony tryb?
4. Czy provider catalog ma pokazywac `research` presety uzytkownikom, czy tylko contributorom?
5. Czy CyberPanel ma byc core providerem, skoro tak czesto wymaga root/sudo?
6. Czy po `directadmin` nie nalezy priorytetowo zbadac Plesk przed CyberPanel?
7. Jakie minimum hosterow `verified` jest potrzebne, zeby mowic marketingowo "supports cPanel"?

## 15. Rekomendacja

Rekomendowany kierunek:

1. **Zostawic MVP ostre:** small.pl/Devil only, zero drugiego adaptera przed stabilnym `v0.1`.
2. **Po MVP uderzyc w cPanel:** najlepszy stosunek rynku, dokumentacji i API.
3. **Budowac preset registry od razu przy cPanel:** bez tego Webox stanie sie "generic UAPI client", a nie produktem z gotowymi odpowiedziami dla hosterow.
4. **DirectAdmin jako trzeci provider:** wazny w PL/EU, ale wymaga wiecej capability detection.
5. **CyberPanel ostroznie:** dobry dla self-hosted power users, slaby jako mainstream shared-hosting target.
6. **Provider support mierzyc fixture'ami, nie checkboxami:** kazdy "verified" preset musi miec dowody w repo.
7. **Sprzedawac konkrety:** "dziala z Twoim hosterem" jest silniejsze niz "ma pluginy".

Najlepsza wersja Webox za rok to nie narzedzie z trzema polowicznymi adapterami. To narzedzie, w ktorym freelancer wybiera `small.pl`, `Cyberin cPanel`, `HITME DirectAdmin` albo `Ultra Web Hosting cPanel`, widzi zielone capability badges i po 5 minutach ma projekt online.
