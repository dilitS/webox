package wizard_test

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/wizard"
)

func TestValidateWorkflowRenderedRejectsUnpinnedUses(t *testing.T) {
	t.Parallel()

	err := wizard.ValidateWorkflowRendered("steps:\n  - uses: actions/checkout@v4\n")
	if err == nil {
		t.Fatal("ValidateWorkflowRendered should reject tag-only uses")
	}
}

func TestValidateWorkflowRenderedRequiresSafetyRails(t *testing.T) {
	t.Parallel()

	workflow := strings.Join([]string{
		"name: webox deploy",
		"on:",
		"  workflow_dispatch:",
		"jobs:",
		"  deploy:",
		"    steps:",
		"      - uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11 # v4.1.1",
		"      - run: rsync -az --delete --exclude='.env' --exclude='node_modules/' --exclude='uploads/' dist/ remote:/path/",
		"      - run: stat -c \"%a %U\" /path/.env",
	}, "\n")
	if err := wizard.ValidateWorkflowRendered(workflow); err != nil {
		t.Fatalf("ValidateWorkflowRendered: %v", err)
	}
}

func TestValidateWorkflowFieldRejectsExpressionInjection(t *testing.T) {
	t.Parallel()

	if err := wizard.ValidateWorkflowField("domain", "app.${{ secrets.BAD }}.example"); err == nil {
		t.Fatal("ValidateWorkflowField should reject GitHub expression injection")
	}
	if err := wizard.ValidateWorkflowField("domain", "app.example.test"); err != nil {
		t.Fatalf("ValidateWorkflowField safe value: %v", err)
	}
}
