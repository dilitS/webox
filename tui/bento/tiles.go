package bento

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/tui/theme"
)

// ProjectRowSnapshot is the renderable projection of one row in the
// projects tile. Rendering is deferred to the tile so the data layer
// (`tui/view.go`) does not have to know about lipgloss styling.
type ProjectRowSnapshot struct {
	// Name is the project display label (typically the domain).
	Name string
	// State is the upper-cased status badge value (ONLINE/OFFLINE/
	// STALE/BUILDING/DEGRADED/UNKNOWN). The renderer maps each
	// value to the matching dot colour from the theme.
	State string
	// Selected marks the currently focused row; the renderer wraps
	// it in a subtle pill so the operator can spot the cursor at
	// a glance.
	Selected bool
}

// projectsTile renders the dashboard project list inside the Ultra grid.
// Each row is a `name + coloured dot` cell; the active row is wrapped
// in a thin primary-coloured pill (matches the reference image).
type projectsTile struct {
	rows  []ProjectRowSnapshot
	width int
}

// NewProjectsTile builds the top-left projects tile from a per-row
// snapshot. The tile owns the visual styling (dot colour + selection
// pill) so the data layer remains presentation-agnostic.
func NewProjectsTile(rows []ProjectRowSnapshot) BentoTile {
	return &projectsTile{rows: append([]ProjectRowSnapshot(nil), rows...)}
}

// ID satisfies [BentoTile].
func (t *projectsTile) ID() string { return "projects" }

// Slot satisfies [BentoTile].
func (t *projectsTile) Slot() Slot { return SlotProjects }

// WithWidth satisfies [WidthAware] so the bento engine can stretch
// the projects column to fill its allocated horizontal slice.
func (t *projectsTile) WithWidth(w int) BentoTile {
	clone := *t
	clone.width = w
	return &clone
}

// Render satisfies [BentoTile].
func (t *projectsTile) Render(mode Mode, focused bool) string {
	if len(t.rows) == 0 {
		return renderTilePanel(tilePanelOptions{
			Header:   "[Active Projects]",
			Body:     "No projects yet.\nPress [n] to start the new-project wizard.",
			Mode:     mode,
			Focused:  focused,
			Accent:   AccentPrimary,
			MinWidth: t.width,
		})
	}
	tokens := theme.Default()

	rendered := make([]string, 0, len(t.rows))
	for _, row := range t.rows {
		dot := lipgloss.NewStyle().
			Foreground(lipgloss.Color(stateColor(row.State, tokens))).
			Render("●")
		name := row.Name
		if row.Selected {
			pill := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(tokens.TextBright)).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(tokens.Primary)).
				Padding(0, 1).
				Render(name)
			rendered = append(rendered, lipgloss.JoinHorizontal(
				lipgloss.Top,
				pill,
				lipgloss.NewStyle().PaddingLeft(1).PaddingTop(1).Render(dot),
			))
			continue
		}
		row := lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.NewStyle().Foreground(lipgloss.Color(tokens.TextBright)).Render(name),
			lipgloss.NewStyle().PaddingLeft(1).Render(dot),
		)
		rendered = append(rendered, "  "+row)
	}
	body := strings.Join(rendered, "\n")
	return renderTilePanel(tilePanelOptions{
		Header:   "[Active Projects]",
		Body:     body,
		Mode:     mode,
		Focused:  focused,
		Accent:   AccentPrimary,
		MinWidth: t.width,
	})
}

// stateColor maps a status badge value (ONLINE/OFFLINE/STALE/...) to
// the theme colour the dot should render in. UNKNOWN falls back to
// the muted colour so the cockpit never shows a colour-less hole.
func stateColor(state string, tokens theme.Theme) string {
	switch strings.ToUpper(state) {
	case "ONLINE":
		return tokens.Success
	case "BUILDING":
		return tokens.Warning
	case "OFFLINE":
		return tokens.Error
	case "STALE":
		return tokens.Muted
	case "DEGRADED":
		return tokens.Degraded
	default:
		return tokens.TextDim
	}
}

