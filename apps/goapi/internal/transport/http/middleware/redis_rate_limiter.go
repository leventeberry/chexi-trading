package middleware

import (
	"context"
	"time"

	"goapi/cache"
)

// RedisRateLimiter implements rate limiting using Redis.
type RedisRateLimiter struct {
	cache             cache.Cache
	requestsPerMinute int
	burstSize         int
	window            time.Duration
}

// NewRedisRateLimiter creates a new Redis-based rate limiter.
func NewRedisRateLimiter(cacheClient cache.Cache, config RateLimiterConfig) *RedisRateLimiter {
	return &RedisRateLimiter{
		cache:             cacheClient,
		requestsPerMinute: config.RequestsPerMinute,
		burstSize:         config.BurstSize,
		window:            cache.RateLimitWindow,
	}
}

// allow checks the distributed limit. On Redis/store error, ok is false and redisErr is non-nil.
// When the store succeeds, redisErr is nil: ok false means the client exceeded the limit (429 path).
func (r *RedisRateLimiter) allow(ctx context.Context, key string) (ok bool, redisErr error) {
	count, err := r.cache.IncrementRateLimit(ctx, key, r.window)
	if err != nil {
		return false, err
	}
	if count > r.requestsPerMinute {
		return false, nil
	}
	return true, nil
}
