package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/config"
	ghsvc "github.com/dilitS/webox/services/github"
	"github.com/dilitS/webox/status"
	"github.com/dilitS/webox/tui/bento"
)

// cicdLogsMaxLines is the hard cap on rendered workflow-log lines.
// `gh run view --log` may return tens of thousands of lines for a
// busy run; the modal stays scrollable but never loads more than the
// last 50 lines (Sprint 10 plan §TASK-10.3 risk mitigation).
const cicdLogsMaxLines = 50

// cicdRunHeaderLayout is the formatter used in the CI/CD pipeline
// tile header. Time-only (HH:MM TZ) keeps the line compact.
const cicdRunHeaderLayout = "15:04 MST"

// cicdLogsModalForm holds the in-memory state for the F8 logs modal.
// Open == true while the modal is visible; the renderer pulls Lines
// straight from this struct so producers never mutate the snapshot
// during render.
type cicdLogsModalForm struct {
	Open         bool
	ProjectID    string
	ProjectAlias string
	RunID        int64
	RunNumber    int
	RunStatus    bento.CICDStatus
	Loading      bool
	Lines        []CICDLogLineSnapshot
	Err          string
	ScrollOffset int
}

// cicdPipelineCacheKey builds the status-cache key used by the
// CI/CD tile poll. Matches Sprint 10 plan §TASK-10.2.
func cicdPipelineCacheKey(ref ghsvc.RepoRef, workflow string) string {
	if workflow == "" {
		workflow = DefaultWorkflowFile
	}
	return status.PrefixGitHubSteps + ref.FullName() + ":" + workflow
}

// scheduleCICDTick emits the next CICDTickMsg after `interval`.
// Polling at GitHubStepsTTL keeps the badge fresh without exhausting
// the gh CLI auth quota (5000/h ≫ 6 polls/min/project).
func scheduleCICDTick(interval time.Duration) tea.Cmd {
	if interval <= 0 {
		interval = status.GitHubStepsTTL
	}
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return CICDTickMsg(t)
	})
}

// pollCICDPipelineCmd fetches the active project's CI/CD pipeline
// snapshot via the SWR cache. Errors are folded into [CICDFetchedMsg]
// so the Update loop can route the rate-limited case to a graceful
// placeholder instead of an alert.
func pollCICDPipelineCmd(m Model) tea.Cmd {
	if m.cicdFetcher == nil {
		return nil
	}
	project, ok := m.selectedProject()
	if !ok {
		return nil
	}
	ref, ok := parseRepoRef(project.Repo)
	if !ok {
		return nil
	}
	cache := m.cache
	fetcher := m.cicdFetcher
	ctx := m.ctx
	projectID := project.ID
	workflow := DefaultWorkflowFile

	return func() tea.Msg {
		key := cicdPipelineCacheKey(ref, workflow)
		result, _, err := status.GetOrFetchMeta(cache, key, status.GitHubStepsTTL,
			func(fetchCtx context.Context) (PipelineFetchResult, error) {
				return fetcher(fetchCtx, ref, workflow)
			}, ctx)
		return CICDFetchedMsg{
			ProjectID: projectID,
			Result:    result,
			Err:       err,
			FetchedAt: time.Now(),
		}
	}
}

// invalidateCICDCacheForProject is called whenever the operator
// switches the selected project on the dashboard. The CI/CD tile
// must re-fetch fresh data immediately so Sprint 10 §TASK-10.4 is
// satisfied.
func invalidateCICDCacheForProject(cache *status.Cache, project config.Project) {
	if cache == nil {
		return
	}
	ref, ok := parseRepoRef(project.Repo)
	if !ok {
		return
	}
	cache.Invalidate(cicdPipelineCacheKey(ref, DefaultWorkflowFile))
}

