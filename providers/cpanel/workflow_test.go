package cpanel_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/dilitS/webox/providers/cpanel"
	"github.com/dilitS/webox/wizard"
)

func TestWorkflowRestartCommand_PassengerTouch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		appPath  string
		expected string
	}{
		{
			"standard_cloudlinux_layout",
			"/home/alice/nodejs/shop-example-com",
			"touch /home/alice/nodejs/shop-example-com/tmp/restart.txt",
		},
		{
			"custom_layout",
			"/srv/apps/bob/myapp",
			"touch /srv/apps/bob/myapp/tmp/restart.txt",
		},
		{
			"strips_embedded_single_quote",
			"/home/eve'sploit/nodejs/app",
			"touch /home/evesploit/nodejs/app/tmp/restart.txt",
		},
		{
			"trims_whitespace",
			"  /home/alice/app  ",
			"touch /home/alice/app/tmp/restart.txt",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := cpanel.WorkflowRestartCommand(tc.appPath)
			if got != tc.expected {
				t.Fatalf("got %q, want %q", got, tc.expected)
			}
			if err := wizard.ValidateWorkflowField("restart_command", got); err != nil {
				t.Fatalf("rendered command rejected by wizard.ValidateWorkflowField: %v", err)
			}
		})
	}
}

func TestWorkflowRsyncExcludes_MentionsCriticalEntries(t *testing.T) {
	t.Parallel()
	got := cpanel.WorkflowRsyncExcludes()

	mustHave := []string{"tmp/", ".env", "node_modules/", ".htaccess"}
	for _, want := range mustHave {
		found := false
		for _, entry := range got {
			if entry == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("excludes missing %q (got %v)", want, got)
		}
	}

	// Defence-in-depth — every entry must pass the wizard's
	// own workflow-field validator so the wizard cannot reject
	// our defaults at render time.
	for _, entry := range got {
		if err := wizard.ValidateWorkflowField("rsync_exclude", entry); err != nil {
			t.Fatalf("exclude %q rejected by validator: %v", entry, err)
		}
	}

	// Helper must return a fresh slice — guard against callers
	// silently mutating package state.
	first := cpanel.WorkflowRsyncExcludes()
	second := cpanel.WorkflowRsyncExcludes()
	if &first[0] == &second[0] {
		t.Fatalf("WorkflowRsyncExcludes returned a shared underlying array")
	}
}

func TestProvider_PrepareWorkflowData_HappyPath(t *testing.T) {
	t.Parallel()
	p := newProvider(t, nil, nil, nil, nil)

	deployPath, restartCmd, excludes, err := p.PrepareWorkflowData("shop.example.com", []string{"uploads/", "tmp/" /* duplicate */})
	if err != nil {
		t.Fatalf("PrepareWorkflowData: %v", err)
	}
	if deployPath != "/home/alice/nodejs/shop-example-com/public" {
		t.Fatalf("deployPath = %q", deployPath)
	}
	if !strings.Contains(restartCmd, "/home/alice/nodejs/shop-example-com/tmp/restart.txt") {
		t.Fatalf("restartCmd missing app root path: %q", restartCmd)
	}
	// User extra `tmp/` must dedupe with the default; `uploads/`
	// must be present in the final list.
	tmpCount := 0
	uploadsFound := false
	for _, e := range excludes {
		if e == "tmp/" {
			tmpCount++
		}
		if e == "uploads/" {
			uploadsFound = true
		}
	}
	if tmpCount != 1 {
		t.Fatalf("expected tmp/ to appear exactly once, got %d", tmpCount)
	}
	if !uploadsFound {
		t.Fatal("expected operator's uploads/ exclude in final list")
	}
}

func TestProvider_PrepareWorkflowData_InvalidDomain(t *testing.T) {
	t.Parallel()
	p := newProvider(t, nil, nil, nil, nil)

	_, _, _, err := p.PrepareWorkflowData("INVALID DOMAIN", nil)
	if !errors.Is(err, wizard.ErrInvalidPlan) {
		t.Fatalf("expected ErrInvalidPlan, got %v", err)
	}
}

func TestProvider_PrepareWorkflowData_RespectsCustomTemplates(t *testing.T) {
	t.Parallel()
	p := newProvider(t, map[string]string{
		"app_root_template":    "/srv/{user}/{app_root}",
		"deploy_path_template": "/srv/{user}/{app_root}/dist",
	}, nil, nil, nil)

	deployPath, restartCmd, _, err := p.PrepareWorkflowData("shop.example.com", nil)
	if err != nil {
		t.Fatalf("PrepareWorkflowData: %v", err)
	}
	if deployPath != "/srv/alice/shop-example-com/dist" {
		t.Fatalf("deployPath = %q, want custom template applied", deployPath)
	}
	if !strings.Contains(restartCmd, "/srv/alice/shop-example-com/tmp/restart.txt") {
		t.Fatalf("restartCmd uses wrong app root: %q", restartCmd)
	}
}
