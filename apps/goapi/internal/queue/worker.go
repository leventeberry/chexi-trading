package queue

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"goapi/config"
	"goapi/internal/events"
	"goapi/logger"
)

// Worker polls Redis and executes registered handlers with retries and dead-letter handling.
type Worker struct {
	rq       *RedisQueue
	reg      *Registry
	cfg      *config.Config
	recorder events.Recorder

	procWG sync.WaitGroup
}

// NewWorker constructs a Redis-backed worker (not used for inline-only mode).
func NewWorker(rq *RedisQueue, reg *Registry, cfg *config.Config, recorder events.Recorder) *Worker {
	return &Worker{
		rq:       rq,
		reg:      reg,
		cfg:      cfg,
		recorder: recorder,
	}
}

// Start runs polling loops until ctx is cancelled, then waits for in-flight jobs up to ShutdownTimeout.
func (w *Worker) Start(ctx context.Context) {
	pollWorkers := 1
	if w.cfg != nil && w.cfg.Queue.Workers > 1 {
		pollWorkers = w.cfg.Queue.Workers
	}
	pollInterval := 500 * time.Millisecond
	if w.cfg != nil && w.cfg.Queue.PollInterval > 0 {
		pollInterval = w.cfg.Queue.PollInterval
	}

	pollCtx, cancelPoll := context.WithCancel(ctx)
	var pollWG sync.WaitGroup
	for i := 0; i < pollWorkers; i++ {
		pollWG.Add(1)
		go func() {
			defer pollWG.Done()
			ticker := time.NewTicker(pollInterval)
			defer ticker.Stop()
			for {
				select {
				case <-pollCtx.Done():
					return
				case <-ticker.C:
					w.pollOnce(pollCtx, ctx)
				}
			}
		}()
	}

	<-ctx.Done()
	cancelPoll()
	pollWG.Wait()

	shutdownTimeout := 30 * time.Second
	if w.cfg != nil && w.cfg.Queue.ShutdownTimeout > 0 {
		shutdownTimeout = w.cfg.Queue.ShutdownTimeout
	}
	done := make(chan struct{})
	go func() {
		w.procWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(shutdownTimeout):
		logger.Log.Warn().Dur("timeout", shutdownTimeout).Msg("queue worker shutdown timed out waiting for in-flight jobs")
	}
}

func (w *Worker) pollOnce(pollCtx, runCtx context.Context) {
	id, err := w.rq.PopDue(pollCtx, time.Now().UTC())
	if err != nil || id == "" {
		return
	}
	w.procWG.Add(1)
	go func(jobID string) {
		defer w.procWG.Done()
		w.processJob(runCtx, jobID)
	}(id)
}

func (w *Worker) processJob(runCtx context.Context, id string) {
	// Redis persistence must complete even when runCtx is cancelled (shutdown);
	// handlers still receive runCtx so they can stop promptly.
	persistCtx := context.WithoutCancel(runCtx)

	job, err := w.rq.LoadJob(persistCtx, id)
	if err != nil {
		if err == ErrJobNotFound {
			return
		}
		logger.Log.Warn().Err(err).Str("job_id", id).Msg("queue: load job failed")
		return
	}

	now := time.Now().UTC()
	job.Status = StatusRunning
	job.UpdatedAt = now
	if err := w.rq.SaveJob(persistCtx, job); err != nil {
		logger.Log.Warn().Err(err).Str("job_id", id).Msg("queue: persist running status failed")
		return
	}

	handlerErr := w.reg.Dispatch(runCtx, job.Type, job.Payload)
	if handlerErr == nil {
		if err := w.rq.DeleteJob(persistCtx, id); err != nil {
			logger.Log.Warn().Err(err).Str("job_id", id).Msg("queue: delete succeeded job failed")
		}
		events.RecordSafe(w.recorder, persistCtx, events.Event{
			OccurredAt: events.NowUTC(),
			EventType:  "queue.job.succeeded",
			Subject:    id,
			Metadata: events.MetadataJSON(map[string]interface{}{
				"job_type": job.Type,
			}),
		})
		return
	}

	w.failJob(persistCtx, job, handlerErr)
}

func (w *Worker) failJob(ctx context.Context, job *Job, handlerErr error) {
	job.Attempts++
	job.LastError = handlerErr.Error()
	job.UpdatedAt = time.Now().UTC()

	// Unknown types are not recoverable; do not waste retries.
	terminal := job.Attempts >= job.MaxAttempts || errors.Is(handlerErr, ErrUnknownJobType)
	if terminal {
		job.Status = StatusDeadLetter
		fin := job.UpdatedAt
		job.FinishedAt = &fin
		raw, err := json.Marshal(job)
		if err != nil {
			logger.Log.Error().Err(err).Str("job_id", job.ID).Msg("queue: marshal DLQ job failed")
			return
		}
		if err := w.rq.PushDeadLetter(ctx, raw); err != nil {
			logger.Log.Warn().Err(err).Str("job_id", job.ID).Msg("queue: push dead letter failed")
		}
		_ = w.rq.DeleteJob(ctx, job.ID)
		logger.Log.Error().
			Err(handlerErr).
			Str("job_id", job.ID).
			Str("job_type", job.Type).
			Int("attempts", job.Attempts).
			Msg("queue: job moved to dead letter")
		events.RecordSafe(w.recorder, ctx, events.Event{
			OccurredAt: events.NowUTC(),
			EventType:  "queue.job.dead_letter",
			Subject:    job.ID,
			Metadata: events.MetadataJSON(map[string]interface{}{
				"job_type": job.Type,
				"error":    job.LastError,
				"attempts": job.Attempts,
			}),
		})
		return
	}

	initial := time.Second
	maxB := 5 * time.Minute
	if w.cfg != nil {
		if w.cfg.Queue.InitialBackoff > 0 {
			initial = w.cfg.Queue.InitialBackoff
		}
		if w.cfg.Queue.MaxBackoff > 0 {
			maxB = w.cfg.Queue.MaxBackoff
		}
	}
	var delay time.Duration
	if w.cfg != nil && w.cfg.Queue.RetryDelayFixed > 0 {
		delay = w.cfg.Queue.RetryDelayFixed
	} else {
		delay = NextBackoff(job.Attempts, initial, maxB)
	}
	job.RunAt = job.UpdatedAt.Add(delay)
	job.Status = StatusQueued

	if err := w.rq.Requeue(ctx, job); err != nil {
		logger.Log.Warn().Err(err).Str("job_id", job.ID).Msg("queue: requeue after failure failed")
		return
	}
	logger.Log.Warn().
		Err(handlerErr).
		Str("job_id", job.ID).
		Str("job_type", job.Type).
		Int("attempt", job.Attempts).
		Dur("delay", delay).
		Msg("queue: job scheduled for retry")

	events.RecordSafe(w.recorder, ctx, events.Event{
		OccurredAt: events.NowUTC(),
		EventType:  "queue.job.retry_scheduled",
		Subject:    job.ID,
		Metadata: events.MetadataJSON(map[string]interface{}{
			"job_type": job.Type,
			"attempt":  job.Attempts,
			"delay_ms": delay.Milliseconds(),
			"error":    job.LastError,
		}),
	})
}