// loadCICDLogsCmd fetches the latest workflow-log tail through the
// pipeline fetcher / logs transport. Always passes through redaction
// at the transport boundary (see services/github/logs.go).
func loadCICDLogsCmd(m Model, runID int64) tea.Cmd {
	if m.cicdLogsFetcher == nil {
		return nil
	}
	project, ok := m.selectedProject()
	if !ok {
		return nil
	}
	ref, ok := parseRepoRef(project.Repo)
	if !ok {
		return nil
	}
	fetcher := m.cicdLogsFetcher
	ctx := m.ctx
	projectID := project.ID

	return func() tea.Msg {
		lines, err := fetcher(ctx, ref, runID, cicdLogsMaxLines)
		snapshot := make([]CICDLogLineSnapshot, len(lines))
		for i, line := range lines {
			snapshot[i] = CICDLogLineSnapshot{
				JobName:  line.JobName,
				StepName: line.StepName,
				Text:     line.Raw,
			}
		}
		return CICDLogsFetchedMsg{
			ProjectID: projectID,
			RunID:     runID,
			Lines:     snapshot,
			Err:       err,
		}
	}
}

// buildCICDPipelineSnapshot composes the bento tile snapshot from the
// per-project cache entry. Returns the placeholder snapshot (with
// just ProjectAlias) when no successful poll has completed yet.
func buildCICDPipelineSnapshot(m Model) (bento.CICDPipelineSnapshot, bool) {
	project, ok := m.selectedProject()
	if !ok {
		return bento.CICDPipelineSnapshot{}, false
	}
	if _, ok := parseRepoRef(project.Repo); !ok {
		return bento.CICDPipelineSnapshot{}, false
	}
	cached, has := m.cicdSnapshots[project.ID]
	snap := bento.CICDPipelineSnapshot{
		ProjectAlias: project.Domain,
		WorkflowName: DefaultWorkflowFile,
	}
	if !has {
		return snap, true
	}

	if cached.RateLimited {
		snap.RateLimited = true
		snap.RateLimitHint = cached.RateLimitHint
	} else if cached.Err != "" {
		snap.ErrorMessage = cached.Err
	}
	if cached.Run != nil {
		snap.RunNumber = cached.Run.RunNumber
		snap.RunStatus = cicdStatusFromGitHub(cached.Run.Status, cached.Run.Conclusion)
		snap.RunSummary = cached.Run.Status
		if !cached.Run.HeaderTime.IsZero() {
			snap.HeaderTime = cached.Run.HeaderTime.Format(cicdRunHeaderLayout)
		}
		snap.Duration = cached.Run.Duration
	}
	for _, step := range cached.Steps {
		snap.Steps = append(snap.Steps, bento.CICDStepSnapshot{
			Number:   step.Number,
			Name:     step.Name,
			Status:   cicdStatusFromGitHub(step.Status, step.Conclusion),
			Duration: formatStepDuration(step.DurationMs, step.Status),
		})
	}
	if cached.Stale {
		snap.Stale = true
	}
	return snap, true
}

// cicdStatusFromGitHub maps the gh CLI / REST status+conclusion pair
// onto the bento tile's [CICDStatus] enum. Mirrors the badge table
// in UX §3.1.
func cicdStatusFromGitHub(runStatus, conclusion string) bento.CICDStatus {
	switch strings.ToLower(conclusion) {
	case "success":
		return bento.CICDStatusSuccess
	case "failure", "timed_out", "startup_failure":
		return bento.CICDStatusFailure
	case "cancelled":
		return bento.CICDStatusCancelled
	case "skipped":
		return bento.CICDStatusSkipped
	}
	switch strings.ToLower(runStatus) {
	case "in_progress":
		return bento.CICDStatusInProgress
	case "queued", "waiting", "requested", "pending":
		return bento.CICDStatusQueued
	case "completed":
		// Completed without a conclusion is unusual; surface as
		// queued so the operator can drill in via F8.
		return bento.CICDStatusQueued
	}
	return bento.CICDStatusUnknown
}

