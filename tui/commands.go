package tui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/dilitS/webox/config"
	ghsvc "github.com/dilitS/webox/services/github"
	"github.com/dilitS/webox/services/httpcheck"
	"github.com/dilitS/webox/status"
	"github.com/dilitS/webox/wizard"
)

// GitHubLastDeployFetcher returns the most recent workflow run for the
// referenced repository. The TUI calls it through a SWR cache so the
// dashboard never blocks on GitHub.
//
// Implementations MUST respect ctx cancellation and return (nil, nil)
// when the repo has no runs yet (not an error).
type GitHubLastDeployFetcher func(ctx context.Context, ref ghsvc.RepoRef, workflow string) (*ghsvc.WorkflowRun, error)

// DefaultWorkflowFile is the workflow filename the wizard commits when
// it bootstraps a GitHub-backed project. The dashboard fetcher uses it
// as the LatestRun lookup key for any project that does not override
// the value.
const DefaultWorkflowFile = "deploy.yml"

// lastDeployPlaceholder is the cell content used when the project has
// no GitHub link yet, when the fetcher is missing, or when the
// fetcher returned an error. Distinct from "no run yet" so the
// dashboard can distinguish "linked but no runs" from "not linked".
const lastDeployPlaceholder = "—"

// lastDeployNoRun is the cell content used when the repo is linked
// but GitHub returned no completed runs yet. The wizard typically
// triggers a workflow_dispatch immediately so this state should be
// short-lived.
const lastDeployNoRun = "no run yet"

// lastDeployUnavailable is the cell content used when the fetcher
// failed (rate limit, offline, gh missing). The dashboard keeps the
// previous successful value when available (SWR fallback).
const lastDeployUnavailable = "unavailable"

const (
	// repoRefParts is the count produced by splitting "owner/name"
	// on "/" — both halves must be present for parsing to succeed.
	repoRefParts = 2

	// hoursPerDay / daysPerMonth are used to humanise [time.Duration]
	// into operator-friendly "Nd ago" / "Nmo ago" strings.
	hoursPerDay  = 24
	daysPerMonth = 30
)

// DefaultConfigPath returns the standard config.json location.
func DefaultConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "webox", "config.json"), nil
}

// DefaultPendingPath returns the standard pending_cleanups.json location.
func DefaultPendingPath() (string, error) {
	return wizard.DefaultPendingPath()
}

// Init returns the initial config-load command for Bubble Tea.
func (m Model) Init() tea.Cmd {
	if m.cfg != nil {
		return nil
	}
	return tea.Batch(loadConfigCmd(m.ctx, m.configPath), loadPendingCmd(m.pendingPath))
}

func loadPendingCmd(path string) tea.Cmd {
	return func() tea.Msg {
		if path == "" {
			resolved, err := DefaultPendingPath()
			if err != nil {
				return PendingLoadedMsg{Err: err}
			}
			path = resolved
		}
		snap, err := wizard.LoadPending(path)
		return PendingLoadedMsg{Snapshot: snap, Err: err}
	}
}

func loadConfigCmd(ctx context.Context, path string) tea.Cmd {
	return func() tea.Msg {
		if path == "" {
			resolved, err := DefaultConfigPath()
			if err != nil {
				return ConfigLoadFailedMsg{Err: err}
			}
			path = resolved
		}
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return ConfigLoadedMsg{Missing: true, Config: config.DefaultConfig()}
			}
			return ConfigLoadFailedMsg{Err: fmt.Errorf("stat config: %w", err)}
		}
		cfg, err := config.Load(ctx, path)
		if err != nil {
			return ConfigLoadFailedMsg{Err: err}
		}
		if cfg != nil && len(cfg.Profiles) == 0 {
			return ConfigLoadedMsg{Missing: true, Config: cfg}
		}
		return ConfigLoadedMsg{Config: cfg}
	}
}

