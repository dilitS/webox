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
}