// ServerOverviewSnapshot is the rich projection rendered inside the
// `[SERVER: <project>]` tile. Lines is the iconified body (one entry
// per row); ProjectAlias drives the tile header. Producers fill only
// the fields they have — missing entries are skipped so the tile
// never shows blank rows.
type ServerOverviewSnapshot struct {
	ProjectAlias string
	Lines        []ServerOverviewLine
}

// ServerOverviewLine is one icon-prefixed row in the server tile.
// Status (optional) lets the renderer paint a coloured dot at the
// end of the line — e.g. "Node.js: v20.11.0 ●" with a success-tinted
// dot when the project is online.
type ServerOverviewLine struct {
	Icon   string
	Label  string
	Value  string
	Status string
}

// overviewTile renders the per-project server overview pane. The new
// design (2026-05-24 refresh) uses iconified key-value rows that match
// the reference image's `[SERVER: <alias>]` panel.
type overviewTile struct {
	snap  ServerOverviewSnapshot
	width int
}

// NewOverviewTile builds the project-overview tile from a snapshot.
// Callers are expected to compute the snapshot from `config.Project`
// + the latest `status.Cache` lookup before each frame.
func NewOverviewTile(snap ServerOverviewSnapshot) BentoTile {
	return &overviewTile{snap: snap}
}

// ID satisfies [BentoTile].
func (t *overviewTile) ID() string { return "overview" }

// Slot satisfies [BentoTile].
func (t *overviewTile) Slot() Slot { return SlotOverview }

// WithWidth satisfies [WidthAware].
func (t *overviewTile) WithWidth(w int) BentoTile {
	clone := *t
	clone.width = w
	return &clone
}

// Render satisfies [BentoTile].
func (t *overviewTile) Render(mode Mode, focused bool) string {
	tokens := theme.Default()
	header := "[SERVER: " + nonEmpty(t.snap.ProjectAlias, "(no project)") + "]"

	if len(t.snap.Lines) == 0 {
		return renderTilePanel(tilePanelOptions{
			Header:   header,
			Body:     "Select a project to inspect status.",
			Mode:     mode,
			Focused:  focused,
			Accent:   AccentPrimary,
			MinWidth: t.width,
		})
	}

	rows := make([]string, 0, len(t.snap.Lines))
	for _, line := range t.snap.Lines {
		icon := nonEmpty(line.Icon, "·")
		row := lipgloss.NewStyle().
			Foreground(lipgloss.Color(tokens.Accent)).
			Render(icon) + " " +
			lipgloss.NewStyle().Foreground(lipgloss.Color(tokens.TextDim)).Render(line.Label+":") + " " +
			lipgloss.NewStyle().Foreground(lipgloss.Color(tokens.TextBright)).Render(line.Value)
		if line.Status != "" {
			dot := lipgloss.NewStyle().
				Foreground(lipgloss.Color(stateColor(line.Status, tokens))).
				Render("●")
			row += " " + dot
		}
		rows = append(rows, row)
	}
	return renderTilePanel(tilePanelOptions{
		Header:   header,
		Body:     strings.Join(rows, "\n"),
		Mode:     mode,
		Focused:  focused,
		Accent:   AccentPrimary,
		MinWidth: t.width,
	})
}

// Placeholder tiles are now bespoke types (metricsPlaceholderTile,
// placeholderCICDTile, logsPlaceholderTile, topologyPlaceholderTile)
// so each can carry its own header label and accent. The previous
// generic placeholderTile struct was removed in the 2026-05-24 design
// refresh.

