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

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/services/httpcheck"
	"github.com/dilitS/webox/status"
)

// DefaultConfigPath returns the standard config.json location.
func DefaultConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "webox", "config.json"), nil
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
