# ADR-0003: Provider Pattern dla obsługi paneli hostingowych

> Status: Accepted · Data: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne ADR: [ADR-0001 TUI](./0001-tui-zamiast-cli.md). Dokumenty: [DESIGN §3](../DESIGN.md#3-provider-pattern), [providers/](../providers/), [CONTRIBUTING §3](../CONTRIBUTING.md#3-jak-doda%C4%87-nowy-provider).

## Kontekst

Hosting współdzielony to niezunifikowany rynek paneli — small.pl ma Devil, większość USA ma cPanel z UAPI, niektórzy europejscy hosterzy DirectAdmin, OpenLiteSpeed-owe shared hosty mają CyberPanel. Każdy panel ma:

- Inną nazwę komend (`devil www add` vs `uapi --user=X Domain adddomain ...` vs `cyberpanel createWebsite ...`).
- Inną semantykę (atomic / non-atomic, idempotent / non-idempotent).
- Inny mechanizm restartu (Devil CLI / Phusion Passenger / pm2 / systemd user units).
- Inne ścieżki plików (`/usr/home/$USER/domains/...` vs `/home/$USER/public_html/...` vs `/home/$USER/$DOMAIN/`).

Webox docelowo wspiera kilka paneli; MVP — jeden. Architekturę trzeba dobrze ustawić od dnia 1, inaczej v0.2 wymaga przepisania.

Możliwe podejścia:

1. **Monolit z `if/switch` na typ panelu** w warstwie biznesowej.
2. **Provider Pattern** — interfejs + adaptery.
3. **Plugin system** z dynamicznym ładowaniem (`.so`, Go plugins).
4. **Mikroserwisy** — webox tylko UI, każdy panel ma własną REST API.

## Decyzja

Wybieramy **Provider Pattern** w kodzie monolitu Go:

- Jeden interfejs `HostingProvider` (kontrakt w [DESIGN §3.2](../DESIGN.md#32-kontrakt--hostingprovider)).
- Adaptery jako pakiety Go (`providers/smallhost`, `providers/cpanel`, …).
- Globalny registry (`Register(name, factory)`) z lookup'em po `config.profiles[].type`.
- Wszystkie adaptery kompilowane in-tree (statyczne linkowanie).
- **Bez dynamic loading** w v1.

Plus konwencja:

- Każdy adapter implementuje **pełen** interfejs (nie częściowy — brak `panic("not implemented")`).
- Różnice między panelami tego samego rodzaju (np. cPanel reseller vs cPanel user, DA z różnymi PHP-FPM) idą do `properties` (string→string map) — patrz [DESIGN §3.3](../DESIGN.md#33-properties-bag).
- `mock` provider w `providers/mock.go` żyje obok prawdziwych — używany w testach. Jeśli dodanie nowej metody wymaga sztucznych zmian w `mock.go`, interfejs jest źle zaprojektowany.

## Dlaczego nie inne podejścia

### `if/switch` na typ panelu

Działa do 2 paneli, rozsypuje się przy 4. Każda nowa metoda wymaga `switch` w 10 miejscach. Logika biznesowa zna specyfikę each providera — brak izolacji.

### Plugin system z dynamic loading

Go ma `plugin` package, ale:

- Działa **tylko na Linux i macOS** (nie Windows).
- Wymaga zgodności wersji Go binarki webox i pluginu (rebuild plugin'u przy każdej zmianie minor Go).
- **Supply chain attack surface** — user ładuje binarkę z internetu. Trudniejsze do audit'u niż in-tree code review.

W v1 odrzucamy. W v2.x być może pojawi się **stable plugin ABI** (np. via WASM lub gRPC sidecar) — wtedy ADR zostanie zaktualizowany.

### Mikroserwisy

Webox ma być **lokalnym narzędziem deweloperskim**, nie systemem rozproszonym. Mikroserwisy = własny serwis do utrzymania per panel + sieciowa latency dla SSH-based komend. Strzał w kolano.

## Konsekwencje

### Pozytywne

- Każdy panel ma jasną granicę odpowiedzialności (jeden plik = jeden adapter).
- Testowalność: mock provider w testach, brak zewnętrznych zależności.
- Społeczność może dodawać adaptery przez PR (in-tree) — review proces zapewnia jakość.
- Wnoszenie nowego ficzeru = dodanie metody do interfejsu **+** implementacja w każdym adapterze. To wymusza, by ficzer był naprawdę uniwersalny.
- Dystrybucja: jedna binarka, brak zarządzania pluginami.

### Negatywne

- **Wymaga commitów do core repo dla nowych providerów.** Community provider nie może żyć "obok" webox. Mitygacja: jasny [CONTRIBUTING §3](../CONTRIBUTING.md#3-jak-doda%C4%87-nowy-provider) + experimental flag.
- Każda zmiana interfejsu = breaking change dla wszystkich adapterów. Mitygacja: rozszerzanie interfejsu z `default fallback` (opcjonalne metody) — patrz §11 poniżej.
- Wzrost rozmiaru binarki z każdym adapterem (~20 KB każdy). Akceptowalne (binarka MVP ~15 MB).

### Neutralne

- Wymusza dyscyplinę architektoniczną — nie zawsze wygodną dla maintainerów (nie wepchną quick fix'a omijającego interfejs).

## Rozszerzanie interfejsu w przyszłości

Dwa wzorce:

1. **Stable extension**: nowa metoda dodawana jako optional w extended interface. Adaptery dziedziczą domyślną „nie wspierane".

   ```text
   type ExtendedProvider interface {
       HostingProvider
       BackupSite(ctx context.Context, domain string) error
   }
   ```

   Logika biznesowa: `if ext, ok := provider.(ExtendedProvider); ok { ... }`.

2. **Breaking**: dodanie metody do `HostingProvider`. Wymaga MAJOR bump (patrz [ROADMAP §2.1](../ROADMAP.md#21-semver)).

W praktyce P0 ficzery wymagają wariantu (2) — preferujemy świadome breaking change, bo wymusza spójność adapterów.

## Alternatywy rozważane

Wszystkie omówione powyżej. Provider Pattern jest jedynym, który spełnia wszystkie wymagania:

- **Testowalny** w izolacji (jednostkowo na poziomie adaptera).
- **Wymusza spójność** poprzez interfejs.
- **Skalowalny** do N paneli.
- **Distrybucyjnie prosty** (jedna binarka).

## Implikacje dla MVP

- W MVP **kod adaptera `smallhost`** jest jedyny realny. Ale interfejs **musi istnieć w pełnej formie**.
- Adapter `mock` istnieje obok, używany w testach jednostkowych całej maszynerii (wizard, dashboard).
- Każdy nowy ficzer P0 musi działać przez interfejs — nie ma "shortcut'u" do specyfiki Devila poza `properties.restart_method = "devil"`.
- Jeśli ficzer P0 wymaga **specyficznej** komendy Devila niedostępnej w interfejsie — albo dodajemy ją do interfejsu (i wszystkie adaptery muszą zaimplementować), albo wyrzucamy ten ficzer z P0.
