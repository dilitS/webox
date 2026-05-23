package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/spinner"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/status"
	"github.com/dilitS/webox/tui/theme"
)

const (
	defaultRefreshInterval = 30 * time.Second
	defaultTerminalWidth   = 100
	defaultTerminalHeight  = 30
)

// StatusFetcher is the side-effect seam used by refresh commands.
type StatusFetcher func(context.Context, []config.Project, *status.Cache) ([]ProjectStatus, error)

// Options configures a TUI model without using package globals.
type Options struct {
	ConfigPath      string
	Cache           *status.Cache
	FetchStatuses   StatusFetcher
	RefreshInterval time.Duration
	InitialWidth    int
	InitialHeight   int
	NewContext      func() (context.Context, context.CancelFunc)
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
}

// New creates a pure initial model. I/O starts only when Init returns a Cmd.
func New(opts Options) Model {
	if opts.Cache == nil {
		opts.Cache = status.NewCache(status.Options{})
	}
	if opts.FetchStatuses == nil {
		opts.FetchStatuses = FetchProjectStatuses
	}
	if opts.RefreshInterval <= 0 {
		opts.RefreshInterval = defaultRefreshInterval
	}
	if opts.NewContext == nil {
		opts.NewContext = func() (context.Context, context.CancelFunc) {
			return context.WithCancel(context.Background())
		}
	}
	ctx, cancel := opts.NewContext()
	spin := spinner.New()
	spin.Spinner = spinner.Dot

	return Model{
		state:           StateDashboard,
		activeTab:       TabOverview,
		width:           fallbackInt(opts.InitialWidth, defaultTerminalWidth),
		height:          fallbackInt(opts.InitialHeight, defaultTerminalHeight),
		statuses:        make(map[string]ProjectStatus),
		configPath:      opts.ConfigPath,
		cache:           opts.Cache,
		fetchStatuses:   opts.FetchStatuses,
		refreshInterval: opts.RefreshInterval,
		ctx:             ctx,
		cancel:          cancel,
		spinner:         spin,
		styles:          theme.NewStyles(theme.Default()),
	}
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
