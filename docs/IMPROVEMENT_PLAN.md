# Webox — Plan poprawek (uzupełnienie AUDIT.md)

> Status: Draft · Ostatnia aktualizacja: 2026-05-22 · Właściciel: @maintainer
>
> **Cel:** Uzupełnia [AUDIT.md](./AUDIT.md) o znaleziska spoza jego zakresu. AUDIT.md zawiera 39 znalezisk (A1–D13) i 5 decyzji otwartych (E1–E5). Ten plik dokumentuje **19 dodatkowych znalezisk**, które AUDIT pominął lub nie rozwinął wystarczająco. Każde ma priorytet i rekomendację.

---

## Spis treści

1. [Relacja z AUDIT.md](#1-relacja-z-auditmd)
2. [Znaleziska krytyczne (blokujące implementację)](#2-znaleziska-krytyczne-blokujące-implementację)
3. [Znaleziska wysokie (niezgodności między dokumentami)](#3-znaleziska-wysokie-niezgodności-między-dokumentami)
4. [Znaleziska średnie (luki bezpieczeństwa i problemy techniczne)](#4-znaleziska-średnie-luki-bezpieczeństwa-i-problemy-techniczne)
5. [Znaleziska niskie (edge case'y i niespójności redakcyjne)](#5-znaleziska-niskie-edge-casey-i-niespójności-redakcyjne)
6. [Kwestia Persony B (agencja) — decyzja](#6-kwestia-persony-b-agencja--decyzja)
7. [Nowe decyzje otwarte](#7-nowe-decyzje-otwarte)
8. [Macierz pokrycia AUDIT vs IMPROVEMENT_PLAN](#8-macierz-pokrycia-audit-vs-improvement_plan)

---

## 1. Relacja z AUDIT.md

[AUDIT.md](./AUDIT.md) pokrywa już:

- ✅ Błędną detekcję keyringa (A1)
- ✅ Niezgodność sygnatury Factory (A2)
- ✅ Martwe anchory (A5)
- ✅ Scope creep — Sound, Topology, Env Merger, Live Log, Bento (A6)
- ✅ Brakujące podrozdziały DESIGN (A7)
- ✅ Race condition PID lockfile (A8)
- ✅ Testy spoza zakresu MVP (B1)
- ✅ `rsync --delete` bez excludes (C6)

**Ten plik NIE powtarza tych znalezisk.** Dokumentuje tylko to, co AUDIT pominął lub potraktował zbyt płytko.

---

## 2. Znaleziska krytyczne (blokujące implementację)

### IMP-1. `DESIGN.md §10` — DAG-based Transactional Engine vs LIFO stack

**Lokalizacja:** [`docs/DESIGN.md §10`](./DESIGN.md#10-dag-based-transactional-engine-wznawialny-rollback)

**Problem:** DESIGN §10 opisuje pełny silnik transakcyjny oparty o acykliczny graf skierowany (`ExecutionDAG`, `StepNode`, `DependsOn`, `NodeID`, `StateFailed`, `StateSuccess`, selektywny rollback, resume). Tymczasem:

- PRD §6 F10 definiuje `Rollback transakcyjny kreatora` jako **"Stos LIFO + `pending_cleanups.json`"**
- ROADMAP §3.2 definiuje zakres MVP jako `"Rollback transakcyjny kreatora + pending_cleanups.json"`

**DAG ≠ LIFO.** To fundamentalnie różne poziomy złożoności. DAG wymaga:
- Topologicznego sortowania węzłów
- Przechowywania stanu per węzeł
- Mechanizmu resume z pomijaniem `StateSuccess`
- Serializacji/deserializacji grafu do `pending_cleanups.json`
- Obsługi współbieżności węzłów (scaffold files + subdomain równolegle)

LIFO wymaga tylko stosu operacji z `Rollback()`.

**Ryzyko:** implementator zacznie kodować DAG kosztem P0, bo DESIGN jest źródłem prawdy architektury.

**Korekta:** dodać na początku `DESIGN.md §10` baner:

```markdown
> ⚠ **v0.1**: implementujemy prosty LIFO stack z `pending_cleanups.json`.
> Pełny DAG z selektywnym rollbackiem i resume → `v0.3+`.
> Opis poniżej to **target architecture**, nie MVP scope.
```

**Priorytet:** P0

---

### IMP-2. `SECURITY.md §4.2` — brak specyfikacji generowania AES-GCM nonce

**Lokalizacja:** [`docs/SECURITY.md §4.2`](./SECURITY.md#42-fallback-dla-środowisk-headless)

**Problem:** format NDJSON (`{"key": "...", "ciphertext": "...", "nonce": "..."}`) z osobnym nonce per wpis. **AES-GCM z powtórzonym noncem to catastrophic failure** — odzyskanie klucza szyfrującego. Dokument nie specyfikuje źródła nonce.

**Ryzyko:** implementator użyje `time.Now().UnixNano()` jako nonce — dwa wpisy w tej samej nanosekundzie (lub ten sam czas po restarcie) → full compromise całego pliku.

**Korekta:** dodać explicite:

```markdown
Każdy nonce generowany przez `crypto/rand.Read(buf)` (96 bitów, 12 bajtów).
Nigdy nie używać nonce czasowego, licznika, ani powtarzalnego źródła.
Nonce jest przechowywany razem z ciphertext w tym samym wpisie NDJSON.
```

**Priorytet:** P0

---

### IMP-3. `SECURITY.md §4.2` — `WEBOX_MASTER_PASSWORD` env var bez oznaczenia ryzyka

**Lokalizacja:** [`docs/SECURITY.md §4.2`](./SECURITY.md#42-fallback-dla-środowisk-headless), linia *"lub przekazany przez `WEBOX_MASTER_PASSWORD` env w trybie CI"*.

**Problem:** zmienne środowiskowe są czytelne dla każdego procesu tego samego usera (`/proc/<pid>/environ`, `ps aux`). W trybie CI to konieczność, ale w trybie deweloperskim to poważne ryzyko.

**Korekta:** dodać wyraźne ostrzeżenie:

```markdown
> ⚠ **CI only.** `WEBOX_MASTER_PASSWORD` jest czytelny dla każdego procesu
> użytkownika. Używaj **wyłącznie** w ephemeral CI kontenerach.
> Na maszynie deweloperskiej zawsze wpisuj master password interaktywnie.
> Webox loguje warning jeśli env var jest ustawiona na nie-CI hoście.
```

**Priorytet:** P0 (security-sensitive)

---

### IMP-4. `SECURITY.md §5.4` — resolution host key mismatch przez `webox doctor security` (v0.2) — ale w v0.1 nie ma alternatywy

**Lokalizacja:** [`docs/SECURITY.md §5.4`](./SECURITY.md#54-zmiana-host-key) vs [`docs/SECURITY.md §7`](./SECURITY.md#7-audyt-sekretów-i-tryb-doctor) vs [`docs/ROADMAP.md §4.2`](./ROADMAP.md#42-zakres-v02)

**Problem:** SECURITY §5.4 mówi: *"rozwiązanie host key mismatch = `webox doctor security --update-host-key`"*. Ale:
- SECURITY §7 mówi: `webox doctor security` → **post-MVP, implementacja v0.2**
- W v0.1 nie ma innego mechanizmu rozwiązania konfliktu host key

**Ryzyko:** w v0.1 user nie może wejść na swój własny serwer po reinstalacji/migracji hosta. Tool jest bezużyteczny do czasu v0.2.

**Korekta:** dorzucić do v0.1 alternatywę. Najprostsza: confirm dialog w TUI z opcją "Accept new host key" + out-of-band verification text (tak jak TOFU):

```markdown
Dla v0.1: host key mismatch wyświetla confirm dialog z:
  - Stary fingerprint vs nowy fingerprint
  - Ostrzeżeniem MITM
  - [ Accept and update known_hosts ]  [ Abort ]
Nie wymaga osobnej komendy CLI.
```

Alternatywnie: `webox doctor` (bez `security`) jest w v0.1 (PRD §12.3). Można dodać `webox doctor --update-host-key <host>` jako wyjątek od reguły "doctor tylko diagnostyczny".

**Priorytet:** P0

---

## 3. Znaleziska wysokie (niezgodności między dokumentami)

### IMP-5. Język dokumentacji (`pl`) vs ADR-0006 (`en` domyślnie) — bariera dla globalnych contributorów

**Lokalizacja:** [`docs/adr/0006`](./adr/0006-jezyk-interfejsu-en-domyslny.md) vs rzeczywisty stan `docs/`.

**Problem:** ADR-0006: *"English jest domyślny. Polski dostępny jako `settings.language = "pl"`."* Rzeczywistość:

| Plik | Język |
|---|---|
| `PRD.md` | PL |
| `DESIGN.md` | PL |
| `SECURITY.md` | PL |
| `ROADMAP.md` | PL |
| `TESTING.md` | PL |
| `CONTRIBUTING.md` | PL |
| `MIGRATION_NOTES.md` | PL |
| `CHANGES.md` | PL |
| `AUDIT.md` | PL |
| Wszystkie 6 ADR-ów | PL |
| `providers/smallhost.md` | PL |
| `providers/cpanel.md` | PL |
| `providers/directadmin.md` | PL |
| `providers/cyberpanel.md` | PL |
| `README.md` | EN |
| `UX.md` mockupy | EN (po poprawce 6.12) |

Globalny contributor, który chce dodać adapter cPanel, **musi czytać po polsku** żeby zrozumieć kontrakt `HostingProvider`, security model, i test strategy. To **realnie i natychmiast zabija community adoption**.

**Opcje:**

| Opcja | Wysiłek | Konsekwencje |
|---|---|---|
| **A: Przepisać docs na EN** | Ogromny. 15+ plików, wysokie ryzyko błędów terminologicznych. | Natychmiastowa dostępność dla globalnej społeczności. Zgodność z ADR-0006. |
| **B: Zmienić ADR-0006 na "PL-primary, EN secondary"** | Minimalny. | Uczciwe, ale zamyka projekt na globalnych contributorów. K6 (community provider) staje się prawie niemożliwe. |
| **C: Zostawić docs PL, dodać `docs/en/` z tłumaczeniem kluczowych sekcji** | Średni. | Kompromis: CONTRIBUTING.md i DESIGN.md kluczowe sekcje po EN. PRD i SECURITY zostają PL. |
| **D: AI-assisted translation (`docs/en/` wygenerowane, zreviewowane)** | Średni. Można to zrobić narzędziowo, ale review terminologii technicznej i tak ręczny. | Najszybsza droga do EN docs. |

**Rekomendacja:** **Opcja C jako minimum dla v0.1, opcja A jako cel na v0.2**. Bez minimalnego EN pokrycia CONTRIBUTING + DESIGN, community adoption = 0.

Minimum viable dla v0.1:
- `CONTRIBUTING.md` → EN (to jest dokument dla contributorów)
- `DESIGN.md §3 Provider Pattern` → EN (kontrakt, który każdy nowy provider musi zrozumieć)
- `docs/providers/smallhost.md` → EN (to jest wzorzec dla wszystkich providerów)

**Priorytet:** P1

---

### IMP-6. CLI flagi proliferują niezgodnie z ADR-0001

**Lokalizacja:** [`docs/adr/0001`](./adr/0001-tui-zamiast-cli.md), decyzja: *"W MVP nie ma CLI flag dla zewnętrznych skryptów (poza `webox doctor`)"*.

**Rzeczywistość w MVP (udokumentowane w SECURITY.md):**

| Flaga / zmienna | Cel | Gdzie zdefiniowane |
|---|---|---|
| `webox doctor` | Diagnostyka | PRD §12.3 |
| `webox doctor --json` | Diagnostyka dla skryptów | SECURITY §7 |
| `webox doctor --bundle` | Crash report bundle | PRD §12.4 |
| `webox doctor --update-host-key` | Rozwiązanie host key mismatch | SECURITY §5.4 |
| `webox doctor --import-known-hosts` | Import kluczy SSH | SECURITY §5.1 |
| `webox doctor --rotate-master` | Rotacja master password | SECURITY §4.4 |
| `webox doctor --migrate-to-fallback` | Migracja keyring → fallback | ADR-0004 |
| `webox doctor --migrate-to-keyring` | Migracja fallback → keyring | ADR-0004 |
| `--debug` | Tryb debug log | SECURITY §9.1 |
| `--no-cache` | Wyłączenie cache | ADR-0005 |
| `--lang=pl` | Override języka | ADR-0006 |
| `WEBOX_EXPERIMENTAL=1` | Feature flag | ROADMAP §3.1 |
| `WEBOX_MASTER_PASSWORD` | Master password dla CI | SECURITY §4.2 |
| `WEBOX_SECRETS_BACKEND=fallback` | Force fallback backend | ADR-0004 |
| `WEBOX_LOG_LEVEL=trace` | Log level | CONTRIBUTING §1.5 |

**Problem:** To nie jest "brak CLI flags". To jest ~15 flag i env var. ADR-0001 jest po prostu nieaktualny.

**Korekta:** zaktualizować ADR-0001:

```markdown
W v0.1 webox wspiera ograniczone CLI flagi dla:
- diagnostyki (`webox doctor [--json|--bundle]`),
- zarządzania host keys (`webox doctor [--update-host-key|--import-known-hosts]`),
- zarządzania sekretami (`webox doctor [--rotate-master|--migrate-*]`).
Brak natomiast CLI flag dla operacji operatorskich (`webox restart`, `webox create`).
Te trafią do v0.3+ jako F22.
```

**Priorytet:** P1

---

### IMP-7. Math się nie zgadza — 20 równoległych fetchy z 3-slotowym SSH pool

**Lokalizacja:** [`docs/adr/0005`](./adr/0005-cache-statusow-projektow.md), zdanie: *"20 równoległych fetchy w goroutynach → wypełniony w ~3 s"*.

**Problem:** DESIGN §5 definiuje `max_connections=3` per host. Przy 20 projektach potrzebujących SSH (Node version check, SSL probe, log path existence), tylko 3 mogą być w locie. Reszta czeka w kolejce. Z timeoutem dial 15s + command exec ~2s:

```
20 fetchy / 3 sloty = ~7 kolejek × ~5s (średnia) = ~35s
```

**"~3 s" jest nierealne.**

**Korekta:** poprawić w ADR-0005:

```markdown
Cold start: ~30–40s dla 20 projektów (ograniczone przez SSH pool = 3).
Przy następnych refreshach: instant (SWR cache hit).
User widzi dane stopniowo — każdy projekt dostaje status gdy jego fetch się kończy,
nie trzeba czekać na wszystkie.
```

**Priorytet:** P1

---

## 4. Znaleziska średnie (luki bezpieczeństwa i problemy techniczne)

### IMP-8. Clipboard clearing "best-effort" — obietnica niemożliwa do spełnienia

**Lokalizacja:** [`docs/SECURITY.md §9.3`](./SECURITY.md#93-schowek-systemowy)

**Problem:** *"Po 30 s webox próbuje wyczyścić clipboard (best-effort)"*. Rzeczywistość:
- **macOS:** `pbcopy` nadpisuje, ale clipboard managers (Alfred, Raycast, Maccy) zachowują historię niezależnie
- **Linux:** X11 ma PRIMARY i CLIPBOARD selection, Wayland nie ma unified API
- **Windows:** wymaga Win32 API
- **Terminal multiplexer (tmux/screen):** ma własny clipboard, niezależny od OS

"Czyszczenie clipboardu" jest **praktycznie niemożliwe do zrealizowania** w sposób, który użytkownik może uznać za skuteczny.

**Korekta:** zamiast obiecywać czyszczenie, ostrzegać i zostawić odpowiedzialność po stronie użytkownika:

```markdown
> ⚠ Sekret skopiowany do schowka. Webox nie może zagwarantować jego usunięcia
> (clipboard managers, terminal multiplexery, OS różnią się mechanizmami).
> Wyczyść schowek ręcznie po użyciu.
```

W TUI: po `Ctrl+Y` pokazać komunikat `Secret copied. Clear your clipboard after use.` zamiast timera.

**Priorytet:** P2

---

### IMP-9. `SECURITY.md §4.3` — `zerocopy.Wipe` nie gwarantuje wymazania w Go

**Lokalizacja:** [`docs/SECURITY.md §4.3`](./SECURITY.md#43-cykl-życia-sekretu-w-pamięci)

**Problem:** (pokryte częściowo przez AUDIT C4) — ale AUDIT sugeruje `memguard.LockedBuffer`. Problem jest głębszy: Go GC może skopiować sekret podczas compacting GC, zostawiając plaintext w starej lokalizacji. `memguard` to library, nie rozwiązanie fundamentalne.

**Korekta (rozszerzenie C4):**

```markdown
Sekrety w pamięci:
- Pobierane na żądanie, nie cache'owane dłużej niż jedna operacja.
- Po użyciu bufor jest `mlock`'owany (zapobiega swapowaniu) i nadpisywany zerami.
- Ograniczenie: Go GC może kopiować pamięć — `memguard.LockedBuffer` mityguje ale nie eliminuje ryzyka.
- Akceptujemy to ryzyko: atakujący potrzebuje fizycznego dostępu do RAM lub core dump.
- `GODEBUG=clobberfree=1` w release build (nadpisuje zwolnione bloki).
```

**Priorytet:** P2

---

### IMP-10. `SECURITY.md §10.4` — `.env` permission check po GHA deploy nie ma kto wykonać

**Lokalizacja:** [`docs/SECURITY.md §10.4`](./SECURITY.md#104-lokalizacja-i-permisje-env-na-serwerze)

**Problem:** tabela mówi: *"Weryfikacja przez Webox: Tak — `ls -la` przez SSH po deploy, porównanie z `provider.GetDeployPath()`"*. Ale w GHA deploy, to **workflow Actions** materializuje `.env` na serwerze — nie webox bezpośrednio. Webox może tylko **post-deploy check** przez osobne połączenie SSH. To oznacza:

1. Workflow kończy deploy
2. Webox (albo TUI, albo osobny krok) SSH-uje i sprawdza permisje
3. Jeśli złe — co robi? Blokuje? Ostrzega? Naprawia?

**Korekta:** dodać krok `verify-env-permissions` do szablonu `deploy.yml`:

```yaml
- name: Verify .env permissions
  run: |
    ssh ${{ secrets.DEPLOY_USER }}@${{ secrets.DEPLOY_HOST }} \
      'stat -c "%a %U" ${{ secrets.DEPLOY_PATH }}/../.env' \
      | grep -q '^600 ' || \
      (echo "::error::.env has insecure permissions" && exit 1)
```

I w `SECURITY.md §10.4` dodać: *"Dla GHA deploy, weryfikacja permisji jest częścią workflow (nie webox). Dla Direct SFTP, webox wykonuje check sam."*

**Priorytet:** P2

---

### IMP-11. Brak koncepcji Jump Host / SSH Proxy

**Lokalizacja:** [`docs/DESIGN.md §5`](./DESIGN.md#5-warstwa-ssh--sftp-connection-pooling), [`docs/PRD.md §10.4`](./PRD.md#104-sieć-i-hosting)

**Problem:** PRD §10.4: *"Wymagane wyjście SSH na port serwera (zwykle 22)"*. Persona B (agencja) często pracuje za firmowym firewallem, gdzie SSH na zewnętrzne hosty idzie przez jump host / bastion. Bez `ProxyJump` / `ProxyCommand`, webox jest niedostępny dla tych użytkowników.

**Decyzja:** (patrz [§6 poniżej](#6-kwestia-persony-b-agencja--decyzja)) — Persona B jest architectural pressure, nie scope dla v0.1. Jump host **NIE jest** w MVP.

**Korekta:** dodać w `DESIGN.md §5` notkę:

```markdown
> ⚠ **Stretch:** wsparcie dla ProxyJump / SSH bastion → `v0.2+`.
> W v0.1 webox łączy się bezpośrednio z hostem docelowym.
```

I dodać pole `properties.ssh_proxy` w `DESIGN.md §3.3` jako placeholder na przyszłość.

**Priorytet:** P2 (świadome odroczenie)

---

## 5. Znaleziska niskie (edge case'y i niespójności redakcyjne)

### IMP-12. `DESIGN.md §9` — "ostateczna niedokończona komenda jest bezpiecznie ponawiana" — niebezpieczne dla operacji nie-idempotentnych

**Lokalizacja:** [`docs/DESIGN.md §9`](./DESIGN.md#9-obsługa-błędów-sieciowych-i-reconnect), krok 3.

**Problem:** *"Jeśli połączenie zostanie odzyskane, ostatnia niedokończona komenda z bufora jest bezpiecznie ponawiana."* Ale jeśli komenda faktycznie **wykonała się na serwerze** przed utratą połączenia (a my nie dostaliśmy odpowiedzi), ponowienie:
- `devil www add` → `ErrSubdomainExists` (OK, błąd zamiast duplikacji)
- `devil mysql add` → duplikat bazy (NIE OK — dwie bazy o różnych nazwach?)
- `devil ssl www add` → nieznane zachowanie

Mitygacja istnieje (rollback stack), ale słowo "bezpiecznie ponawiana" jest mylące.

**Korekta:**

```markdown
Po reconnect: system sprawdza stan ostatniej operacji na serwerze (jeśli provider wspiera
idempotentne sprawdzenie, np. `devil www list | grep <domain>`).
Jeśli operacja już się wykonała — kontynuuje. Jeśli nie — ponawia.
Dla operacji bez idempotentnego checku — przechodzi do rollbacku całego kroku.
```

**Priorytet:** P3

---

### IMP-13. `DESIGN.md §10` — resume DAG po `PutSecrets` na GitHub (write-only API)

**Lokalizacja:** [`docs/DESIGN.md §10`](./DESIGN.md#10-dag-based-transactional-engine-wznawialny-rollback)

**Problem:** Jeśli DAG zapisze GitHub secrets (write-only), a potem crashnie, resume nie może sprawdzić czy sekret już istnieje — GitHub API zwraca tylko metadane, nie wartość. Resume może:
- Nadpisać sekret tą samą wartością (OK, idempotentne z zewnątrz)
- Ale wartość mogła być zmieniona ręcznie między crash a resume → utrata

**Korekta:** dodać w `DESIGN.md §10` (lub `SECURITY.md §10.3`) notkę:

```markdown
GitHub Secrets API jest write-only. Resume DAG traktuje `PutSecret` jako idempotentne —
nadpisuje wartość tą samą, która była w `Params` w momencie crashu.
Nie próbuje odczytać stanu z GH.
```

**Priorytet:** P3

---

### IMP-14. `DESIGN.md §16.1 — Sinusoidalne pulsowanie ramek` a wydajność

**Lokalizacja:** [`docs/DESIGN.md §16.1`](./DESIGN.md#161-sinusoidalne-pulsowanie-ramek-border-pulsing)

**Problem:** Kod pokazuje tick co `80ms` (`12.5 fps`) z obliczeniami `math.Sin()` i `oklchToHex()` w pętli `Update()`. Dla TUI z 20 projektami, każde odświeżenie 80ms oznacza:
- `Update()` wywoływana 12.5 razy/sekundę
- Każde wywołanie renderuje od nowa ramki Bento przez Lipgloss
- Lipgloss nie jest zaprojektowany do 60 fps renderowania — każda klatka to realokacja stringów

To **niepotrzebnie obciąża CPU** dla efektu kosmetycznego.

**Korekta:** ale to stretch (AUDIT A6) — więc po prostu oznaczamy jako `> ⚠ Stretch: pulsowanie ramek wymaga benchmarków wydajnościowych przed wdrożeniem.`.

**Priorytet:** P3

---

### IMP-15. `providers/smallhost.md §5.4` — DNS readiness probe blokuje wizard na 48h dla custom domain

**Lokalizacja:** [`docs/providers/smallhost.md §5.4`](./providers/smallhost.md#54-ssl--dns-not-propagated)

**Problem:** *"DNS readiness probe (`dig` / `net.LookupHost` z timeoutem)"* — dla custom domain DNS może propagować do 48h. Wizard nie może czekać 48h.

**Korekta:** rozdzielić flow:

```markdown
Dla subdomen small.pl (*.user.smallhost.pl): DNS instant → sprawdzamy, idziemy dalej.
Dla custom domain: Webox **nie czeka** na DNS. Wizard kończy się sukcesem, projekt
dostaje status `SSL_PENDING` z informacją: "Custom domain DNS not yet resolved.
SSL will be issued automatically on next status refresh (max 48h)."
Webox retry `SetupSSL` w tle co 15 minut aż DNS się rozpropaguje.
```

**Priorytet:** P3

---

### IMP-16. `TESTING.md §3.3` — fixture'e z "realnym testowym kontem" a ich nieodtwarzalność

**Lokalizacja:** [`docs/TESTING.md §3.3`](./TESTING.md#33-fixturey-output-devil)

**Problem:** fixture'e outputu `devil` są przechwytywane ręcznie z konta testowego. Jeśli `devil` zmieni format outputu (nowa wersja panelu small.pl), fixture'e dezaktualizują się i testy przechodzą na fałszywie — testują stary format, nie ten z produkcji.

**Korekta:** dodać w `TESTING.md` (lub CI workflow):

```markdown
Co miesiąc: `make test-integration-live` (uruchamiane tylko przez maintainera)
odpala testy z realnym kontem i aktualizuje fixture'y.
CI nightly może wykryć rozbieżności (ale nie auto-aktualizować).
```

**Priorytet:** P3

---

### IMP-17. Workflow `deploy.yml` — brak cache'owania `node_modules`

**Lokalizacja:** [`docs/providers/smallhost.md §6`](./providers/smallhost.md#6-deployment-workflow-szablon)

**Problem:** szablon workflow: `npm ci` bez cache. Każdy deploy = pełne `npm ci` od zera. Dla małych projektów ~30s, dla monorepo ~3min. To wlicza się w GHA minutes (darmowe 2000 min/miesiąc dla prywatnych repo).

**Korekta:** dodać do szablonu `deploy.yml`:

```yaml
- uses: actions/cache@v4
  with:
    path: ~/.npm
    key: ${{ runner.os }}-node-${{ hashFiles('**/package-lock.json') }}
    restore-keys: ${{ runner.os }}-node-
```

**Priorytet:** P3

---

### IMP-18. `PRD.md §8 K6` — 6 miesięcy na community provider jest agresywne

**Lokalizacja:** [`docs/PRD.md §8`](./PRD.md#8-kryteria-sukcesu--mierzalne) i [`docs/ROADMAP.md §2.3`](./ROADMAP.md#23-kryteria-ga-v10)

**Problem:** (Częściowo pokryte przez AUDIT B6, który dodaje "nie blokuje GA".) Ale dodatkowo: jeśli dokumentacja jest po polsku (IMP-5), to community provider w 6 miesięcy jest praktycznie niemożliwe.

**Korekta:** zaktualizować K6:

```markdown
K6: 1 community-provided provider w ciągu 12 miesięcy od v0.1 (lub 6 miesięcy
od momentu udostępnienia EN dokumentacji technicznej — whichever is later).
Niespełnienie nie blokuje GA, ale przesuwa je o kolejne 6 miesięcy.
```

**Priorytet:** P3

---

### IMP-19. `CONTRIBUTING.md §2.1` — `gocyclo` max 15 dla metod providera

**Lokalizacja:** [`docs/CONTRIBUTING.md §2.1`](./CONTRIBUTING.md#21-linter)

**Problem:** metody providera typu `SetupSSL` z logiką sprawdzenia DNS → sprawdzenia istniejącego certyfikatu → sprawdzenia rate limit → wykonania `devil ssl www add` → parsowania outputu → walidacji — naturalnie wyjdą 16-20 cyclomatic complexity. Limit 15 będzie notorycznie łamany.

**Korekta:** `gocyclo` max 20, z wyjątkiem per-funkcja (`//nolint:gocyclo`) tylko za zgodą review.

**Priorytet:** P3

---

## 6. Kwestia Persony B (agencja) — decyzja

**Pytanie:** *"kwestia agencji do wywalenia bo to jest chyba nie na ten etap?"*

**Analiza:** PRD §4.2 definiuje Personę B jako *"architectural pressure, nie source obowiązków zakresowych dla v0.1"*. To poprawne rozróżnienie — persona istnieje **tylko** po to, żeby wymusić multi-provider od dnia 1.

### Co Persona B wymusza (i to jest dobre):

| Wymuszenie | Status |
|---|---|
| `HostingProvider` interface od dnia 1 | ✅ Już w DESIGN.md |
| Multi-provider w configu (`profiles[]`) | ✅ Już zaplanowane |
| Keyring fallback dla headless (Linux server) | ✅ W SECURITY.md |
| Topology map (przegląd wielu serwerów) | Niepotrzebne w v0.1 |

### Czego Persona B NIE powinna wymuszać (i to jest do usunięcia/przesunięcia):

| Element | Problem | Decyzja |
|---|---|---|
| Jump host / SSH proxy (IMP-11) | Agencja za firewallem. | **v0.2+** — dodajemy placeholder. |
| Multi-provider dashboard agregujący projekty z różnych serwerów | v0.1 ma tylko small.pl. | **v0.2** — zgodnie z ROADMAP. |
| GitHub Teams / organization-level repo management | Wymaga `org:admin` scope. | **v0.3+** — osobny ficzer. |
| Onboarding nowego pracownika (kopiowanie configu) | F20 (export/import) jest P2. | **v0.3** — zgodnie z ROADMAP. |

### Decyzja:

**Persona B zostaje w PRD §4.2 jako architectural pressure.** NIE usuwamy jej. Ale:

1. Dodajemy wyraźną adnotację: *"Persona B nie generuje żadnych nowych obowiązków implementacyjnych do v0.2."*
2. Wszystkie elementy, które Persona B mogłaby wymuszać (jump host, multi-provider dashboard, org-level GH), są oznaczone jako `v0.2+` lub `stretch`.
3. IMP-11 (jump host) trafia do DESIGN.md §5 jako placeholder, nie implementacja.

**Uzasadnienie:** usunięcie Persony B z dokumentacji byłoby błędem — to ona uzasadnia Provider Pattern od dnia 1. Bez niej, implementator mógłby pokusić się o hardcodowanie `smallhost` w logice biznesowej (bo "tylko jeden provider w MVP").

---

## 7. Nowe decyzje otwarte

### F1. Czy `CONTRIBUTING.md` i kluczowe sekcje `DESIGN.md` tłumaczymy na EN przed implementacją?

**Kontekst:** IMP-5 (bariera językowa).

**Rekomendacja:** Tak. Minimum: `CONTRIBUTING.md` + `DESIGN.md §3 (Provider Pattern)` + `providers/smallhost.md` → EN przed pierwszym PR z kodem.

### F2. Czy `webox doctor` wspiera `--update-host-key` w v0.1?

**Kontekst:** IMP-4 (resolution host key mismatch).

**Rekomendacja:** Tak. `webox doctor --update-host-key <host>` jako wyjątek od reguły "doctor tylko diagnostyczny". Albo confirm dialog w TUI jako alternatywa. Osobiście rekomenduję confirm dialog (prostsze, nie wymaga CLI).

### F3. Czy DAG-based Transactional Engine idzie do v0.3+ czy zostaje w DESIGN.md jako target architecture?

**Kontekst:** IMP-1.

**Rekomendacja:** LIFO stack w v0.1 (zgodnie z PRD). DAG zostaje w DESIGN.md jako target architecture z banerem. Implementacja DAG → v0.3, po tym jak LIFO działa stabilnie.

### F4. Czy szablon `deploy.yml` dodaje cache npma?

**Kontekst:** IMP-17.

**Rekomendacja:** Tak. To 4 linijki YAML, zero downside.

---

## 8. Macierz pokrycia AUDIT vs IMPROVEMENT_PLAN

| Obszar problemu | AUDIT.md | IMPROVEMENT_PLAN |
|---|---|---|
| Keyring detekcja | A1 (szczegółowo) | — |
| Factory sygnatura | A2 | — |
| Generyczny kod w DESIGN §8 | A3 | — |
| Typo `CPINalled` | A4 | — |
| Martwe anchory | A5 | — |
| Scope creep (sound, topology, etc.) | A6 | — |
| Brakujące podrozdziały DESIGN | A7 | — |
| PID lockfile | A8 | — |
| Testy `/env` w MVP | B1 | — |
| Lint wersje (`golangci-lint v2`) | B3, B4 | — |
| Bento jako stretch | B5 | — |
| GH token scope (`org:admin`) | B7 | — |
| `rsync --delete` bez excludes | C6 | — |
| `zerocopy.Wipe` → `memguard` | C4 | IMP-9 (rozszerzenie) |
| **DAG vs LIFO mismatch** | — | **IMP-1** |
| **AES-GCM nonce unspecified** | — | **IMP-2** |
| **`WEBOX_MASTER_PASSWORD` ryzyko** | — | **IMP-3** |
| **Host key mismatch bez resolution w v0.1** | — | **IMP-4** |
| **Język dokumentacji PL vs ADR-0006 EN** | — | **IMP-5** |
| **CLI flagi vs ADR-0001** | — | **IMP-6** |
| **Math SSH pool vs 20 fetchy** | — | **IMP-7** |
| **Clipboard clearing niemożliwe** | — | **IMP-8** |
| **`.env` permission check w GHA** | — | **IMP-10** |
| **Jump host brak** | — | **IMP-11** |
| **Reconnect + retry non-idempotent** | — | **IMP-12** |
| **DAG + GH Secrets write-only** | — | **IMP-13** |
| **Sinusoidal pulsing CPU cost** | — | **IMP-14** |
| **DNS probe blokuje wizard** | — | **IMP-15** |
| **Fixtures stale vs dev zmiany** | — | **IMP-16** |
| **Deploy workflow bez npm cache** | — | **IMP-17** |
| **K6 community provider timeline** | B6 (częściowo) | **IMP-18** |
| **gocyclo 15 za nisko** | — | **IMP-19** |
| **Persona B scope decision** | — | **§6** |

---

## Podsumowanie — co zrobić przed implementacją

### Przed pierwszym PR z kodem (must-have):

| ID | Co | Gdzie |
|---|---|---|
| IMP-1 | Baner DAG vs LIFO | `DESIGN.md §10` |
| IMP-2 | Specyfikacja nonce AES-GCM | `SECURITY.md §4.2` |
| IMP-3 | Ostrzeżenie `WEBOX_MASTER_PASSWORD` | `SECURITY.md §4.2` |
| IMP-4 | Resolution host key mismatch w v0.1 | `SECURITY.md §5.4` + nowy stan TUI |
| A1–A8 | Wszystkie P0 z AUDIT.md | Różne pliki |

### Przed v0.1 release:

| ID | Co |
|---|---|
| IMP-5 | Minimum EN docs (CONTRIBUTING + DESIGN §3 + smallhost.md) |
| IMP-6 | Aktualizacja ADR-0001 (CLI flagi) |
| IMP-7 | Poprawka math w ADR-0005 |
| IMP-10 | `verify-env-permissions` krok w `deploy.yml` |
| IMP-11 | Placeholder ProxyJump w DESIGN §5 |

### W v0.2 (razem z drugim providerem):

| ID | Co |
|---|---|
| IMP-5 (pełne) | Pełne EN pokrycie docs |
| IMP-8 | Usunięcie obietnicy clipboard clearing |
| IMP-11 | Implementacja ProxyJump |

### W v0.3+:

| ID | Co |
|---|---|
| IMP-1 (DAG) | Pełny DAG-based Transactional Engine |