// NewMetricsPlaceholderTile returns the header-metrics placeholder used
// when no live metrics snapshot is available yet (initial render before
// the first SSH poll completes). Once metrics arrive, the view layer
// swaps it for [NewHeaderMetricsTile].
//
// As of the 2026-05-24 design refresh, metrics also surface in the
// cockpit-wide status bar rendered above the bento grid. The tile is
// retained for terminals where the status bar is suppressed (Standard
// fallback < 120 cols) and for unit-test parity.
func NewMetricsPlaceholderTile() BentoTile {
	return &metricsPlaceholderTile{}
}

type metricsPlaceholderTile struct{}

// ID satisfies [BentoTile].
func (metricsPlaceholderTile) ID() string { return "header-metrics" }

// Slot satisfies [BentoTile].
func (metricsPlaceholderTile) Slot() Slot { return SlotMetrics }

// Render satisfies [BentoTile].
func (metricsPlaceholderTile) Render(mode Mode, focused bool) string {
	return renderTilePanel(tilePanelOptions{
		Header:  "[Header Metrics]",
		Body:    "CPU / RAM / Disk pulse\nAwaiting first SSH poll…",
		Mode:    mode,
		Focused: focused,
		Accent:  AccentPrimary,
	})
}

// HeaderMetricsSnapshot is the view-layer projection of
// [sshmetrics.Metrics] consumed by [NewHeaderMetricsTile]. The bento
// engine does not depend on the metrics package directly, so the
// snapshot decouples the rendering layer from the polling pipeline.
type HeaderMetricsSnapshot struct {
	ProfileAlias string
	UptimeLabel  string
	LoadLabel    string
	RAMLabel     string
	RTTLabel     string
	Stale        bool
}

type headerMetricsTile struct {
	snap HeaderMetricsSnapshot
}

// NewHeaderMetricsTile constructs the Bento Ultra header-metrics tile
// from a snapshot. The renderer adds a `[LIVE]` / `[STALE]` indicator
// based on snap.Stale so the operator can tell at a glance whether the
// numbers are fresh.
func NewHeaderMetricsTile(snap HeaderMetricsSnapshot) BentoTile {
	return &headerMetricsTile{snap: snap}
}

// ID satisfies [BentoTile].
func (t *headerMetricsTile) ID() string { return "header-metrics" }

// Slot satisfies [BentoTile].
func (t *headerMetricsTile) Slot() Slot { return SlotMetrics }

// Render satisfies [BentoTile].
func (t *headerMetricsTile) Render(mode Mode, focused bool) string {
	tokens := theme.Default()
	indicator := "LIVE"
	indicatorBg := tokens.Success
	if t.snap.Stale {
		indicator = "STALE"
		indicatorBg = tokens.Warning
	}
	pill := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(tokens.SurfaceBase)).
		Background(lipgloss.Color(indicatorBg)).
		Padding(0, 1).
		Render(indicator)
	body := strings.Join([]string{
		pill + " " + nonEmpty(t.snap.ProfileAlias, "(no profile)"),
		"Uptime: " + nonEmpty(t.snap.UptimeLabel, "—"),
		"Load:   " + nonEmpty(t.snap.LoadLabel, "—"),
		"RAM:    " + nonEmpty(t.snap.RAMLabel, "—"),
		"Ping:   " + nonEmpty(t.snap.RTTLabel, "—"),
	}, "\n")
	return renderTilePanel(tilePanelOptions{
		Header:  "[Header Metrics]",
		Body:    body,
		Mode:    mode,
		Focused: focused,
		Accent:  AccentPrimary,
	})
}

// NewCICDPlaceholderTile returns the CI/CD pipeline placeholder used
// before the operator selects a GitHub-linked project (or while the
// first poll is still in flight). The view layer swaps it for
// [NewCICDPipelineTile] once a [CICDPipelineSnapshot] is available.
func NewCICDPlaceholderTile() BentoTile {
	return &placeholderCICDTile{}
}

// placeholderCICDTile is the cyan-accented placeholder shown until the
// first pipeline poll completes. Kept as a dedicated type so it can
// carry the [CI/CD PIPELINE: Main Branch] header and AccentCyan.
type placeholderCICDTile struct {
	width int
}

