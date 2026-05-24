package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/dilitS/webox/ssh"
)

func TestClassifyHostKeyErr(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil → no classification", nil, ""},
		{"unrelated error", errors.New("io: connection refused"), ""},
		{"mismatch sentinel", ssh.ErrHostKeyMismatch, hostKeyKindMismatch},
		{"wrapped mismatch", fmt.Errorf("dial smallhost: %w", ssh.ErrHostKeyMismatch), hostKeyKindMismatch},
		{"unknown sentinel", ssh.ErrHostKeyUnknown, hostKeyKindUnknown},
		{"db required is NOT classified", ssh.ErrHostKeyDBRequired, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := classifyHostKeyErr(tc.err); got != tc.want {
				t.Errorf("classifyHostKeyErr(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestRenderHostKeyModal_ClosedReturnsEmpty(t *testing.T) {
	t.Parallel()
	out := renderHostKeyModal(hostKeyModalForm{Open: false}, 100, "~/.ssh/known_hosts")
	if out != "" {
		t.Fatalf("closed modal should be empty, got:\n%s", out)
	}
}

func TestRenderHostKeyModal_MismatchEmitsExactInstructions(t *testing.T) {
	t.Parallel()

	form := hostKeyModalForm{
		Open:     true,
		Host:     "s1.small.pl",
		Port:     22,
		Username: "demo",
		Kind:     hostKeyKindMismatch,
	}
	out := renderHostKeyModal(form, 120, "/home/demo/.ssh/known_hosts")

	for _, needle := range []string{
		"Host key mismatch",
		"s1.small.pl",
		"User: demo",
		// MUST surface the exact recovery command so the operator
		// does not have to context-switch to documentation.
		"ssh-keygen -R s1.small.pl -f /home/demo/.ssh/known_hosts",
		// MUST cite the security policy section so the rationale
		// is auditable.
		"SECURITY §5",
		// MUST tell the operator to verify OUT OF BAND.
		"OUT OF BAND",
		// MUST NOT silently continue the connection.
		"does NOT continue",
		// MUST show the standard close affordance.
		"Esc",
	} {
		if !strings.Contains(out, needle) {
			t.Errorf("missing %q\n--- modal ---\n%s", needle, out)
		}
	}
}

// TestRenderHostKeyModal_NeverLeaksKeyMaterial is the headline
// security regression guard for the Sprint 14 modal: even if a
// future contributor adds a "Fingerprint" or "Offered key" field to
// `hostKeyModalForm`, the renderer must keep ignoring it. The test
// is brittle by design — any new field that ends up rendered will
// fail this assertion.
func TestRenderHostKeyModal_NeverLeaksKeyMaterial(t *testing.T) {
	t.Parallel()

	form := hostKeyModalForm{
		Open: true,
		Host: "evil.example.com",
		Kind: hostKeyKindMismatch,
	}
	out := renderHostKeyModal(form, 120, "~/.ssh/known_hosts")

	for _, forbidden := range []string{
		"AAAAB3", // SSH ed25519/RSA public key prefix.
		"ssh-ed25519",
		"ssh-rsa",
		"ecdsa-sha2",
		"SHA256:",
		"MD5:",
	} {
		if strings.Contains(out, forbidden) {
			t.Errorf("modal leaks key material: contains %q\n--- modal ---\n%s",
				forbidden, out)
		}
	}
}

func TestRenderHostKeyModal_UnknownKindUsesTOFUCopy(t *testing.T) {
	t.Parallel()

	form := hostKeyModalForm{
		Open: true,
		Host: "new.smallhost.pl",
		Kind: hostKeyKindUnknown,
	}
	out := renderHostKeyModal(form, 120, "~/.ssh/known_hosts")

	for _, needle := range []string{
		"Unknown host key",
		"first connection",
		// Even for unknown (not mismatch) keys we MUST advise
		// out-of-band verification before TOFU.
		"OUT OF BAND",
	} {
		if !strings.Contains(out, needle) {
			t.Errorf("missing %q\n--- modal ---\n%s", needle, out)
		}
	}
}

func TestRenderHostKeyModal_NonDefaultPortRendersBracketForm(t *testing.T) {
	t.Parallel()

	form := hostKeyModalForm{
		Open: true,
		Host: "edge.example.com",
		Port: 2222,
		Kind: hostKeyKindMismatch,
	}
	out := renderHostKeyModal(form, 120, "~/.ssh/known_hosts")
	if !strings.Contains(out, "ssh-keygen -R [edge.example.com]:2222") {
		t.Fatalf("expected `[host]:port` form for non-default port\n--- modal ---\n%s", out)
	}
}

func TestDefaultKnownHostsPath(t *testing.T) {
	t.Parallel()
	if got := defaultKnownHostsPath(""); got != "~/.ssh/known_hosts" {
		t.Errorf("empty home should fall back to ~/.ssh/known_hosts, got %q", got)
	}
	if got := defaultKnownHostsPath("/home/demo"); got != "/home/demo/.ssh/known_hosts" {
		t.Errorf("home expansion drift: %q", got)
	}
}
