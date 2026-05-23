package doctor_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dilitS/webox/secrets"
	"github.com/dilitS/webox/services/doctor"
)

func TestGitHubDoctor_AllGreen(t *testing.T) {
	t.Parallel()

	deps := doctor.GitHubDeps{
		LookupGH: func(context.Context) (string, error) { return "/usr/local/bin/gh", nil },
		AuthStatus: func(context.Context) (string, error) {
			return "github.com\n  Logged in to github.com account demo-user (keyring)\n  Active account: true\n  Token scopes: 'repo', 'workflow'", nil
		},
		RateLimit: func(context.Context) (*doctor.GitHubRateLimit, error) {
			return &doctor.GitHubRateLimit{
				Limit:     5000,
				Remaining: 4982,
				Reset:     time.Now().Add(1 * time.Hour).Unix(),
			}, nil
		},
		PATPresent: func() (bool, error) { return true, nil },
	}

	d := doctor.NewGitHub(deps, doctor.Options{Now: func() time.Time { return time.Date(2026, 5, 23, 5, 0, 0, 0, time.UTC) }})
	report := d.Run(context.Background())

	if report.Summary.Fail != 0 {
		t.Fatalf("expected zero fail, got %+v", report.Summary)
	}
	want := []string{
		"github.gh_cli_available",
		"github.gh_auth_status",
		"github.api_rate_limit",
		"github.pat_keyring_slot",
	}
	for _, id := range want {
		if !hasCheck(report.Checks, id) {
			t.Fatalf("missing check %q in report: %#v", id, report.Checks)
		}
	}
}

func TestGitHubDoctor_GHMissingDoesNotFailHard(t *testing.T) {
	t.Parallel()

	deps := doctor.GitHubDeps{
		LookupGH: func(context.Context) (string, error) { return "", errors.New("gh: command not found") },
		AuthStatus: func(context.Context) (string, error) {
			return "", errors.New("gh: command not found")
		},
		RateLimit:  func(context.Context) (*doctor.GitHubRateLimit, error) { return nil, errors.New("offline") },
		PATPresent: func() (bool, error) { return false, secrets.ErrKeyringUnavailable },
	}
	d := doctor.NewGitHub(deps, doctor.Options{})
	report := d.Run(context.Background())

	if report.Summary.Fail != 0 {
		t.Fatalf("missing gh should produce warns, not fails: %+v", report.Summary)
	}
	if report.Summary.Warn < 2 {
		t.Fatalf("expected at least 2 warns, got %+v", report.Summary)
	}
}

func TestGitHubDoctor_RateLimitNearExhaustionWarns(t *testing.T) {
	t.Parallel()

	deps := doctor.GitHubDeps{
		LookupGH:   func(context.Context) (string, error) { return "/opt/gh", nil },
		AuthStatus: func(context.Context) (string, error) { return "Logged in to github.com account demo", nil },
		RateLimit: func(context.Context) (*doctor.GitHubRateLimit, error) {
			return &doctor.GitHubRateLimit{Limit: 5000, Remaining: 50, Reset: time.Now().Add(45 * time.Minute).Unix()}, nil
		},
		PATPresent: func() (bool, error) { return true, nil },
	}
	d := doctor.NewGitHub(deps, doctor.Options{})
	report := d.Run(context.Background())

	rate := pickCheck(report.Checks, "github.api_rate_limit")
	if rate == nil {
		t.Fatal("rate-limit check missing")
	}
	if rate.Status != doctor.StatusWarn {
		t.Fatalf("rate-limit status = %s, want warn", rate.Status)
	}
}

func TestGitHubDoctor_AuthStatusRedactsPATValues(t *testing.T) {
	t.Parallel()

	noisy := "github.com\n  Token: ghp_" + strings.Repeat("Z", 40) + "\n  Active account: true"
	deps := doctor.GitHubDeps{
		LookupGH:   func(context.Context) (string, error) { return "/opt/gh", nil },
		AuthStatus: func(context.Context) (string, error) { return noisy, nil },
		RateLimit: func(context.Context) (*doctor.GitHubRateLimit, error) {
			return &doctor.GitHubRateLimit{Limit: 100, Remaining: 99}, nil
		},
		PATPresent: func() (bool, error) { return true, nil },
	}
	d := doctor.NewGitHub(deps, doctor.Options{})
	report := d.Run(context.Background())

	auth := pickCheck(report.Checks, "github.gh_auth_status")
	if auth == nil {
		t.Fatal("auth check missing")
	}
	if strings.Contains(auth.Message+" "+auth.Hint, "ghp_") {
		t.Fatalf("doctor leaked the PAT into the report: %s / %s", auth.Message, auth.Hint)
	}
}

