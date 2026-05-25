package tui

import (
	"context"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/internal/telemetry"
	ghsvc "github.com/dilitS/webox/services/github"
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

// GitHubLogsFetcher is the seam used by the F8 modal: returns the
// (already redacted) tail of a workflow run's combined log stream.
// Splitting it out from [GitHubPipelineFetcher] keeps the polling and
// modal-open code paths independently testable.
type GitHubLogsFetcher func(ctx context.Context, ref ghsvc.RepoRef, runID int64, maxLines int) ([]ghsvc.WorkflowLogLine, error)

// Options configures a TUI model without using package globals.
type Options struct {
	ConfigPath       string
	PendingPath      string
	Cache            *status.Cache
	FetchStatuses    StatusFetcher
	GitHubLastDeploy GitHubLastDeployFetcher
	GitHubPipeline   GitHubPipelineFetcher
	GitHubLogs       GitHubLogsFetcher
	RefreshInterval  time.Duration
	InitialWidth     int
	InitialHeight    int
	NewContext       func() (context.Context, context.CancelFunc)
	WizardRunner     WizardRunner
	// LayoutOverride mirrors `WEBOX_LAYOUT` for tests: when non-empty
	// it bypasses the env-var lookup performed by [New].
	LayoutOverride string
	// Now is the time source used by the cockpit's status bar +
	// header. Tests inject a deterministic stub; production leaves
	// it nil so [New] falls back to [time.Now].
	Now func() time.Time
	// MockHeaderMetrics seeds the cockpit's metrics snapshot at boot.
	// Production callers leave it zero; the mock launcher populates
	// it so the offline demo shows realistic numbers immediately.
	MockHeaderMetrics HeaderMetricsSnapshot
	// MockLiveLogLines seeds the live-log ring buffer at boot. Used
	// by the mock launcher so the "Live Server Logs" tile is
	// non-empty without an SSH connection.
	MockLiveLogLines []LiveLogLine
	// MockCICDSnapshots seeds the per-project CI/CD snapshot map.
	// Keyed by project ID; the renderer renders straight from the
	// map without an HTTP/GitHub call.
	MockCICDSnapshots map[string]cicdSnapshotEntry
	// PreloadedConfig short-circuits the on-disk config loader. When
	// non-nil [Init] skips [loadConfigCmd] and the cockpit boots
	// straight into the dashboard with the supplied config. Used
	// exclusively by `--mock` mode today; production leaves it nil.
	PreloadedConfig *config.Config
	// Trace is the local-only observability sink. Production wires
	// either [telemetry.Disabled] (default) or a [telemetry.FileSink]
	// opened from `--debug-trace=PATH`. The cockpit emits coarse
	// events (state transitions, refresh ticks, modal opens) without
	// blocking — see TASK-14.6.
	Trace telemetry.Sink
}

// HeaderMetricsSnapshot is the in-process projection of the latest SSH
// metrics poll consumed by the status bar and (in fallback layouts)
// the header-metrics tile. Production fills it from the
// `services/sshmetrics` poller; the mock launcher hardcodes plausible
// values so the offline demo stays in motion.
type HeaderMetricsSnapshot struct {
	ProfileAlias string
	UptimeLabel  string
	LoadLabel    string
	RAMLabel     string
	RTTLabel     string
	UpdatedAt    time.Time
	Stale        bool
}

// Model contains all mutable TUI state. It is copied by value by Update,
// matching Bubble Tea's MVU pattern.
type Model struct {
	state           State
	activeTab       DetailTab
	width           int
	height          int
	selectedIndex   int
	viewportOffsetY int
	helpVisible     bool
	alert           string

	// focusedTile is the cockpit tile currently receiving Tab-routed
	// scroll events (Sprint 14 TASK-14.2). nil = no focus, the
	// global viewport scroll handles PgUp / PgDn / Home / End.
	// Non-nil = the focused slot's scroll routing wins; the chrome
	// renders the tile with a double-line border for visual feedback.
	focusedTile *bento.Slot
	// tileScrollOffsets stores the per-tile scroll offset. Persisted
	// on the Model so it survives across the value-typed Update
	// returns; the bento engine reads it via WithTileScrollOffsets
	// each frame.
	tileScrollOffsets map[bento.Slot]int

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

	initForm        initWizardForm
	projectForm     projectWizardForm
	resumeForm      resumeWizardForm
	actionForm      projectActionForm
	importForm      importPreviewForm
	liveLogs        liveLogsForm
	headerMetrics   HeaderMetricsSnapshot
	cicdSnapshots   map[string]cicdSnapshotEntry
	cicdModal       cicdLogsModalForm
	hostKeyModal    hostKeyModalForm
	cicdFetcher     GitHubPipelineFetcher
	cicdLogsFetcher GitHubLogsFetcher
	wizardRunner    WizardRunner
	wizardStack     *wizardStackSlot
	pendingPath     string
	layoutOverride  string
	nowFn           func() time.Time
	trace           telemetry.Sink

	// Sprint 20 TASK-20.2 Provider Catalog state. Lives on the
	// model so the catalog screen can keep cursor + detail
	// expansion across re-renders without owning a separate
	// state machine.
	catalog providerCatalogState
}

// providerCatalogState is the view-level state for the
// StateProviderCatalog screen. The model owns it (rather than
// the surface adapter) because the cursor must survive across
// `Update` calls — Bubble Tea returns a fresh value-typed
// adapter every frame.
type providerCatalogState struct {
	// SelectedID tracks the cursor row. Empty value selects no
	// row (initial state); the renderer falls back to the first
	// catalog entry when the operator hits arrow keys.
	SelectedID string
	// ShowDetail toggles the deep-dive bottom strip. Defaults
	// to false so the catalog opens in "list mode" — pressing
	// Enter expands.
	ShowDetail bool
	// CopyHint surfaces the most recent clipboard ack ("briefing
	// copied to clipboard") or failure ("clipboard unavailable").
	// Cleared on the next keypress.
	CopyHint string
}

// liveLogsForm captures the per-session state of the Sprint 09 live-log
// tab. The buffer is owned by [components.RingBuffer] so producers
// (services/sshtail) can push concurrently while the renderer reads via
// `Snapshot()`. AutoScroll mirrors the operator's "follow" preference;
// when false, the renderer keeps the buffer pinned to ScrollOffset.
type liveLogsForm struct {
	ProjectID    string
	Domain       string
	LogPath      string
	AutoScroll   bool
	Buffer       *components.RingBuffer[LiveLogLine]
	ScrollOffset int
	StreamErr    string
	Connected    bool
}

// LiveLogLine is the in-memory record stored in the live-log ring
// buffer. It mirrors `services/sshtail.Line` minus the Timestamp field
// (the view layer renders relative offsets, not absolute timestamps).
type LiveLogLine struct {
	Level    string
	Text     string
	Redacted bool
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

	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}

	cicdSnapshots := opts.MockCICDSnapshots
	if cicdSnapshots == nil {
		cicdSnapshots = make(map[string]cicdSnapshotEntry)
	}

	trace := opts.Trace
	if trace == nil {
		trace = telemetry.Disabled
	}

	m := Model{
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
		cicdSnapshots:   cicdSnapshots,
		cicdFetcher:     opts.GitHubPipeline,
		cicdLogsFetcher: opts.GitHubLogs,
		headerMetrics:   opts.MockHeaderMetrics,
		nowFn:           nowFn,
		cfg:             opts.PreloadedConfig,
		trace:           trace,
	}

	if opts.PreloadedConfig != nil {
		m.selectedIndex = clampIndex(0, len(cfgProjects(opts.PreloadedConfig)))
		if opts.FetchStatuses != nil {
			if seeded, err := opts.FetchStatuses(ctx, cfgProjects(opts.PreloadedConfig), opts.Cache); err == nil {
				m = m.applyMockStatuses(seeded)
			}
		}
	}

	if len(opts.MockLiveLogLines) > 0 {
		buf := components.NewRingBuffer[LiveLogLine](liveLogsCapacity)
		for _, line := range opts.MockLiveLogLines {
			buf.Push(line)
		}
		m.liveLogs = liveLogsForm{
			AutoScroll: true,
			Buffer:     buf,
			Connected:  true,
		}
	}

	return m
}

