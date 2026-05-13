package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestRedisQueue_AdminRetry_FromDeadLetter(t *testing.T) {
	t.Parallel()
	q, _, cleanup := testRedisQueue(t)
	defer cleanup()
	ctx := context.Background()

	job := Job{
		ID:          "dead-id-1",
		Type:        "email.send_verification",
		Payload:     json.RawMessage(`{"to":"x@y.z","raw_token":"tok"}`),
		Status:      StatusDeadLetter,
		Attempts:    3,
		MaxAttempts: 5,
		LastError:   "boom",
		CreatedAt:   time.Now().UTC().Add(-time.Hour),
		UpdatedAt:   time.Now().UTC(),
	}
	raw, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.PushDeadLetter(ctx, raw); err != nil {
		t.Fatal(err)
	}

	if err := q.AdminRetryJob(ctx, "dead-id-1"); err != nil {
		t.Fatalf("AdminRetryJob: %v", err)
	}

	n, err := q.DeadLetterDepth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("dlq depth = %d", n)
	}

	loaded, err := q.LoadJob(ctx, "dead-id-1")
	if err != nil {
		t.Fatalf("LoadJob: %v", err)
	}
	if loaded.Status != StatusQueued {
		t.Fatalf("status = %s", loaded.Status)
	}
	if loaded.Attempts != 0 {
		t.Fatalf("attempts = %d", loaded.Attempts)
	}
	if loaded.LastError != "" {
		t.Fatalf("last_error = %q", loaded.LastError)
	}
}

func TestRedisQueue_AdminRetry_NotFound(t *testing.T) {
	t.Parallel()
	q, _, cleanup := testRedisQueue(t)
	defer cleanup()
	err := q.AdminRetryJob(context.Background(), "missing-id")
	if !errors.Is(err, ErrAdminRetryTargetNotFound) {
		t.Fatalf("got %v", err)
	}
}