func scheduleRefresh(interval time.Duration) tea.Cmd {
	if interval <= 0 {
		interval = defaultRefreshInterval
	}
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return RefreshTickMsg(t)
	})
}

func refreshVisibleProjectsCmd(m Model) tea.Cmd {
	projects := append([]config.Project(nil), m.visibleProjects()...)
	cache := m.cache
	fetch := m.fetchStatuses
	ctx := m.ctx
	return func() tea.Msg {
		statuses, err := fetch(ctx, projects, cache)
		if err != nil {
			return StatusRefreshFailedMsg{Err: err}
		}
		return StatusRefreshedMsg{Statuses: statuses}
	}
}

func (m Model) visibleProjects() []config.Project {
	if m.cfg == nil {
		return nil
	}
	return m.cfg.Projects
}

func saveProfileCmd(ctx context.Context, configPath string, profile config.Profile, current *config.Config) tea.Cmd {
	return func() tea.Msg {
		cfg := current
		if cfg == nil {
			cfg = config.DefaultConfig()
		}
		updated := *cfg
		updated.Profiles = append([]config.Profile(nil), cfg.Profiles...)
		updated.Profiles = append(updated.Profiles, profile)

		path := configPath
		if path == "" {
			resolved, err := DefaultConfigPath()
			if err != nil {
				return ProfileSaveFailedMsg{Err: err}
			}
			path = resolved
		}
		if err := config.Save(ctx, path, &updated); err != nil {
			return ProfileSaveFailedMsg{Err: err}
		}
		return ProfileSavedMsg{Config: &updated}
	}
}

func wizardPreflightCmd(ctx context.Context, runner WizardRunner, profile config.Profile, alias string) tea.Cmd {
	return func() tea.Msg {
		_, err := runner.Preflight(ctx, profile)
		return ProjectWizardPreflightMsg{ProfileAlias: alias, Err: err}
	}
}

func wizardDomainCheckCmd(ctx context.Context, runner WizardRunner, profile config.Profile, domain string) tea.Cmd {
	return func() tea.Msg {
		err := runner.CheckDomainAvailable(ctx, profile, domain)
		if err == nil {
			return ProjectWizardDomainCheckedMsg{Domain: domain, Available: true}
		}
		msg := ProjectWizardDomainCheckedMsg{Domain: domain, Available: false}
		msg.Err = err
		return msg
	}
}

func wizardExecuteCmd(ctx context.Context, runner WizardRunner, profile config.Profile, plan wizard.ProvisionPlan, slot *wizardStackSlot, pendingPath string, cfg *config.Config, configPath string) tea.Cmd {
	return func() tea.Msg {
		wizardID := uuid.NewString()
		persist := wizard.NewFilePersisterWithProfile(pendingPath, wizardID, plan.ProfileAlias)
		stack := wizard.NewStack(persist, wizardID)
		slot.set(stack)

		report, err := runner.Execute(ctx, profile, plan, stack)
		msg := ProjectWizardExecutedMsg{Plan: plan, Report: report, Err: err}
		if err != nil {
			return msg
		}
		newProject := config.Project{
			ID:           wizardID,
			Domain:       plan.Domain,
			ProfileAlias: plan.ProfileAlias,
			Stack:        plan.Stack,
			NodeVersion:  plan.NodeVersion,
		}
		updated := *cfg
		updated.Projects = append([]config.Project(nil), cfg.Projects...)
		updated.Projects = append(updated.Projects, newProject)

		path := configPath
		if path == "" {
			resolved, perr := DefaultConfigPath()
			if perr != nil {
				msg.SaveErr = perr
				return msg
			}
			path = resolved
		}
		if err := config.Save(ctx, path, &updated); err != nil {
			msg.SaveErr = err
			return msg
		}
		_ = wizard.RemovePending(pendingPath)
		slot.set(nil)
		msg.ProjectID = newProject.ID
		msg.ProjectCfg = &updated
		if report != nil {
			msg.Credentials = report.Credentials
		}
		return msg
	}
}

