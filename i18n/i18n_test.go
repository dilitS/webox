package i18n

import "testing"

func TestT_DefaultCatalogUsesEnglish(t *testing.T) {
	t.Parallel()

	if got := T("doctor.title"); got != "webox doctor" {
		t.Fatalf("T(doctor.title) = %q, want %q", got, "webox doctor")
	}
}

func TestCatalogT(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		language string
		key      string
		args     []any
		want     string
	}{
		{
			name:     "polish summary",
			language: "pl",
			key:      "doctor.summary",
			args:     []any{1, 2, 3, 4},
			want:     "podsumowanie: 1 ok, 2 ostrzeżeń, 3 błędów, 4 pominiętych",
		},
		{
			name:     "english fallback for unsupported language",
			language: "de",
			key:      "doctor.ssh_agent_missing",
			want:     "SSH_AUTH_SOCK is not set.",
		},
		{
			name:     "unknown key fails soft",
			language: "en",
			key:      "doctor.unknown",
			want:     "doctor.unknown",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := New(tt.language).T(tt.key, tt.args...)
			if got != tt.want {
				t.Fatalf("New(%q).T(%q) = %q, want %q", tt.language, tt.key, got, tt.want)
			}
		})
	}
}
