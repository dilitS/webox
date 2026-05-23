package tui

import (
	"time"

	"github.com/dilitS/webox/config"
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
