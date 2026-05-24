package views

import (
	"fmt"
	"strings"
)

const (
	liveLogsMinWidth   = 80
	liveLogsMaxWidth   = 120
	liveLogsTailWindow = 18 // Default visible rows in the Standard cockpit.
)

// RenderLiveLogs draws the Sprint 09 live-log tab. Behaviour:
//
//   - When no project is selected, the panel shows a friendly hint to
//     return to the dashboard.
//   - The status strip reports the active log file, stream state, and
//     buffer fill so the operator knows whether they are looking at
//     live data or buffered history.
//   - Lines render in chronological order (oldest at the top, newest
//     at the bottom). Each row is prefixed with a single-glyph level
//     marker so colourblind operators can still tell ERROR from INFO.
func RenderLiveLogs(s Screen) string {
	project, ok := selectedProject(s)
	if !ok {
		return s.Styles.Panel.
			Width(clamp(s.Width, liveLogsMinWidth, liveLogsMaxWidth)).
			Render("No project selected.\n\nesc: back to dashboard")
	}

	width := clamp(s.Width, liveLogsMinWidth, liveLogsMaxWidth)
	tabs := strings.Join([]string{
		s.Styles.Muted.Render("[1] Overview"),
		s.Styles.Muted.Render("[2] Env Diff - unlocked in v0.2"),
		s.Styles.Muted.Render("[3] Database - unlocked in v0.2"),
		"[4] Logs",
	}, "  ")

	snap := s.LiveLogs
	streamMode := "Tail -f: Off"
	if snap.AutoScroll {
		streamMode = "Tail -f: On"
	}
	connected := "Connected"
	if !snap.Connected {
		connected = "Disconnected"
	}
	statusLine := fmt.Sprintf(
		"Active File: %s · Stream: %s · %s · Buffer: %d/%d lines",
		nonEmptyValue(snap.LogPath, "(awaiting log path)"),
		streamMode,
		connected,
		snap.BufferUsed,
		fallbackBufferCap(snap.BufferCap),
	)

	body := []string{
		"📜 [Live Logs: " + project.Domain + "]",
		"",
		tabs,
		"",
		statusLine,
		"",
		renderLiveLogLines(s, snap),
		"",
		s.Styles.Muted.Render("[f] toggle auto-scroll  [c] clear buffer  [Esc] back"),
		s.Styles.HelpHints.Render("up/down: scroll · q: quit"),
	}
	if snap.Err != "" {
		body = append(body, "", s.Styles.Alert.Render("stream error: "+snap.Err))
	}
	if s.Alert != "" {
		body = append(body, "", s.Styles.Alert.Render(s.Alert))
	}

	return s.Styles.ActivePanel.Width(width).Render(strings.Join(body, "\n"))
}

func renderLiveLogLines(s Screen, snap LiveLogsSnapshot) string {
	if len(snap.Lines) == 0 {
		return s.Styles.Muted.Render("Waiting for the first log line…")
	}
	rows := make([]string, 0, len(snap.Lines))
	for _, line := range snap.Lines {
		marker, style := levelMarker(s, line.Level)
		text := line.Text
		if line.Redacted {
			text += "  " + s.Styles.Muted.Render("(redacted)")
		}
		rows = append(rows, style.Render(marker+" "+text))
	}
	return strings.Join(rows, "\n")
}

func levelMarker(s Screen, level string) (marker string, style stylesAdapter) {
	switch level {
	case "ERROR":
		return "✗", stylesAdapter{render: s.Styles.Alert.Render}
	case "WARN":
		return "!", stylesAdapter{render: s.Styles.HelpHints.Render}
	case "DEBUG":
		return "›", stylesAdapter{render: s.Styles.Muted.Render}
	case "INFO":
		return "·", stylesAdapter{render: passthrough}
	default:
		return "·", stylesAdapter{render: passthrough}
	}
}

// stylesAdapter is a tiny shim so the level-marker switch above stays a
// pure function — we hand the renderer back a function value instead
// of leaking lipgloss styles outside the helper.
type stylesAdapter struct {
	render func(...string) string
}

// Render applies the adapter to one row. Variadic to match lipgloss.
func (a stylesAdapter) Render(s string) string {
	if a.render == nil {
		return s
	}
	return a.render(s)
}

func passthrough(in ...string) string { return strings.Join(in, " ") }

func nonEmptyValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func fallbackBufferCap(capacity int) int {
	if capacity == 0 {
		return liveLogsTailWindow
	}
	return capacity
}
