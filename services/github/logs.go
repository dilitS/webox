package github

import (
	"strings"

	"github.com/dilitS/webox/internal/log"
)

// ghLogPrefix is the marker `gh run view --log` prepends to every
// line: `<jobName>\t<stepName>\t<timestamp>\t<message>`. We split on
// the first three tabs so log payloads containing tab characters do
// not lose data.
const ghLogPrefix = "\t"

// parseGHLogLines parses the raw `gh run view --log` output into the
// public [WorkflowLogLine] projection. Every line passes through
// `internal/log.Redact` BEFORE it leaves this function so callers
// (including the modal renderer and the snapshot tests) never see
// raw secrets.
//
// If maxLines > 0 the returned slice is capped to the last N
// post-redaction lines; if maxLines <= 0 every parsed line is
// returned.
func parseGHLogLines(raw []byte, maxLines int) []WorkflowLogLine {
	if len(raw) == 0 {
		return nil
	}
	text := string(raw)
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")

	out := make([]WorkflowLogLine, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		redacted := log.Redact(line)
		jobName, stepName, message := splitGHLogLine(redacted)
		out = append(out, WorkflowLogLine{
			JobName:  jobName,
			StepName: stepName,
			Raw:      message,
		})
	}

	if maxLines > 0 && len(out) > maxLines {
		out = out[len(out)-maxLines:]
	}
	return out
}

// splitGHLogLine cuts a `<job>\t<step>\t<msg>` payload into its
// components. Lines without two tabs (older gh versions, or stderr
// noise) are returned with the whole text in the message slot.
//
// ghLogColumns mirrors the documented gh CLI format: <job>, <step>,
// <message>. Older versions may emit only <job>, <message> (2 cols).
const (
	ghLogColumnsFull    = 3
	ghLogColumnsLegacy  = 2
	ghLogColumnsMissing = 0
)

func splitGHLogLine(line string) (jobName, stepName, message string) {
	parts := strings.SplitN(line, ghLogPrefix, ghLogColumnsFull)
	switch len(parts) {
	case ghLogColumnsFull:
		return parts[0], parts[1], parts[2]
	case ghLogColumnsLegacy:
		return parts[0], "", parts[1]
	default:
		_ = ghLogColumnsMissing
		return "", "", line
	}
}
