// Package events provides append-only domain / audit recording with pluggable backends.
//
// Extension points (plugins / multi-sink):
//   - Implement Recorder for custom sinks (webhooks, queues, external SIEM).
//   - Compose MultiRecorder([]Recorder{postgres, custom}) to fan-out (see multi.go).
//   - New event types: use stable dot-separated names (e.g. "billing.invoice.paid") and put
//     structured fields in Metadata JSON; avoid new columns unless indexing/query demands it.
//
// Pseudocode — register a webhook sink:
//
//	rec := events.MultiRecorder(
//	    events.NewPostgresRecorder(db),
//	    events.NewWebhookRecorder(cfg.WebhookURL), // future
//	)
package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"goapi/logger"
)

// Event is a single append-only record. Metadata holds arbitrary JSON for forward-compatible extensions.
type Event struct {
	OccurredAt  time.Time
	EventType   string
	ActorUserID *uuid.UUID
	Subject     string
	Metadata    json.RawMessage
	RequestID   string
}

// Recorder persists or forwards events. Implementations must not panic.
type Recorder interface {
	Record(ctx context.Context, e Event) error
}

// NoOpRecorder discards all events (tests / recorder disabled).
type NoOpRecorder struct{}

// Record implements Recorder.
func (NoOpRecorder) Record(ctx context.Context, e Event) error {
	return nil
}

// RecordSafe calls Record and logs failures without failing the caller's request path.
func RecordSafe(r Recorder, ctx context.Context, e Event) {
	if r == nil {
		return
	}
	if err := r.Record(ctx, e); err != nil {
		logger.Log.Warn().Err(err).Str("event_type", e.EventType).Msg("event recorder failed")
	}
}

// NormalizeMetadata returns valid JSON for DB storage.
func NormalizeMetadata(m json.RawMessage) json.RawMessage {
	if len(m) == 0 || string(m) == "null" {
		return json.RawMessage(`{}`)
	}
	return m
}

// MetadataJSON builds metadata from any JSON-marshalable value.
func MetadataJSON(v interface{}) json.RawMessage {
	if v == nil {
		return json.RawMessage(`{}`)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(b)
}

// NowUTC returns time for OccurredAt fields.
func NowUTC() time.Time {
	return time.Now().UTC()
}
