// Package i18n loads the embedded translation tables and exposes a
// T(key, args...) helper used across the TUI.
//
// MVP ships English and Polish with English as the default per
// docs/adr/0006-jezyk-interfejsu-en-domyslny.md. Translations are
// validated at build time so missing keys fail CI rather than reaching
// end users; see Makefile target `i18n-check`.
package i18n
