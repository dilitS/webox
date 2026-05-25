package presets_test

import (
	"slices"
	"testing"

	"github.com/dilitS/webox/presets"
)

func TestStatusValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   presets.Status
		want bool
	}{
		{"research", presets.StatusResearch, true},
		{"candidate", presets.StatusCandidate, true},
		{"verified", presets.StatusVerified, true},
		{"deprecated", presets.StatusDeprecated, true},
		{"community", presets.StatusCommunity, true},
		{"empty", "", false},
		{"unknown", "experimental", false},
		{"uppercase", "VERIFIED", false},
		{"surrounded", " verified ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.in.Valid(); got != tt.want {
				t.Fatalf("Status(%q).Valid() = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestAllStatusesIsClosedSetAndOrdered(t *testing.T) {
	t.Parallel()

	statuses := presets.AllStatuses()
	want := []presets.Status{
		presets.StatusResearch,
		presets.StatusCandidate,
		presets.StatusVerified,
		presets.StatusDeprecated,
		presets.StatusCommunity,
	}
	if !slices.Equal(statuses, want) {
		t.Fatalf("AllStatuses() = %v, want %v", statuses, want)
	}
}

func TestAllPanelAPIsIsClosedSet(t *testing.T) {
	t.Parallel()

	want := []presets.PanelAPI{
		presets.PanelAPIUAPI,
		presets.PanelAPIDirectAdminAPI,
		presets.PanelAPICyberPanelCLI,
		presets.PanelAPIDevilCLI,
		presets.PanelAPISSHOnly,
		presets.PanelAPINone,
	}
	if got := presets.AllPanelAPIs(); !slices.Equal(got, want) {
		t.Fatalf("AllPanelAPIs() mismatch:\n got %v\nwant %v", got, want)
	}
}

func TestPresetRegion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		markets []string
		want    string
	}{
		{"empty markets falls to advanced", nil, presets.RegionAdvanced},
		{"PL is Poland", []string{"PL"}, presets.RegionPoland},
		{"global tag is Global", []string{"global"}, presets.RegionGlobal},
		{"DE is Europe", []string{"DE"}, presets.RegionEurope},
		{"UK is Europe", []string{"UK"}, presets.RegionEurope},
		{"GB is Europe", []string{"GB"}, presets.RegionEurope},
		{"US is Global (not Europe)", []string{"US"}, presets.RegionGlobal},
		{"first wins (PL,DE → Poland)", []string{"PL", "DE"}, presets.RegionPoland},
		{"first wins (DE,PL → Europe)", []string{"DE", "PL"}, presets.RegionEurope},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := presets.Preset{Markets: tt.markets}
			if got := p.Region(); got != tt.want {
				t.Fatalf("Region(%v) = %q, want %q", tt.markets, got, tt.want)
			}
		})
	}
}

func TestCapabilityBadges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   presets.Preset
		want []string
	}{
		{
			name: "fully verified preset",
			in: presets.Preset{
				Panel: presets.Panel{
					API:         presets.PanelAPIUAPI,
					SSHRequired: true,
				},
				Capabilities: presets.Capabilities{
					NodeRuntime:     "passenger",
					SSLProvider:     "autossl",
					DatabaseEngines: []presets.DatabaseEngine{presets.DatabaseMySQL},
					LogsPathKnown:   true,
					SafeRestart:     true,
				},
				Verified: presets.Verified{
					FixtureDir: "testing/fixtures/cpanel/",
				},
			},
			want: []string{"SSH", "API", "Node", "SSL", "DB", "Logs", "Safe Restart", "Fixtures"},
		},
		{
			name: "ssh-only static preset",
			in: presets.Preset{
				Panel: presets.Panel{
					API:         presets.PanelAPISSHOnly,
					SSHRequired: true,
				},
				Capabilities: presets.Capabilities{
					NodeRuntime:     "static_only",
					SSLProvider:     "letsencrypt",
					DatabaseEngines: []presets.DatabaseEngine{presets.DatabaseNone},
				},
			},
			want: []string{"SSH", "SSL"},
		},
		{
			name: "dangerous preset (no safe restart, no fixtures)",
			in: presets.Preset{
				Panel: presets.Panel{
					API:         presets.PanelAPICyberPanelCLI,
					SSHRequired: true,
				},
				Capabilities: presets.Capabilities{
					NodeRuntime:     "manual_pm2",
					SSLProvider:     "letsencrypt",
					DatabaseEngines: []presets.DatabaseEngine{presets.DatabaseMySQL},
				},
			},
			want: []string{"SSH", "API", "Node", "SSL", "DB"},
		},
		{
			name: "unknown runtime hides Node badge",
			in: presets.Preset{
				Panel: presets.Panel{
					API:         presets.PanelAPIUAPI,
					SSHRequired: true,
				},
				Capabilities: presets.Capabilities{
					NodeRuntime:     "unknown",
					SSLProvider:     "letsencrypt",
					DatabaseEngines: []presets.DatabaseEngine{presets.DatabaseMySQL},
				},
			},
			want: []string{"SSH", "API", "SSL", "DB"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.in.CapabilityBadges(); !slices.Equal(got, tt.want) {
				t.Fatalf("CapabilityBadges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCapabilityBadgesIsDeterministic(t *testing.T) {
	t.Parallel()

	p := presets.Preset{
		Panel: presets.Panel{
			API:         presets.PanelAPIUAPI,
			SSHRequired: true,
		},
		Capabilities: presets.Capabilities{
			NodeRuntime:     "passenger",
			SSLProvider:     "autossl",
			DatabaseEngines: []presets.DatabaseEngine{presets.DatabaseMySQL},
			LogsPathKnown:   true,
			SafeRestart:     true,
		},
		Verified: presets.Verified{FixtureDir: "testing/fixtures/cpanel/"},
	}
	first := p.CapabilityBadges()
	for i := 0; i < 64; i++ {
		got := p.CapabilityBadges()
		if !slices.Equal(got, first) {
			t.Fatalf("CapabilityBadges(): non-deterministic order on iter %d:\n got %v\nfirst %v", i, got, first)
		}
	}
}
