# Sprint 23 — Second Provider OR Public Launch (Decision Sprint)

> **Daty:** 2026-07-07 → 2026-07-20 (2 tygodnie) · **Cel:** Po zamknięciu cPanel adaptera (Sprint 22), wybór między drugim adapterem (DirectAdmin / CyberPanel) a pełnym Public Launch'em (Sprint 16 redux). Decyzja podejmowana na retro Sprintu 22.
>
> **Status:** 📝 Decision-pending · **Predecessor:** [Sprint 22](sprint-22-cpanel-adapter-mutations.md) (cPanel adapter part 2 + v0.2.0-rc1).

## Kontekst

Po Sprincie 22 Webox ma:
- Pełny cPanel adapter (read + mutate ops).
- `v0.2.0-rc1` jako pre-release na GitHub.
- Preset registry z 6 entries (smallhost-devil + cpanel-generic verified, reszta candidate / research).

Co dalej? **Trzy drogi, jeden sprint na każdą.**

Sprint 23 to **decision sprint**: pierwszy dzień to ocena rzeczywistości po `v0.2.0-rc1`, wybór ścieżki, dopiero potem implementacja. Plan tutaj jest *opcjonalny per-ścieżka* — szczegółowy plan ostatecznej ścieżki powstaje na retro Sprintu 22.

## Ścieżka A — DirectAdmin Adapter (rozszerza catalog providerów)

**Kiedy wybrać:** Sprint 22 zamknął się czysto, cPanel adapter działa stabilnie, operatorzy proszą o DirectAdmin (drugi co do wielkości panel w PL po cPanel).

**Cel:** Powtarzamy wzorzec z Sprintu 21+22 dla DirectAdmin: Live API client + SSH fallback + `webox doctor directadmin` + wizard integration + GHA template.

**Skrócony backlog (full plan after path selection):**

- **TASK-23.A.1** — `providers/directadmin/liveapi/` read-only client (Live API: JSON endpoint, basic auth). [L]
- **TASK-23.A.2** — SSH fallback (`directadmin` szuka `da-ssl-install` / `da-create-user` CLI binarki). [M]
- **TASK-23.A.3** — `webox doctor directadmin` CLI. [M]
- **TASK-23.A.4** — `directadmin-generic` preset graduates from `research` → `verified` after live account validation. [S]
- **TASK-23.A.5** — Adapter implementation + wizard integration + GHA template. [L]
- **TASK-23.A.6** — E2E + release notes + `v0.2.1` bump (DirectAdmin add). [S]

**Gating:** wymaga DirectAdmin test account. Same problem class as Sprint 22 TASK-22.0; jeśli nie ma test accountu do startu, fallback do Ścieżki C (Public Launch).

**Ryzyko:** DirectAdmin Live API ma 2 inkarnacje (Legacy text + Live JSON); preset musi wybierać per-host.

## Ścieżka B — CyberPanel Adapter (ekosystem alternatywa)

**Kiedy wybrać:** Sprint 22 ujawnił, że cPanel ekosystem ma większe długi techniczne niż się spodziewaliśmy (np. UAPI permission model jest cięższy niż docs sugerują), a CyberPanel oferuje czystszy API surface (open-source, REST, dokumentacja w GitHubie).

**Cel:** Pierwsza wersja CyberPanel adaptera (read-only) + decyzja czy continue w v0.3 czy zostaje na półce do v0.4+.

**Skrócony backlog:**

- **TASK-23.B.1** — `providers/cyberpanel/api/` REST client (basic auth + token endpoint). [L]
- **TASK-23.B.2** — `webox doctor cyberpanel` CLI. [M]
- **TASK-23.B.3** — `cyberpanel-generic` preset live validation. [M]
- **TASK-23.B.4** — Spike: szacujemy effort dla pełnego adaptera + mutating ops; jeśli > 1.5 sprintu, zostawiamy na v0.4. [S]
- **TASK-23.B.5** — Sprint review + carry-over decision. [S]

**Ryzyko:** CyberPanel wymaga `root` na hoście (vs cPanel z unprivileged token). Threat model się rozszerza — security review z maintainerem przed merge.

## Ścieżka C — Public Launch Redux (Sprint 16 reborn)

**Kiedy wybrać:** Sprint 22 zamknął `v0.2.0-rc1`, ale operator audience jest priorytetem nad kolejnym adapterem. Cel: dostać pierwszych 100 GitHub stars i 10 użytkowników nie-maintainerów.

**Cel:** Pełny launch motion: Reddit / Show HN / r/golang / r/selfhosted / r/programowanie + 2 partner outreach (małe hostery PL) + landing page polish + Webox PRO landing teaser.

**Skrócony backlog:**

- **TASK-23.C.1** — Reddit launch posts (4 subreddits, tailored copy). [M]
- **TASK-23.C.2** — Show HN submission (timing + comment plan). [M]
- **TASK-23.C.3** — Partner outreach (H88, mydevil, small.pl — sponsor / co-marketing). [M]
- **TASK-23.C.4** — Landing page redesign (EN-first; PL link side-by-side). [L]
- **TASK-23.C.5** — Webox PRO teaser landing (paid plan + early bird waitlist). [M]
- **TASK-23.C.6** — Issue triage cadence — 4h/dzień zarezerwowane na community Q&A. [—]
- **TASK-23.C.7** — Launch retro + v0.2.0 GA decision (tag z RC1 jeśli launch poszedł czysto). [S]

**Ryzyko:** Launch wymaga TUI confidence na "obcych" terminalach (iTerm, Windows Terminal, gnome-terminal); Sprint 22 musi domknąć cPanel TUI smoke test w `make smoke-test`. Bez tego launch ryzykuje negatywnym fidbekiem.

## Decision matrix (do wypełnienia na retro Sprintu 22)

| Kryterium | A. DirectAdmin | B. CyberPanel | C. Public Launch |
|---|---|---|---|
| Test account ready? | … | … | n/a |
| cPanel adapter stable? | wymaga ✅ | wymaga ✅ | wymaga ✅ |
| Operator demand signal | … | … | … |
| Effort estimate (zaufanie 1-5) | … | … | … |
| Risk to v0.2.0 GA timeline | … | … | … |

**Decyzja:** wybrana ścieżka + uzasadnienie. Pełny plan Sprintu 23 powstaje *po* tej decyzji (na podstawie szkieletu powyżej).

## Outcome (wypełnij po sprincie)

- 📌 Path selected: <A / B / C + dlaczego>
- ✅ Done: <fill as tasks close>
- ⏭️ Carry-over: <task → Sprint 24 + reason>
- 📌 Decyzje: <ADR jeśli powstał>
- 🧠 Surprises: <co się okazało inne niż w docs>
- 📊 Metrics: zależne od ścieżki — adapter coverage / launch GitHub stars / partner replies.
