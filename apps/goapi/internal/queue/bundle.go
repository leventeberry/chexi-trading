package queue

import (
	"goapi/config"
	"goapi/internal/events"
	"goapi/logger"

	"github.com/redis/go-redis/v9"
)

// BundleDeps carries integrations for the queue bundle.
type BundleDeps struct {
	Recorder events.Recorder
}

// WebhookDeliverWithoutWorker is true when jobs are enqueued to Redis async but no in-process
// worker is running (webhook deliveries must be invoked synchronously or they would never run).
func WebhookDeliverWithoutWorker(cfg *config.Config, rdb *redis.Client) bool {
	return ShouldUseRedisQueue(cfg, rdb) && cfg != nil && !cfg.Queue.WorkerEnabled
}

// ShouldUseRedisQueue reports whether async Redis-backed processing should be used.
func ShouldUseRedisQueue(cfg *config.Config, rdb *redis.Client) bool {
	if cfg == nil || rdb == nil {
		return false
	}
	if !cfg.Queue.Enabled {
		return false
	}
	if !cfg.Queue.AsyncEnabled {
		return false
	}
	return true
}

// NewBundle picks Redis + worker when possible; otherwise inline synchronous execution.
func NewBundle(rdb *redis.Client, cfg *config.Config, reg *Registry, deps BundleDeps) (Enqueuer, *Worker) {
	rec := deps.Recorder
	if ShouldUseRedisQueue(cfg, rdb) {
		rq := NewRedisQueue(rdb, cfg)
		if cfg != nil && !cfg.Queue.WorkerEnabled {
			logger.Log.Warn().
				Str("hint", "Run a worker process with JOB_WORKER_ENABLED=true or disable Redis async (JOB_ASYNC_ENABLED=false) to use inline job execution; otherwise jobs accumulate in Redis until consumed").
				Msg("queue: Redis enqueue enabled but worker disabled (JOB_WORKER_ENABLED=false)")
			return rq, nil
		}
		logger.Log.Info().Msg("queue: async Redis worker enabled")
		w := NewWorker(rq, reg, cfg, rec)
		return rq, w
	}
	logger.Log.Warn().Msg("queue: using inline synchronous job execution (no async worker)")
	return NewInlineQueue(reg, rec, cfg), nil
}
