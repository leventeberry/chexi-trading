package config

import (
	"strings"
	"testing"
)

func TestValidateJWTSecretForEnvironment_DevelopmentAllowsShortSecret(t *testing.T) {
	if err := validateJWTSecretForEnvironment("development", "x"); err != nil {
		t.Fatalf("development should allow short secret: %v", err)
	}
}

func TestValidateJWTSecretForEnvironment_StagingRequiresLength(t *testing.T) {
	if err := validateJWTSecretForEnvironment("staging", strings.Repeat("a", 31)); err == nil {
		t.Fatal("expected error for short secret in staging")
	}
	if err := validateJWTSecretForEnvironment("staging", strings.Repeat("b", 32)); err != nil {
		t.Fatalf("unexpected error for 32-char secret: %v", err)
	}
}

func TestValidateJWTSecretForEnvironment_StagingRejectsDenylist(t *testing.T) {
	if err := validateJWTSecretForEnvironment("staging", "replace-with-strong-random-secret"); err == nil {
		t.Fatal("expected denylist error")
	}
}

func TestValidateJWTSecretForEnvironment_ProductionEnvRequiresStrongSecret(t *testing.T) {
	if err := validateJWTSecretForEnvironment("production", "short"); err == nil {
		t.Fatal("expected error")
	}
}
