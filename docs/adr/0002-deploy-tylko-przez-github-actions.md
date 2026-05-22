# ADR-0002: Deploy kodu tylko przez GitHub Actions

> Status: Accepted · Data: 2026-05-22 · Właściciel: @maintainer
>
> Pokrewne ADR: [ADR-0003 Provider Pattern](./0003-provider-pattern.md), [ADR-0004 Sekrety](./0004-przechowywanie-sekretow-keyring.md). Dokumenty: [PRD §6](../PRD.md#6-ficzery--z-priorytetami), [DESIGN §13](../DESIGN.md#13-integracja-z-githubem).

## Kontekst

Webox zarządza pełnym lifecycle'em projektu na hostingu współdzielonym, włącznie z deploymentem kodu. Możliwe kanały deploymentu:

1. **GitHub Actions** workflow w repo projektu → rsync/scp do serwera.
2. **Lokalny push** — webox przesyła pliki przez SFTP z maszyny dewelopera bezpośrednio na serwer.
3. **Buildy na serwerze** — `git pull` + `npm install` + restart, wszystko on-host.
4. **GitLab CI / Gitea Actions** — analogicznie do GH, ale inny vendor.
5. **Hybrid** — kod przez GHA, ale `.env` i assety przez SFTP.

Po debacie pojawiła się też **sprzeczność w monolicie PRD**: pierwsza sekcja deklarowała „deploy tylko przez GH Actions", a sub-widoki `/env` i `/storage` jawnie używają SFTP. Trzeba to rozstrzygnąć.

## Decyzja

**Regula:** deploy kodu = **GitHub Actions only** w MVP. **Operacje administracyjne** (edycja `.env`, transfer assetów do persistent storage, restart, logi) = bezpośredni SSH/SFTP z webox.

**To dwa różne kanały**, nie sprzeczność. Doprecyzowanie:

| Operacja | Kanał | Powód |
|---|---|---|
| `git push origin main` → build → deploy | GitHub Actions | Reproducible build, build artifact poza serwerem hostingowym, spójność środowisk, mała powierzchnia GH token vs SSH key na build maszynie. |
| Dodanie / zmiana sekretu aplikacji (np. `DB_PASSWORD`) | webox → lokalny secure store (`managed`) → GitHub Secrets API + opcjonalnie SFTP | GitHub Secrets są write-only targetem dla CI. Natychmiastowy sync `.env` jest możliwy tylko, jeśli Webox ma wartość lokalnie lub user poda ją w bieżącej sesji. Patrz [SECURITY.md §10.3](../SECURITY.md#103-github-secrets-jako-kana%C5%82-deployu-nie-%C5%BAr%C3%B3d%C5%82o-odczytu). |
| Podgląd / quick edit klucza w `.env` (widok `/env`) | SSH/SFTP (webox bezpośrednio) | Operacja administracyjna. Edycja może dotyczyć wartości `server-only`; Webox ostrzega, jeśli runtime i GitHub repo secrets mogą się rozjechać. |
| Upload assetów (img, video) do `public/uploads/` | SFTP (webox) | Pliki nie powinny być w Git (rozmiar). Persistent storage zarządzany ręcznie. |
| Restart aplikacji | SSH (webox) | Operacja administracyjna, immediate response, nie wymaga build artifact. |
| Tail logów | SSH (webox) | Real-time, nie ma sensu wciągać Git'a. |
| Tworzenie subdomeny | SSH + Devil CLI (webox) | Konfiguracja hostingu, nie kod. |

Tym samym `/env` i `/storage` (post-MVP) nie łamią reguły "deploy kodu = GHA" — to operacje administracyjne. Natomiast **GitHub Secrets są kanałem dystrybucji do workflow, a nie odczytywalnym vaultem** — patrz [SECURITY.md §10](../SECURITY.md#10-zarz%C4%85dzanie-env-i-sekrety-aplikacji).

> **Nota (2026-05-22):** Oryginalna tabela mówiła "Edycja `.env` = SSH/SFTP (webox bezpośrednio)". Po doprecyzowaniu architektury sekretów (SECURITY.md §10) reguła jest: GitHub Secrets = write-only deploy target dla CI, SFTP = kanał operacyjny, a lokalny secure store = jedyne miejsce, z którego Webox może bezpośrednio odczytać wartość po własnym zapisie.

## Dlaczego nie inne CI vendor

| Vendor | Powód odrzucenia w MVP |
|---|---|
| **GitLab CI** | Drugi rynek; persona z PRD korzysta z GH. Adapter GitLab API różny — wymagałby abstrakcji "CI Provider" obok "Hosting Provider". Out of MVP scope. |
| **Gitea Actions** | Samohostowane, mała baza userów w docelowej niszy. Architekturalnie kompatybilne (Actions API podobne do GH), więc adapter w przyszłości jest możliwy. |
| **Bitbucket Pipelines** | Mała baza. |
| **CircleCI / Travis / Drone** | Konfiguracja per-vendor — odpada bez "CI Provider" abstrakcji. |
| **Lokalny push przez rsync z maszyny dewelopera** | Wymaga klucza SSH na lokalnej maszynie — to mamy. **Ale**: brak reproducible builds, brak audit trail, każdy dev musi mieć identyczne `node_modules`. |

## Konsekwencje

### Pozytywne

- Spójność środowiska build (GitHub-hosted runner Ubuntu 22.04, Node version z `package.json`).
- Audit trail każdego deploymentu w GH Actions UI.
- Brak buildów na serwerze hostingowym — small.pl ma ograniczone zasoby CPU/RAM.
- Reproducibility: ten sam commit → ten sam artefakt.
- Łatwy rollback (re-run workflow z poprzedniego commit'a).
- GHA dla publicznych repo darmowe; dla prywatnych free tier 2000 min/miesiąc.

### Negatywne

- **Wymaga konta GitHub.** User na GitLab/Gitea nie może użyć webox bez migracji. Akceptujemy w MVP — patrz [PRD §11](../PRD.md#11-za%C5%82o%C5%BCenia-i-ryzyka-produktowe).
- GHA Free tier limit dla prywatnych repo (2000 min/m). Marek z 20 projektami × 10 buildów/m × 2 min = 400 min — z zapasem. Agencja z 60 projektami może uderzyć w limit.
- **Sieć:** runner GitHub musi mieć dostęp SSH do serwera hostingowego. Firewall klienta blokujący IP runnerów GH = problem. Niektóre hosty (np. small.pl) dopuszczają wszystkie IP. Inne wymagają whitelistowania zakresów IP GH. Webox nie zarządza whitelistą — to ręczna konfiguracja u providera.
- Token GitHub z `secrets:write` + `workflow:write` to dużo uprawnień. Mitygacja: fine-grained PAT (patrz [SECURITY.md §6.1](../SECURITY.md#61-wymagane-scope-fine-grained-pat--zalecane)).
- Generujemy `deploy.yml` z szablonu — user może go potem edytować ręcznie, co utrudnia upgrade szablonu. Mitygacja: webox loguje diff przy regeneracji, prosi user'a o confirm.

### Neutralne

- Adapter `CIProvider` w przyszłości może abstraktować GHA / GitLab / Gitea — architectural choice, nie blokujący.

## Alternatywy rozważane

| Alternatywa | Rozważona | Decyzja |
|---|---|---|
| Lokalny push przez SFTP | Tak | Odrzucono dla deploymentu kodu (brak audit trail, brak reproducible builds). |
| Build na serwerze | Tak | Odrzucono — small.pl ma 256 MB RAM dla buildów, `npm install` next.js przekracza. |
| Multi-CI z dnia 1 (GHA + GitLab + Gitea) | Tak | Odrzucono jako out-of-scope MVP. Możliwe w v0.3+ przez `CIProvider`. |
| Webhook push z hosting (panel triggeruje deploy) | Tak | Odrzucono — Devil nie ma webhooków; też wymagałoby publicznie eksponowanego endpointu. |
| `git pull` po SSH na serwerze | Tak | Odrzucono — build potrzebny przed pushem (z dist'em), pull tylko = strona statyczna nie buildowana. |

## Ograniczenia decyzji

- Decyzja jest ograniczona do **MVP / v0.1–v0.2**.
- W v0.3+ rozważamy `CIProvider` abstrakcję; ADR pozostanie w mocy dopóki nie powstanie zastępczy.
- Reguła "deploy kodu = GHA" jest **twarda**. Reguła "operacje administracyjne = SSH bezpośredni" jest twarda. Nie ma trzeciego trybu (np. ręczny `webox deploy` z lokalnym buildem) — w razie potrzeby user może uruchomić workflow ręcznie z GH UI.
