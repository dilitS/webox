//nolint:revive // WorkflowData fields are template variables documented by docs/providers/smallhost.md §6.
package assets

import (
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/dilitS/webox/wizard"
)

//go:embed workflows/*/deploy.yml
var workflowFS embed.FS

type WorkflowData struct {
	Domain         string
	DeployHost     string
	DeployUser     string
	DeployPath     string
	DistDir        string
	BuildCommand   string
	RestartCommand string
	RsyncExcludes  []string
}

func RenderDeployWorkflow(stack string, data WorkflowData) (string, error) {
	if err := validateWorkflowData(data); err != nil {
		return "", err
	}
	raw, err := workflowFS.ReadFile("workflows/" + stack + "/deploy.yml")
	if err != nil {
		return "", fmt.Errorf("assets: workflow template for stack %q: %w", stack, err)
	}
	tmpl, err := template.New(stack+"/deploy.yml").Delims("[[", "]]").Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("assets: parse workflow template %q: %w", stack, err)
	}
	var out strings.Builder
	if err := tmpl.Execute(&out, data); err != nil {
		return "", fmt.Errorf("assets: render workflow template %q: %w", stack, err)
	}
	rendered := out.String()
	if err := wizard.ValidateWorkflowRendered(rendered); err != nil {
		return "", err
	}
	return rendered, nil
}

func validateWorkflowData(data WorkflowData) error {
	fields := map[string]string{
		"domain":          data.Domain,
		"deploy_host":     data.DeployHost,
		"deploy_user":     data.DeployUser,
		"deploy_path":     data.DeployPath,
		"dist_dir":        data.DistDir,
		"build_command":   data.BuildCommand,
		"restart_command": data.RestartCommand,
	}
	for name, value := range fields {
		if value == "" {
			return fmt.Errorf("%w: workflow field %s is required", wizard.ErrInvalidPlan, name)
		}
		if err := wizard.ValidateWorkflowField(name, value); err != nil {
			return err
		}
	}
	for _, exclude := range data.RsyncExcludes {
		if err := wizard.ValidateWorkflowField("rsync_exclude", exclude); err != nil {
			return err
		}
	}
	return nil
}
