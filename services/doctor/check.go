package doctor

import (
	"context"
	"time"
)

// Severity mirrors docs/DESIGN.md §15.3 and indicates how seriously the
// operator should treat a given check.
type Severity string

// Severity values used by doctor checks.
const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityFatal Severity = "fatal"
)

// Status is the per-check outcome used in the structured JSON report.
type Status string

// Status values used by doctor checks and report summaries.
const (
	StatusOK      Status = "ok"
	StatusWarn    Status = "warn"
	StatusFail    Status = "fail"
	StatusSkipped Status = "skipped"
)

// Check is a single doctor probe.
type Check interface {
	Run(ctx context.Context) Result
}

type checkFunc func(context.Context) Result

// Run adapts a bare function to the [Check] interface.
func (fn checkFunc) Run(ctx context.Context) Result { return fn(ctx) }

// Result is one doctor check entry.
type Result struct {
	ID       string   `json:"id"`
	Category string   `json:"category"`
	Severity Severity `json:"severity"`
	Status   Status   `json:"status"`
	Message  string   `json:"message"`
	Hint     string   `json:"hint,omitempty"`
}

// Summary is the per-status aggregate in the doctor report.
type Summary struct {
	OK      int `json:"ok"`
	Warn    int `json:"warn"`
	Fail    int `json:"fail"`
	Skipped int `json:"skipped"`
}

// Platform reports the runtime environment that generated the report.
type Platform struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	GoVersion string `json:"go_version"`
}

// Report is the machine-readable form returned by `webox doctor --json`.
type Report struct {
	SchemaVersion int       `json:"schema_version"`
	GeneratedAt   time.Time `json:"generated_at"`
	WeboxVersion  string    `json:"webox_version"`
	Platform      Platform  `json:"platform"`
	Checks        []Result  `json:"checks"`
	Summary       Summary   `json:"summary"`
}
