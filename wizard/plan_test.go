package wizard_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/dilitS/webox/providers"
	_ "github.com/dilitS/webox/providers/smallhost" // register smallhost validators for ValidatePlan tests
	"github.com/dilitS/webox/wizard"
)

// smallhostValidators is the registered validator set used by every
// test in this file. Resolved once via the registry so the test
// stays decoupled from the smallhost package while still exercising
// the real production validators.
func smallhostValidators(t *testing.T) providers.PlanValidators {
	t.Helper()
	set, err := providers.PlanValidatorsFor("smallhost")
	if err != nil {
		t.Fatalf("smallhost validators not registered: %v", err)
	}
	return set
}

func TestIsValidStack(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		in   string
		want bool
	}{
		{"static", wizard.StackStatic, true},
		{"vite-react", wizard.StackViteReact, true},
		{"node-express", wizard.StackNodeExpress, true},
		{"empty", "", false},
		{"nextjs", "nextjs", false},
		{"injection", "static;rm -rf /", false},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := wizard.IsValidStack(tt.in); got != tt.want {
				t.Fatalf("IsValidStack(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsValidDBKind(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		in   string
		want bool
	}{
		{"mysql", providers.DatabaseMySQL, true},
		{"postgres", providers.DatabasePostgres, true},
		{"empty", "", false},
		{"sqlite", "sqlite", false},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := wizard.IsValidDBKind(tt.in); got != tt.want {
				t.Fatalf("IsValidDBKind(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsDBRequiredForStack(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		stack string
		want  bool
	}{
		{wizard.StackStatic, false},
		{wizard.StackViteReact, false},
		{wizard.StackNodeExpress, true},
		{"unknown", false},
	} {
		tt := tt
		t.Run(tt.stack, func(t *testing.T) {
			t.Parallel()
			if got := wizard.IsDBRequiredForStack(tt.stack); got != tt.want {
				t.Fatalf("IsDBRequiredForStack(%q) = %v, want %v", tt.stack, got, tt.want)
			}
		})
	}
}

func TestDefaultNodeVersion(t *testing.T) {
	t.Parallel()

	if got := wizard.DefaultNodeVersion(wizard.StackNodeExpress); got == "" {
		t.Fatalf("DefaultNodeVersion(node-express) = empty, want a default")
	}
	if got := wizard.DefaultNodeVersion("nonexistent"); got != "" {
		t.Fatalf("DefaultNodeVersion(unknown) = %q, want empty", got)
	}
}

func TestValidatePlan(t *testing.T) {
	t.Parallel()

	base := wizard.ProvisionPlan{
		ProfileAlias: "main",
		Stack:        wizard.StackNodeExpress,
		Domain:       "app.demo.smallhost.pl",
		NodeVersion:  "22",
		DBKind:       providers.DatabaseMySQL,
		DBName:       "app_main",
	}

	cases := []struct {
		name    string
		mutate  func(*wizard.ProvisionPlan)
		wantErr bool
		wantSub string
	}{
		{name: "happy node-express + mysql", mutate: func(*wizard.ProvisionPlan) {}, wantErr: false},
		{
			name: "static stack no db",
			mutate: func(p *wizard.ProvisionPlan) {
				p.Stack = wizard.StackStatic
				p.DBKind = ""
				p.DBName = ""
			},
			wantErr: false,
		},
		{
			name:    "missing profile",
			mutate:  func(p *wizard.ProvisionPlan) { p.ProfileAlias = "" },
			wantErr: true,
			wantSub: "profile_alias",
		},
		{
			name:    "invalid stack",
			mutate:  func(p *wizard.ProvisionPlan) { p.Stack = "bun" },
			wantErr: true,
			wantSub: "stack",
		},
		{
			name:    "invalid domain",
			mutate:  func(p *wizard.ProvisionPlan) { p.Domain = "BadDomain.." },
			wantErr: true,
			wantSub: "invalid domain",
		},
		{
			name:    "missing node version",
			mutate:  func(p *wizard.ProvisionPlan) { p.NodeVersion = "" },
			wantErr: true,
			wantSub: "node_version",
		},
		{
			name:    "invalid node version",
			mutate:  func(p *wizard.ProvisionPlan) { p.NodeVersion = "$(reboot)" },
			wantErr: true,
			wantSub: "invalid node version",
		},
		{
			name:    "db kind without name",
			mutate:  func(p *wizard.ProvisionPlan) { p.DBName = "" },
			wantErr: true,
			wantSub: "must be set together",
		},
		{
			name:    "db name without kind",
			mutate:  func(p *wizard.ProvisionPlan) { p.DBKind = "" },
			wantErr: true,
			wantSub: "must be set together",
		},
		{
			name:    "invalid db kind",
			mutate:  func(p *wizard.ProvisionPlan) { p.DBKind = "redis" },
			wantErr: true,
			wantSub: "db_kind",
		},
		{
			name:    "invalid db name",
			mutate:  func(p *wizard.ProvisionPlan) { p.DBName = "Bad-Name" },
			wantErr: true,
			wantSub: "invalid database name",
		},
	}

	validators := smallhostValidators(t)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plan := base
			tc.mutate(&plan)
			err := wizard.ValidatePlan(plan, validators)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ValidatePlan(%+v) = nil, want error", plan)
				}
				if !errors.Is(err, wizard.ErrInvalidPlan) {
					t.Fatalf("err = %v, want wrapped ErrInvalidPlan", err)
				}
				if tc.wantSub != "" && !strings.Contains(err.Error(), tc.wantSub) {
					t.Fatalf("err = %v, want substring %q", err, tc.wantSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidatePlan(%+v) = %v, want nil", plan, err)
			}
		})
	}
}

func TestValidatePlan_RequiresCompleteValidatorSet(t *testing.T) {
	t.Parallel()

	plan := wizard.ProvisionPlan{
		ProfileAlias: "main",
		Stack:        wizard.StackNodeExpress,
		Domain:       "app.demo.smallhost.pl",
		NodeVersion:  "22",
	}
	err := wizard.ValidatePlan(plan, providers.PlanValidators{})
	if !errors.Is(err, wizard.ErrInvalidPlan) {
		t.Fatalf("err = %v, want wrapped ErrInvalidPlan", err)
	}
	if !strings.Contains(err.Error(), "validator set is incomplete") {
		t.Fatalf("err = %v, want incomplete-validator-set message", err)
	}
}

func TestSupportedStacksAlignedWithDefaults(t *testing.T) {
	t.Parallel()
	for _, stack := range wizard.SupportedStacks {
		if got := wizard.DefaultNodeVersion(stack); got == "" {
			t.Errorf("stack %q missing DefaultNodeVersion entry", stack)
		}
	}
}
