package tui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/services/httpcheck"
	"github.com/dilitS/webox/status"
	"github.com/dilitS/webox/wizard"
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
	return loadConfigCmd(m.ctx, m.configPath)
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
		persist := wizard.NewFilePersister(pendingPath, wizardID)
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

// FetchProjectStatuses performs the Sprint 04 read-only status probes.
func FetchProjectStatuses(ctx context.Context, projects []config.Project, cache *status.Cache) ([]ProjectStatus, error) {
	out := make([]ProjectStatus, 0, len(projects))
	for _, project := range projects {
		snapshot := ProjectStatus{
			ProjectID:   project.ID,
			HTTPHealth:  "pending",
			SSLDaysLeft: -1,
			NodeVersion: project.NodeVersion,
			LastDeploy:  "pending Sprint 06",
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
		out = append(out, snapshot)
	}
	return out, nil
}
