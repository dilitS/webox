package tui

import (
	"testing"
	"time"

	ghsvc "github.com/dilitS/webox/services/github"
)

func TestParseRepoRef(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in       string
		wantRef  ghsvc.RepoRef
		wantBool bool
	}{
		{"demo/app", ghsvc.RepoRef{Owner: "demo", Name: "app"}, true},
		{"  demo/app  ", ghsvc.RepoRef{Owner: "demo", Name: "app"}, true},
		{"missing-slash", ghsvc.RepoRef{}, false},
		{"/no-owner", ghsvc.RepoRef{}, false},
		{"no-name/", ghsvc.RepoRef{}, false},
		{"", ghsvc.RepoRef{}, false},
	}
	for _, tt := range cases {
		ref, ok := parseRepoRef(tt.in)
		if ok != tt.wantBool {
			t.Errorf("parseRepoRef(%q) ok = %v, want %v", tt.in, ok, tt.wantBool)
		}
		if ref != tt.wantRef {
			t.Errorf("parseRepoRef(%q) ref = %+v, want %+v", tt.in, ref, tt.wantRef)
		}
	}
}

func TestHumanizeAge(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   time.Duration
		want string
	}{
		{-time.Second, "just now"},
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{2 * time.Hour, "2h ago"},
		{50 * time.Hour, "2d ago"},
		{60 * 24 * time.Hour, "2mo ago"},
	}
	for _, tt := range cases {
		if got := humanizeAge(tt.in); got != tt.want {
			t.Errorf("humanizeAge(%s) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatLastDeploy(t *testing.T) {
	t.Parallel()

	if got := formatLastDeploy(nil); got != lastDeployNoRun {
		t.Errorf("nil run = %q, want %q", got, lastDeployNoRun)
	}

	run := &ghsvc.WorkflowRun{
		Status:     "completed",
		Conclusion: "success",
		UpdatedAt:  time.Now().Add(-10 * time.Minute),
	}
	if got := formatLastDeploy(run); got == "" || got == lastDeployNoRun {
		t.Errorf("success run formatted to %q", got)
	}

	failure := &ghsvc.WorkflowRun{Status: "completed", Conclusion: "failure", UpdatedAt: time.Now().Add(-time.Hour)}
	if got := formatLastDeploy(failure); got == "" {
		t.Errorf("failure run formatted to empty")
	}

	queued := &ghsvc.WorkflowRun{Status: "queued", UpdatedAt: time.Now().Add(-5 * time.Minute)}
	if got := formatLastDeploy(queued); got == "" {
		t.Errorf("queued run formatted to empty")
	}
}

func TestStackForImportType(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"nodejs":  "node-express",
		"Node":    "node-express",
		"node.js": "node-express",
		"static":  "static",
		"html":    "static",
		"":        "",
		"php":     "",
	}
	for in, want := range cases {
		if got := stackForImportType(in); got != want {
			t.Errorf("stackForImportType(%q) = %q, want %q", in, got, want)
		}
	}
}
