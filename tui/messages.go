package tui

import (
	"time"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/wizard"
)

// ConfigLoadedMsg carries local config metadata into Update. Missing
// is explicit because [config.Load] returns defaults for an absent
// file (an intentional contract, not an error) — the TUI uses Missing
// to route into the init wizard.
type ConfigLoadedMsg struct {
	Config  *config.Config
	Missing bool
}

// ConfigLoadFailedMsg is rendered as a dashboard-blocking error.
type ConfigLoadFailedMsg struct {
	Err error
}

// ProjectStatus is the read-only status snapshot rendered per project
// on the dashboard and the project-detail Overview tab.
type ProjectStatus struct {
	ProjectID   string
	HTTPHealth  string
	SSLDaysLeft int
	NodeVersion string
	LastDeploy  string
	State       ProjectState
	Stale       bool
	FetchedAt   time.Time
}

// ProjectState is intentionally small in MVP. More nuanced states such as
// degraded remain design tokens until their owning features ship.
type ProjectState string

const (
	// ProjectUnknown means no status probe has completed yet.
	ProjectUnknown ProjectState = "UNKNOWN"
	// ProjectOnline means the last HTTP probe succeeded.
	ProjectOnline ProjectState = "ONLINE"
	// ProjectBuilding is reserved for future deploy-progress states.
	ProjectBuilding ProjectState = "BUILDING"
	// ProjectOffline means the last HTTP probe failed or returned an error class.
	ProjectOffline ProjectState = "OFFLINE"
	// ProjectStale means local config and provider state may have drifted.
	ProjectStale ProjectState = "STALE"
)

// StatusRefreshedMsg merges background status data into the model.
type StatusRefreshedMsg struct {
	Statuses []ProjectStatus
}

// StatusRefreshFailedMsg records refresh errors without leaking raw output.
type StatusRefreshFailedMsg struct {
	Err error
}

// RefreshTickMsg drives periodic SWR-backed status refresh.
type RefreshTickMsg time.Time

// ProfileSavedMsg is dispatched after the init-wizard persists the
// first profile. Triggers the transition to the dashboard.
type ProfileSavedMsg struct {
	Config *config.Config
}

// ProfileSaveFailedMsg surfaces a save failure to the init-wizard
// review screen without leaving the form.
type ProfileSaveFailedMsg struct {
	Err error
}

// ProjectWizardPreflightMsg carries the provider preflight result.
// `Err` is non-nil when the provider rejected the probe; the wizard
// surfaces the failure inside the Domain step rather than aborting
// to Dashboard.
type ProjectWizardPreflightMsg struct {
	ProfileAlias string
	Err          error
}

// ProjectWizardDomainCheckedMsg is the response to the duplicate
// subdomain probe (`ListSubdomains`). `Available` is false when the
// panel already has the domain.
type ProjectWizardDomainCheckedMsg struct {
	Domain    string
	Available bool
	Err       error
}

// ProjectWizardExecutedMsg carries the [wizard.Execute] outcome
// (success or [wizard.ExecutionFailedError]). The wizard view
// transitions to Done on success or Failure on error.
type ProjectWizardExecutedMsg struct {
	Plan        wizard.ProvisionPlan
	Report      *wizard.ProvisionReport
	Err         error
	ProjectID   string
	ProjectCfg  *config.Config
	SaveErr     error
	Credentials *wizard.DatabaseCredentials
}

// ProjectWizardRolledBackMsg is the [Stack.Rollback] outcome
// dispatched after the operator chooses rollback from the failure
// remediation menu.
type ProjectWizardRolledBackMsg struct {
	Results []wizard.CleanupResult
	Err     error
}

// PendingLoadedMsg carries pending_cleanups.json state discovered during
// startup. Missing snapshots are represented by Snapshot=nil and Err=nil.
type PendingLoadedMsg struct {
	Snapshot *wizard.PendingCleanups
	Err      error
}

// PendingDiscardedMsg reports the result of deleting pending_cleanups.json
// after the operator typed the confirmation phrase.
type PendingDiscardedMsg struct {
	Err error
}

// ProjectActionKind names the project-detail action the operator
// triggered. The TUI uses the kind to route the resulting message
// (restart / ssl renew / log tail) to its own state slot without
// adding a new state per action.
type ProjectActionKind string

