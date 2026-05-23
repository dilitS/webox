package log

import "regexp"

const replacement = "[REDACTED]"

type redactionRule struct {
	pattern     *regexp.Regexp
	replacement string
}

var redactionRules = []redactionRule{
	{
		pattern:     regexp.MustCompile(`(?s)-{5}BEGIN [A-Z ]*PRIVATE KEY-{5}.*?-{5}END [A-Z ]*PRIVATE KEY-{5}`),
		replacement: replacement,
	},
	{
		pattern:     regexp.MustCompile(`(?i)(Authorization:\s*Bearer\s+)\S+`),
		replacement: `${1}` + replacement,
	},
	{
		pattern:     regexp.MustCompile(`://([^:\s/@]+):([^@\s/]+)@`),
		replacement: `://$1:` + replacement + `@`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)("(?:password|passwd|token|secret|api_key|key)"\s*:\s*")[^"]*(")`),
		replacement: `${1}` + replacement + `${2}`,
	},
	{
		pattern:     regexp.MustCompile(`(?m)^([A-Za-z0-9_]*(?:PASSWORD|PASSWD|SECRET|TOKEN|API_KEY|PRIVATE_KEY|KEY)[A-Za-z0-9_]*=).+$`),
		replacement: `${1}` + replacement,
	},
	{
		pattern:     regexp.MustCompile(`gh[opusr]_(?:[A-Za-z0-9]|\r?\n){36,255}`),
		replacement: replacement,
	},
	{
		pattern:     regexp.MustCompile(`github_pat_(?:[A-Za-z0-9_]|\r?\n){20,255}`),
		replacement: replacement,
	},
	{
		pattern:     regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		replacement: replacement,
	},
	{
		pattern:     regexp.MustCompile(`sk-[A-Za-z0-9_-]{16,}`),
		replacement: replacement,
	},
	{
		pattern:     regexp.MustCompile(`ssh-rsa\s+[A-Za-z0-9+/=]{40,}`),
		replacement: replacement,
	},
	// JWT: three base64url segments separated by dots. The header
	// segment always begins with `eyJ` (base64-url of `{"`); using it
	// as an anchor keeps the false-positive rate negligible.
	{
		pattern:     regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{20,}\b`),
		replacement: replacement,
	},
	// Generic key=value / key: value style secrets in CLI args, JSON
	// payloads, env lines, and config dumps. The value capture stops
	// at whitespace, ampersand, comma, or quote so we don't bleed the
	// surrounding sentence into the redaction marker.
	{
		pattern:     regexp.MustCompile(`(?i)\b(password|passwd|token|secret|api[_-]?key|access[_-]?key)\s*[:=]\s*([^\s&"',]{4,})`),
		replacement: `${1}=` + replacement,
	},
	// MySQL / PostgreSQL `-p<password>` form (no space between flag
	// and value). Anchored to the binary name so generic `-p` flags
	// in unrelated tools (curl, ssh) are not touched.
	{
		pattern:     regexp.MustCompile(`(?i)\b(mysql|mysqldump|psql)\b([^\n]*?)\s-p([^\s\-]\S*)`),
		replacement: `${1}${2} -p` + replacement,
	},
}

// Redact replaces known secret-shaped substrings with "[REDACTED]".
// It is pure: no I/O, no logging, no mutation of package state.
func Redact(input string) string {
	out := input
	for _, rule := range redactionRules {
		out = rule.pattern.ReplaceAllString(out, rule.replacement)
	}
	return out
}
