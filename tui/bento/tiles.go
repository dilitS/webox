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

// NewMetricsPlaceholderTile returns the header-metrics placeholder
// scheduled to ship in Sprint 09 (live CPU/RAM/Disk via SSH).
func NewMetricsPlaceholderTile() BentoTile {
	return &placeholderTile{
		id:     "header-metrics",
		slot:   SlotMetrics,
		header: "[Header Metrics]",
		subtext: []string{
			"CPU / RAM / Disk pulse",
			"Live wiring: Sprint 09",
		},
	}
}

// NewCICDPlaceholderTile returns the CI/CD pipeline placeholder scheduled
// to ship in Sprint 10 (live GitHub Actions polling + log modal).
func NewCICDPlaceholderTile() BentoTile {
	return &placeholderTile{
		id:     "cicd-pipeline",
		slot:   SlotCICD,
		header: "[CI/CD Pipeline]",
		subtext: []string{
			"GitHub Actions stream",
			"Live wiring: Sprint 10",
		},
	}
}

// NewLogsPlaceholderTile returns the live-log placeholder scheduled to
// ship in Sprint 09 (SSH tail -f with ring buffer + ANSI parsing).
func NewLogsPlaceholderTile() BentoTile {
	return &placeholderTile{
		id:     "live-logs",
		slot:   SlotLogs,
		header: "[Live Micro-Logs]",
		subtext: []string{
			"SSH tail with secret redaction",
			"Live wiring: Sprint 09",
		},
	}
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
