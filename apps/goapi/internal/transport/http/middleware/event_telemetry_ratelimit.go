package middleware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"goapi/config"
)

var (
	eventTelemetryRL     *RateLimiter
	eventTelemetryRLOnce sync.Once
)

// EventTelemetryRateLimit applies a dedicated token bucket per IP for admin GET/POST /api/v1/events.
func EventTelemetryRateLimit(cfg *config.Config) gin.HandlerFunc {
	if cfg == nil {
		cfg = config.Get()
	}
	eventTelemetryRLOnce.Do(func() {
		eventTelemetryRL = NewRateLimiter(RateLimiterConfig{
			RequestsPerMinute: cfg.Events.TelemetryRequestsPerMinute,
			BurstSize:         cfg.Events.TelemetryBurstSize,
		})
	})
	return func(c *gin.Context) {
		if eventTelemetryRL == nil || !eventTelemetryRL.allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Event telemetry rate limit exceeded",
			})
			return
		}
		c.Next()
	}
}
