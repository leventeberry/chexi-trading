package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/internal/events"
	"goapi/models"
	"gorm.io/gorm"
)

// ClientEventInput is accepted by POST /api/v1/events (SPA / mobile telemetry).
type ClientEventInput struct {
	EventType string                 `json:"event_type" binding:"required"`
	Subject   string                 `json:"subject"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// ListEventLogs returns paginated rows from event_log (admin only).
//
// @Summary      List event log
// @Description  Paginated audit and telemetry history. Requires admin JWT.
// @Tags         events
// @Security     BearerAuth
// @Produce      json
// @Param        limit       query  int     false  "Max rows (default 50, max 200)"
// @Param        offset      query  int     false  "Offset for pagination"
// @Param        event_type  query  string  false  "Filter by event_type"
// @Success      200  {object}  map[string]interface{}  "data, total, limit, offset"
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /events [get]
func ListEventLogs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database unavailable"})
			return
		}
		limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
		if err != nil || limit < 1 {
			limit = 50
		}
		if limit > 200 {
			limit = 200
		}
		offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
		if err != nil || offset < 0 {
			offset = 0
		}
		eventType := strings.TrimSpace(c.Query("event_type"))

		ctx := c.Request.Context()
		countQ := db.WithContext(ctx).Model(&models.EventLog{})
		listQ := db.WithContext(ctx).Model(&models.EventLog{})
		if eventType != "" {
			countQ = countQ.Where("event_type = ?", eventType)
			listQ = listQ.Where("event_type = ?", eventType)
		}
		var total int64
		if err := countQ.Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count events"})
			return
		}
		var rows []models.EventLog
		if err := listQ.Order("occurred_at DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list events"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"data":   rows,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

// IngestClientEvent records a client-originated telemetry row (admin only; rate-limited per IP).
//
// @Summary      Record client / UI telemetry event
// @Description  Inserts a telemetry row into event_log. Requires admin JWT; actor_user_id is taken from the token.
// @Tags         events
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      ClientEventInput  true  "Telemetry payload"
// @Success      202   {object}  map[string]string  "recorded"
// @Failure      400   {object}  map[string]string  "bad request"
// @Failure      401   {object}  map[string]string
// @Failure      403   {object}  map[string]string
// @Router       /events [post]
func IngestClientEvent(rec events.Recorder) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input ClientEventInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		normalized := normalizeClientEventType(input.EventType)
		if normalized == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event_type"})
			return
		}
		meta := events.MetadataJSON(input.Metadata)
		if len(meta) == 0 || string(meta) == "null" {
			meta = events.MetadataJSON(map[string]interface{}{})
		}

		var actor *uuid.UUID
		if v, ok := c.Get("userID"); ok {
			if s, ok := v.(string); ok {
				if id, err := uuid.Parse(s); err == nil {
					actor = &id
				}
			}
		}
		subject := strings.TrimSpace(input.Subject)
		if len(subject) > 512 {
			subject = subject[:512]
		}

		e := events.Event{
			OccurredAt:  events.NowUTC(),
			EventType:   normalized,
			ActorUserID: actor,
			Subject:     subject,
			Metadata:    meta,
			RequestID:   events.RequestIDFromContext(c.Request.Context()),
		}
		events.RecordSafe(rec, c.Request.Context(), e)
		c.JSON(http.StatusAccepted, gin.H{"status": "recorded"})
	}
}

func normalizeClientEventType(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	if len(s) > 120 {
		s = s[:120]
	}
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
			continue
		}
		if unicode.IsSpace(r) {
			b.WriteRune('_')
		}
	}
	out := b.String()
	out = strings.Trim(out, "._-")
	if out == "" {
		return ""
	}
	return "client." + out
}
