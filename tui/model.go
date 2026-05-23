package tui

import (
	"context"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/status"
	"github.com/dilitS/webox/tui/bento"
	"github.com/dilitS/webox/tui/components"
	"github.com/dilitS/webox/tui/theme"
)

// layoutOverrideEnv is the environment variable a power user can set to
// pin the cockpit to a specific Bento mode (see [bento.ParseLayoutOverride]).
// We resolve it once in [New] so the rest of the model sees a stable
// value across the lifetime of the process.
const layoutOverrideEnv = "WEBOX_LAYOUT"

const (
	defaultRefreshInterval = 30 * time.Second
	defaultTerminalWidth   = 100
	defaultTerminalHeight  = 30
)

// StatusFetcher is the side-effect seam used by refresh commands.
type StatusFetcher func(context.Context, []config.Project, *status.Cache) ([]ProjectStatus, error)

// Options configures a TUI model without using package globals.
type Options struct {
	ConfigPath       string
	PendingPath      string
	Cache            *status.Cache
	FetchStatuses    StatusFetcher
	GitHubLastDeploy GitHubLastDeployFetcher
	RefreshInterval  time.Duration
	InitialWidth     int
	InitialHeight    int
	NewContext       func() (context.Context, context.CancelFunc)
	WizardRunner     WizardRunner
	// LayoutOverride mirrors `WEBOX_LAYOUT` for tests: when non-empty
	// it bypasses the env-var lookup performed by [New].
	LayoutOverride string
}

// Model contains all mutable TUI state. It is copied by value by Update,
// matching Bubble Tea's MVU pattern.
type Model struct {
	state         State
	activeTab     DetailTab
	width         int
	height        int
	selectedIndex int
	helpVisible   bool
	alert         string

	cfg      *config.Config
	statuses map[string]ProjectStatus

	configPath      string
	cache           *status.Cache
	fetchStatuses   StatusFetcher
	refreshInterval time.Duration
	ctx             context.Context
	cancel          context.CancelFunc
	spinner         spinner.Model
	styles          theme.Styles

	initForm       initWizardForm
	projectForm    projectWizardForm
	resumeForm     resumeWizardForm
	actionForm     projectActionForm
	importForm     importPreviewForm
	wizardRunner   WizardRunner
	wizardStack    *wizardStackSlot
	pendingPath    string
	layoutOverride string
}

// importPreviewForm holds in-memory state for the read-only import
// preview (PRD F9). Loading is true while the scan command is in
// flight; Rows is the joined view of provider subdomains × local
// projects; Err captures any scan failure so the renderer can flag it
// without crashing.
type importPreviewForm struct {
	Loading bool
	Rows    []ImportRow
	Err     string
	Saving  bool
}

// projectActionForm holds the in-memory state of a dashboard action
// (restart / ssl renew / log tail). Empty Kind == no action in
// flight; the renderer keeps the last completed action visible so the
// operator can read the output before triggering the next one.
type projectActionForm struct {
	Kind      ProjectActionKind
	ProjectID string
	Running   bool
	Output    []byte
	Err       error
}

// New creates a pure initial model. I/O starts only when Init returns a Cmd.
func New(opts Options) Model {
	if opts.Cache == nil {
		opts.Cache = status.NewCache(status.Options{})
	}
	if opts.FetchStatuses == nil {
		fetcher := opts.GitHubLastDeploy
		opts.FetchStatuses = func(ctx context.Context, projects []config.Project, cache *status.Cache) ([]ProjectStatus, error) {
			return FetchProjectStatusesWithGitHub(ctx, projects, cache, fetcher)
		}
	}
	if opts.RefreshInterval <= 0 {
		opts.RefreshInterval = defaultRefreshInterval
	}
	if opts.NewContext == nil {
		opts.NewContext = func() (context.Context, context.CancelFunc) {
			return context.WithCancel(context.Background())
		}
	}
	if opts.WizardRunner == nil {
		opts.WizardRunner = DefaultWizardRunner()
	}
	ctx, cancel := opts.NewContext()

	override := opts.LayoutOverride
	if override == "" {
		override = os.Getenv(layoutOverrideEnv)
	}
	width := fallbackInt(opts.InitialWidth, defaultTerminalWidth)
	height := fallbackInt(opts.InitialHeight, defaultTerminalHeight)
	mode := bento.Resolve(width, height, override)
	spin := components.NewAdaptiveSpinner(mode.String())

	return Model{
		state:           StateDashboard,
		activeTab:       TabOverview,
		width:           width,
		height:          height,
		statuses:        make(map[string]ProjectStatus),
		configPath:      opts.ConfigPath,
		pendingPath:     opts.PendingPath,
		cache:           opts.Cache,
		fetchStatuses:   opts.FetchStatuses,
		refreshInterval: opts.RefreshInterval,
		ctx:             ctx,
		cancel:          cancel,
		spinner:         spin,
		styles:          theme.NewStyles(theme.Default()),
		wizardRunner:    opts.WizardRunner,
		initForm:        newInitWizardForm(),
		layoutOverride:  override,
	}
}

// BentoMode returns the resolved Bento mode for the current viewport
// and any [layoutOverrideEnv] override captured at construction time.
// Exposing it on the model keeps the cockpit's routing logic centralised
// (view.go, components, tests).
func (m Model) BentoMode() bento.Mode {
	return bento.Resolve(m.width, m.height, m.layoutOverride)
}

// State returns the current top-level route.
func (m Model) State() State { return m.state }

// ActiveTab returns the current project-detail tab.
func (m Model) ActiveTab() DetailTab { return m.activeTab }

// SelectedIndex returns the selected dashboard row.
func (m Model) SelectedIndex() int { return m.selectedIndex }

// Alert returns the transient alert text.
func (m Model) Alert() string { return m.alert }

// ProjectStatus returns status metadata by project id.
func (m Model) ProjectStatus(projectID string) (ProjectStatus, bool) {
	got, ok := m.statuses[projectID]
	return got, ok
}

// ImportSnapshot exposes the read-only import preview state for
// renderers and tests. Returns (zero, false) when no scan has run.
func (m Model) ImportSnapshot() (ImportSnapshot, bool) {
	if !m.importForm.Loading && len(m.importForm.Rows) == 0 && m.importForm.Err == "" {
		return ImportSnapshot{}, false
	}
	snap := ImportSnapshot{
		Loading: m.importForm.Loading,
		Rows:    m.importForm.Rows,
		Err:     m.importForm.Err,
		Total:   len(m.importForm.Rows),
	}
	for _, row := range m.importForm.Rows {
		if row.Managed {
			snap.Managed++
		} else {
			snap.Unmanaged++
		}
	}
	return snap, true
}

func (m Model) withState(state State) Model {
	m.state = state
	return m
}

func (m Model) withConfig(cfg *config.Config) Model {
	m.cfg = cfg
	m.state = StateDashboard
	m.selectedIndex = clampIndex(m.selectedIndex, len(cfgProjects(cfg)))
	return m
}

func (m Model) withStatuses(statuses map[string]ProjectStatus) Model {
	m.statuses = statuses
	return m
}

func fallbackInt(value, def int) int {
	if value == 0 {
		return def
	}
	return value
}

func cfgProjects(cfg *config.Config) []config.Project {
	if cfg == nil {
		return nil
	}
	return cfg.Projects
}

func clampIndex(idx, count int) int {
	if count <= 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if idx >= count {
		return count - 1
	}
	return idx
}
