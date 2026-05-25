package views

import (
	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/tui/theme"
)

// ProjectStatus is the view-layer copy of dashboard status metadata.
type ProjectStatus struct {
	ProjectID   string
	HTTPHealth  string
	SSLDaysLeft int
	NodeVersion string
	LastDeploy  string
	State       string
	Stale       bool
}

// InitWizardSnapshot is the view-layer copy of the init-wizard form
// state. Step is an int instead of the typed enum to keep the view
// package free of `tui` imports (would be a cycle).
type InitWizardSnapshot struct {
	Step   int
	Alias  string
	Host   string
	Port   string
	User   string
	Err    string
	Saving bool
}

// ProjectWizardSnapshot is the view-layer copy of the project-wizard
// form. Step semantics mirror [tui.ProjectWizardStep]; the renderer
// switches on the int value.
type ProjectWizardSnapshot struct {
	Step            int
	ProfileAlias    string
	Stack           string
	Domain          string
	NodeVersion     string
	DBWanted        bool
	DBKind          string
	DBName          string
	Err             string
	Executing       bool
	SubdomainOK     bool
	SubdomainErr    string
	SSLOK           bool
	SSLErr          string
	DatabaseOK      bool
	DatabaseErr     string
	RolledBack      bool
	RollbackResults []RollbackResultSnapshot
	RollbackErr     string
}

// RollbackResultSnapshot is the per-step rollback outcome rendered in
// the failure UI's progress strip.
type RollbackResultSnapshot struct {
	Name string
	Err  string
}

// ResumeWizardSnapshot is the view-layer copy of resume-on-launch state.
type ResumeWizardSnapshot struct {
	WizardID      string
	ProfileAlias  string
	UpdatedAt     string
	StepNames     []string
	Err           string
	Discarding    bool
	DiscardPhrase string
	ConfirmInput  string
	RollingBack   bool
	Results       []RollbackResultSnapshot
}

// ProjectActionSnapshot is the view-layer copy of the active /
// last-completed dashboard action (restart / ssl renew / log tail).
// Kind == "" means no action ever ran for the current session.
type ProjectActionSnapshot struct {
	Kind      string
	ProjectID string
	Running   bool
	Output    string
	Err       string
}

// ImportRowSnapshot is the view-layer row in the import preview
// table. Managed reflects whether the local config already knows
// about the domain.
type ImportRowSnapshot struct {
	ProfileAlias string
	Domain       string
	Type         string
	NodeVersion  string
	Managed      bool
}

// ImportPreviewSnapshot is the view-layer copy of the import-preview
// form. Loading=true while the scan command is in flight; Err is the
// stringified scan error when the runner failed.
type ImportPreviewSnapshot struct {
	Loading   bool
	Saving    bool
	Err       string
	Rows      []ImportRowSnapshot
	Total     int
	Managed   int
	Unmanaged int
}

// LiveLogLineSnapshot is one entry rendered in the Sprint 09 live-log
// tab. Level uses the upper-case strings emitted by
// [components.LogLevel.String] so the view package stays free of the
// `tui/components` import (would be a cycle when `components` later
// gains theme-dependent renderers).
type LiveLogLineSnapshot struct {
	Level    string
	Text     string
	Redacted bool
}

// LiveLogsSnapshot is the view-layer copy of the live-log tab state.
// The slice is the most-recent N lines of the ring buffer in
// insertion order (oldest first); the renderer reverses for display.
type LiveLogsSnapshot struct {
	Domain     string
	LogPath    string
	AutoScroll bool
	Connected  bool
	BufferUsed int
	BufferCap  int
	Lines      []LiveLogLineSnapshot
	Err        string
}

// SecretMetaSnapshot is the view-layer projection of
// [config.SecretMeta]. The view package cannot import `config`
// without creating a cycle (config → views → renderers → ...),
// so we mirror the operator-visible fields here. Plaintext
// values NEVER cross this boundary — Webox keeps them in the
// keyring or AES-GCM fallback (`docs/SECURITY.md §4`).
type SecretMetaSnapshot struct {
	// Key is the env var name (e.g. `DATABASE_URL`).
	Key string
	// Source is one of `managed`, `server_only`, `external`.
	Source string
	// CreatedAt / LastRotated / LastSyncedGitHub /
	// LastSyncedServer formatted to "2006-01-02" so the
	// renderer can compare against the rotation reminder
	// without re-parsing.
	CreatedAt        string
	LastRotated      string
	LastSyncedGitHub string
	LastSyncedServer string
	// RotationReminderDays mirrors `config.SecretMeta`. Zero
	// means "no reminder configured".
	RotationReminderDays int
	// Stale reports whether `now - LastRotated >
	// RotationReminderDays`. Pre-computed so the renderer does
	// not need a clock dependency.
	Stale bool
}

// CICDMiniSnapshot is the compact view-layer projection of the
// CI/CD pipeline state, used by the Standard cockpit mini-bento
// strip and any future at-a-glance summaries. It MUST stay smaller
// than [github/services.WorkflowSummary] so future renderers can
// derive the strip without paying for the full step list.
//
// Status is the upper-case verb (`SUCCESS`, `FAILED`,
// `IN_PROGRESS`, `QUEUED`, `CANCELLED`, `SKIPPED`, `UNKNOWN`).
// FailedStep is the short name of the failing job step when the
// run conclusion is FAILED — empty otherwise. Branch is the head
// ref. JobName is the workflow's `name:` field. Empty fields are
// expected when no run has been observed yet for the current
// project; the renderer falls back to a `pending` ribbon.
type CICDMiniSnapshot struct {
	HasRun     bool
	Status     string
	JobName    string
	Branch     string
	RunNumber  int64
	FailedStep string
	UpdatedAt  string
}

// Screen contains the immutable data needed by pure render functions.
type Screen struct {
	Width         int
	Height        int
	SelectedIndex int
	ActiveTab     string
	Alert         string
	HelpVisible   bool
	Spinner       string
	Config        *config.Config
	Statuses      map[string]ProjectStatus
	Styles        theme.Styles
	InitForm      InitWizardSnapshot
	ProjectForm   ProjectWizardSnapshot
	ResumeForm    ResumeWizardSnapshot
	ActionForm    ProjectActionSnapshot
	ImportForm    ImportPreviewSnapshot
	LiveLogs      LiveLogsSnapshot
	Connections   []string
	CICDMini      CICDMiniSnapshot
	// Secrets carries the per-project secret metadata Webox
	// knows about (managed/server_only/external sources). It is
	// the only data Webox can show on the Env Diff tab without
	// reaching back to the provider — the sensitive values stay
	// in the keyring / secrets.enc by design.
	Secrets []SecretMetaSnapshot
}
