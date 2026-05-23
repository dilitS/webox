package providers_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dilitS/webox/providers"
)

// stubProvider is a no-op HostingProvider used only to assert that the
// registry hands us back the exact instance the factory returned. It
// is defined per-test instead of in a shared helper because every test
// wants to assert on a different concrete value (factory error,
// pointer identity, …) and a shared helper would muddy the signal.
type stubProvider struct {
	name string
	cfg  providers.ProviderConfig
}

func (s *stubProvider) Name() string { return s.name }

func (s *stubProvider) CreateSubdomain(context.Context, string, string) error { return nil }

func (s *stubProvider) SetupSSL(context.Context, string) error { return nil }

func (s *stubProvider) CreateDatabase(context.Context, string, string) (user, password string, err error) {
	return "", "", nil
}

func (s *stubProvider) RestartNodeApp(context.Context, string) error { return nil }

func (s *stubProvider) GetDeployPath(string) string { return "" }

func (s *stubProvider) GetLogPath(string) string { return "" }

func (s *stubProvider) CheckStatus(context.Context) (*providers.ProviderStatus, error) {
	return nil, nil
}

func (s *stubProvider) ListSubdomains(context.Context) ([]providers.Subdomain, error) {
	return nil, nil
}

func (s *stubProvider) RemoveSubdomain(context.Context, string) error { return nil }

func (s *stubProvider) RemoveDatabase(context.Context, string, string) error { return nil }

func (s *stubProvider) RemoveSSL(context.Context, string) error { return nil }

// registerOnce is a tiny test helper that registers a factory and
// schedules its removal at the end of the test. Because Register lives
// on a package-level singleton, every test that calls it MUST clean up
// — leaking entries between tests would defeat the duplicate-name
// guardrail we are about to verify.
func registerOnce(t *testing.T, name string, factory providers.Factory) {
	t.Helper()
	if err := providers.Register(name, factory); err != nil {
		t.Fatalf("Register(%q): %v", name, err)
	}
	t.Cleanup(func() { providers.Unregister(name) })
}

func TestRegister_RejectsEmptyType(t *testing.T) {
	err := providers.Register("", func(providers.ProviderConfig) (providers.HostingProvider, error) {
		return &stubProvider{}, nil
	})
	if !errors.Is(err, providers.ErrInvalidProviderConfig) {
		t.Fatalf("err = %v, want wrap of ErrInvalidProviderConfig", err)
	}
}

func TestRegister_RejectsNilFactory(t *testing.T) {
	err := providers.Register("smallhost-test", nil)
	if !errors.Is(err, providers.ErrInvalidProviderConfig) {
		t.Fatalf("err = %v, want wrap of ErrInvalidProviderConfig", err)
	}
}

func TestRegister_RejectsDuplicate(t *testing.T) {
	registerOnce(t, "stub-dup", func(providers.ProviderConfig) (providers.HostingProvider, error) {
		return &stubProvider{name: "stub-dup"}, nil
	})

	err := providers.Register("stub-dup", func(providers.ProviderConfig) (providers.HostingProvider, error) {
		return nil, nil
	})
	if !errors.Is(err, providers.ErrProviderAlreadyRegistered) {
		t.Fatalf("err = %v, want wrap of ErrProviderAlreadyRegistered", err)
	}
}

func TestUnregister_IsIdempotent(t *testing.T) {
	registerOnce(t, "stub-unreg", func(providers.ProviderConfig) (providers.HostingProvider, error) {
		return &stubProvider{name: "stub-unreg"}, nil
	})

	if !providers.Unregister("stub-unreg") {
		t.Fatalf("Unregister returned false for known type")
	}
	if providers.Unregister("stub-unreg") {
		t.Fatalf("Unregister returned true for already-unregistered type")
	}
}

func TestNames_ReturnsSortedSnapshot(t *testing.T) {
	registerOnce(t, "stub-zeta", func(providers.ProviderConfig) (providers.HostingProvider, error) {
		return &stubProvider{name: "stub-zeta"}, nil
	})
	registerOnce(t, "stub-alpha", func(providers.ProviderConfig) (providers.HostingProvider, error) {
		return &stubProvider{name: "stub-alpha"}, nil
	})

	got := providers.Names()
	var saw [2]bool
	prev := ""
	for _, n := range got {
		if n == "stub-alpha" {
			saw[0] = true
		}
		if n == "stub-zeta" {
			saw[1] = true
		}
		if prev != "" && n < prev {
			t.Fatalf("Names() not sorted: %v", got)
		}
		prev = n
	}
	if !saw[0] || !saw[1] {
		t.Fatalf("Names() missing expected entries: %v", got)
	}
}

func TestNew_RejectsEmptyType(t *testing.T) {
	_, err := providers.New(providers.ProviderConfig{Alias: "main", Host: "h", User: "u"})
	if !errors.Is(err, providers.ErrInvalidProviderConfig) {
		t.Fatalf("err = %v, want ErrInvalidProviderConfig", err)
	}
}

