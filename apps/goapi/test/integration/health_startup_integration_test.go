//go:build integration

package main

import (
	"os"
	"strings"
	"testing"

	"goapi/config"
	"goapi/initializers"
)

func TestHealthEndpoint_Contract(t *testing.T) {
	w, err := makeRequest("GET", "/health", nil, "")
	if err != nil {
		t.Fatalf("make request: %v", err)
	}
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200 body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, expected := range []string{`"status":"healthy"`, `"database":{"status":"healthy"}`, `"cache":{"status":"healthy"}`, `"timestamp":`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("response missing %q: %s", expected, body)
		}
	}
}

func TestVersionedMigrations_IntegrationPathIsExecutable(t *testing.T) {
	if os.Getenv("USE_VERSIONED_MIGRATIONS") != "true" {
		t.Skip("integration harness did not enable USE_VERSIONED_MIGRATIONS")
	}

	cfg := config.Load()
	if err := initializers.RunVersionedMigrations(cfg, os.Getenv("MIGRATIONS_DIR")); err != nil {
		t.Fatalf("RunVersionedMigrations returned error: %v", err)
	}
}
