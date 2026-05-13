package queue

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"goapi/config"
	"goapi/internal/events"
)

func TestWorker_ProcessesJobSuccess(t *testing.T) {
	t.Parallel()

	q, _, cleanup := testRedisQueue(t)
	defer cleanup()

	done := make(chan struct{})
	reg := NewRegistry()
	reg.Register("worker.ok", func(ctx context.Context, payload json.RawMessage) error {
		close(done)
		return nil
	})

	cfg := &config.Config{}
	cfg.Queue.Workers = 1
	cfg.Queue.PollInterval = 10 * time.Millisecond
	cfg.Queue.ShutdownTimeout = 2 * time.Second
	cfg.Queue.MaxAttempts = 2
	cfg.Queue.InitialBackoff = time.Millisecond
	cfg.Queue.MaxBackoff = time.Millisecond

	w := NewWorker(q, reg, cfg, events.NoOpRecorder{})

	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		w.Start(ctx)
	}()

	if err := q.Enqueue(ctx, "worker.ok", json.RawMessage(`{}`), EnqueueOptions{}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler not invoked")
	}

	cancel()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestWorker_MovesToDeadLetterAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	q, client, cleanup := testRedisQueue(t)
	defer cleanup()

	reg := NewRegistry()
	reg.Register("worker.fail", func(ctx context.Context, payload json.RawMessage) error {
		return errors.New("boom")
	})

	cfg := &config.Config{}
	cfg.Queue.Workers = 1
	cfg.Queue.PollInterval = 5 * time.Millisecond
	cfg.Queue.ShutdownTimeout = 3 * time.Second
	cfg.Queue.MaxAttempts = 1
	cfg.Queue.InitialBackoff = time.Millisecond
	cfg.Queue.MaxBackoff = time.Millisecond

	w := NewWorker(q, reg, cfg, events.NoOpRecorder{})

	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		w.Start(ctx)
	}()

	if err := q.Enqueue(ctx, "worker.fail", json.RawMessage(`{}`), EnqueueOptions{MaxAttempts: 1}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	deadline := time.After(4 * time.Second)
	tick := time.NewTicker(15 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			cancel()
			<-finished
			t.Fatal("DLQ not populated in time")
		case <-tick.C:
			n, err := client.LLen(context.Background(), dlqRedisKey()).Result()
			if err != nil {
				t.Fatal(err)
			}
			if n >= 1 {
				cancel()
				select {
				case <-finished:
				case <-time.After(3 * time.Second):
					t.Fatal("worker did not stop")
				}
				return
			}
		}
	}
}

func TestWorker_RetriesThenSucceeds(t *testing.T) {
	t.Parallel()

	q, _, cleanup := testRedisQueue(t)
	defer cleanup()

	done := make(chan struct{})
	var calls atomic.Int32
	reg := NewRegistry()
	reg.Register("worker.retry_ok", func(ctx context.Context, payload json.RawMessage) error {
		if calls.Add(1) < 3 {
			return errors.New("transient")
		}
		close(done)
		return nil
	})

	cfg := &config.Config{}
	cfg.Queue.Workers = 1
	cfg.Queue.PollInterval = 8 * time.Millisecond
	cfg.Queue.ShutdownTimeout = 3 * time.Second
	cfg.Queue.MaxAttempts = 5
	cfg.Queue.InitialBackoff = time.Millisecond
	cfg.Queue.MaxBackoff = 10 * time.Millisecond

	w := NewWorker(q, reg, cfg, events.NoOpRecorder{})

	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		w.Start(ctx)
	}()

	if err := q.Enqueue(ctx, "worker.retry_ok", json.RawMessage(`{}`), EnqueueOptions{}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		cancel()
		<-finished
		t.Fatal("handler did not succeed after retries")
	}

	cancel()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestWorker_UnknownJobTypeDeadLettersImmediately(t *testing.T) {
	t.Parallel()

	q, client, cleanup := testRedisQueue(t)
	defer cleanup()

	reg := NewRegistry()

	cfg := &config.Config{}
	cfg.Queue.Workers = 1
	cfg.Queue.PollInterval = 5 * time.Millisecond
	cfg.Queue.ShutdownTimeout = 3 * time.Second
	cfg.Queue.MaxAttempts = 5
	cfg.Queue.InitialBackoff = time.Millisecond
	cfg.Queue.MaxBackoff = time.Millisecond

	w := NewWorker(q, reg, cfg, events.NoOpRecorder{})

	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		w.Start(ctx)
	}()

	if err := q.Enqueue(ctx, "no.handler.for.this", json.RawMessage(`{}`), EnqueueOptions{}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	deadline := time.After(4 * time.Second)
	tick := time.NewTicker(15 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			cancel()
			<-finished
			t.Fatal("DLQ not populated for unknown job type")
		case <-tick.C:
			n, err := client.LLen(context.Background(), dlqRedisKey()).Result()
			if err != nil {
				t.Fatal(err)
			}
			if n >= 1 {
				cancel()
				select {
				case <-finished:
				case <-time.After(3 * time.Second):
					t.Fatal("worker did not stop")
				}
				return
			}
		}
	}
}

func TestWorker_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	q, _, cleanup := testRedisQueue(t)
	defer cleanup()

	reg := NewRegistry()
	cfg := &config.Config{}
	cfg.Queue.Workers = 1
	cfg.Queue.PollInterval = 200 * time.Millisecond
	cfg.Queue.ShutdownTimeout = 2 * time.Second

	w := NewWorker(q, reg, cfg, events.NoOpRecorder{})

	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		w.Start(ctx)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not stop after context cancel")
	}
}
