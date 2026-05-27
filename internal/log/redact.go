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
	// cPanel UAPI: `Authorization: cpanel <user>:<token>`. The user
	// is not sensitive on its own (and being able to see it makes
	// post-incident triage easier) but the token after the colon
	// must never reach a log line. Preserve the literal `cpanel `
	// prefix and the username so the redaction is still legible.
	// See providers/cpanel/uapi/transport.go where this exact
	// header shape is emitted.
	{
		pattern:     regexp.MustCompile(`(?i)(Authorization:\s*cpanel\s+[^:\s]+:)\S+`),
		replacement: `${1}` + replacement,
	},
	// DirectAdmin Live API: `Authorization: Basic <base64(user:loginkey)>`.
	// The base64 envelope embeds both username and the login key, so
	// we redact the whole opaque blob — partial redaction would still
	// leak the username when an operator runs `echo <b64> | base64 -d`
	// on a copy-pasted log fragment.
	{
		pattern:     regexp.MustCompile(`(?i)(Authorization:\s*Basic\s+)\S+`),
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
	// surrounding sentence into the redaction marker. The alternation
	// also includes `login[_-]?key` (DirectAdmin's bearer credential
	// name — distinct from cPanel's "token") and `loginkey` styled
	// as `loginkey=…` from `webox doctor directadmin --loginkey=` CLI
	// arg or `DA_LOGIN_KEY=…` env dumps.
	{
		pattern:     regexp.MustCompile(`(?i)\b(password|passwd|token|secret|api[_-]?key|access[_-]?key|login[_-]?key)\s*[:=]\s*([^\s&"',]{4,})`),
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
