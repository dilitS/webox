package bento

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/tui/theme"
)

// projectsTile renders the dashboard project list inside the Ultra grid.
// In Sprint 09+ the tile gains inline status badges; for now it consumes
// the same pre-formatted strings that the Standard cockpit uses so the
// two layouts stay visually consistent.
type projectsTile struct {
	rows []string
}

// NewProjectsTile builds the top-left projects tile. Pass already
// formatted rows (e.g. `"> alpha.example.com [ONLINE]"`) so the tile
// remains presentation-agnostic.
func NewProjectsTile(rows []string) BentoTile {
	return &projectsTile{rows: append([]string(nil), rows...)}
}

// ID satisfies [BentoTile].
func (t *projectsTile) ID() string { return "projects" }

// Slot satisfies [BentoTile].
func (t *projectsTile) Slot() Slot { return SlotProjects }

// Render satisfies [BentoTile].
func (t *projectsTile) Render(mode Mode, focused bool) string {
	body := strings.Join(t.rows, "\n")
	if len(t.rows) == 0 {
		body = "No projects yet.\nPress [n] to start the new-project wizard."
	}
	return renderTilePanel(tilePanelOptions{
		Header:    "[Projects]",
		Body:      body,
		Mode:      mode,
		Focused:   focused,
		EmptyHint: "",
	})
}

// overviewTile renders the per-project overview pane: HTTP, SSL, Node,
// repository, last deploy. Lines are pre-rendered by view.go so the tile
// does not depend on `config.Project`.
type overviewTile struct {
	domain string
	lines  []string
}

// NewOverviewTile builds the project-overview tile. Pass an empty domain
// when no project is selected; the tile shows a "select a project" hint.
func NewOverviewTile(domain string, lines []string) BentoTile {
	return &overviewTile{
		domain: domain,
		lines:  append([]string(nil), lines...),
	}
}

// ID satisfies [BentoTile].
func (t *overviewTile) ID() string { return "overview" }

// Slot satisfies [BentoTile].
func (t *overviewTile) Slot() Slot { return SlotOverview }

// Render satisfies [BentoTile].
func (t *overviewTile) Render(mode Mode, focused bool) string {
	var body strings.Builder
	if t.domain != "" {
		body.WriteString(t.domain)
		body.WriteString("\n")
	}
	body.WriteString(strings.Join(t.lines, "\n"))
	return renderTilePanel(tilePanelOptions{
		Header:  "[Overview]",
		Body:    body.String(),
		Mode:    mode,
		Focused: focused,
	})
}

// placeholderTile is the bento cell stub used in Sprint 08 to lock the
// Ultra silhouette in place before Sprints 09-11 wire the live data.
// Each placeholder advertises the sprint that will fill it so the
// operator can answer "what is this empty box?" at a glance.
type placeholderTile struct {
	id      string
	slot    Slot
	header  string
	subtext []string
}

// ID satisfies [BentoTile].
func (t *placeholderTile) ID() string { return t.id }

// Slot satisfies [BentoTile].
func (t *placeholderTile) Slot() Slot { return t.slot }

// Render satisfies [BentoTile].
func (t *placeholderTile) Render(mode Mode, focused bool) string {
	return renderTilePanel(tilePanelOptions{
		Header:  t.header,
		Body:    strings.Join(t.subtext, "\n"),
		Mode:    mode,
		Focused: focused,
	})
}

// NewMetricsPlaceholderTile returns the header-metrics placeholder used
// when no live metrics snapshot is available yet (initial render before
// the first SSH poll completes). Once metrics arrive, the view layer
// swaps it for [NewHeaderMetricsTile].
func NewMetricsPlaceholderTile() BentoTile {
	return &placeholderTile{
		id:     "header-metrics",
		slot:   SlotMetrics,
		header: "[Header Metrics]",
		subtext: []string{
			"CPU / RAM / Disk pulse",
			"Awaiting first SSH poll…",
		},
	}
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
	indicator := "[LIVE]"
	if t.snap.Stale {
		indicator = "[STALE]"
	}
	body := strings.Join([]string{
		indicator + " " + nonEmpty(t.snap.ProfileAlias, "(no profile)"),
		"Uptime: " + nonEmpty(t.snap.UptimeLabel, "—"),
		"Load: " + nonEmpty(t.snap.LoadLabel, "—"),
		"RAM: " + nonEmpty(t.snap.RAMLabel, "—"),
		"Ping: " + nonEmpty(t.snap.RTTLabel, "—"),
	}, "\n")
	return renderTilePanel(tilePanelOptions{
		Header:  "[Header Metrics]",
		Body:    body,
		Mode:    mode,
		Focused: focused,
	})
}

