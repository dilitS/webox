# Webox — Dokumentacja

> Status: Stable navigation guide · Ostatnia aktualizacja: 2026-05-22 · Właściciel: @maintainer

## TL;DR

Ten katalog to **źródło prawdy** dla decyzji produktowych i architektonicznych Webox. Jeśli root [`README.md`](../README.md) odpowiada na pytanie "czym jest ten projekt?", to `docs/` odpowiada na pytanie "jak dokładnie ma działać, dlaczego tak i gdzie są granice zakresu?".

MVP (`v0.1`) celuje wyłącznie w **small.pl / Devil**, ale dokumentacja od dnia 1 opisuje system tak, żeby późniejsze providery nie wymagały przepisywania całej architektury.

## Od czego zacząć

Jeśli trafiasz tu pierwszy raz, nie czytaj losowo.

| Rola / potrzeba | Czytaj najpierw | Potem |
|---|---|---|
| Chcesz zrozumieć sens projektu | [PRD.md](./PRD.md) | [UX.md](./UX.md), [ROADMAP.md](./ROADMAP.md) |
| Chcesz wejść w implementację | [DESIGN.md](./DESIGN.md) | [TESTING.md](./TESTING.md), [CONTRIBUTING.md](./CONTRIBUTING.md) |
| Chcesz zrobić review bezpieczeństwa | [SECURITY.md](./SECURITY.md) | [adr/0004-przechowywanie-sekretow-keyring.md](./adr/0004-przechowywanie-sekretow-keyring.md), [DESIGN.md](./DESIGN.md) |
| Chcesz dodać nowego providera | [providers/smallhost.md](./providers/smallhost.md) | [CONTRIBUTING.md](./CONTRIBUTING.md), wybrane ADR-y |
| Chcesz zobaczyć jak monolit został rozbity | [MIGRATION_NOTES.md](./MIGRATION_NOTES.md) | [CHANGES.md](../CHANGES.md) |

## Mapa dokumentacji

Każdy plik odpowiada na inne pytanie.

| Pytanie | Plik |
|---|---|
| Co to za narzędzie, kto je dostanie i dlaczego ma sens? | [PRD.md](./PRD.md) |
| Jak to jest zbudowane wewnątrz: architektura, kontrakty, model danych? | [DESIGN.md](./DESIGN.md) |
| Jak wygląda interfejs i jak porusza się po nim użytkownik? | [UX.md](./UX.md) |
| Jakie są zagrożenia i polityki bezpieczeństwa? | [SECURITY.md](./SECURITY.md) |
| Jak to testujemy i co przechodzi przez CI? | [TESTING.md](./TESTING.md) |
| Co jest w MVP, co później, a czego nie robimy? | [ROADMAP.md](./ROADMAP.md) |
| Jak wnosić kod, tłumaczenia i nowych providerów? | [CONTRIBUTING.md](./CONTRIBUTING.md) |
| Dlaczego wybraliśmy konkretne decyzje architektoniczne? | [adr/](./adr/) |
| Jak działa referencyjny provider MVP? | [providers/smallhost.md](./providers/smallhost.md) |
| Co wiemy o providerach post-MVP? | [providers/cpanel.md](./providers/cpanel.md), [providers/directadmin.md](./providers/directadmin.md), [providers/cyberpanel.md](./providers/cyberpanel.md) |
| Jakie luki znaleziono przed implementacją i jak je zamknięto? | [AUDIT.md](./AUDIT.md) |
| Jak oryginalny monolit został rozłożony na obecne docs? | [MIGRATION_NOTES.md](./MIGRATION_NOTES.md) |

## Status projektu

```text
Koncepcja  →  Pre-MVP  →  MVP (v0.1)  →  v0.2  →  v0.3+  →  GA (v1.0)
                          ▲
                          tutaj jesteśmy
```

Istnieje już realny ból użytkownika i wcześniejszy shell proof-of-concept dla `small.pl`. Celem obecnej dokumentacji nie jest "wymyślić coś ładnego", tylko przekuć to w narzędzie, które da się implementować bez chaosu i bez rozjechania zakresu.

## Zasady tej dokumentacji

To nie jest luźny zestaw notatek. Ten katalog ma kilka twardych reguł:

1. **PRD opisuje co i dlaczego, a nie kod.**
2. **DESIGN opisuje kontrakty i granice modułów, a nie przypadkowe szczegóły implementacji.**
3. **UX pokazuje tylko to, co użytkownik faktycznie zobaczy i zrobi.**
4. **SECURITY i TESTING nie są dodatkami na końcu projektu, tylko częścią definicji produktu.**
5. **ADR-y zapisują decyzje, które inaczej wróciłyby w nieskończonych dyskusjach.**
6. **Provider docs bez realnej weryfikacji mają status research, nie "prawie gotowe".**

## Słowniczek pojęć

| Skrót / pojęcie | Znaczenie |
|---|---|
| **TUI** | Text User Interface — UI rysowane w terminalu, np. `lazygit`, `k9s`. |
| **MVU** | Model–View–Update — pętla zdarzeń stosowana przez Bubble Tea. |
| **Provider Pattern** | Jeden interfejs `HostingProvider`, wiele adapterów dla różnych paneli. Patrz [DESIGN.md §3](./DESIGN.md#3-provider-pattern). |
| **SFTP** | Kanał plikowy nad SSH. |
| **Devil** | CLI panelu hostingowego small.pl. |
| **TOFU** | Trust On First Use — pierwsza akceptacja host key, potem ścisła weryfikacja. |
| **ADR** | Architecture Decision Record — zapis jednej decyzji z kontekstem i konsekwencjami. |
| **Keyring** | Systemowy magazyn sekretów. |
| **GHA** | GitHub Actions — standardowy kanał deploymentu kodu w Webox. |
| **Stale project** | Projekt obecny w configu, ale ręcznie usunięty z panelu lub serwera. |

## Jak czytać te docs skutecznie

1. Zacznij od produktu, nie od architektury: [PRD.md](./PRD.md).
2. Potem przejdź do doświadczenia użytkownika: [UX.md](./UX.md).
3. Dopiero potem wchodź w technikalia: [DESIGN.md](./DESIGN.md).
4. Jeśli dotykasz sekretów, SSH albo `.env`, czytaj równolegle [SECURITY.md](./SECURITY.md).
5. Jeśli planujesz implementację, nie pomijaj [TESTING.md](./TESTING.md) i [CONTRIBUTING.md](./CONTRIBUTING.md).

## Uwaga praktyczna

Publiczny, repozytoryjny opis projektu jest w root [`README.md`](../README.md).  
Ten plik jest wewnętrzną bramą do dokumentacji, a nie marketingowym landing page'em repo.
