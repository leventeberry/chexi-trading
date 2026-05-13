package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"goapi/config"
	"goapi/internal/queue"
)

const adminJobsFailedDefaultLimit = 50
const adminJobsFailedMaxLimit = 100

// AdminJobsDeps holds queue observability dependencies (nil RedisQueue when inline-only mode).
type AdminJobsDeps struct {
	Cfg        *config.Config
	RedisQueue *queue.RedisQueue
}

// AdminJobsHealth returns GET /api/v1/admin/jobs/health.
func AdminJobsHealth(deps AdminJobsDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := deps.Cfg
		if cfg == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration unavailable"})
			return
		}
		out := gin.H{
			"queue_enabled":            cfg.Queue.Enabled,
			"async_redis_queue_active": deps.RedisQueue != nil,
			"worker_enabled":           cfg.Queue.WorkerEnabled,
			"pending_count":            nil,
			"failed_count":             nil,
			"last_error":               nil,
		}
		if deps.RedisQueue == nil {
			c.JSON(http.StatusOK, out)
			return
		}
		ctx := c.Request.Context()
		pending, err := deps.RedisQueue.DueDepth(ctx)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"queue_enabled":            cfg.Queue.Enabled,
				"async_redis_queue_active": true,
				"worker_enabled":           cfg.Queue.WorkerEnabled,
				"error":                    "queue metrics unavailable",
			})
			return
		}
		failed, err := deps.RedisQueue.DeadLetterDepth(ctx)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"queue_enabled":            cfg.Queue.Enabled,
				"async_redis_queue_active": true,
				"worker_enabled":           cfg.Queue.WorkerEnabled,
				"pending_count":            pending,
				"error":                    "dead letter metrics unavailable",
			})
			return
		}
		out["pending_count"] = pending
		out["failed_count"] = failed
		if nj, err := deps.RedisQueue.NewestDeadLetter(ctx); err == nil && nj != nil && nj.LastError != "" {
			le := queue.SanitizeLastError(nj.LastError)
			if le != "" {
				out["last_error"] = le
			}
		}
		c.JSON(http.StatusOK, out)
	}
}

// AdminJobsFailed returns GET /api/v1/admin/jobs/failed (dead-letter snapshots, redacted payloads).
func AdminJobsFailed(deps AdminJobsDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RedisQueue == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "async redis queue is not active; failed job list unavailable"})
			return
		}
		limit := adminJobsFailedDefaultLimit
		if s := c.Query("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				limit = n
			}
		}
		if limit > adminJobsFailedMaxLimit {
			limit = adminJobsFailedMaxLimit
		}
		ctx := c.Request.Context()
		jobs, err := deps.RedisQueue.PeekDeadLetters(ctx, 0, int64(limit-1))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read dead letter queue"})
			return
		}
		items := make([]gin.H, 0, len(jobs))
		for _, j := range jobs {
			items = append(items, gin.H{
				"id":           j.ID,
				"type":         j.Type,
				"status":       j.Status,
				"attempts":     j.Attempts,
				"max_attempts": j.MaxAttempts,
				"created_at":   j.CreatedAt,
				"updated_at":   j.UpdatedAt,
				"finished_at":  j.FinishedAt,
				"last_error":   queue.SanitizeLastError(j.LastError),
				"payload":      queue.RedactPayloadJSON(j.Payload),
			})
		}
		c.JSON(http.StatusOK, gin.H{"data": items, "total": len(items)})
	}
}

// AdminJobsRetry returns POST /api/v1/admin/jobs/:id/retry.
func AdminJobsRetry(deps AdminJobsDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RedisQueue == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "async redis queue is not active; retry unavailable"})
			return
		}
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing job id"})
			return
		}
		ctx := c.Request.Context()
		err := deps.RedisQueue.AdminRetryJob(ctx, id)
		if err != nil {
			if errors.Is(err, queue.ErrAdminRetryTargetNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "job not found for retry"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "retry failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "job scheduled for retry", "id": id})
	}
}
