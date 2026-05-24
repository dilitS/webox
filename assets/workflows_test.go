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