// defaultTailLines mirrors the smallhost adapter default so the
// dashboard view does not have to know the line cap.
const defaultTailLines = 200

func projectActionCmd(ctx context.Context, runner WizardRunner, kind ProjectActionKind, profile config.Profile, project config.Project, cache *status.Cache) tea.Cmd {
	return func() tea.Msg {
		msg := ProjectActionCompletedMsg{Kind: kind, ProjectID: project.ID}
		switch kind {
		case ProjectActionRestart:
			msg.Err = runner.RestartApp(ctx, profile, project.Domain)
			if cache != nil && msg.Err == nil {
				cache.InvalidateEvent(status.EventRestart)
			}
		case ProjectActionSSLRenew:
			msg.Err = runner.RenewSSL(ctx, profile, project.Domain)
			if cache != nil && msg.Err == nil {
				cache.InvalidateEvent(status.EventRenewSSL)
			}
		case ProjectActionLogs:
			out, err := runner.TailLog(ctx, profile, project.Domain, defaultTailLines)
			msg.Output = out
			msg.Err = err
		}
		return msg
	}
}

// importScanCmd queries every configured profile for the subdomains
// the panel knows about, joins them with the local `config.Projects`,
// and returns an [ImportScanCompletedMsg]. The command is read-only:
// it never mutates server state or the local config — only the
// follow-up [importPersistCmd] writes anything.
func importScanCmd(ctx context.Context, runner WizardRunner, cfg *config.Config) tea.Cmd {
	profiles := make([]config.Profile, 0)
	if cfg != nil {
		profiles = append(profiles, cfg.Profiles...)
	}
	managed := make(map[string]struct{})
	if cfg != nil {
		for _, project := range cfg.Projects {
			managed[strings.ToLower(project.Domain)] = struct{}{}
		}
	}
	return func() tea.Msg {
		rows := make([]ImportRow, 0)
		for _, profile := range profiles {
			subdomains, err := runner.ListProviderSubdomains(ctx, profile)
			if err != nil {
				return ImportScanCompletedMsg{Err: fmt.Errorf("profile %s: %w", profile.Alias, err), ProfilesScanned: len(profiles)}
			}
			for _, sub := range subdomains {
				row := ImportRow{
					ProfileAlias: profile.Alias,
					Domain:       sub.Domain,
					Type:         sub.Type,
					NodeVersion:  sub.NodeVersion,
				}
				if _, ok := managed[strings.ToLower(sub.Domain)]; ok {
					row.Managed = true
				}
				rows = append(rows, row)
			}
		}
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Domain < rows[j].Domain })
		return ImportScanCompletedMsg{Rows: rows, ProfilesScanned: len(profiles)}
	}
}

// importPersistCmd writes one stub [config.Project] per unmanaged
// row. The function never touches the server — it only adds local
// metadata so the dashboard can show the rows as `STALE` until the
// operator runs the wizard for them.
func importPersistCmd(ctx context.Context, configPath string, cfg *config.Config, rows []ImportRow) tea.Cmd {
	return func() tea.Msg {
		path := configPath
		if path == "" {
			resolved, err := DefaultConfigPath()
			if err != nil {
				return ImportPersistedMsg{Err: err}
			}
			path = resolved
		}
		base := cfg
		if base == nil {
			base = config.DefaultConfig()
		}
		updated := *base
		updated.Projects = append([]config.Project(nil), base.Projects...)
		now := time.Now().UTC()
		for _, row := range rows {
			stack := stackForImportType(row.Type)
			imported := now
			updated.Projects = append(updated.Projects, config.Project{
				ID:           uuid.NewString(),
				Domain:       row.Domain,
				ProfileAlias: row.ProfileAlias,
				NodeVersion:  row.NodeVersion,
				Stack:        stack,
				ImportedAt:   &imported,
			})
		}
		if err := config.Save(ctx, path, &updated); err != nil {
			return ImportPersistedMsg{Err: err}
		}
		return ImportPersistedMsg{Config: &updated, ImportedRows: len(rows)}
	}
}

