package version_test

import (
	"testing"

	"github.com/dilitS/webox/internal/version"
)

func TestFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		commit  string
		date    string
		want    string
	}{
		{
			name:    "all fields populated",
			version: "v0.1.0",
			commit:  "abc1234",
			date:    "2026-05-22T10:00:00Z",
			want:    "webox v0.1.0 (abc1234) built 2026-05-22T10:00:00Z",
		},
		{
			name:   "empty version falls back to v0.0.0-dev",
			commit: "abc1234",
			date:   "2026-05-22T10:00:00Z",
			want:   "webox v0.0.0-dev (abc1234) built 2026-05-22T10:00:00Z",
		},
		{
			name:    "empty commit falls back to unknown",
			version: "v0.1.0",
			date:    "2026-05-22T10:00:00Z",
			want:    "webox v0.1.0 (unknown) built 2026-05-22T10:00:00Z",
		},
		{
			name:    "empty date falls back to unknown",
			version: "v0.1.0",
			commit:  "abc1234",
			want:    "webox v0.1.0 (abc1234) built unknown",
		},
		{
			name: "all fields empty falls back to all defaults",
			want: "webox v0.0.0-dev (unknown) built unknown",
		},
		{
			name:    "long-form commit hash is preserved verbatim",
			version: "v0.2.0-rc1",
			commit:  "0123456789abcdef0123456789abcdef01234567",
			date:    "2026-06-01T08:30:00Z",
			want:    "webox v0.2.0-rc1 (0123456789abcdef0123456789abcdef01234567) built 2026-06-01T08:30:00Z",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := version.Format(tt.version, tt.commit, tt.date)
			if got != tt.want {
				t.Errorf("Format(%q, %q, %q)\n  got = %q\n want = %q", tt.version, tt.commit, tt.date, got, tt.want)
			}
		})
	}
}

func TestString_UsesPackageVars(t *testing.T) {
	saveV, saveC, saveD := version.Version, version.Commit, version.Date
	t.Cleanup(func() {
		version.Version = saveV
		version.Commit = saveC
		version.Date = saveD
	})

	version.Version = "v9.9.9"
	version.Commit = "deadbee"
	version.Date = "2099-12-31T23:59:59Z"

	want := "webox v9.9.9 (deadbee) built 2099-12-31T23:59:59Z"
	if got := version.String(); got != want {
		t.Errorf("String()\n  got = %q\n want = %q", got, want)
	}
}

func TestString_AppliesDefaultsWhenLdflagsUnset(t *testing.T) {
	saveV, saveC, saveD := version.Version, version.Commit, version.Date
	t.Cleanup(func() {
		version.Version = saveV
		version.Commit = saveC
		version.Date = saveD
	})

	version.Version = ""
	version.Commit = ""
	version.Date = ""

	want := "webox v0.0.0-dev (unknown) built unknown"
	if got := version.String(); got != want {
		t.Errorf("String()\n  got = %q\n want = %q", got, want)
	}
}