// NewCICDPlaceholderTile returns the CI/CD pipeline placeholder used
// before the operator selects a GitHub-linked project (or while the
// first poll is still in flight). The view layer swaps it for
// [NewCICDPipelineTile] once a [CICDPipelineSnapshot] is available.
func NewCICDPlaceholderTile() BentoTile {
	return &placeholderTile{
		id:     "cicd-pipeline",
		slot:   SlotCICD,
		header: "[CI/CD Pipeline]",
		subtext: []string{
			"GitHub Actions stream",
			"No GitHub-linked project selected.",
			"Press [n] to create a new project.",
		},
	}
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
	snap CICDPipelineSnapshot
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

// Render satisfies [BentoTile].
func (t *cicdPipelineTile) Render(mode Mode, focused bool) string {
	var b strings.Builder

	indicator := "[LIVE]"
	if t.snap.Stale {
		indicator = "[STALE]"
	}
	if t.snap.RateLimited {
		indicator = "[LIMITED]"
	}
	headerLine := indicator + " " + nonEmpty(t.snap.ProjectAlias, "(no project)")
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
		runLine := "Build #" + intString(t.snap.RunNumber) + ": " + cicdStatusLabel(t.snap.RunStatus)
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

	for _, step := range t.snap.Steps {
		stepLine := "[" + intString(step.Number) + "] " + step.Name + " " + cicdStatusBadge(step.Status)
		if step.Duration != "" {
			stepLine += " · " + step.Duration
		}
		b.WriteString(stepLine)
		b.WriteString("\n")
	}

	if len(t.snap.Steps) > 0 {
		b.WriteString("\n[F8] View logs · [Enter] Open run")
	}

	return renderTilePanel(tilePanelOptions{
		Header:  "[CI/CD Pipeline]",
		Body:    strings.TrimRight(b.String(), "\n"),
		Mode:    mode,
		Focused: focused,
	})
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
	return &placeholderTile{
		id:     "live-logs",
		slot:   SlotLogs,
		header: "[Live Micro-Logs]",
		subtext: []string{
			"SSH tail with secret redaction",
			"Select a project to start streaming.",
		},
	}
}

// MicroLogLine is the view-layer projection of one tail entry. The
// bento package intentionally avoids depending on `services/sshtail`
// directly so the layout engine remains pure.
type MicroLogLine struct {
	Level    string
	Text     string
	Redacted bool
}

type microLogsTile struct {
	domain string
	lines  []MicroLogLine
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

// ID satisfies [BentoTile].
func (t *microLogsTile) ID() string { return "live-logs" }

// Slot satisfies [BentoTile].
func (t *microLogsTile) Slot() Slot { return SlotLogs }

// Render satisfies [BentoTile].
func (t *microLogsTile) Render(mode Mode, focused bool) string {
	if len(t.lines) == 0 {
		return renderTilePanel(tilePanelOptions{
			Header:  "[Live Micro-Logs]",
			Body:    "Streaming " + nonEmpty(t.domain, "—") + " — waiting for first line.",
			Mode:    mode,
			Focused: focused,
		})
	}
	rows := make([]string, 0, len(t.lines)+1)
	rows = append(rows, "Stream: "+nonEmpty(t.domain, "—"))
	for _, line := range t.lines {
		marker := "·"
		switch line.Level {
		case "ERROR":
			marker = "✗"
		case "WARN":
			marker = "!"
		case "DEBUG":
			marker = "›"
		case "INFO":
			marker = "·"
		}
		row := marker + " " + line.Text
		if line.Redacted {
			row += "  (redacted)"
		}
		rows = append(rows, row)
	}
	return renderTilePanel(tilePanelOptions{
		Header:  "[Live Micro-Logs]",
		Body:    strings.Join(rows, "\n"),
		Mode:    mode,
		Focused: focused,
	})
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
	return &placeholderTile{
		id:     "topology",
		slot:   SlotTopology,
		header: "[Topology]",
		subtext: []string{
			"Service dependency graph",
			"Live wiring: Sprint 11",
		},
	}
}

// tilePanelOptions captures the per-call presentation knobs. Keeping a
// single options struct (rather than a long positional argument list)
// makes future additions (icons, badges) source-compatible.
type tilePanelOptions struct {
	Header    string
	Body      string
	Mode      Mode
	Focused   bool
	EmptyHint string
}

// renderTilePanel composes a single bento cell: header line + body, wrapped
// in a rounded-border panel. The border color tracks the focus state so
// the operator always knows which tile reacts to keystrokes.
func renderTilePanel(opts tilePanelOptions) string {
	tokens := theme.Default()
	border := lipgloss.Color(tokens.Muted)
	if opts.Focused {
		border = lipgloss.Color(tokens.Primary)
	}

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(tokens.Primary)).
		Render(opts.Header)

	body := opts.Body
	if body == "" && opts.EmptyHint != "" {
		body = opts.EmptyHint
	}

	content := header + "\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color(tokens.TextBright)).
		Render(body)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Render(content)
}
