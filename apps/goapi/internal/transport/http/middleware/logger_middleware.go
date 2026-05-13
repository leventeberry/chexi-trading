package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"goapi/logger"
)

// RequestLogger returns a middleware that logs HTTP requests with details.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()
		statusCode := c.Writer.Status()
		method := c.Request.Method

		// Access logs: path only (no query) so OAuth codes, tokens, etc. never hit request logs.
		baseEvent := logger.Log.
			With().
			Str("method", method).
			Str("path", path).
			Str("proto", c.Request.Proto).
			Int("status_code", statusCode).
			Dur("latency", latency).
			Str("client_ip", clientIP).
			Str("user_agent", userAgent).
			Logger()

		if statusCode >= 500 {
			baseEvent.Error().Msg("HTTP Request")
		} else if statusCode >= 400 {
			baseEvent.Warn().Msg("HTTP Request")
		} else {
			baseEvent.Info().Msg("HTTP Request")
		}
	}
}