// emit forwards a structured event into the local-only trace sink.
// Production cockpits use [telemetry.Disabled] (no-op) unless
// `--debug-trace=PATH` is set; tests substitute a recording sink to
// assert on transitions. The method intentionally accepts loose
// `map[string]any` fields rather than typed structs so add-events
// stay a one-liner — the trace is for human debugging, not API
// contracts.
//
// The call is non-blocking by design: [telemetry.FileSink.Record]
// drops events when the queue is full so the Update hot path is
// never stalled by disk pressure.
func (m Model) emit(name string, fields map[string]any) {
	if m.trace == nil || !m.trace.Enabled() {
		return
	}
	m.trace.Record(m.ctx, telemetry.Event{Name: name, Fields: fields})
}

// applyMockStatuses copies a slice of [ProjectStatus] into the
// keyed status map. Exposed only for the mock launcher; production
// status messages route through [Update] / [StatusRefreshedMsg].
func (m Model) applyMockStatuses(statuses []ProjectStatus) Model {
	for _, s := range statuses {
		m.statuses[s.ProjectID] = s
	}
	return m
}

// metricsAreStale reports whether the cached metrics snapshot is older
// than the SSH poll TTL. The cockpit status bar uses it to switch the
// LIVE pill to STALE without dimming the cells.
func (m Model) metricsAreStale() bool {
	if m.headerMetrics.Stale {
		return true
	}
	if m.headerMetrics.UpdatedAt.IsZero() {
		return false
	}
	if m.nowFn == nil {
		return false
	}
	return m.nowFn().Sub(m.headerMetrics.UpdatedAt) > defaultRefreshInterval
}

// metricsHaveAnyData reports whether at least one cell in the header
// metrics snapshot is populated. When false, the status bar paints a
// "PENDING" pill instead of the green LIVE one.
func (m Model) metricsHaveAnyData() bool {
	h := m.headerMetrics
	return h.UptimeLabel != "" || h.LoadLabel != "" || h.RAMLabel != "" || h.RTTLabel != "" || h.ProfileAlias != ""
}

// activeProfileAlias returns the alias of the profile bound to the
// currently selected project. Empty when no project is selected or the
// project has no matching profile (defensive: should not happen in
// production but the mock launcher seeds projects without profiles).
func (m Model) activeProfileAlias() string {
	if h := m.headerMetrics.ProfileAlias; h != "" {
		return h
	}
	project, ok := m.selectedProject()
	if !ok {
		return ""
	}
	return project.ProfileAlias
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
