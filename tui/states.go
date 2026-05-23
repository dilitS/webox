package tui

// State is the top-level TUI route. The list mirrors docs/DESIGN.md §12;
// Sprint 04 enabled InitWizard / Dashboard / ProjectDetail; Sprint 05
// adds the StateProjectWizard route for new-project provisioning.
type State int

const (
	// StateInitWizard is shown when no local config exists yet. In
	// Sprint 05 this state hosts the first-run profile capture flow
	// (see [InitWizardStep]).
	StateInitWizard State = iota
	// StateDashboard is the read-only project list and overview screen.
	StateDashboard
	// StateProjectDetail is the focused Overview tab for one project.
	StateProjectDetail
	// StateProjectWizard is the multi-step new-project flow added in
	// Sprint 05 (see [ProjectWizardStep]).
	StateProjectWizard
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
	case StateProjectWizard:
		return "ProjectWizard"
	case StateCommandPalette:
		return "CommandPalette"
	case StateConfirmDialog:
		return "ConfirmDialog"
	default:
		return "Unknown"
	}
}

// InitWizardStep enumerates the sub-states of the first-run profile
// capture flow. The order matches the screen sequence in
// docs/UX.md §11.1 (zero-friction bootstrapping) — webox does NOT
// generate SSH keys or PAT here; that arrives in v0.2 (auto-inject).
//
// MVP captures profile metadata only and never touches the network.
type InitWizardStep int

const (
	// InitStepWelcome shows the first-run hero screen and waits for
	// Enter to begin the profile form.
	InitStepWelcome InitWizardStep = iota
	// InitStepAlias captures the human-readable profile alias.
	InitStepAlias
	// InitStepHost captures the SSH host (e.g. `s1.small.pl`).
	InitStepHost
	// InitStepPort captures the SSH port (defaults to 22).
	InitStepPort
	// InitStepUser captures the SSH user.
	InitStepUser
	// InitStepReview shows the collected values and asks the operator
	// to confirm before save.
	InitStepReview
	// InitStepDone parks until the parent transitions to Dashboard.
	InitStepDone
)

// ProjectWizardStep enumerates the sub-states of the new-project
// wizard. Order matches docs/UX.md §11.2 and PRD §6 F3; the database
// step is skipped automatically for static / vite-react stacks unless
// the user opts in.
type ProjectWizardStep int

const (
	// ProjectStepProfile picks the [config.Profile] for provisioning.
	ProjectStepProfile ProjectWizardStep = iota
	// ProjectStepStack picks the stack from [wizard.SupportedStacks].
	ProjectStepStack
	// ProjectStepDBChoice asks whether to provision a database (only
	// when the stack would skip by default).
	ProjectStepDBChoice
	// ProjectStepDBKind picks `mysql` or `postgresql`.
	ProjectStepDBKind
	// ProjectStepDBName captures the database identifier.
	ProjectStepDBName
	// ProjectStepDomain captures the subdomain and runs provider
	// preflight + duplicate check.
	ProjectStepDomain
	// ProjectStepReview displays the collected plan and asks for
	// confirmation before execution.
	ProjectStepReview
	// ProjectStepExecuting renders the active provisioning progress.
	ProjectStepExecuting
	// ProjectStepFailure offers the remediation menu (rollback or
	// keep). Reached only when [wizard.Execute] returns an error.
	ProjectStepFailure
	// ProjectStepRollingBack renders rollback progress when the
	// operator picks rollback from the remediation menu.
	ProjectStepRollingBack
	// ProjectStepDone parks until the parent transitions to Dashboard.
	ProjectStepDone
)

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
