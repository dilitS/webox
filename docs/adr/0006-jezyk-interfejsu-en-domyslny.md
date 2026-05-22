# ADR-0006: Język interfejsu — angielski domyślnie, polski opcjonalnie

> Status: Accepted · Data: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne ADR: [ADR-0003 Provider Pattern](./0003-provider-pattern.md). Dokumenty: [UX §10](../UX.md#10-internacjonalizacja), [CONTRIBUTING §4](../CONTRIBUTING.md#4-jak-doda%C4%87-t%C5%82umaczenie).

## Kontekst

Webox jest open-source narzędziem dla deweloperów hostingu współdzielonego. Pierwotne PRD miało **niespójność**: deklarowało „default English", ale wszystkie mockupy były po polsku. Trzeba ustalić jeden język domyślny i strategię i18n.

Czynniki:

- Większość docelowych userów MVP to PL (small.pl to polski hosting).
- Społeczność contributorów Go jest globalna; community-provided providery (cPanel, DA) wymagają zrozumiałego dla wszystkich UI.
- Pisanie aplikacji `de-novo` po polsku spowoduje pain przy tłumaczeniu na EN później.
- Pisanie po angielsku z PL jako opt-in pozwala każdej narodowości dorzucać swoje tłumaczenia.

Opcje:

1. PL domyślnie, EN jako tłumaczenie.
2. EN domyślnie, PL jako tłumaczenie.
3. Dwujęzyczność hardcoded (PL + EN side-by-side w UI).
4. Auto-detect z `LANG` env var.

## Decyzja

**English** jest domyślny. **Polski** dostępny jako `settings.language = "pl"` lub flagą `--lang=pl`. Pierwszy dodatkowy pakiet językowy = `translations/pl.json`.

`LANG` env var jest **respektowany** — jeśli `LANG=pl_PL.UTF-8` i `pl.json` istnieje, webox auto-wybiera PL. User może override w `/settings`.

Wszystkie stringi w kodzie żyją jako klucze (`dashboard.title`, `wizard.step1.title`); brak hardcoded'owanych wartości w `view.go`.

## Dlaczego EN domyślnie

### Open-source community

- Contributorzy z całego świata. PL onboarding zniechęca 95 % populacji deweloperów.
- Code comments, error messages, log lines i tak są po angielsku (idiom Go).
- Issues / PR / Discussions po angielsku — UI po polsku łamie spójność.

### Międzynarodowy zasięg target persony

- Persona B (agencja) zarządza projektami klientów z różnych krajów; wspólny język UI w team'ie = EN.
- Webox aspiruje do bycia używanym z DirectAdmin (większy rynek europejski/azjatycki) — PL by tam zaszkodziło.

### Translation workflow

- Pisanie EN-first → drugi język to czyste tłumaczenie kluczy.
- PL-first → angielskie tłumaczenie jest "podrabianą wersją" oryginału (gorsza jakość terminologii technicznej).

### Terminologia techniczna

- "Provider", "host key", "rollback", "deploy" — i tak EN we wszystkich kontekstach polskich. Polskie tłumaczenia ("dostawca", "klucz hosta", "wycofanie") są niezgrabne lub niespotykane.
- Mieszanie EN-terms w PL-stringach jest gorsze niż wszystko po EN.

## Dlaczego nie inne opcje

### PL domyślnie

Powyżej. Plus: trudniej zwerbować contributorów globalnych do projektu, który zaczął jako "polski".

### Dwujęzyczność side-by-side

Terminal jest wąski; podwajanie tekstu = niemożliwy layout dla 100×30.

### Auto-detect bez fallback'u na EN

Niebezpieczne — `LANG=de_DE` użytkownik zobaczyłby niemiecki (którego nie mamy), webox wybierałby random. Decyzja: detect → EN jeśli `<lang>.json` brakuje.

## Konsekwencje

### Pozytywne

- Onboarding contributorów = brak bariery językowej.
- Spójność z kontekstem dev (logi, komenty, GH).
- Łatwiejsze marketing'owo (English-first README).
- Polski jako "pierwsze tłumaczenie" wzorcowe — pokazuje community jak dodawać swoje języki.

### Negatywne

- **Polscy power-userzy** w MVP zobaczą EN domyślnie. Workaround: `--lang=pl` flag + `LANG=pl_PL.UTF-8` autodetect.
- Tłumaczenie wymaga utrzymania — synchronizacja kluczy `en.json` ↔ `pl.json` (`make i18n-check`).
- Terminologia "fachowa" (`stale project`, `properties bag`, `host key`) wymaga decyzji per-translation. PL: zostawiamy EN albo robimy slownik (`stale → przestarzały`, `host key → klucz hosta`).

### Neutralne

- Trzecie tłumaczenia (DE, ES, FR) idą tym samym sloto'em jak PL.

## Strategia migracji (jeśli kiedyś zmienimy zdanie)

`settings.language` jest persisted w configu. Jeśli kiedyś chcemy zmienić default na auto-detect z fallbackiem na lokalizację, migracja schematu (patrz [DESIGN §6.4](../DESIGN.md#64-migracje-schematu)) doda nowe pole `language_strategy` z wartością `manual` dla istniejących configów, `auto` dla nowych.

## Wpływ na docs

- **Mockupy w `UX.md` muszą być po angielsku.** ✓ Wszystkie przepisane.
- **Komunikaty błędów w `DESIGN.md` / `SECURITY.md`** — przykłady stringów EN. ✓
- **PRD / DESIGN / SECURITY / TESTING mogą pozostać PL jako dokumenty strategiczne maintainera** do czasu publicznego launch, o ile nie są jedyną drogą onboardingu zewnętrznego contributora.
- **Contributor surface musi być EN przed publicznym `v0.1`:** README, CONTRIBUTING, Provider Pattern quickstart (`DESIGN §3` albo osobny extract) i provider template muszą wystarczyć do napisania adaptera bez znajomości polskiego.
- **CONTRIBUTING § Tłumaczenia** — wyjaśnia jak dodać język.

## Alternatywy rozważane

Wszystkie powyżej. EN-domyślny + opt-in PL jest jedyną kombinacją, która:

- Otwiera projekt na globalną społeczność.
- Pozwala polskiemu userowi mieć PL w jednym kliknięciu.
- Nie wymusza wyboru "PL or EN" na maintainerach na każdym PR.
