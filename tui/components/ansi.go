package components

import (
	"regexp"
	"strings"
)

// LogLevel is the rendering-level classification of a log line. The
// cockpit uses it to colour-code entries in the live-log tile; the
// SSH streamer (services/sshtail) stamps it on every emitted Line so
// the view layer never has to parse twice.
type LogLevel int

const (
	// LevelUnknown means the parser could not place the line. Empty
	// lines and binary blobs land here.
	LevelUnknown LogLevel = iota
	// LevelDebug — verbose diagnostic information.
	LevelDebug
	// LevelInfo — normal operational events (default for unprefixed
	// non-empty text so the live tail never goes blank).
	LevelInfo
	// LevelWarn — recoverable anomalies; coloured amber.
	LevelWarn
	// LevelError — unrecoverable or alarming events; coloured red.
	LevelError
)

// String returns the upper-case label used in tile rendering.
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ansiSGRPattern matches CSI sequences (ESC [ ... terminator) — the
// common SGR/cursor/erase escapes most log emitters produce. The
// terminator class covers the printable ASCII final byte range per
// ECMA-48 (`@`–`~`), spelled out so revive does not flag the
// suspicious-char-range heuristic.
var ansiSGRPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z@\[\\\]^_` + "`" + `{|}~]`)

// ansiOSCPattern strips OSC sequences (ESC ] ... BEL or ST). Hyperlinks
// emitted by modern tools (gh, kubectl) use this and would otherwise
// appear as garbage in the log tile.
var ansiOSCPattern = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)

// ansiResidualPattern catches stray BEL/CSI introducers after the
// structured patterns above ran.
var ansiResidualPattern = regexp.MustCompile(`[\x07\x1b]`)

// ANSIStrip removes terminal control sequences from line. It is the
// canonical pre-render hook for log lines; the streamer also calls it
// before pushing into the ring buffer so snapshot diffs stay stable.
func ANSIStrip(line string) string {
	if line == "" {
		return ""
	}
	out := ansiOSCPattern.ReplaceAllString(line, "")
	out = ansiSGRPattern.ReplaceAllString(out, "")
	out = ansiResidualPattern.ReplaceAllString(out, "")
	return out
}

// ParseLogLevel inspects line for well-known severity markers. The
// detection order mirrors the false-positive tolerance documented in
// `docs/sprints/sprint-09-live-log-stream.md`:
//
//  1. ANSI colour hint (red ⇒ ERROR, yellow ⇒ WARN) — non-strippable
//     because production loggers use it as the *only* level cue.
//  2. Structured prefixes (`[ERROR]`, `ERROR:`, JSON `"level":"warn"`).
//  3. Apache combined log `[error]` / `[warn]` markers.
//  4. Heuristic word-boundary scan for `error`/`warn`/`debug` tokens.
//  5. Fallback to LevelInfo for non-empty text (so the tail tile never
//     shows a sea of UNKNOWN labels for plain-text logs).
func ParseLogLevel(line string) LogLevel {
	if line == "" {
		return LevelUnknown
	}

	if level, ok := levelFromANSI(line); ok {
		return level
	}

	stripped := strings.TrimSpace(ANSIStrip(line))
	if stripped == "" {
		return LevelUnknown
	}

	lower := strings.ToLower(stripped)

	switch {
	case containsLevelToken(lower, "error"):
		return LevelError
	case containsLevelToken(lower, "warn"):
		return LevelWarn
	case containsLevelToken(lower, "debug"):
		return LevelDebug
	case containsLevelToken(lower, "info"):
		return LevelInfo
	}
	return LevelInfo
}

// levelFromANSI extracts the severity from raw ANSI colour escapes. We
// only honour the explicit foreground colours documented by the corpus
// in `tui/components/ansi_test.go`; everything else falls through to
// the textual classifier.
func levelFromANSI(line string) (LogLevel, bool) {
	switch {
	case strings.Contains(line, "\x1b[31m"), strings.Contains(line, "\x1b[91m"),
		strings.Contains(line, "\x1b[1;31m"):
		return LevelError, true
	case strings.Contains(line, "\x1b[33m"), strings.Contains(line, "\x1b[93m"),
		strings.Contains(line, "\x1b[1;33m"):
		return LevelWarn, true
	}
	return LevelUnknown, false
}

// containsLevelToken returns true when token is present as a discrete
// word or as a bracketed/JSON value. We avoid plain `strings.Contains`
// so "errorlessly" does not light up as ERROR.
func containsLevelToken(line, token string) bool {
	for _, marker := range []string{
		"[" + token + "]",
		"[" + token + ":",
		token + ":",
		token + "=",
		`"level":"` + token + `"`,
	} {
		if strings.Contains(line, marker) {
			return true
		}
	}
	// Word-boundary fallback (still cheap for log-line sized inputs).
	idx := strings.Index(line, token)
	for idx >= 0 {
		before := idx == 0 || !isWordRune(line[idx-1])
		after := idx+len(token) == len(line) || !isWordRune(line[idx+len(token)])
		if before && after {
			return true
		}
		next := strings.Index(line[idx+1:], token)
		if next < 0 {
			break
		}
		idx += 1 + next
	}
	return false
}

func isWordRune(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') ||
		b == '_'
}
