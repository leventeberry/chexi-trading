package events

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestWithRequestID_RoundTrip(t *testing.T) {
	t.Parallel()
	ctx := WithRequestID(context.Background(), "req-trace-abc")
	if got := RequestIDFromContext(ctx); got != "req-trace-abc" {
		t.Fatalf("RequestIDFromContext = %q, want %q", got, "req-trace-abc")
	}
}

func TestRequestIDFromContext_MissingReturnsEmpty(t *testing.T) {
	t.Parallel()
	if got := RequestIDFromContext(context.Background()); got != "" {
		t.Fatalf("RequestIDFromContext = %q, want empty", got)
	}
}

func TestWithActorUserID_RoundTrip(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	ctx := WithActorUserID(context.Background(), id)
	got, ok := ActorUserIDFromContext(ctx)
	if !ok {
		t.Fatal("ActorUserIDFromContext: expected ok true")
	}
	if got != id {
		t.Fatalf("ActorUserIDFromContext = %v, want %v", got, id)
	}
}

func TestActorUserIDFromContext_Missing(t *testing.T) {
	t.Parallel()
	_, ok := ActorUserIDFromContext(context.Background())
	if ok {
		t.Fatal("ActorUserIDFromContext: expected ok false")
	}
}

func TestContextChaining_RequestIDAndActor(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	ctx := WithRequestID(context.Background(), "rid-1")
	ctx = WithActorUserID(ctx, id)
	if RequestIDFromContext(ctx) != "rid-1" {
		t.Fatalf("request id lost after WithActorUserID")
	}
	aid, ok := ActorUserIDFromContext(ctx)
	if !ok || aid != id {
		t.Fatalf("actor id lost or wrong: %v ok=%v", aid, ok)
	}
}
