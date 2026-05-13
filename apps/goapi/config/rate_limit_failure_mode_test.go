package config

import "testing"

func TestResolveRateLimitRedisFailureMode_Defaults(t *testing.T) {
	t.Parallel()
	mode, warn, fatal := ResolveRateLimitRedisFailureMode(EnvironmentProduction, "")
	if fatal != nil || warn || mode != RateLimitRedisFailureLocalFallback {
		t.Fatalf("production empty: got mode=%q warn=%v fatal=%v", mode, warn, fatal)
	}
	mode, warn, fatal = ResolveRateLimitRedisFailureMode(EnvironmentDevelopment, "")
	if fatal != nil || warn || mode != RateLimitRedisFailureFailOpen {
		t.Fatalf("development empty: got mode=%q warn=%v fatal=%v", mode, warn, fatal)
	}
}

func TestResolveRateLimitRedisFailureMode_CanonicalCase(t *testing.T) {
	t.Parallel()
	mode, warn, fatal := ResolveRateLimitRedisFailureMode(EnvironmentProduction, "FAIL_CLOSED")
	if fatal != nil || warn || mode != RateLimitRedisFailureFailClosed {
		t.Fatalf("got mode=%q warn=%v fatal=%v", mode, warn, fatal)
	}
}

func TestResolveRateLimitRedisFailureMode_InvalidUsesDefault(t *testing.T) {
	t.Parallel()
	mode, warn, fatal := ResolveRateLimitRedisFailureMode(EnvironmentStaging, "not-a-mode")
	if fatal != nil || !warn || mode != RateLimitRedisFailureLocalFallback {
		t.Fatalf("got mode=%q warn=%v fatal=%v", mode, warn, fatal)
	}
}

func TestResolveRateLimitRedisFailureMode_FailOpenForbiddenInStaging(t *testing.T) {
	t.Parallel()
	_, _, fatal := ResolveRateLimitRedisFailureMode(EnvironmentStaging, "fail_open")
	if fatal == nil {
		t.Fatal("expected fatal error for fail_open in staging")
	}
	_, _, fatal = ResolveRateLimitRedisFailureMode(EnvironmentProduction, "fail_open")
	if fatal == nil {
		t.Fatal("expected fatal error for fail_open in production")
	}
}

func TestResolveRateLimitRedisFailureMode_FailOpenAllowedInDevelopment(t *testing.T) {
	t.Parallel()
	mode, warn, fatal := ResolveRateLimitRedisFailureMode(EnvironmentDevelopment, "fail_open")
	if fatal != nil || warn || mode != RateLimitRedisFailureFailOpen {
		t.Fatalf("got mode=%q warn=%v fatal=%v", mode, warn, fatal)
	}
}