// ID satisfies [BentoTile].
func (*placeholderCICDTile) ID() string { return "cicd-pipeline" }

// Slot satisfies [BentoTile].
func (*placeholderCICDTile) Slot() Slot { return SlotCICD }

// WithWidth satisfies [WidthAware].
func (t *placeholderCICDTile) WithWidth(w int) BentoTile {
	clone := *t
	clone.width = w
	return &clone
}

// Render satisfies [BentoTile].
func (t *placeholderCICDTile) Render(mode Mode, focused bool) string {
	return renderTilePanel(tilePanelOptions{
		Header:   "[CI/CD PIPELINE: Main Branch]",
		Body:     "GitHub Actions stream\nNo GitHub-linked project selected.\nPress [n] to create a new project.",
		Mode:     mode,
		Focused:  focused,
		Accent:   AccentCyan,
		MinWidth: t.width,
	})
}

// CICDStatus enumerates the badge rendering modes used by
// [CICDStepSnapshot]. Keeping the enum centralised lets snapshot
// producers (`tui/view.go`) stay free of `services/github` imports.
type CICDStatus int

// CICDStatus enum values mirror the GitHub Actions step status
// vocabulary (queued, in_progress, completed × conclusion). The
// renderer maps each to a UX-§3.1 badge.
const (
	CICDStatusUnknown CICDStatus = iota
	CICDStatusQueued
	CICDStatusInProgress
	CICDStatusSuccess
	CICDStatusFailure
	CICDStatusCancelled
	CICDStatusSkipped
)

// CICDStepSnapshot is the per-step projection rendered inside the
// CI/CD tile. The shape mirrors the documented numbered-list cell
// pattern in UX §3.1: `[N] <name> <badge>`.
type CICDStepSnapshot struct {
	Number int
	Name   string
	Status CICDStatus
	// Duration is rendered after the badge ("✓ 12s"). When zero, the
	// renderer omits it so queued steps stay clean.
	Duration string
}

// CICDPipelineSnapshot is the full tile projection: header line
// (`Build #N: STATUS · DURATION`), step list, and the optional
// rate-limit footer that TASK-10.5 surfaces when GitHub returns a
// `429`/`x-ratelimit-remaining: 0` response.
type CICDPipelineSnapshot struct {
	ProjectAlias  string
	WorkflowName  string
	RunNumber     int
	RunStatus     CICDStatus
	RunSummary    string // "completed", "in_progress", etc.
	HeaderTime    string // RFC formatted timestamp (already formatted)
	Duration      string
	Steps         []CICDStepSnapshot
	Stale         bool
	RateLimited   bool
	RateLimitHint string // "Reset in 12min" when known.
	ErrorMessage  string // populated when GitHub call failed (non-rate-limit).
}

type cicdPipelineTile struct {
	snap  CICDPipelineSnapshot
	width int
}

// NewCICDPipelineTile renders the live GitHub Actions tile. The
// snapshot is computed in `tui/view.go` from a `status.Cache` lookup so
// the bento layer remains presentation-only (no API knowledge, no
// secrets, no goroutines).
func NewCICDPipelineTile(snap CICDPipelineSnapshot) BentoTile {
	return &cicdPipelineTile{snap: snap}
}

// ID satisfies [BentoTile].
func (t *cicdPipelineTile) ID() string { return "cicd-pipeline" }

// Slot satisfies [BentoTile].
func (t *cicdPipelineTile) Slot() Slot { return SlotCICD }

// WithWidth satisfies [WidthAware].
func (t *cicdPipelineTile) WithWidth(w int) BentoTile {
	clone := *t
	clone.width = w
	return &clone
}