// stackForImportType maps the panel-reported subdomain type to the
// closest [wizard.SupportedStacks] token. Unknown types degrade to
// the empty string so the dashboard renders them as "unknown" rather
// than mislabelling a static site as Node.js.
func stackForImportType(panelType string) string {
	switch strings.ToLower(strings.TrimSpace(panelType)) {
	case "nodejs", "node", "node.js":
		return "node-express"
	case "static", "html":
		return "static"
	default:
		return ""
	}
}

func pendingDiscardCmd(path string) tea.Cmd {
	return func() tea.Msg {
		if path == "" {
			resolved, err := DefaultPendingPath()
			if err != nil {
				return PendingDiscardedMsg{Err: err}
			}
			path = resolved
		}
		return PendingDiscardedMsg{Err: wizard.RemovePending(path)}
	}
}

func resumeRollbackCmd(ctx context.Context, runner WizardRunner, cfg *config.Config, snap *wizard.PendingCleanups, pendingPath string) tea.Cmd {
	return func() tea.Msg {
		if snap == nil {
			return ProjectWizardRolledBackMsg{}
		}
		profile, ok := ProfileByAlias(cfg, snap.ProfileAlias)
		if !ok {
			return ProjectWizardRolledBackMsg{Err: fmt.Errorf("%w: resume profile %q not found", wizard.ErrInvalidPlan, snap.ProfileAlias)}
		}
		persist := wizard.NewFilePersisterWithProfile(pendingPath, snap.WizardID, snap.ProfileAlias)
		stack := wizard.NewStack(persist, snap.WizardID)
		if err := stack.LoadSnapshot(snap.Steps); err != nil {
			return ProjectWizardRolledBackMsg{Err: err}
		}
		results, err := runner.Rollback(ctx, profile, stack)
		if err == nil {
			_ = wizard.RemovePending(pendingPath)
		}
		return ProjectWizardRolledBackMsg{Results: results, Err: err}
	}
}

func wizardRollbackCmd(ctx context.Context, runner WizardRunner, profile config.Profile, slot *wizardStackSlot, pendingPath string) tea.Cmd {
	return func() tea.Msg {
		stack := slot.get()
		if stack == nil {
			return ProjectWizardRolledBackMsg{}
		}
		results, err := runner.Rollback(ctx, profile, stack)
		if err == nil {
			_ = wizard.RemovePending(pendingPath)
			slot.set(nil)
		}
		return ProjectWizardRolledBackMsg{Results: results, Err: err}
	}
}

// FetchProjectStatuses performs read-only status probes (HTTP, TLS,
// SSH node version) per project without GitHub integration. New call
// sites should prefer [FetchProjectStatusesWithGitHub] so the dashboard
// can render a real last-deploy timestamp.
func FetchProjectStatuses(ctx context.Context, projects []config.Project, cache *status.Cache) ([]ProjectStatus, error) {
	return FetchProjectStatusesWithGitHub(ctx, projects, cache, nil)
}

