package secrets

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestDetectWithClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mock       *mockKeyringClient
		wantOS     bool
		wantErr    error
		wantDelete bool
	}{
		{
			name:       "happy path returns os backend and cleans probe",
			mock:       &mockKeyringClient{},
			wantOS:     true,
			wantDelete: true,
		},
		{
			name: "unsupported platform surfaces keyring unavailable",
			mock: &mockKeyringClient{
				setErr: keyring.ErrUnsupportedPlatform,
			},
			wantErr: ErrKeyringUnavailable,
		},
		{
			name: "not found after successful set is broken keyring",
			mock: &mockKeyringClient{
				getErr: keyring.ErrNotFound,
			},
			wantErr:    ErrBrokenKeyring,
			wantDelete: true,
		},
		{
			name: "transient set error surfaces keyring unavailable",
			mock: &mockKeyringClient{
				setErr: errors.New("dbus unavailable"),
			},
			wantErr: ErrKeyringUnavailable,
		},
		{
			name: "transient get error cleans probe then surfaces keyring unavailable",
			mock: &mockKeyringClient{
				getErr: errors.New("dbus timeout"),
			},
			wantErr:    ErrKeyringUnavailable,
			wantDelete: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			backend, err := detectWithClient(tt.mock)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("detectWithClient() err = %v, want errors.Is(_, %v)", err, tt.wantErr)
				}
				if backend != nil {
					t.Fatalf("detectWithClient() backend = %#v, want nil on error", backend)
				}
			} else if err != nil {
				t.Fatalf("detectWithClient() err = %v, want nil", err)
			}

			if tt.wantOS {
				if _, ok := backend.(*osKeyringBackend); !ok {
					t.Fatalf("detectWithClient() backend = %T, want *osKeyringBackend", backend)
				}
			}
			if tt.mock.deletedProbe != tt.wantDelete {
				t.Fatalf("probe cleanup = %v, want %v", tt.mock.deletedProbe, tt.wantDelete)
			}
		})
	}
}

func TestOSKeyringBackend(t *testing.T) {
	t.Parallel()

	client := &mockKeyringClient{}
	backend := &osKeyringBackend{client: client}

	if err := backend.Set("github-token", []byte("value")); err != nil {
		t.Fatalf("Set() = %v, want nil", err)
	}
	got, err := backend.Get("github-token")
	if err != nil {
		t.Fatalf("Get() = %v, want nil", err)
	}
	if string(got) != "value" {
		t.Fatalf("Get() = %q, want %q", got, "value")
	}
	if err := backend.Delete("github-token"); err != nil {
		t.Fatalf("Delete() = %v, want nil", err)
	}
	if err := backend.Delete("github-token"); err != nil {
		t.Fatalf("Delete() missing key = %v, want nil idempotent delete", err)
	}
}

func TestOSKeyringBackendErrors(t *testing.T) {
	t.Parallel()

	t.Run("get missing maps to secret not found", func(t *testing.T) {
		t.Parallel()

		backend := &osKeyringBackend{client: &mockKeyringClient{}}
		_, err := backend.Get("missing")
		if !errors.Is(err, ErrSecretNotFound) {
			t.Fatalf("Get(missing) = %v, want ErrSecretNotFound", err)
		}
	})

	t.Run("get wraps backend error", func(t *testing.T) {
		t.Parallel()

		backendErr := errors.New("keychain locked")
		backend := &osKeyringBackend{client: &mockKeyringClient{getErr: backendErr}}
		_, err := backend.Get("github-token")
		if !errors.Is(err, backendErr) {
			t.Fatalf("Get(backend error) = %v, want wrapped backend error", err)
		}
	})

	t.Run("set wraps backend error", func(t *testing.T) {
		t.Parallel()

		backendErr := errors.New("write denied")
		backend := &osKeyringBackend{client: &mockKeyringClient{setErr: backendErr}}
		err := backend.Set("github-token", []byte("value"))
		if !errors.Is(err, backendErr) {
			t.Fatalf("Set(backend error) = %v, want wrapped backend error", err)
		}
	})

	t.Run("delete wraps backend error", func(t *testing.T) {
		t.Parallel()

		backendErr := errors.New("delete denied")
		backend := &osKeyringBackend{client: &mockKeyringClient{deleteErr: backendErr}}
		err := backend.Delete("github-token")
		if !errors.Is(err, backendErr) {
			t.Fatalf("Delete(backend error) = %v, want wrapped backend error", err)
		}
	})
}

func TestDetectWithClient_MismatchedProbeValue(t *testing.T) {
	t.Parallel()

	backend, err := detectWithClient(&mockKeyringClient{getValue: "different"})
	if !errors.Is(err, ErrBrokenKeyring) {
		t.Fatalf("detectWithClient(mismatched probe) = %v, want ErrBrokenKeyring", err)
	}
	if backend != nil {
		t.Fatalf("detectWithClient(mismatched probe) backend = %#v, want nil", backend)
	}
}

func TestGoKeyringClientWithMockInit(t *testing.T) {
	keyring.MockInit()

	client := goKeyringClient{}
	if err := client.Set(keyringService, "unit-test", "value"); err != nil {
		t.Fatalf("goKeyringClient.Set() = %v", err)
	}
	got, err := client.Get(keyringService, "unit-test")
	if err != nil {
		t.Fatalf("goKeyringClient.Get() = %v", err)
	}
	if got != "value" {
		t.Fatalf("goKeyringClient.Get() = %q, want value", got)
	}
	if err := client.Delete(keyringService, "unit-test"); err != nil {
		t.Fatalf("goKeyringClient.Delete() = %v", err)
	}
}

func TestDetectWithGoKeyringMock(t *testing.T) {
	keyring.MockInit()

	backend, err := Detect()
	if err != nil {
		t.Fatalf("Detect() with keyring.MockInit() = %v, want nil", err)
	}
	if _, ok := backend.(*osKeyringBackend); !ok {
		t.Fatalf("Detect() backend = %T, want *osKeyringBackend", backend)
	}
}
