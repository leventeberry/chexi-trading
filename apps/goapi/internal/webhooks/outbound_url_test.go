package webhooks

import (
	"context"
	"testing"

	"goapi/config"
)

func cfgEnv(env string) *config.Config {
	c := &config.Config{}
	c.Environment = env
	return c
}

func TestValidateOutboundWebhookURL_Staging_HTTPSPublicLiteral(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := cfgEnv(config.EnvironmentStaging)
	// TEST-NET-3 (RFC 5737): documentation / global unicast in Go
	if err := ValidateOutboundWebhookURL(ctx, cfg, "https://203.0.113.7/webhook", DefaultLookupHost); err != nil {
		t.Fatal(err)
	}
}

func TestValidateOutboundWebhookURL_Staging_HTTPRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := cfgEnv(config.EnvironmentStaging)
	if err := ValidateOutboundWebhookURL(ctx, cfg, "http://203.0.113.7/webhook", DefaultLookupHost); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateOutboundWebhookURL_Staging_LoopbackAndMetadataRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := cfgEnv(config.EnvironmentStaging)
	cases := []string{
		"https://127.0.0.1/x",
		"https://[::1]/x",
		"https://169.254.169.254/latest/meta-data/",
		"https://10.0.0.1/x",
		"https://192.168.1.1/x",
		"https://172.16.0.1/x",
	}
	for _, u := range cases {
		if err := ValidateOutboundWebhookURL(ctx, cfg, u, DefaultLookupHost); err == nil {
			t.Fatalf("expected error for %q", u)
		}
	}
}

func TestValidateOutboundWebhookURL_UserinfoRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := cfgEnv(config.EnvironmentStaging)
	stub := func(context.Context, string) ([]string, error) {
		return []string{"203.0.113.1"}, nil
	}
	if err := ValidateOutboundWebhookURL(ctx, cfg, "https://user:pass@hooks.example.com/h", stub); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateOutboundWebhookURL_StubDNS_PrivateIPRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := cfgEnv(config.EnvironmentStaging)
	stub := func(context.Context, string) ([]string, error) {
		return []string{"10.0.0.1"}, nil
	}
	if err := ValidateOutboundWebhookURL(ctx, cfg, "https://hooks.example.com/h", stub); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateOutboundWebhookURL_StubDNS_PublicIPAccepted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := cfgEnv(config.EnvironmentStaging)
	stub := func(context.Context, string) ([]string, error) {
		return []string{"203.0.113.42"}, nil
	}
	if err := ValidateOutboundWebhookURL(ctx, cfg, "https://hooks.example.com/h", stub); err != nil {
		t.Fatal(err)
	}
}

func TestValidateOutboundWebhookURL_Development_LocalhostHTTP(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := cfgEnv(config.EnvironmentDevelopment)
	stub := func(context.Context, string) ([]string, error) {
		return []string{"127.0.0.1"}, nil
	}
	if err := ValidateOutboundWebhookURL(ctx, cfg, "http://localhost:8089/p", stub); err != nil {
		t.Fatal(err)
	}
}

func TestValidateOutboundWebhookURL_Development_LiteralLoopbackHTTP(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := cfgEnv(config.EnvironmentDevelopment)
	if err := ValidateOutboundWebhookURL(ctx, cfg, "http://127.0.0.1:8089/p", DefaultLookupHost); err != nil {
		t.Fatal(err)
	}
}

func TestValidateOutboundWebhookURL_Development_NonLocalHTTPRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := cfgEnv(config.EnvironmentDevelopment)
	stub := func(context.Context, string) ([]string, error) {
		return []string{"203.0.113.1"}, nil
	}
	if err := ValidateOutboundWebhookURL(ctx, cfg, "http://hooks.example.com/h", stub); err == nil {
		t.Fatal("expected error for non-loopback http")
	}
}

func TestValidateOutboundWebhookURL_Development_HTTPSLoopbackRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := cfgEnv(config.EnvironmentDevelopment)
	if err := ValidateOutboundWebhookURL(ctx, cfg, "https://127.0.0.1:1/x", DefaultLookupHost); err == nil {
		t.Fatal("expected error for https to loopback")
	}
}

func TestValidateOutboundWebhookURL_Development_PrivateLiteralRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := cfgEnv(config.EnvironmentDevelopment)
	if err := ValidateOutboundWebhookURL(ctx, cfg, "http://10.0.0.1/x", DefaultLookupHost); err == nil {
		t.Fatal("expected error for private IP with http (not an allowed loopback host form)")
	}
}

func TestValidateOutboundWebhookURL_NilConfig(t *testing.T) {
	t.Parallel()
	if err := ValidateOutboundWebhookURL(context.Background(), nil, "https://203.0.113.1/x", DefaultLookupHost); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateOutboundWebhookURL_NilLookup(t *testing.T) {
	t.Parallel()
	cfg := cfgEnv(config.EnvironmentStaging)
	if err := ValidateOutboundWebhookURL(context.Background(), cfg, "https://203.0.113.1/x", nil); err == nil {
		t.Fatal("expected error")
	}
}