const (
	// ProjectActionRestart restarts the Node.js application bound
	// to the project domain.
	ProjectActionRestart ProjectActionKind = "restart"
	// ProjectActionSSLRenew re-runs SetupSSL idempotently.
	ProjectActionSSLRenew ProjectActionKind = "ssl_renew"
	// ProjectActionLogs tails the project log files.
	ProjectActionLogs ProjectActionKind = "logs"
)

// ProjectActionCompletedMsg carries the outcome of a dashboard action
// (restart / ssl renew / log tail). Output is non-empty only for
// log-tail; the other kinds set it to nil so the renderer can
// distinguish "no output" from "no output yet".
type ProjectActionCompletedMsg struct {
	Kind      ProjectActionKind
	ProjectID string
	Output    []byte
	Err       error
}

// ImportRow is one row in the dashboard import preview. It represents
// a subdomain reported by a provider, joined with whether
// `config.json` already manages it. Managed rows are kept in the
// preview so the operator sees the full account picture and can spot
// drift (different Node version, missing project, etc.).
type ImportRow struct {
	// ProfileAlias is the source [config.Profile.Alias] that
	// reported this subdomain. Carried so the writer step can map
	// it back into a stub [config.Project].
	ProfileAlias string

	// Domain is the fully-qualified subdomain reported by the
	// provider — for small.pl typically `<sub>.<user>.smallhost.pl`.
	Domain string

	// Type mirrors [providers.Subdomain.Type]: "nodejs", "static",
	// "php", or any provider-specific stack token.
	Type string

	// NodeVersion is the major version reported by the panel for
	// Type=="nodejs" subdomains; empty string otherwise.
	NodeVersion string

	// Managed is true when this domain already appears in
	// `config.json` under [config.Project.Domain]. The import flow
	// only writes stubs for `!Managed` rows.
	Managed bool
}

// ImportScanCompletedMsg is the response to the dashboard import scan
// command. Rows is sorted by Domain; ProfilesScanned counts how many
// profiles the runner queried so the UI can render a per-profile
// summary line.
type ImportScanCompletedMsg struct {
	Rows            []ImportRow
	ProfilesScanned int
	Err             error
}

// ImportPersistedMsg is dispatched after the writer persists the
// imported stubs. Config carries the updated config so the parent
// model can swap it in atomically.
type ImportPersistedMsg struct {
	Config       *config.Config
	ImportedRows int
	Err          error
}

// ImportSnapshot is the read-only view of the import form state used
// by the renderer and by tests through [Model.ImportSnapshot]. The
// counts are recomputed from Rows on every transition so the renderer
// never sees stale aggregates.
type ImportSnapshot struct {
	Loading   bool
	Total     int
	Managed   int
	Unmanaged int
	Rows      []ImportRow
	Err       string
}

// CICDTickMsg drives periodic CI/CD pipeline polling. The dashboard
// emits one tick per [GitHubStepsTTL] interval (10s) which triggers
// `pollCICDPipelineCmd` for the selected project.
type CICDTickMsg time.Time

// CICDFetchedMsg carries a CI/CD pipeline poll outcome. The model
// updates the per-project snapshot map and (if the polled project
// matches the dashboard selection) re-renders the tile.
type CICDFetchedMsg struct {
	ProjectID string
	Result    PipelineFetchResult
	Err       error
	// FetchedAt records when the poll finished so the renderer can
	// compute the freshness badge ([LIVE]/[STALE]).
	FetchedAt time.Time
}

// CICDLogsFetchedMsg carries the response to the F8 modal open
// command. ProjectID + RunID identify the run; Lines is already
// passed through `internal/log.Redact` (transport boundary).
type CICDLogsFetchedMsg struct {
	ProjectID string
	RunID     int64
	Lines     []CICDLogLineSnapshot
	Err       error
}

// CICDLogLineSnapshot is the view-layer projection of one redacted
// workflow-log line. Kept in `tui` (not in `services/github`) so the
// modal renderer does not depend on the transport package.
type CICDLogLineSnapshot struct {
	JobName  string
	StepName string
	Text     string
}
