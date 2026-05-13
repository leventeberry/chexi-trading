package config

import (
	"strings"
	"testing"
)

func TestResolveDatabaseSSLModeForEnvironment_DefaultsToDisableInDevelopmentAndTest(t *testing.T) {
	t.Parallel()

	for _, env := range []string{EnvironmentDevelopment, EnvironmentTest} {
		env := env
		t.Run(env, func(t *testing.T) {
			t.Parallel()

			mode, err := resolveDatabaseSSLModeForEnvironment(env, "")
			if err != nil {
				t.Fatalf("expected no error for %s, got %v", env, err)
			}
			if mode != dbSSLModeDisable {
				t.Fatalf("expected default %q, got %q", dbSSLModeDisable, mode)
			}
		})
	}
}

func TestResolveDatabaseSSLModeForEnvironment_RejectsInvalidValue(t *testing.T) {
	t.Parallel()

	if _, err := resolveDatabaseSSLModeForEnvironment(EnvironmentDevelopment, "not-a-mode"); err == nil {
		t.Fatal("expected invalid DB_SSLMODE error")
	}
}

func TestResolveDatabaseSSLModeForEnvironment_ProductionPolicy(t *testing.T) {
	t.Parallel()

	rejected := []string{
		dbSSLModeDisable,
		dbSSLModeAllow,
		dbSSLModePrefer,
		dbSSLModeRequire,
		dbSSLModeVerifyCA,
	}
	for _, mode := range rejected {
		mode := mode
		t.Run("rejects_"+mode, func(t *testing.T) {
			t.Parallel()
			if _, err := resolveDatabaseSSLModeForEnvironment(EnvironmentProduction, mode); err == nil {
				t.Fatalf("expected %q to be rejected in production", mode)
			}
		})
	}

	acceptedMode, err := resolveDatabaseSSLModeForEnvironment(EnvironmentProduction, dbSSLModeVerifyFull)
	if err != nil {
		t.Fatalf("expected verify-full to be accepted in production: %v", err)
	}
	if acceptedMode != dbSSLModeVerifyFull {
		t.Fatalf("expected %q, got %q", dbSSLModeVerifyFull, acceptedMode)
	}

	if _, err := resolveDatabaseSSLModeForEnvironment(EnvironmentProduction, ""); err == nil {
		t.Fatal("expected empty DB_SSLMODE to be rejected in production")
	}
}

func TestResolveDatabaseSSLModeForEnvironment_StagingPolicy(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{dbSSLModeDisable, dbSSLModeAllow, dbSSLModePrefer} {
		mode := mode
		t.Run("rejects_"+mode, func(t *testing.T) {
			t.Parallel()
			if _, err := resolveDatabaseSSLModeForEnvironment(EnvironmentStaging, mode); err == nil {
				t.Fatalf("expected %q to be rejected in staging", mode)
			}
		})
	}

	for _, mode := range []string{dbSSLModeRequire, dbSSLModeVerifyCA, dbSSLModeVerifyFull} {
		mode := mode
		t.Run("accepts_"+mode, func(t *testing.T) {
			t.Parallel()
			got, err := resolveDatabaseSSLModeForEnvironment(EnvironmentStaging, mode)
			if err != nil {
				t.Fatalf("expected %q to be accepted in staging: %v", mode, err)
			}
			if got != mode {
				t.Fatalf("expected %q, got %q", mode, got)
			}
		})
	}

	if _, err := resolveDatabaseSSLModeForEnvironment(EnvironmentStaging, ""); err == nil {
		t.Fatal("expected empty DB_SSLMODE to be rejected in staging")
	}
}

func TestPostgresURL_IncludesConfiguredSSLMode(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	cfg.Database.User = "user"
	cfg.Database.Pass = "pass"
	cfg.Database.Host = "localhost"
	cfg.Database.Port = "5432"
	cfg.Database.Name = "goapi"
	cfg.Database.SSLMode = dbSSLModeVerifyCA

	url := PostgresURL(cfg)
	if !strings.Contains(url, "sslmode="+dbSSLModeVerifyCA) {
		t.Fatalf("expected URL to include sslmode=%s, got %q", dbSSLModeVerifyCA, url)
	}
}