// Render satisfies [BentoTile].
func (t *cicdPipelineTile) Render(mode Mode, focused bool) string {
	tokens := theme.Default()
	var b strings.Builder

	indicator := "[LIVE]"
	indicatorTone := tokens.Success
	if t.snap.Stale {
		indicator = "[STALE]"
		indicatorTone = tokens.Warning
	}
	if t.snap.RateLimited {
		indicator = "[LIMITED]"
		indicatorTone = tokens.Warning
	}
	pill := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(tokens.SurfaceBase)).
		Background(lipgloss.Color(indicatorTone)).
		Padding(0, 1).
		Render(strings.Trim(indicator, "[]"))
	headerLine := pill + " " + nonEmpty(t.snap.ProjectAlias, "(no project)")
	if t.snap.WorkflowName != "" {
		headerLine += " · " + t.snap.WorkflowName
	}
	b.WriteString(headerLine)
	b.WriteString("\n")

	if t.snap.RateLimited {
		b.WriteString("GitHub rate limit reached. Cached data shown.")
		if t.snap.RateLimitHint != "" {
			b.WriteString(" " + t.snap.RateLimitHint + ".")
		}
		b.WriteString("\n")
	} else if t.snap.ErrorMessage != "" {
		b.WriteString(t.snap.ErrorMessage)
		b.WriteString("\n")
	}

	if t.snap.RunNumber > 0 {
		runLabel := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(tokens.TextBright)).
			Render("Build #" + intString(t.snap.RunNumber))
		runLine := runLabel + ": " + cicdStatusLabelStyled(t.snap.RunStatus, tokens)
		if t.snap.Duration != "" {
			runLine += " · " + t.snap.Duration
		}
		if t.snap.HeaderTime != "" {
			runLine += " (" + t.snap.HeaderTime + ")"
		}
		b.WriteString(runLine)
		b.WriteString("\n")
	} else if !t.snap.RateLimited && t.snap.ErrorMessage == "" {
		b.WriteString("No workflow run yet for main branch.\n")
	}

	if len(t.snap.Steps) > 0 {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(tokens.TextDim)).
			Render("Pipeline Steps:"))
		b.WriteString("\n")
	}
	for _, step := range t.snap.Steps {
		numberCell := lipgloss.NewStyle().
			Foreground(lipgloss.Color(tokens.Accent)).
			Render("[" + intString(step.Number) + "]")
		badge := cicdStatusBadgeStyled(step.Status, tokens)
		stepLine := numberCell + " " +
			lipgloss.NewStyle().Foreground(lipgloss.Color(tokens.TextBright)).Render(step.Name) + " " +
			badge
		if step.Duration != "" {
			stepLine += " " + lipgloss.NewStyle().
				Foreground(lipgloss.Color(tokens.TextDim)).
				Render("· "+step.Duration)
		}
		b.WriteString(stepLine)
		b.WriteString("\n")
	}

	if len(t.snap.Steps) > 0 {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(tokens.Accent)).
			Render("View Details (F8)"))
		b.WriteString(" ")
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(tokens.TextDim)).
			Render("· [Enter] Open run"))
	}

	return renderTilePanel(tilePanelOptions{
		Header:   "[CI/CD PIPELINE: Main Branch]",
		Body:     strings.TrimRight(b.String(), "\n"),
		Mode:     mode,
		Focused:  focused,
		Accent:   AccentCyan,
		MinWidth: t.width,
	})
}

// cicdStatusBadgeStyled returns the per-step badge as a styled string
// (coloured glyph + bold). The 2026-05-24 refresh paints the badge in
// the matching status tone so the CI/CD tile reads at a glance.
func cicdStatusBadgeStyled(s CICDStatus, tokens theme.Theme) string {
	colour := tokens.TextDim
	switch s {
	case CICDStatusSuccess:
		colour = tokens.Success
	case CICDStatusFailure:
		colour = tokens.Error
	case CICDStatusInProgress, CICDStatusQueued:
		colour = tokens.Warning
	case CICDStatusSkipped, CICDStatusCancelled:
		colour = tokens.Muted
	case CICDStatusUnknown:
		colour = tokens.TextDim
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(colour)).
		Render(cicdStatusBadge(s))
}

