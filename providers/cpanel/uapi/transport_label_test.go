package uapi

import (
	"testing"
)

func TestComposite_TransportDelegatesToLegs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		primary   Reader
		secondary Reader
		want      string
	}{
		{"both-wired", &fakeReader{transport: "HTTPS"}, &fakeReader{transport: "SSH"}, "HTTPS+SSH"},
		{"primary-only", &fakeReader{transport: "HTTPS"}, nil, "HTTPS"},
		{"secondary-only", nil, &fakeReader{transport: "SSH"}, "SSH"},
		{"neither-wired", nil, nil, "?"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := &Composite{Primary: tc.primary, Secondary: tc.secondary}
			if got := c.Transport(); got != tc.want {
				t.Errorf("Transport() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClient_TransportReturnsHTTPS(t *testing.T) {
	t.Parallel()
	c, err := NewClient("https://cpanel.example.com:2083", "operator", "t0k3n", nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if got := c.Transport(); got != "HTTPS" {
		t.Errorf("Transport() = %q, want HTTPS", got)
	}
}

func TestSSHFallback_TransportReturnsSSH(t *testing.T) {
	t.Parallel()
	fb, err := NewSSHFallback(&fakeSSHRunner{}, "operator")
	if err != nil {
		t.Fatalf("NewSSHFallback: %v", err)
	}
	if got := fb.Transport(); got != "SSH" {
		t.Errorf("Transport() = %q, want SSH", got)
	}
}
