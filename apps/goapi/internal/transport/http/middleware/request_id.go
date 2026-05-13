package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/internal/events"
)

// RequestID ensures X-Request-ID on the response and stores the ID on the request context
// for audit/event correlation.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = uuid.New().String()
		}
		c.Writer.Header().Set("X-Request-ID", rid)
		ctx := events.WithRequestID(c.Request.Context(), rid)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