// cicdStatusLabelStyled returns the run-header label coloured by tone.
func cicdStatusLabelStyled(s CICDStatus, tokens theme.Theme) string {
	colour := tokens.TextDim
	switch s {
	case CICDStatusSuccess:
		colour = tokens.Success
	case CICDStatusFailure:
		colour = tokens.Error
	case CICDStatusInProgress, CICDStatusQueued:
		colour = tokens.Warning
	case CICDStatusSkipped, CICDStatusCancelled:
		colour = tokens.Muted
	case CICDStatusUnknown:
		colour = tokens.TextDim
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(colour)).
		Render(cicdStatusLabel(s))
}

// cicdStatusBadge returns the per-step badge string. The mapping
// matches UX §3.1 (Premium Badges of Status).
func cicdStatusBadge(s CICDStatus) string {
	switch s {
	case CICDStatusSuccess:
		return "✓"
	case CICDStatusFailure:
		return "✗"
	case CICDStatusInProgress:
		return "⏳"
	case CICDStatusQueued:
		return "…"
	case CICDStatusSkipped:
		return "⊘"
	case CICDStatusCancelled:
		return "⊗"
	case CICDStatusUnknown:
		return "?"
	default:
		return "?"
	}
}

// cicdStatusLabel returns the verbose label rendered in the build
// header line. Lowercase verbs match the gh CLI vocabulary.
func cicdStatusLabel(s CICDStatus) string {
	switch s {
	case CICDStatusSuccess:
		return "SUCCESS ✓"
	case CICDStatusFailure:
		return "FAILED ✗"
	case CICDStatusInProgress:
		return "IN_PROGRESS ⏳"
	case CICDStatusQueued:
		return "QUEUED …"
	case CICDStatusSkipped:
		return "SKIPPED ⊘"
	case CICDStatusCancelled:
		return "CANCELLED ⊗"
	case CICDStatusUnknown:
		return "UNKNOWN ?"
	default:
		return "UNKNOWN ?"
	}
}

// intString is a tiny helper kept here so the bento package stays
// `strconv`-free (we already pull `strings` and `lipgloss`).
func intString(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	const base = 10
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%base)
		n /= base
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// NewLogsPlaceholderTile returns the live-log placeholder used when no
// project is selected (or before the first stream line arrives). The
// view layer swaps it for [NewMicroLogsTile] once tail data flows.
func NewLogsPlaceholderTile() BentoTile {
	return &logsPlaceholderTile{}
}

type logsPlaceholderTile struct {
	width int
}

// ID satisfies [BentoTile].
func (*logsPlaceholderTile) ID() string { return "live-logs" }

// Slot satisfies [BentoTile].
func (*logsPlaceholderTile) Slot() Slot { return SlotLogs }

// WithWidth satisfies [WidthAware].
func (t *logsPlaceholderTile) WithWidth(w int) BentoTile {
	clone := *t
	clone.width = w
	return &clone
}

// Render satisfies [BentoTile].
func (t *logsPlaceholderTile) Render(mode Mode, focused bool) string {
	return renderTilePanel(tilePanelOptions{
		Header:   "[Live Server Logs]",
		Body:     "SSH tail with secret redaction\nSelect a project to start streaming.",
		Mode:     mode,
		Focused:  focused,
		Accent:   AccentPrimary,
		MinWidth: t.width,
	})
}

// MicroLogLine is the view-layer projection of one tail entry. The
// bento package intentionally avoids depending on `services/sshtail`
// directly so the layout engine remains pure.
type MicroLogLine struct {
	// Timestamp is the formatted clock prefix (e.g. "14:32:10").
	// Empty strings are skipped so unit tests can feed legacy data
	// without timing fixtures.
	Timestamp string
	Level     string
	Source    string
	Text      string
	Redacted  bool
}

type microLogsTile struct {
	domain string
	lines  []MicroLogLine
	width  int
}

