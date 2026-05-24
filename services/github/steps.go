//nolint:revive // Public DTOs mirror the GitHub Actions REST schema and the bento tile contract.
package github

import "time"

// Step is one row in the CI/CD tile's "pipeline" view. The shape
// mirrors `gh run view <runID> --json jobs --jq '.jobs[].steps[]'`
// and the REST `GET /repos/{owner}/{repo}/actions/runs/{run_id}/jobs`
// projection so transports can populate it without translation.
type Step struct {
	JobID       int64
	JobName     string
	Number      int
	Name        string
	Status      string // "queued", "in_progress", "completed"
	Conclusion  string // "success", "failure", "cancelled", "skipped", ""
	StartedAt   time.Time
	CompletedAt time.Time
	DurationMs  int64
}

// WorkflowRunSummary captures the metadata the CI/CD tile renders in
// its header line ("Build #412: SUCCESS ✓ (14:12 GMT)"). It is the
// projection [GetWorkflowSteps] returns alongside the steps slice so
// the caller does not need a second round-trip just to render the
// header.
type WorkflowRunSummary struct {
	RunID       int64
	Number      int
	Name        string
	Status      string
	Conclusion  string
	HTMLURL     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	StartedAt   time.Time
	CompletedAt time.Time
}

// IsTerminal reports whether the run reached a final conclusion. The
// CI/CD tile uses it to decide between rendering the static badge and
// the live elapsed-time spinner.
func (s WorkflowRunSummary) IsTerminal() bool {
	return s.Status == "completed"
}

// WorkflowLogLine is one line of the workflow's combined log stream
// (gh CLI `gh run view --log`). The bento `[CI/CD Pipeline]` tile's
// F8 modal renders the last 50 lines redacted via internal/log.Redact.
type WorkflowLogLine struct {
	JobName  string
	StepName string
	Raw      string
}

// jobsResponse / jobResponse / stepResponse map the wire schema. They
// are private so callers cannot accidentally couple to GitHub's exact
// field naming.
type jobsResponse struct {
	Jobs []jobResponse `json:"jobs"`
}

type jobResponse struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	Status      string         `json:"status"`
	Conclusion  string         `json:"conclusion"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt time.Time      `json:"completed_at"`
	Steps       []stepResponse `json:"steps"`
}

type stepResponse struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
	Number      int       `json:"number"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// flatten copies the parsed jobs payload into the public [Step] slice
// the CI/CD tile consumes. Order matches the wire (jobs in dispatch
// order, steps numbered 1..N within each job).
func (r jobsResponse) flatten() []Step {
	var out []Step
	for _, job := range r.Jobs {
		for _, step := range job.Steps {
			out = append(out, Step{
				JobID:       job.ID,
				JobName:     job.Name,
				Number:      step.Number,
				Name:        step.Name,
				Status:      step.Status,
				Conclusion:  step.Conclusion,
				StartedAt:   step.StartedAt,
				CompletedAt: step.CompletedAt,
				DurationMs:  diffMillis(step.StartedAt, step.CompletedAt),
			})
		}
	}
	return out
}

func diffMillis(start, end time.Time) int64 {
	if start.IsZero() || end.IsZero() {
		return 0
	}
	d := end.Sub(start)
	if d < 0 {
		return 0
	}
	return d.Milliseconds()
}