func TestGitHubDoctor_JSONIsStableAndDoesNotLeakSecrets(t *testing.T) {
	t.Parallel()

	deps := doctor.GitHubDeps{
		LookupGH: func(context.Context) (string, error) { return "/opt/gh", nil },
		AuthStatus: func(context.Context) (string, error) {
			return "Logged in to github.com account demo (keyring)\n  Token: ghp_" + strings.Repeat("A", 40), nil
		},
		RateLimit: func(context.Context) (*doctor.GitHubRateLimit, error) {
			return &doctor.GitHubRateLimit{Limit: 5000, Remaining: 4900}, nil
		},
		PATPresent: func() (bool, error) { return true, nil },
	}
	d := doctor.NewGitHub(deps, doctor.Options{Now: func() time.Time { return time.Unix(0, 0).UTC() }})
	report := d.Run(context.Background())

	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"ghp_", "ghs_", "github_pat_", "PRIVATE KEY"} {
		if strings.Contains(string(raw), banned) {
			t.Fatalf("JSON contains forbidden substring %q:\n%s", banned, raw)
		}
	}
}

// TestGitHubDoctor_AuthStatus_HandlesCheckmarkPrefix is a regression
// test for gh 2.40+ output, which prefixes the status line with a
// `✓ ` glyph. Without the stripStatusGlyph helper, the parser would
// silently drop the line and downgrade the doctor result to WARN
// despite the user being properly authenticated.
func TestGitHubDoctor_AuthStatus_HandlesCheckmarkPrefix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "gh 2.40+ checkmark",
			raw:  "github.com\n  ✓ Logged in to github.com account dilitS (keyring)\n  - Active account: true",
			want: "dilitS",
		},
		{
			name: "gh 2.40+ failure glyph still surfaces login line",
			raw:  "github.com\n  x Logged in to github.com account stale (oauth_token)",
			want: "stale",
		},
		{
			name: "legacy gh ≤2.39 (as <user>)",
			raw:  "  Logged in to github.com as alice (oauth_token)",
			want: "alice",
		},
		{
			name: "no login line",
			raw:  "github.com\n  not authenticated",
			want: "",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deps := doctor.GitHubDeps{
				LookupGH:   func(context.Context) (string, error) { return "/opt/gh", nil },
				AuthStatus: func(context.Context) (string, error) { return tc.raw, nil },
				RateLimit: func(context.Context) (*doctor.GitHubRateLimit, error) {
					return &doctor.GitHubRateLimit{Limit: 100, Remaining: 99}, nil
				},
				PATPresent: func() (bool, error) { return true, nil },
			}
			d := doctor.NewGitHub(deps, doctor.Options{})
			report := d.Run(context.Background())
			auth := pickCheck(report.Checks, "github.gh_auth_status")
			if auth == nil {
				t.Fatal("auth check missing")
			}
			if tc.want == "" {
				if auth.Status != doctor.StatusWarn {
					t.Fatalf("expected WARN when no account parsed, got %s", auth.Status)
				}
				return
			}
			if auth.Status != doctor.StatusOK {
				t.Fatalf("expected OK status for parsed account, got %s (msg: %s)", auth.Status, auth.Message)
			}
			if !strings.Contains(auth.Message, tc.want) {
				t.Fatalf("message %q does not contain %q", auth.Message, tc.want)
			}
		})
	}
}

func hasCheck(checks []doctor.Result, id string) bool {
	for _, c := range checks {
		if c.ID == id {
			return true
		}
	}
	return false
}

func pickCheck(checks []doctor.Result, id string) *doctor.Result {
	for i := range checks {
		if checks[i].ID == id {
			return &checks[i]
		}
	}
	return nil
}
