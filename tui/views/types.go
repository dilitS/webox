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
}
