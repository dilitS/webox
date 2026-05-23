package ssh

import (
	"errors"
	"net"
	"strings"
	"testing"

	cryptossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestBuildClientConfig_RequiresHostKeyDB(t *testing.T) {
	t.Parallel()

	_, err := BuildClientConfig(Options{
		Target: validTarget(),
		Auth:   []cryptossh.AuthMethod{cryptossh.Password("ignored")},
	})
	if !errors.Is(err, ErrHostKeyDBRequired) {
		t.Fatalf("BuildClientConfig without HostKeyDB err = %v, want errors.Is(_, ErrHostKeyDBRequired)", err)
	}
}

func TestBuildClientConfig_HostKeyAlgorithms(t *testing.T) {
	t.Parallel()

	cfg, err := BuildClientConfig(Options{
		Target:    validTarget(),
		Auth:      []cryptossh.AuthMethod{cryptossh.Password("ignored")},
		HostKeyDB: stubHostKeyDB{},
	})
	if err != nil {
		t.Fatalf("BuildClientConfig: %v", err)
	}

	mustContain := []string{
		cryptossh.KeyAlgoED25519,
		cryptossh.KeyAlgoRSASHA512,
		cryptossh.KeyAlgoRSASHA256,
		cryptossh.KeyAlgoECDSA256,
	}
	for _, algo := range mustContain {
		if !contains(strings.Join(cfg.HostKeyAlgorithms, ","), algo) {
			t.Errorf("HostKeyAlgorithms missing %q, got %v", algo, cfg.HostKeyAlgorithms)
		}
	}

	mustNotContain := []string{
		cryptossh.KeyAlgoRSA, // ssh-rsa (SHA-1)
		dssWireName,          // ssh-dss is deprecated upstream, but we still assert we never list it
	}
	for _, banned := range mustNotContain {
		for _, algo := range cfg.HostKeyAlgorithms {
			if algo == banned {
				t.Errorf("HostKeyAlgorithms must reject %q without legacy compat flag, got %v", banned, cfg.HostKeyAlgorithms)
			}
		}
	}
}

func TestBuildClientConfig_LegacyCompatAddsSSHRSA(t *testing.T) {
	t.Parallel()

	cfg, err := BuildClientConfig(Options{
		Target:                validTarget(),
		Auth:                  []cryptossh.AuthMethod{cryptossh.Password("ignored")},
		HostKeyDB:             stubHostKeyDB{},
		LegacyAlgorithmCompat: true,
	})
	if err != nil {
		t.Fatalf("BuildClientConfig: %v", err)
	}

	found := false
	for _, algo := range cfg.HostKeyAlgorithms {
		if algo == cryptossh.KeyAlgoRSA {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("LegacyAlgorithmCompat=true should add %q to HostKeyAlgorithms, got %v", cryptossh.KeyAlgoRSA, cfg.HostKeyAlgorithms)
	}
}

func TestBuildClientConfig_TimeoutAndUserPropagate(t *testing.T) {
	t.Parallel()

	target := Target{Host: "h.example.com", Port: 22, User: "alice"}
	cfg, err := BuildClientConfig(Options{
		Target:    target,
		Auth:      []cryptossh.AuthMethod{cryptossh.Password("ignored")},
		HostKeyDB: stubHostKeyDB{},
		Timeout:   1234,
	})
	if err != nil {
		t.Fatalf("BuildClientConfig: %v", err)
	}
	if cfg.User != "alice" {
		t.Errorf("cfg.User = %q, want %q", cfg.User, target.User)
	}
	if cfg.Timeout != 1234 {
		t.Errorf("cfg.Timeout = %v, want 1234", cfg.Timeout)
	}
	if len(cfg.Auth) != 1 {
		t.Errorf("cfg.Auth should have 1 method, got %d", len(cfg.Auth))
	}
	if cfg.HostKeyCallback == nil {
		t.Error("cfg.HostKeyCallback is nil — auto-accept is forbidden")
	}
}

func TestBuildClientConfig_HostKeyCallback_UnknownVsMismatch(t *testing.T) {
	t.Parallel()

	knownPub := dummyPublicKey(t, "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFhEdfRDPYI0lWvy/dimkUPC4PCFB0K6HVT/9xVT4Yvy known@example.com")
	otherPub := dummyPublicKey(t, "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA other@example.com")

	tests := []struct {
		name         string
		dbErr        error
		wantSentinel error
	}{
		{
			name:         "unknown host returns ErrHostKeyUnknown",
			dbErr:        &knownhosts.KeyError{Want: nil},
			wantSentinel: ErrHostKeyUnknown,
		},
		{
			name: "mismatched host returns ErrHostKeyMismatch",
			dbErr: &knownhosts.KeyError{
				Want: []knownhosts.KnownKey{{Key: knownPub}},
			},
			wantSentinel: ErrHostKeyMismatch,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := BuildClientConfig(Options{
				Target:    validTarget(),
				Auth:      []cryptossh.AuthMethod{cryptossh.Password("ignored")},
				HostKeyDB: stubHostKeyDB{err: tt.dbErr},
			})
			if err != nil {
				t.Fatalf("BuildClientConfig: %v", err)
			}

			cbErr := cfg.HostKeyCallback("s1.small.pl:22", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}, otherPub)
			if !errors.Is(cbErr, tt.wantSentinel) {
				t.Fatalf("HostKeyCallback err = %v, want errors.Is(_, %v)", cbErr, tt.wantSentinel)
			}
			if !strings.Contains(cbErr.Error(), "s1.small.pl") {
				t.Errorf("wrapped err should mention hostname for operator log, got %q", cbErr.Error())
			}
		})
	}
}