// NewMicroLogsTile renders the bottom-centre live-tail tile populated
// from a ring buffer snapshot. The caller is expected to clamp the
// slice to the most recent N lines (defaults to 5 in the standard
// cockpit and 8 in Ultra+) before passing it in.
func NewMicroLogsTile(domain string, lines []MicroLogLine) BentoTile {
	cp := make([]MicroLogLine, len(lines))
	copy(cp, lines)
	return &microLogsTile{domain: domain, lines: cp}
}

// NewMicroLogsTileWithWidth is the wide variant used in the cockpit's
// bottom row, where the tile spans the full grid width. The min-width
// keeps the panel from collapsing to the longest line when the buffer
// shrinks to a handful of rows.
func NewMicroLogsTileWithWidth(domain string, lines []MicroLogLine, width int) BentoTile {
	tile := NewMicroLogsTile(domain, lines).(*microLogsTile)
	tile.width = width
	return tile
}

// WithWidth satisfies [WidthAware].
func (t *microLogsTile) WithWidth(w int) BentoTile {
	clone := *t
	clone.width = w
	return &clone
}

// ID satisfies [BentoTile].
func (t *microLogsTile) ID() string { return "live-logs" }

// Slot satisfies [BentoTile].
func (t *microLogsTile) Slot() Slot { return SlotLogs }

// Render satisfies [BentoTile].
func (t *microLogsTile) Render(mode Mode, focused bool) string {
	tokens := theme.Default()
	if len(t.lines) == 0 {
		return renderTilePanel(tilePanelOptions{
			Header:   "[Live Server Logs]",
			Body:     "Streaming " + nonEmpty(t.domain, "—") + " — waiting for first line.",
			Mode:     mode,
			Focused:  focused,
			Accent:   AccentPrimary,
			MinWidth: t.width,
		})
	}
	rows := make([]string, 0, len(t.lines))
	for _, line := range t.lines {
		rows = append(rows, renderLogLine(line, tokens))
	}
	return renderTilePanel(tilePanelOptions{
		Header:   "[Live Server Logs]",
		Body:     strings.Join(rows, "\n"),
		Mode:     mode,
		Focused:  focused,
		Accent:   AccentPrimary,
		MinWidth: t.width,
	})
}

// renderLogLine formats one log entry as
// `[HH:MM:SS] LEVEL - source: message`, painting the level cell in
// the matching theme tone. Source/Timestamp are optional.
func renderLogLine(line MicroLogLine, tokens theme.Theme) string {
	var b strings.Builder
	if line.Timestamp != "" {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(tokens.TextDim)).
			Render("[" + line.Timestamp + "] "))
	}
	level := strings.ToUpper(strings.TrimSpace(line.Level))
	if level == "" {
		level = "INFO"
	}
	b.WriteString(styleLevelCell(level, tokens))
	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color(tokens.TextDim)).
		Render(" - "))
	if line.Source != "" {
		b.WriteString(lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(tokens.TextBright)).
			Render(line.Source))
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(tokens.TextDim)).
			Render(": "))
	}
	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color(tokens.TextBright)).
		Render(line.Text))
	if line.Redacted {
		b.WriteString(" ")
		b.WriteString(lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color(tokens.Muted)).
			Render("(redacted)"))
	}
	return b.String()
}

// styleLevelCell renders the four supported log levels as fixed-width
// coloured tokens. Unknown levels fall back to a dim INFO so the layout
// never jumps if a producer emits an unexpected level.
func styleLevelCell(level string, tokens theme.Theme) string {
	const cellWidth = 5
	pad := func(s string) string {
		if len(s) >= cellWidth {
			return s
		}
		return s + strings.Repeat(" ", cellWidth-len(s))
	}
	switch level {
	case "ERROR":
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(tokens.Error)).
			Render(pad("ERROR"))
	case "WARN":
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(tokens.Warning)).
			Render(pad("WARN"))
	case "DEBUG":
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(tokens.Degraded)).
			Render(pad("DEBUG"))
	default:
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(tokens.Accent)).
			Render(pad("INFO"))
	}
}

