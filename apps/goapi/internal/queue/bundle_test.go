package queue

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"goapi/config"
	"goapi/internal/events"
)

func TestNewBundle_WorkerDisabledStillReturnsRedisEnqueuer(t *testing.T) {
	t.Parallel()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
		srv.Close()
	})

	cfg := &config.Config{}
	cfg.Queue.Enabled = true
	cfg.Queue.AsyncEnabled = true
	cfg.Queue.WorkerEnabled = false

	reg := NewRegistry()
	enq, w := NewBundle(client, cfg, reg, BundleDeps{Recorder: events.NoOpRecorder{}})
	if w != nil {
		t.Fatalf("expected nil worker, got %v", w)
	}
	rq, ok := enq.(*RedisQueue)
	if !ok {
		t.Fatalf("expected *RedisQueue, got %T", enq)
	}
	if err := rq.Enqueue(context.Background(), "t", json.RawMessage(`{}`), EnqueueOptions{}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
}