// FetchProjectStatusesWithGitHub is the dashboard-grade probe that
// adds a SWR-cached `last_deploy` lookup against GitHub Actions. The
// fetcher is optional; nil produces the same output as
// [FetchProjectStatuses] but with a "—" placeholder for unlinked
// projects.
func FetchProjectStatusesWithGitHub(
	ctx context.Context,
	projects []config.Project,
	cache *status.Cache,
	fetcher GitHubLastDeployFetcher,
) ([]ProjectStatus, error) {
	out := make([]ProjectStatus, 0, len(projects))
	for _, project := range projects {
		snapshot := ProjectStatus{
			ProjectID:   project.ID,
			HTTPHealth:  "pending",
			SSLDaysLeft: -1,
			NodeVersion: project.NodeVersion,
			LastDeploy:  lastDeployPlaceholder,
			State:       ProjectUnknown,
		}

		httpResult, httpMeta, httpErr := status.GetOrFetchMeta(cache, status.PrefixHTTP+project.Domain, status.HTTPStatusTTL, func(ctx context.Context) (httpcheck.HTTPResult, error) {
			return httpcheck.ProbeHTTP(ctx, "https://"+project.Domain, httpcheck.HTTPOptions{})
		}, ctx)
		if httpErr == nil {
			snapshot.HTTPHealth = fmt.Sprintf("%d %s", httpResult.StatusCode, httpResult.Class)
			snapshot.State = ProjectOffline
			if httpResult.StatusCode >= 200 && httpResult.StatusCode <= 399 {
				snapshot.State = ProjectOnline
			}
			snapshot.Stale = httpMeta.IsStale
			snapshot.FetchedAt = httpMeta.FetchedAt
		} else {
			snapshot.HTTPHealth = "offline"
			snapshot.State = ProjectOffline
		}

		tlsResult, _, tlsErr := status.GetOrFetchMeta(cache, status.PrefixSSL+project.Domain, status.SSLCertTTL, func(ctx context.Context) (httpcheck.TLSResult, error) {
			return httpcheck.ProbeTLS(ctx, net.JoinHostPort(project.Domain, "443"), httpcheck.TLSOptions{})
		}, ctx)
		if tlsErr == nil {
			snapshot.SSLDaysLeft = tlsResult.DaysLeft
		}
		if project.ImportedAt != nil {
			snapshot.State = ProjectStale
			snapshot.Stale = true
		}

		snapshot.LastDeploy = resolveLastDeploy(ctx, cache, fetcher, project)

		out = append(out, snapshot)
	}
	return out, nil
}

func resolveLastDeploy(
	ctx context.Context,
	cache *status.Cache,
	fetcher GitHubLastDeployFetcher,
	project config.Project,
) string {
	if fetcher == nil {
		return lastDeployPlaceholder
	}
	ref, ok := parseRepoRef(project.Repo)
	if !ok {
		return lastDeployPlaceholder
	}
	key := status.PrefixGitHubLastDeploy + ref.FullName() + ":" + DefaultWorkflowFile
	run, _, err := status.GetOrFetchMeta(cache, key, status.GitHubLastDeployTTL, func(ctx context.Context) (*ghsvc.WorkflowRun, error) {
		return fetcher(ctx, ref, DefaultWorkflowFile)
	}, ctx)
	if err != nil {
		return lastDeployUnavailable
	}
	return formatLastDeploy(run)
}

// parseRepoRef splits "owner/name" into a [ghsvc.RepoRef]. Returns
// false for any value that does not match the canonical form so the
// caller can fall back to the "—" placeholder instead of issuing a
// fetch that GitHub would reject.
func parseRepoRef(raw string) (ghsvc.RepoRef, bool) {
	parts := strings.SplitN(strings.TrimSpace(raw), "/", repoRefParts)
	if len(parts) != repoRefParts || parts[0] == "" || parts[1] == "" {
		return ghsvc.RepoRef{}, false
	}
	return ghsvc.RepoRef{Owner: parts[0], Name: parts[1]}, true
}

func formatLastDeploy(run *ghsvc.WorkflowRun) string {
	if run == nil {
		return lastDeployNoRun
	}
	stamp := run.UpdatedAt
	if run.StartedAt != nil && !run.StartedAt.IsZero() {
		stamp = *run.StartedAt
	}
	rel := humanizeAge(time.Since(stamp))
	switch {
	case run.Conclusion == "success":
		return rel + " · success"
	case run.Conclusion != "":
		return rel + " · " + run.Conclusion
	case run.Status != "":
		return rel + " · " + run.Status
	default:
		return rel
	}
}

func humanizeAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < hoursPerDay*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < daysPerMonth*hoursPerDay*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours())/hoursPerDay)
	default:
		return fmt.Sprintf("%dmo ago", int(d.Hours())/(hoursPerDay*daysPerMonth))
	}
}
