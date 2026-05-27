package cpanel_test

import (
	"errors"
	"testing"

	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/providers/cpanel"
)

// newConfig assembles a minimal valid [providers.ProviderConfig] for
// the cpanel factory; tests override Properties to exercise the parser.
func newConfig(props map[string]string) providers.ProviderConfig {
	if props == nil {
		props = map[string]string{}
	}
	return providers.ProviderConfig{
		Alias:      "vh",
		Type:       "cpanel",
		Host:       "panel.vh.pl",
		Port:       22,
		User:       "alice",
		Properties: props,
	}
}

func TestNew_RejectsWrongType(t *testing.T) {
	t.Parallel()
	cfg := newConfig(nil)
	cfg.Type = "smallhost"
	_, err := cpanel.New(cfg)
	if !errors.Is(err, providers.ErrInvalidProviderConfig) {
		t.Fatalf("expected ErrInvalidProviderConfig, got %v", err)
	}
}

func TestNew_DefaultProperties(t *testing.T) {
	t.Parallel()
	provider, err := cpanel.New(newConfig(nil))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	p := provider.(*cpanel.Provider)
	props := p.Properties()
	wantTemplates := map[string]string{
		"AppRootTemplate":    "/home/{user}/nodejs/{app_root}",
		"DeployPathTemplate": "/home/{user}/nodejs/{app_root}/public",
		"LogPathTemplate":    "/home/{user}/nodejs/{app_root}/logs",
	}
	got := map[string]string{
		"AppRootTemplate":    props.AppRootTemplate,
		"DeployPathTemplate": props.DeployPathTemplate,
		"LogPathTemplate":    props.LogPathTemplate,
	}
	for k, v := range wantTemplates {
		if got[k] != v {
			t.Fatalf("%s = %q, want %q", k, got[k], v)
		}
	}
	if props.RestartMethod != "passenger" {
		t.Fatalf("RestartMethod = %q, want passenger", props.RestartMethod)
	}
	if props.NodeSelector != "cloudlinux_selector" {
		t.Fatalf("NodeSelector = %q, want cloudlinux_selector", props.NodeSelector)
	}
	if props.SSLProvider != "autossl" {
		t.Fatalf("SSLProvider = %q, want autossl", props.SSLProvider)
	}
	if props.DomainKind != "addon" {
		t.Fatalf("DomainKind = %q, want addon", props.DomainKind)
	}
	if props.APIPort != 2083 {
		t.Fatalf("APIPort = %d, want 2083", props.APIPort)
	}
}

func TestNew_PropertyRejects(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		props map[string]string
	}{
		{"invalid_restart_method", map[string]string{"restart_method": "kill -9"}},
		{"invalid_node_selector", map[string]string{"node_selector": "nvm"}},
		{"invalid_ssl_provider", map[string]string{"ssl_provider": "zerossl"}},
		{"invalid_domain_kind", map[string]string{"domain_kind": "alias"}},
		{"api_port_not_int", map[string]string{"api_port": "abc"}},
		{"api_port_out_of_range", map[string]string{"api_port": "0"}},
		{"api_port_above_max", map[string]string{"api_port": "65536"}},
		{"autossl_poll_negative", map[string]string{"autossl_poll_seconds": "-1"}},
		{"autossl_poll_too_large", map[string]string{"autossl_poll_seconds": "601"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := cpanel.New(newConfig(tc.props))
			if !errors.Is(err, providers.ErrInvalidProviderConfig) {
				t.Fatalf("expected ErrInvalidProviderConfig, got %v", err)
			}
		})
	}
}

func TestNew_PropertyOverrides(t *testing.T) {
	t.Parallel()
	provider, err := cpanel.New(newConfig(map[string]string{
		"app_root_template":    "/srv/apps/{user}/{app_root}",
		"deploy_path_template": "/srv/apps/{user}/{app_root}/dist",
		"log_path_template":    "/var/log/{user}/{app_root}",
		"api_port":             "2087",
		"domain_kind":          "subdomain",
		"ssl_provider":         "manual",
		"autossl_poll_seconds": "30",
	}))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	p := provider.(*cpanel.Provider)
	props := p.Properties()
	if props.AppRootTemplate != "/srv/apps/{user}/{app_root}" {
		t.Fatalf("AppRootTemplate = %q", props.AppRootTemplate)
	}
	if props.APIPort != 2087 {
		t.Fatalf("APIPort = %d", props.APIPort)
	}
	if props.DomainKind != "subdomain" {
		t.Fatalf("DomainKind = %q", props.DomainKind)
	}
	if props.SSLProvider != "manual" {
		t.Fatalf("SSLProvider = %q", props.SSLProvider)
	}
	if props.AutoSSLPollSeconds != 30 {
		t.Fatalf("AutoSSLPollSeconds = %d", props.AutoSSLPollSeconds)
	}
}

func TestProvider_NameAndConfig(t *testing.T) {
	t.Parallel()
	cfg := newConfig(nil)
	provider, err := cpanel.New(cfg)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if provider.Name() != "cpanel" {
		t.Fatalf("Name = %q, want cpanel", provider.Name())
	}
	p := provider.(*cpanel.Provider)
	if p.Config().Alias != "vh" {
		t.Fatalf("Config().Alias = %q", p.Config().Alias)
	}
}
