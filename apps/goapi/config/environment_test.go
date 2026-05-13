package config

import (
	"strings"
	"testing"
)

func TestResolveEnvironment_PreferenceOrder(t *testing.T) {
	t.Run("prefers APP_ENV over GO_ENV and ENV", func(t *testing.T) {
		env, err := resolveEnvironmentFromLookup(func(key string) string {
			switch key {
			case "APP_ENV":
				return " test "
			case "GO_ENV":
				return "production"
			case "ENV":
				return "staging"
			default:
				return ""
			}
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env != EnvironmentTest {
			t.Fatalf("expected %q, got %q", EnvironmentTest, env)
		}
	})

	t.Run("uses GO_ENV when APP_ENV is empty", func(t *testing.T) {
		env, err := resolveEnvironmentFromLookup(func(key string) string {
			switch key {
			case "APP_ENV":
				return "  "
			case "GO_ENV":
				return " staging "
			case "ENV":
				return "production"
			default:
				return ""
			}
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env != EnvironmentStaging {
			t.Fatalf("expected %q, got %q", EnvironmentStaging, env)
		}
	})

	t.Run("uses ENV as final fallback", func(t *testing.T) {
		env, err := resolveEnvironmentFromLookup(func(key string) string {
			switch key {
			case "APP_ENV", "GO_ENV":
				return ""
			case "ENV":
				return " production "
			default:
				return ""
			}
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env != EnvironmentProduction {
			t.Fatalf("expected %q, got %q", EnvironmentProduction, env)
		}
	})
}

func TestResolveEnvironment_NormalizationAndAliases(t *testing.T) {
	t.Run("normalizes case and surrounding spaces", func(t *testing.T) {
		env, err := resolveEnvironmentFromLookup(func(key string) string {
			if key == "APP_ENV" {
				return "  PrOdUcTiOn  "
			}
			return ""
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env != EnvironmentProduction {
			t.Fatalf("expected %q, got %q", EnvironmentProduction, env)
		}
	})

	t.Run("maps dev alias", func(t *testing.T) {
		env, err := resolveEnvironmentFromLookup(func(key string) string {
			if key == "APP_ENV" {
				return "dev"
			}
			return ""
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env != EnvironmentDevelopment {
			t.Fatalf("expected %q, got %q", EnvironmentDevelopment, env)
		}
	})

	t.Run("maps prod alias", func(t *testing.T) {
		env, err := resolveEnvironmentFromLookup(func(key string) string {
			if key == "APP_ENV" {
				return "prod"
			}
			return ""
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env != EnvironmentProduction {
			t.Fatalf("expected %q, got %q", EnvironmentProduction, env)
		}
	})
}

func TestResolveEnvironment_InvalidValueHandling(t *testing.T) {
	t.Run("defaults to development for non-production-like invalid values", func(t *testing.T) {
		env, err := resolveEnvironmentFromLookup(func(key string) string {
			if key == "APP_ENV" {
				return "sandbox"
			}
			return ""
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env != EnvironmentDevelopment {
			t.Fatalf("expected %q, got %q", EnvironmentDevelopment, env)
		}
	})

	t.Run("fails for production-like invalid values", func(t *testing.T) {
		_, err := resolveEnvironmentFromLookup(func(key string) string {
			if key == "APP_ENV" {
				return "prodution"
			}
			return ""
		})
		if err == nil {
			t.Fatal("expected error for invalid production-like value")
		}
		if !strings.Contains(err.Error(), "invalid environment") {
			t.Fatalf("expected invalid environment error, got: %v", err)
		}
	})
}

func TestResolveEnvironment_EmptyDefaultsToDevelopment(t *testing.T) {
	env, err := resolveEnvironmentFromLookup(func(string) string { return "" })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env != EnvironmentDevelopment {
		t.Fatalf("expected %q, got %q", EnvironmentDevelopment, env)
	}
}
