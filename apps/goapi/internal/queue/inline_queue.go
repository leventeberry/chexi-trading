package queue

import (
	"context"
	"encoding/json"

	"goapi/config"
	"goapi/internal/events"
	"goapi/logger"
)

// InlineQueue executes handlers synchronously on the caller goroutine (fallback when Redis/async is off).
type InlineQueue struct {
	reg      *Registry
	recorder events.Recorder
	cfg      *config.Config
}

// NewInlineQueue builds an inline/synchronous queue implementation.
func NewInlineQueue(reg *Registry, recorder events.Recorder, cfg *config.Config) *InlineQueue {
	return &InlineQueue{reg: reg, recorder: recorder, cfg: cfg}
}

// Enqueue implements Enqueuer by dispatching immediately.
func (q *InlineQueue) Enqueue(ctx context.Context, jobType string, payload json.RawMessage, opts EnqueueOptions) error {
	logger.Log.Warn().
		Str("job_type", jobType).
		Msg("queue: executing job inline (Redis unavailable or async queue disabled)")
	meta := map[string]interface{}{
		"job_type": jobType,
		"mode":     "inline",
	}
	if q.cfg != nil {
		meta["environment"] = q.cfg.Environment
	}
	events.RecordSafe(q.recorder, ctx, events.Event{
		OccurredAt: events.NowUTC(),
		EventType:  "queue.inline_execute",
		Metadata:   events.MetadataJSON(meta),
	})
	return q.reg.Dispatch(ctx, jobType, payload)
}
