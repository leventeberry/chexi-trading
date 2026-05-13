package events

import (
	"context"
)

// MultiRecorder forwards each event to every sink in order. Use for plugin-style fan-out
// (Postgres + webhook + metrics). Failures from individual sinks are logged by RecordSafe callers;
// Record returns the first error if strict chaining is needed — here we combine errors loosely.
//
// Example:
//
//	r := MultiRecorder(NewPostgresRecorder(db), NoOpRecorder{})
type MultiRecorder []Recorder

// Record implements Recorder.
func (m MultiRecorder) Record(ctx context.Context, e Event) error {
	var firstErr error
	for _, sink := range m {
		if sink == nil {
			continue
		}
		if err := sink.Record(ctx, e); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
