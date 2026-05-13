package config

import (
	"fmt"
	"os"
	"strings"
)

const (
	EnvironmentDevelopment = "development"
	EnvironmentTest        = "test"
	EnvironmentStaging     = "staging"
	EnvironmentProduction  = "production"
)

var environmentAliases = map[string]string{
	"dev":  EnvironmentDevelopment,
	"prod": EnvironmentProduction,
}

var supportedEnvironments = map[string]struct{}{
	EnvironmentDevelopment: {},
	EnvironmentTest:        {},
	EnvironmentStaging:     {},
	EnvironmentProduction:  {},
}

// ResolveEnvironment returns the canonical environment based on APP_ENV, GO_ENV, then ENV.
// Behavior:
// - Empty values default to development.
// - Aliases are normalized (dev -> development, prod -> production).
// - Invalid values default to development unless they look production-like, in which case an error is returned.
func ResolveEnvironment() (string, error) {
	return resolveEnvironmentFromLookup(os.Getenv)
}

func resolveEnvironmentFromLookup(lookup func(string) string) (string, error) {
	raw, _ := firstEnvironmentRawValue(lookup)
	if raw == "" {
		return EnvironmentDevelopment, nil
	}

	normalized := NormalizeEnvironment(raw)
	if _, ok := supportedEnvironments[normalized]; ok {
		return normalized, nil
	}

	if looksProductionLike(raw) {
		return "", fmt.Errorf("invalid environment %q: expected one of development, test, staging, production", raw)
	}
	return EnvironmentDevelopment, nil
}

func firstEnvironmentRawValue(lookup func(string) string) (string, string) {
	for _, key := range []string{"APP_ENV", "GO_ENV", "ENV"} {
		raw := strings.TrimSpace(lookup(key))
		if raw != "" {
			return raw, key
		}
	}
	return "", ""
}

// NormalizeEnvironment lowercases, trims spaces, and canonicalizes aliases.
func NormalizeEnvironment(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if alias, ok := environmentAliases[normalized]; ok {
		return alias
	}
	return normalized
}

func looksProductionLike(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(v, "prod") || strings.Contains(v, "stag")
}

// IsProductionEnvironment reports whether env is canonical production.
func IsProductionEnvironment(env string) bool {
	return NormalizeEnvironment(env) == EnvironmentProduction
}

// IsStagingOrProductionEnvironment reports whether env is canonical staging/production.
func IsStagingOrProductionEnvironment(env string) bool {
	switch NormalizeEnvironment(env) {
	case EnvironmentStaging, EnvironmentProduction:
		return true
	default:
		return false
	}
}
