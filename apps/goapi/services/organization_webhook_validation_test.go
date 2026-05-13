package services

import (
	"context"
	"testing"

	"goapi/config"
)

func TestValidateWebhookURL_Development_LocalhostHTTP(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := &config.Config{Environment: config.EnvironmentDevelopment}
	if err := validateWebhookURL(ctx, cfg, "http://localhost:8089/p"); err != nil {
		t.Fatal(err)
	}
	if err := validateWebhookURL(ctx, cfg, "http://127.0.0.1:8089/p"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateWebhookURL_Development_NonLocalHTTPRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := &config.Config{Environment: config.EnvironmentDevelopment}
	if err := validateWebhookURL(ctx, cfg, "http://example.com/h"); err == nil {
		t.Fatal("expected error for non-local http")
	}
}

func TestValidateWebhookURL_Staging_HTTPSOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := &config.Config{Environment: config.EnvironmentStaging}
	if err := validateWebhookURL(ctx, cfg, "https://203.0.113.9/x"); err != nil {
		t.Fatal(err)
	}
	if err := validateWebhookURL(ctx, cfg, "http://203.0.113.9/x"); err == nil {
		t.Fatal("expected error for http in staging")
	}
}

func TestValidateWebhookURL_Staging_BlocksInternalTargets(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := &config.Config{Environment: config.EnvironmentStaging}
	for _, u := range []string{
		"https://127.0.0.1/x",
		"https://169.254.169.254/x",
		"https://10.1.2.3/x",
	} {
		if err := validateWebhookURL(ctx, cfg, u); err == nil {
			t.Fatalf("expected error for %q", u)
		}
	}
}

func TestNormalizeWebhookEvents_DedupesAndSorts(t *testing.T) {
	out, err := normalizeWebhookEvents([]string{
		WebhookEventOrganizationNoteCreated,
		WebhookEventOrganizationUpdated,
		WebhookEventOrganizationNoteCreated,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0] != WebhookEventOrganizationNoteCreated || out[1] != WebhookEventOrganizationUpdated {
		t.Fatalf("unexpected %v", out)
	}
}

func TestNormalizeWebhookEvents_InvalidRejected(t *testing.T) {
	if _, err := normalizeWebhookEvents([]string{"unknown.event"}); err == nil {
		t.Fatal("expected error")
	}
}