func TestBuildClientConfig_HostKeyCallback_NilOnMatch(t *testing.T) {
	t.Parallel()

	cfg, err := BuildClientConfig(Options{
		Target:    validTarget(),
		Auth:      []cryptossh.AuthMethod{cryptossh.Password("ignored")},
		HostKeyDB: stubHostKeyDB{err: nil},
	})
	if err != nil {
		t.Fatalf("BuildClientConfig: %v", err)
	}

	if err := cfg.HostKeyCallback("s1.small.pl:22", &net.TCPAddr{}, nil); err != nil {
		t.Fatalf("HostKeyCallback on match returned %v, want nil", err)
	}
}

func TestBuildClientConfig_HostKeyCallback_PreservesNonKeyError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom: filesystem unavailable")
	cfg, err := BuildClientConfig(Options{
		Target:    validTarget(),
		Auth:      []cryptossh.AuthMethod{cryptossh.Password("ignored")},
		HostKeyDB: stubHostKeyDB{err: sentinel},
	})
	if err != nil {
		t.Fatalf("BuildClientConfig: %v", err)
	}

	gotErr := cfg.HostKeyCallback("s1.small.pl:22", &net.TCPAddr{}, nil)
	if !errors.Is(gotErr, sentinel) {
		t.Fatalf("non-KeyError db failures must propagate unchanged, got %v", gotErr)
	}
	if errors.Is(gotErr, ErrHostKeyUnknown) || errors.Is(gotErr, ErrHostKeyMismatch) {
		t.Fatalf("non-KeyError should not be re-mapped to host-key sentinels, got %v", gotErr)
	}
}

func validTarget() Target {
	return Target{Host: "s1.small.pl", Port: 22, User: "u"}
}

func dummyPublicKey(t *testing.T, authorizedKeyLine string) cryptossh.PublicKey {
	t.Helper()
	pub, _, _, _, err := cryptossh.ParseAuthorizedKey([]byte(authorizedKeyLine))
	if err != nil {
		t.Fatalf("ParseAuthorizedKey: %v", err)
	}
	return pub
}
