package config

import "testing"

func TestResolveSwaggerEnabled_DefaultsToEnabledInDevelopment(t *testing.T) {
	t.Setenv("SWAGGER_ENABLED", "")
	if enabled := resolveSwaggerEnabled("development"); !enabled {
		t.Fatal("expected swagger enabled by default in development")
	}
}

func TestResolveSwaggerEnabled_DefaultsToDisabledInProduction(t *testing.T) {
	t.Setenv("SWAGGER_ENABLED", "")
	if enabled := resolveSwaggerEnabled("production"); enabled {
		t.Fatal("expected swagger disabled by default in production")
	}
}

func TestResolveSwaggerEnabled_ExplicitOverrideInProduction(t *testing.T) {
	t.Setenv("SWAGGER_ENABLED", "true")
	if enabled := resolveSwaggerEnabled("production"); !enabled {
		t.Fatal("expected explicit override to enable swagger in production")
	}
}
