package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/config"
	"goapi/internal/events"
)

// HTTPEventAudit persists one http.request event per finished request when AUDIT_HTTP_ENABLED=true.
// Use AUDIT_HTTP_MUTATING_ONLY=false to include GET/HEAD/OPTIONS (high volume).
func HTTPEventAudit(rec events.Recorder, cfg *config.Config) gin.HandlerFunc {
	if cfg == nil || !cfg.Audit.HTTPEnabled {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if rec == nil {
			return
		}
		method := c.Request.Method
		if cfg.Audit.HTTPMutatingOnly && isSafeHTTPMethod(method) {
			return
		}
		latency := time.Since(start)
		status := c.Writer.Status()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		fullPath := requestPathForAuditMetadata(path, raw)
		meta := events.MetadataJSON(map[string]interface{}{
			"method":      method,
			"path":        fullPath,
			"status_code": status,
			"latency_ms":  latency.Milliseconds(),
			"client_ip":   c.ClientIP(),
			"user_agent":  c.Request.UserAgent(),
		})
		var actor *uuid.UUID
		if v, ok := c.Get("userID"); ok {
			if s, ok := v.(string); ok {
				if id, err := uuid.Parse(s); err == nil {
					actor = &id
				}
			}
		}
		e := events.Event{
			OccurredAt:  events.NowUTC(),
			EventType:   "http.request",
			ActorUserID: actor,
			Metadata:    meta,
			RequestID:   events.RequestIDFromContext(c.Request.Context()),
		}
		events.RecordSafe(rec, c.Request.Context(), e)
	}
}

func isSafeHTTPMethod(m string) bool {
	switch strings.ToUpper(m) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}
