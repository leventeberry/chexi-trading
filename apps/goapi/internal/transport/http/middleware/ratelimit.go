package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"goapi/cache"
	"goapi/config"
)

// RateLimiterConfig holds configuration for rate limiting.
type RateLimiterConfig struct {
	RequestsPerMinute int
	BurstSize         int
}

type rateLimiterEntry struct {
	tokens     int
	lastUpdate time.Time
	mu         sync.Mutex
}

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	config      RateLimiterConfig
	entries     map[string]*rateLimiterEntry
	mu          sync.RWMutex
	cleanupTick *time.Ticker
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		config:  config,
		entries: make(map[string]*rateLimiterEntry),
	}

	rl.cleanupTick = time.NewTicker(5 * time.Minute)
	go rl.cleanup()

	return rl
}

func (rl *RateLimiter) cleanup() {
	for range rl.cleanupTick.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, entry := range rl.entries {
			entry.mu.Lock()
			if now.Sub(entry.lastUpdate) > 10*time.Minute {
				delete(rl.entries, ip)
			}
			entry.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	entry, exists := rl.entries[ip]
	if !exists {
		entry = &rateLimiterEntry{
			tokens:     rl.config.BurstSize,
			lastUpdate: time.Now(),
		}
		rl.entries[ip] = entry
	}
	rl.mu.Unlock()

	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(entry.lastUpdate)

	tokensToAdd := int(elapsed.Minutes() * float64(rl.config.RequestsPerMinute))
	if tokensToAdd > 0 {
		entry.tokens = min(entry.tokens+tokensToAdd, rl.config.BurstSize)
		entry.lastUpdate = now
	}

	if entry.tokens > 0 {
		entry.tokens--
		return true
	}

	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var (
	globalRateLimiter      *RateLimiter
	globalRedisRateLimiter *RedisRateLimiter
	rateLimiterOnce        sync.Once
	useRedis               bool
)

// RateLimitMiddleware returns middleware that rate limits requests per IP.
func RateLimitMiddleware() gin.HandlerFunc {
	return RateLimitMiddlewareWithCache(nil, config.Get())
}

// RateLimitMiddlewareWithCache rate limits per IP using Redis when supported, else in-memory.
// cfg must be non-nil in production; RateLimitMiddleware() supplies config.Get() for legacy callers.
func RateLimitMiddlewareWithCache(cacheClient cache.Cache, cfg *config.Config) gin.HandlerFunc {
	if cfg == nil {
		cfg = config.Get()
	}
	rateLimiterOnce.Do(func() {
		rateLimitConfig := RateLimiterConfig{
			RequestsPerMinute: cfg.RateLimit.RequestsPerMinute,
			BurstSize:         cfg.RateLimit.BurstSize,
		}

		if cacheClient != nil && cacheClient.SupportsDistributedRateLimit() {
			globalRateLimiter = NewRateLimiter(rateLimitConfig)
			globalRedisRateLimiter = NewRedisRateLimiter(cacheClient, rateLimitConfig)
			useRedis = true
			return
		}
		globalRateLimiter = NewRateLimiter(rateLimitConfig)
		useRedis = false
	})

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		ctx := c.Request.Context()

		var allowed bool
		if useRedis && globalRedisRateLimiter != nil {
			ok, redisErr := globalRedisRateLimiter.allow(ctx, clientIP)
			if redisErr != nil {
				switch cfg.RateLimit.RedisFailureMode {
				case config.RateLimitRedisFailureFailClosed:
					c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
						"error": "Rate limiter unavailable.",
					})
					return
				case config.RateLimitRedisFailureLocalFallback:
					if globalRateLimiter == nil {
						c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
							"error": "Rate limiter unavailable.",
						})
						return
					}
					allowed = globalRateLimiter.allow(clientIP)
				case config.RateLimitRedisFailureFailOpen:
					allowed = true
				default:
					c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
						"error": "Rate limiter unavailable.",
					})
					return
				}
			} else {
				allowed = ok
			}
		} else {
			allowed = globalRateLimiter.allow(clientIP)
		}

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Please try again later.",
			})
			return
		}

		c.Next()
	}
}

// ShutdownRateLimiter stops background cleanup for the in-memory rate limiter (call during graceful shutdown).
func ShutdownRateLimiter() {
	if globalRateLimiter != nil && globalRateLimiter.cleanupTick != nil {
		globalRateLimiter.cleanupTick.Stop()
	}
}
