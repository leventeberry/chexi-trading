package config

import (
	"fmt"
	"strings"
)

// Redis-backed rate limiter behavior when IncrementRateLimit returns an error.
const (
	RateLimitRedisFailureFailClosed    = "fail_closed"
	RateLimitRedisFailureLocalFallback = "local_fallback"
	RateLimitRedisFailureFailOpen      = "fail_open"
)

var canonicalRateLimitRedisFailureModes = map[string]string{
	"fail_closed":    RateLimitRedisFailureFailClosed,
	"local_fallback": RateLimitRedisFailureLocalFallback,
	"fail_open":      RateLimitRedisFailureFailOpen,
}

// DefaultRateLimitRedisFailureMode returns the mode used when the env var is unset.
func DefaultRateLimitRedisFailureMode(env string) string {
	if IsStagingOrProductionEnvironment(env) {
		return RateLimitRedisFailureLocalFallback
	}
	return RateLimitRedisFailureFailOpen
}

// CanonicalRateLimitRedisFailureMode maps a case-insensitive value to a canonical mode name.
func CanonicalRateLimitRedisFailureMode(raw string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(raw))
	mode, ok := canonicalRateLimitRedisFailureModes[key]
	return mode, ok
}

// ValidateRateLimitRedisFailureModeForEnvironment returns an error if mode is forbidden for env
// (e.g. fail_open in staging/production).
func ValidateRateLimitRedisFailureModeForEnvironment(env, mode string) error {
	if IsStagingOrProductionEnvironment(env) && mode == RateLimitRedisFailureFailOpen {
		return fmt.Errorf("RATE_LIMIT_REDIS_FAILURE_MODE=%s is not allowed in %s", RateLimitRedisFailureFailOpen, NormalizeEnvironment(env))
	}
	return nil
}

// ResolveRateLimitRedisFailureMode parses RATE_LIMIT_REDIS_FAILURE_MODE.
// When raw is empty, returns the environment default (no warn).
// When raw is invalid, returns the environment default and warnInvalid=true.
// When raw is fail_open in staging/production, returns fatal error.
func ResolveRateLimitRedisFailureMode(env, raw string) (mode string, warnInvalid bool, fatal error) {
	def := DefaultRateLimitRedisFailureMode(env)
	if strings.TrimSpace(raw) == "" {
		return def, false, nil
	}
	canon, ok := CanonicalRateLimitRedisFailureMode(raw)
	if !ok {
		return def, true, nil
	}
	if err := ValidateRateLimitRedisFailureModeForEnvironment(env, canon); err != nil {
		return "", false, err
	}
	return canon, false, nil
}
