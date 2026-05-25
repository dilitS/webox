> **Estimated time:** 1 h · **Difficulty:** 🟢 translation · **Labels:** `good-first-issue` `help wanted` `documentation`
>
> **No Go knowledge required.** This issue is the perfect first PR for someone who wants to contribute without coding.
>
> **Maintainer pair-review available** — comment below before you start.

## What we want

Add a **German (`de`) translation** to the Webox i18n catalog. After this issue is closed, running `webox doctor` with `LANG=de_DE.UTF-8` (or `WEBOX_LANG=de`) should render every operator-facing string in German.

Webox currently ships `en` (default) and `pl` (Polish) — see [`i18n/i18n.go`](https://github.com/dilitS/webox/blob/main/i18n/i18n.go). The translation surface is **intentionally small** for v0.1: about 30 short strings related to `webox doctor`, the wizard, and the cockpit footer. The full catalog moves to external JSON tables in Sprint 07 — this issue scopes to the inline `tables` map only.

## How to start (15 minutes)

1. Read [`i18n/i18n.go`](https://github.com/dilitS/webox/blob/main/i18n/i18n.go) — the entire i18n contract is ~65 lines. The `tables` map at the bottom is what you edit.

2. Copy the `"pl"` block and rename it to `"de"`. Translate **values only**. Never rename keys.

   ```go
   "de": {
       "doctor.title":             "webox doctor",
       "doctor.summary":           "Zusammenfassung: %d ok, %d Warnungen, %d Fehler, %d übersprungen",
       "doctor.config_dir_ok":     "Konfigurationsverzeichnis %s ist beschreibbar.",
       "doctor.ssh_agent_missing": "SSH_AUTH_SOCK ist nicht gesetzt.",
       "doctor.fallback_warn":     "Geheimspeicher: Fallback (OS-Schlüsselbund nicht verfügbar).",
   },
   ```

3. Add `"de"` to the accepted-language switch in `func New(language string) Catalog`:

   ```go
   case "pl", "en", "de":
       return Catalog{language: language}
   ```

4. Add a test case in [`i18n/i18n_test.go`](https://github.com/dilitS/webox/blob/main/i18n/i18n_test.go) — clone the existing `"pl"` test, change the language code to `"de"`, and assert one translated string.

5. Run `make test` to confirm everything passes.

## Acceptance criteria

- [ ] `i18n/i18n.go` `tables` map has a `"de"` block with **every key** present in `"en"` (no missing keys, no surplus keys).
- [ ] `New("de").T("doctor.title")` returns the German rendering.
- [ ] `i18n/i18n_test.go` has at least one assertion for the new language.
- [ ] `make test` green.
- [ ] CHANGELOG `[Unreleased] / Added` entry.
- [ ] Translation quality:
  - Use formal `Sie` form where second-person address appears.
  - Use standard tech terminology (`Schlüsselbund` not `Schlüsselring` for keyring, `Konfiguration` not `Setup`).
  - Avoid Anglicisms (`Backend` is fine — it is standard tech German; `Frontend` ditto).

## Not in scope (defer to a separate PR)

- External JSON translation tables (Sprint 07 / v0.2+).
- TUI cockpit string translations (the tile labels, key bindings) — those still live in `tui/` and will be migrated to i18n in Sprint 07. Stick to the `tables` map for this PR.
- Adding `fr` / `es` / `it` — please open a separate issue per language so reviewers can focus.

## Useful references

- [ADR-0006 — English-default UI](https://github.com/dilitS/webox/blob/main/docs/adr/0006-jezyk-interfejsu-en-domyslny.md) (in Polish — Google Translate works; the gist is "EN is canonical; non-EN translations are explicit, manually verified, and never machine-translated in code paths").
- [`i18n/i18n.go`](https://github.com/dilitS/webox/blob/main/i18n/i18n.go) — the entire contract.
- [`i18n/i18n_test.go`](https://github.com/dilitS/webox/blob/main/i18n/i18n_test.go) — test pattern to copy.
- [German tech terminology cheat-sheet](https://github.com/i18next/i18next/discussions/1817) — community-maintained reference for the trickier words.
