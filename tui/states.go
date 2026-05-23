package tui

// State is the top-level TUI route. The list mirrors docs/DESIGN.md §12;
// Sprint 04 enables only InitWizard, Dashboard, and ProjectDetail.
type State int

const (
	// StateInitWizard is shown when no local config exists yet.
	StateInitWizard State = iota
	// StateDashboard is the read-only project list and overview screen.
	StateDashboard
	// StateProjectDetail is the focused Overview tab for one project.
	StateProjectDetail
	// StateCommandPalette is reserved for the Sprint 07 palette.
	StateCommandPalette
	// StateConfirmDialog is reserved for future destructive confirmations.
	StateConfirmDialog
)

func (s State) String() string {
	switch s {
	case StateInitWizard:
		return "InitWizard"
	case StateDashboard:
		return "Dashboard"
	case StateProjectDetail:
		return "ProjectDetail"
	case StateCommandPalette:
		return "CommandPalette"
	case StateConfirmDialog:
		return "ConfirmDialog"
	default:
		return "Unknown"
	}
}

// DetailTab names the project-detail tabs. In v0.1 only TabOverview is
// enabled; the others are visible as roadmap markers.
type DetailTab int

const (
	// TabOverview is the only enabled project detail tab in MVP.
	TabOverview DetailTab = iota
	// TabEnvDiff is visible but disabled until v0.2.
	TabEnvDiff
	// TabDatabase is visible but disabled until v0.2.
	TabDatabase
	// TabLogs is visible but disabled until v0.2 live/tail work.
	TabLogs
)

func (t DetailTab) String() string {
	switch t {
	case TabOverview:
		return "Overview"
	case TabEnvDiff:
		return "Env Diff"
	case TabDatabase:
		return "Database"
	case TabLogs:
		return "Logs"
	default:
		return "Unknown"
	}
}

// Enabled reports whether the tab can be opened in the current MVP surface.
func (t DetailTab) Enabled() bool {
	return t == TabOverview
}
