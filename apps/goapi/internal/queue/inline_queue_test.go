package queue

import (
	"context"
	"encoding/json"
	"testing"

	"goapi/config"
	"goapi/internal/events"
)

func TestInlineQueue_DispatchRegisteredHandler(t *testing.T) {
	t.Parallel()

	var called bool
	reg := NewRegistry()
	reg.Register("inline.test", func(ctx context.Context, payload json.RawMessage) error {
		called = true
		return nil
	})
	cfg := &config.Config{}
	cfg.Environment = config.EnvironmentTest
	q := NewInlineQueue(reg, events.NoOpRecorder{}, cfg)

	err := q.Enqueue(context.Background(), "inline.test", json.RawMessage(`{}`), EnqueueOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("handler not invoked")
	}
}

func TestInlineQueue_UnknownJobType(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	cfg := &config.Config{}
	q := NewInlineQueue(reg, events.NoOpRecorder{}, cfg)

	err := q.Enqueue(context.Background(), "missing", json.RawMessage(`{}`), EnqueueOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if err != ErrUnknownJobType {
		t.Fatalf("err = %v", err)
	}
}