func TestNew_RejectsUnknownType(t *testing.T) {
	_, err := providers.New(providers.ProviderConfig{
		Type:  "definitely-not-registered",
		Alias: "main",
		Host:  "h",
		User:  "u",
	})
	if !errors.Is(err, providers.ErrUnknownProvider) {
		t.Fatalf("err = %v, want ErrUnknownProvider", err)
	}
}

func TestNew_ValidatesInvariants(t *testing.T) {
	registerOnce(t, "stub-val", func(cfg providers.ProviderConfig) (providers.HostingProvider, error) {
		return &stubProvider{name: cfg.Type, cfg: cfg}, nil
	})

	tests := []struct {
		name string
		cfg  providers.ProviderConfig
	}{
		{"missing alias", providers.ProviderConfig{Type: "stub-val", Host: "h", User: "u"}},
		{"bad alias chars", providers.ProviderConfig{Type: "stub-val", Alias: "Bad Alias", Host: "h", User: "u"}},
		{"missing host", providers.ProviderConfig{Type: "stub-val", Alias: "main", User: "u"}},
		{"missing user", providers.ProviderConfig{Type: "stub-val", Alias: "main", Host: "h"}},
		{"negative port", providers.ProviderConfig{Type: "stub-val", Alias: "main", Host: "h", User: "u", Port: -1}},
		{"port too high", providers.ProviderConfig{Type: "stub-val", Alias: "main", Host: "h", User: "u", Port: 70000}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := providers.New(tc.cfg)
			if !errors.Is(err, providers.ErrInvalidProviderConfig) {
				t.Fatalf("err = %v, want wrap of ErrInvalidProviderConfig", err)
			}
		})
	}
}

func TestNew_NormalisesDefaults(t *testing.T) {
	var captured providers.ProviderConfig
	registerOnce(t, "stub-norm", func(cfg providers.ProviderConfig) (providers.HostingProvider, error) {
		captured = cfg
		return &stubProvider{name: cfg.Type, cfg: cfg}, nil
	})

	provider, err := providers.New(providers.ProviderConfig{
		Type:  "stub-norm",
		Alias: "main",
		Host:  "h",
		User:  "u",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if provider == nil {
		t.Fatalf("provider is nil")
	}
	if captured.Port != providers.DefaultSSHPort {
		t.Errorf("Port = %d, want %d", captured.Port, providers.DefaultSSHPort)
	}
	if captured.Properties == nil {
		t.Error("Properties is nil; want empty map")
	}
}

func TestNew_PropagatesFactoryError(t *testing.T) {
	sentinel := errors.New("synthetic factory failure")
	registerOnce(t, "stub-factory-err", func(providers.ProviderConfig) (providers.HostingProvider, error) {
		return nil, sentinel
	})

	_, err := providers.New(providers.ProviderConfig{
		Type:  "stub-factory-err",
		Alias: "main",
		Host:  "h",
		User:  "u",
	})
	if err == nil {
		t.Fatalf("err is nil, want %v", sentinel)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want wrap of %v", err, sentinel)
	}
}

// TestHostingProviderShape pins the canonical method set so an
// accidental signature change is caught at compile time by the
// reflect-based assertion below. The same assertion documents the
// stable surface for tooling such as docs/DESIGN.md §3.
func TestHostingProviderShape(t *testing.T) {
	var hp providers.HostingProvider = &stubProvider{}
	if hp.Name() != "" {
		t.Fatal("stubProvider should return empty name in shape test")
	}

	want := []string{
		"Name",
		"CreateSubdomain",
		"SetupSSL",
		"CreateDatabase",
		"RestartNodeApp",
		"GetDeployPath",
		"GetLogPath",
		"CheckStatus",
		"ListSubdomains",
		"RemoveSubdomain",
		"RemoveDatabase",
		"RemoveSSL",
	}

	got := make([]string, 0, len(want))
	tp := reflect.TypeOf(hp)
	for i := range tp.NumMethod() {
		got = append(got, tp.Method(i).Name)
	}
	missing := make([]string, 0)
	for _, m := range want {
		if _, ok := tp.MethodByName(m); !ok {
			missing = append(missing, m)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("HostingProvider missing methods: %v (have: %v)", missing, got)
	}
}

// Smoke check: ensure error message includes provider name when factory
// fails, so operator logs can identify the offending adapter without
// requiring a debugger.
func TestNew_ErrorMessageMentionsProviderName(t *testing.T) {
	want := "synthetic"
	registerOnce(t, "stub-err-msg", func(providers.ProviderConfig) (providers.HostingProvider, error) {
		return nil, errors.New(want)
	})

	_, err := providers.New(providers.ProviderConfig{
		Type:  "stub-err-msg",
		Alias: "main",
		Host:  "h",
		User:  "u",
	})
	if err == nil {
		t.Fatalf("err is nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "stub-err-msg") {
		t.Errorf("error %q should mention provider type", msg)
	}
	if !strings.Contains(msg, want) {
		t.Errorf("error %q should include factory error %q", msg, want)
	}
}
