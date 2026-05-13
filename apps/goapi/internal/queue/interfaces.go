package queue

import (
	"context"
	"encoding/json"
)

// Handler processes a single job payload (already validated JSON).
type Handler func(ctx context.Context, payload json.RawMessage) error

// Enqueuer schedules background work. Implementations may execute synchronously (inline fallback).
type Enqueuer interface {
	Enqueue(ctx context.Context, jobType string, payload json.RawMessage, opts EnqueueOptions) error
}
