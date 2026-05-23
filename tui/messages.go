package tui

import (
	"time"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/wizard"
)

// ConfigLoadedMsg carries local config metadata into Update. Missing is
// explicit because config.Load returns defaults for absent files in Sprint 01.
type ConfigLoadedMsg struct {
	Config  *config.Config
	Missing bool
}

// ConfigLoadFailedMsg is rendered as a dashboard-blocking error.
type ConfigLoadFailedMsg struct {
	Err error
}

// ProjectStatus is the read-only status snapshot shown by Sprint 04.
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
