package queue

import (
	"context"
	"encoding/json"
	"sync"
)

// Registry maps job types to handlers (thread-safe).
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register binds a job type to a handler (panics if typ is empty).
func (r *Registry) Register(typ string, h Handler) {
	if typ == "" {
		panic("queue: Register empty job type")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[typ] = h
}

// Lookup returns the handler or nil.
func (r *Registry) Lookup(typ string) Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.handlers[typ]
}

// Dispatch invokes the handler for typ or returns ErrUnknownJobType.
func (r *Registry) Dispatch(ctx context.Context, typ string, payload json.RawMessage) error {
	h := r.Lookup(typ)
	if h == nil {
		return ErrUnknownJobType
	}
	return h(ctx, payload)
}
