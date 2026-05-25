package tui

// State is the top-level TUI route. The list mirrors docs/DESIGN.md §12.
type State int

const (
	// StateInitWizard is shown when no local config exists yet and
	// hosts the first-run profile capture flow (see [InitWizardStep]).
	StateInitWizard State = iota
	// StateDashboard is the project list and overview screen.
	StateDashboard
	// StateProjectDetail is the focused Overview tab for one project.
	StateProjectDetail
	// StateProjectWizard is the multi-step new-project flow
	// (see [ProjectWizardStep]).
	StateProjectWizard
	// StateResumeWizard is shown on launch when pending_cleanups.json
	// exists and the operator must choose rollback, keep, or discard.
	StateResumeWizard
	// StateImportPreview shows a read-only diff between projects in
	// the local config and the subdomains the provider reports
	// (PRD F9). The operator can accept the unmanaged rows to seed
	// stub `config.Project` entries without mutating the server.
	StateImportPreview
	// StateCommandPalette hosts the `/` fuzzy command launcher.
	StateCommandPalette
	// StateConfirmDialog renders modal destructive confirmations.
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
	case StateResumeWizard:
		return "ResumeWizard"
	case StateImportPreview:
		return "ImportPreview"
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

// DetailTab names the project-detail tabs.
//
// Sprint 20 TASK-20.4 — every tab is now enabled in MVP. The
// previous v0.2 placeholders (Env Diff + Database) ship as
// read-only views: Env Diff lists the project's `SecretsMeta`
// rotation status; Database is a stack-aware connection
// cheatsheet. Neither tab calls the provider — they consume
// only data already cached in `config.Project`.
type DetailTab int

const (
	// TabOverview shows the project health summary and action bar.
	TabOverview DetailTab = iota
	// TabEnvDiff lists the project's secret metadata (no values).
	TabEnvDiff
	// TabDatabase shows a per-stack DB connection cheatsheet.
	TabDatabase
	// TabLogs hosts the Sprint 09 live SSH tail (`tail -f` with
	// redaction, ring buffer, ANSI parser).
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
//
// Sprint 20 TASK-20.4 — Env Diff + Database join Overview + Logs in
// MVP as read-only views (no provider I/O). The method stays on
// [DetailTab] so older tests / future contributors that compare
// against `Enabled()` continue to find a single source of truth.
func (t DetailTab) Enabled() bool {
	return t == TabOverview || t == TabEnvDiff || t == TabDatabase || t == TabLogs
}

// liveLogsCapacity is the default ring buffer capacity for the Sprint 09
// live-log tab. 1024 mirrors `components.DefaultRingCapacity` rounded
// to the next power of two for easier debug-time inspection.
const liveLogsCapacity = 1024