// formatStepDuration renders the per-step duration cell. Sub-second
// durations collapse to "<1s"; longer ones use Go's default Duration
// formatting (`12.3s`, `1m24s`, ...). In-progress steps render an
// elapsed-time-style placeholder so the tile never goes blank.
func formatStepDuration(ms int64, runStatus string) string {
	if runStatus == "in_progress" {
		return "running"
	}
	if ms <= 0 {
		return ""
	}
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return "<1s"
	}
	d = d.Round(time.Second)
	return d.String()
}

// cicdSnapshotEntry is the per-project cache record kept in the
// model. The fields mirror the renderer's needs (badge, header,
// rate-limit) so the renderer never reaches back into
// `services/github`.
type cicdSnapshotEntry struct {
	Run           *cicdRunSummary
	Steps         []ghsvc.Step
	Stale         bool
	RateLimited   bool
	RateLimitHint string
	Err           string
	FetchedAt     time.Time
}

// cicdRunSummary is the slim header projection consumed by the tile
// (the WorkflowRun struct lives in `services/github` and carries
// extra fields the renderer does not need).
type cicdRunSummary struct {
	RunID      int64
	RunNumber  int
	Status     string
	Conclusion string
	HeaderTime time.Time
	Duration   string
}

// summarizeRun copies the fields the renderer needs out of a
// [ghsvc.WorkflowRun]. Returns nil when run is nil so the caller can
// hand the renderer a sentinel for the "no run yet" branch.
func summarizeRun(run *ghsvc.WorkflowRun) *cicdRunSummary {
	if run == nil {
		return nil
	}
	header := run.UpdatedAt
	if run.StartedAt != nil && !run.StartedAt.IsZero() {
		header = *run.StartedAt
	}
	var dur string
	if run.StartedAt != nil && !run.StartedAt.IsZero() {
		end := run.UpdatedAt
		if d := end.Sub(*run.StartedAt); d > 0 {
			dur = d.Round(time.Second).String()
		}
	}
	return &cicdRunSummary{
		RunID:      run.ID,
		RunNumber:  run.RunNumber,
		Status:     run.Status,
		Conclusion: run.Conclusion,
		HeaderTime: header,
		Duration:   dur,
	}
}

// extractRateLimitInfo turns a [ghsvc.ErrRateLimited] error into the
// hint we render below the tile header. The hint is intentionally
// short ("Reset in 12min") so it fits inside the Standard cockpit
// without truncation.
func extractRateLimitInfo(err error) (limited bool, hint string) {
	if err == nil {
		return false, ""
	}
	if !errors.Is(err, ghsvc.ErrRateLimited) {
		return false, ""
	}
	// Best-effort parse of any "reset=<rfc>" / "in 12m" hint the
	// transport surfaced. We keep this conservative: when no parse
	// matches, just return the generic phrase.
	msg := err.Error()
	if idx := strings.Index(msg, "reset="); idx >= 0 {
		stamp := strings.TrimSpace(msg[idx+len("reset="):])
		if t, parseErr := time.Parse(time.RFC3339, stamp); parseErr == nil {
			return true, "Reset in " + formatResetDelta(time.Until(t))
		}
	}
	return true, "Retry after backoff"
}

func formatResetDelta(d time.Duration) string {
	if d <= 0 {
		return "<1min"
	}
	if d < time.Minute {
		return "<1min"
	}
	const minutesPerHour = 60
	mins := int(d.Round(time.Minute).Minutes())
	if mins >= minutesPerHour {
		hours := mins / minutesPerHour
		mins %= minutesPerHour
		if mins == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%02dm", hours, mins)
	}
	return fmt.Sprintf("%dmin", mins)
}

// selectedProject returns the currently highlighted dashboard project
// (if any) so polling commands can target a single repo.
func (m Model) selectedProject() (config.Project, bool) {
	projects := cfgProjects(m.cfg)
	if len(projects) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(projects) {
		return config.Project{}, false
	}
	return projects[m.selectedIndex], true
}
