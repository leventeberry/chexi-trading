package queue

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"goapi/config"
)

func testRedisQueue(t *testing.T) (*RedisQueue, *redis.Client, func()) {
	t.Helper()

	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: srv.Addr(),
	})
	cfg := &config.Config{}
	cfg.Queue.MaxAttempts = 3
	cfg.Queue.DeadLetterMaxCap = 100
	q := NewRedisQueue(client, cfg)
	cleanup := func() {
		_ = client.Close()
		srv.Close()
	}
	return q, client, cleanup
}

func TestRedisQueue_EnqueuePopDueRoundTrip(t *testing.T) {
	t.Parallel()

	q, _, cleanup := testRedisQueue(t)
	defer cleanup()

	ctx := context.Background()
	payload := json.RawMessage(`{"hello":"world"}`)
	if err := q.Enqueue(ctx, "test.job", payload, EnqueueOptions{}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	id, err := q.PopDue(ctx, time.Now().UTC().Add(time.Minute))
	if err != nil {
		t.Fatalf("PopDue: %v", err)
	}
	if id == "" {
		t.Fatal("expected job id")
	}

	job, err := q.LoadJob(ctx, id)
	if err != nil {
		t.Fatalf("LoadJob: %v", err)
	}
	if job.Type != "test.job" {
		t.Fatalf("job.Type = %q", job.Type)
	}
	if string(job.Payload) != `{"hello":"world"}` {
		t.Fatalf("payload = %s", job.Payload)
	}
}

func TestRedisQueue_RequeueAndDeadLetterHelpers(t *testing.T) {
	t.Parallel()

	q, _, cleanup := testRedisQueue(t)
	defer cleanup()
	ctx := context.Background()

	if err := q.Enqueue(ctx, "x", json.RawMessage(`{}`), EnqueueOptions{MaxAttempts: 1}); err != nil {
		t.Fatal(err)
	}
	id, err := q.PopDue(ctx, time.Now().UTC().Add(time.Minute))
	if err != nil || id == "" {
		t.Fatalf("pop: id=%q err=%v", id, err)
	}
	job, err := q.LoadJob(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	job.RunAt = time.Now().UTC().Add(time.Second)
	if err := q.Requeue(ctx, job); err != nil {
		t.Fatalf("Requeue: %v", err)
	}

	snap := []byte(`{"dead":true}`)
	if err := q.PushDeadLetter(ctx, snap); err != nil {
		t.Fatalf("PushDeadLetter: %v", err)
	}
}
