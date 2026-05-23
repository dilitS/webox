package wizard

import (
	"fmt"
	"regexp"
	"strings"
)

var pinnedUsesPattern = regexp.MustCompile(`uses:\s*[^@\s]+@[a-f0-9]{40}(\s+#\s*.+)?$`)

// ValidateWorkflowField rejects values that would let user input become
// GitHub expression syntax or shell structure inside deploy.yml.
func ValidateWorkflowField(name, value string) error {
	if strings.Contains(value, "${{") || strings.Contains(value, "}}") {
		return fmt.Errorf("%w: workflow field %s contains GitHub expression syntax", ErrInvalidPlan, name)
	}
	if strings.ContainsAny(value, "\r\n`") || strings.Contains(value, "$(") {
		return fmt.Errorf("%w: workflow field %s contains shell metacharacters", ErrInvalidPlan, name)
	}
	return nil
}

// ValidateWorkflowRendered performs conservative structural checks on a
// rendered GitHub Actions workflow before it is committed. We avoid a new
// YAML dependency until maintainer sign-off; this still enforces the
// security invariants Webox must not violate.
func ValidateWorkflowRendered(rendered string) error {
	if strings.TrimSpace(rendered) == "" {
		return fmt.Errorf("%w: workflow is empty", ErrInvalidPlan)
	}
	required := []string{
		"name: webox deploy",
		"workflow_dispatch:",
		"rsync",
		"--delete",
		"--exclude='.env'",
		"--exclude='node_modules/'",
		"stat -c",
	}
	for _, needle := range required {
		if !strings.Contains(rendered, needle) {
			return fmt.Errorf("%w: workflow missing %q", ErrInvalidPlan, needle)
		}
	}
	for _, line := range strings.Split(rendered, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "uses:") && !strings.HasPrefix(trimmed, "- uses:") {
			continue
		}
		normalized := strings.TrimPrefix(trimmed, "- ")
		if !pinnedUsesPattern.MatchString(normalized) {
			return fmt.Errorf("%w: workflow action is not pinned to a full SHA: %s", ErrInvalidPlan, trimmed)
		}
	}
	return nil
}
