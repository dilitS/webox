package i18n

import "fmt"

const (
	defaultLanguage = "en"
)

// Catalog is a tiny in-memory translation table. Sprint 01 only needs a
// stub for doctor-facing strings; the full translation loader with
// external JSON tables lands in Sprint 07.
type Catalog struct {
	language string
}

// New returns a catalog for the requested language. Unsupported
// languages fall back to English per ADR-0006.
func New(language string) Catalog {
	switch language {
	case "pl", "en":
		return Catalog{language: language}
	default:
		return Catalog{language: defaultLanguage}
	}
}

// T formats the translated value for key using fmt.Sprintf semantics.
// Unknown keys fail soft and return the key itself so callers never
// render an empty string during early skeleton phases.
func (c Catalog) T(key string, args ...any) string {
	template, ok := tables[c.language][key]
	if !ok {
		template = tables[defaultLanguage][key]
	}
	if template == "" {
		template = key
	}
	if len(args) == 0 {
		return template
	}
	return fmt.Sprintf(template, args...)
}

// T uses the default English catalog.
func T(key string, args ...any) string {
	return New(defaultLanguage).T(key, args...)
}

var tables = map[string]map[string]string{
	"en": {
		"doctor.title":             "webox doctor",
		"doctor.summary":           "summary: %d ok, %d warn, %d fail, %d skipped",
		"doctor.config_dir_ok":     "Config directory %s is writable.",
		"doctor.ssh_agent_missing": "SSH_AUTH_SOCK is not set.",
		"doctor.fallback_warn":     "Secrets backend: fallback (OS keyring unavailable).",
	},
	"pl": {
		"doctor.title":             "webox doctor",
		"doctor.summary":           "podsumowanie: %d ok, %d ostrzeżeń, %d błędów, %d pominiętych",
		"doctor.config_dir_ok":     "Katalog konfiguracyjny %s jest zapisywalny.",
		"doctor.ssh_agent_missing": "SSH_AUTH_SOCK nie jest ustawiony.",
		"doctor.fallback_warn":     "Backend sekretów: fallback (systemowy keyring niedostępny).",
	},
}
