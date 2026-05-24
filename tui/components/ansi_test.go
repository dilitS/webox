package components_test

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/tui/components"
)

func TestANSIStripRemovesSGRSequences(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello world", "hello world"},
		{"sgr color", "\x1b[31mERR\x1b[0m boom", "ERR boom"},
		{"bold + color", "\x1b[1;33mWARN\x1b[0m", "WARN"},
		{"cursor move", "\x1b[2Aclear", "clear"},
		{"erase line", "\x1b[2Kreset", "reset"},
		{"truecolor", "\x1b[38;2;255;128;0mOK\x1b[0m", "OK"},
		{"OSC link", "\x1b]8;;https://example\x07click\x1b]8;;\x07", "click"},
		{"BEL only", "ding\x07", "ding"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := components.ANSIStrip(tc.in); got != tc.want {
				t.Fatalf("ANSIStrip(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseLogLevelRecognisesCommonPrefixes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		line string
		want components.LogLevel
	}{
		{"bracketed info", "[INFO] starting server", components.LevelInfo},
		{"bracketed lowercase info", "[info] starting server", components.LevelInfo},
		{"prefixed warn", "WARN: queue full", components.LevelWarn},
		{"prefixed error colon", "ERROR: oops", components.LevelError},
		{"node js error", "Error: connection refused", components.LevelError},
		{"morgan style", "GET /healthz 200 12ms", components.LevelInfo},
		{"apache combined error", `[Sat May 23 09:00:00 2026] [error] [client 1.2.3.4] denied`, components.LevelError},
		{"debug", "[DEBUG] worker tick", components.LevelDebug},
		{"json log warn", `{"level":"warn","msg":"slow query"}`, components.LevelWarn},
		{"json log error", `{"level":"error","msg":"db down"}`, components.LevelError},
		{"plain text", "starting", components.LevelInfo},
		{"empty line", "", components.LevelUnknown},
		{"ansi red sequence implies error", "\x1b[31mfailed connect\x1b[0m", components.LevelError},
		{"ansi yellow sequence implies warn", "\x1b[33mretrying\x1b[0m", components.LevelWarn},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := components.ParseLogLevel(tc.line); got != tc.want {
				t.Fatalf("ParseLogLevel(%q) = %s, want %s", tc.line, got, tc.want)
			}
		})
	}
}

func TestLogLevelString(t *testing.T) {
	t.Parallel()

	cases := map[components.LogLevel]string{
		components.LevelInfo:    "INFO",
		components.LevelWarn:    "WARN",
		components.LevelError:   "ERROR",
		components.LevelDebug:   "DEBUG",
		components.LevelUnknown: "UNKNOWN",
	}
	for level, want := range cases {
		if got := level.String(); got != want {
			t.Errorf("%v.String() = %q, want %q", level, got, want)
		}
	}
}

func TestParseLogLevelIsRobustToLongLines(t *testing.T) {
	t.Parallel()

	long := "[ERROR] " + strings.Repeat("x", 10_000)
	if got := components.ParseLogLevel(long); got != components.LevelError {
		t.Fatalf("ParseLogLevel(very long) = %s, want ERROR", got)
	}
}