// nonEmpty returns fallback when value is the empty string.
func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// NewTopologyPlaceholderTile returns the service-topology placeholder
// scheduled to ship in Sprint 11 (ASCII graph with live status pulse).
func NewTopologyPlaceholderTile() BentoTile {
	return &topologyPlaceholderTile{}
}

type topologyPlaceholderTile struct {
	width int
}

// ID satisfies [BentoTile].
func (*topologyPlaceholderTile) ID() string { return "topology" }

// Slot satisfies [BentoTile].
func (*topologyPlaceholderTile) Slot() Slot { return SlotTopology }

// WithWidth satisfies [WidthAware].
func (t *topologyPlaceholderTile) WithWidth(w int) BentoTile {
	clone := *t
	clone.width = w
	return &clone
}

// Render satisfies [BentoTile].
func (t *topologyPlaceholderTile) Render(mode Mode, focused bool) string {
	return renderTilePanel(tilePanelOptions{
		Header:   "[Topology]",
		Body:     "Service dependency graph\nLive wiring: Sprint 11",
		Mode:     mode,
		Focused:  focused,
		Accent:   AccentCyan,
		MinWidth: t.width,
	})
}

// TileAccent picks the border / header colour of a bento tile. The
// cockpit groups tiles into two visual columns:
//   - AccentPrimary  → magenta-violet (Projects, Server, Logs)
//   - AccentCyan     → cyan (CI/CD pipeline)
//
// Other accents are reserved for future surfaces (e.g. AccentDegraded
// for a "stale data" overlay).
type TileAccent int

const (
	// AccentPrimary paints the cockpit's primary column tiles.
	AccentPrimary TileAccent = iota
	// AccentCyan paints the CI/CD column tile.
	AccentCyan
	// AccentWarning paints attention-grabbing tiles (e.g. stale).
	AccentWarning
	// AccentError paints error states (e.g. offline server).
	AccentError
)

// tilePanelOptions captures the per-call presentation knobs. Keeping a
// single options struct (rather than a long positional argument list)
// makes future additions (icons, badges) source-compatible.
type tilePanelOptions struct {
	Header    string
	Body      string
	Mode      Mode
	Focused   bool
	Accent    TileAccent
	EmptyHint string
	// MinWidth, when > 0, forces the tile to expand to that width so
	// sibling cells in the same row align even when the body is
	// short. The renderer never truncates content below MinWidth.
	MinWidth int
}

// accentColor returns the hex string for the requested tile accent.
// Returning hex (not lipgloss.Color) lets tests assert on it directly.
func accentColor(t TileAccent, tokens theme.Theme) string {
	switch t {
	case AccentCyan:
		return tokens.Accent
	case AccentWarning:
		return tokens.Warning
	case AccentError:
		return tokens.Error
	default:
		return tokens.Primary
	}
}

// renderTilePanel composes a single bento cell: header line + body, wrapped
// in a rounded-border panel. The border color tracks the accent so the
// cockpit can visually group tiles into role columns; the focus state
// brightens the same accent without swapping it.
func renderTilePanel(opts tilePanelOptions) string {
	tokens := theme.Default()
	accentHex := accentColor(opts.Accent, tokens)
	// 2026-05-24 refresh: tiles keep their accent border in both
	// focus states so the operator can identify panels by colour
	// alone. Focus is communicated through the brighter header
	// foreground; the border stays accented.
	border := lipgloss.Color(accentHex)
	_ = opts.Focused

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(accentHex)).
		Render(opts.Header)

	body := opts.Body
	if body == "" && opts.EmptyHint != "" {
		body = opts.EmptyHint
	}

	content := header + "\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color(tokens.TextBright)).
		Render(body)

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1)
	if opts.MinWidth > 0 {
		style = style.Width(opts.MinWidth)
	}
	return style.Render(content)
}
