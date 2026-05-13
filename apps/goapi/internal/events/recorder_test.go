package events

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestNormalizeMetadata_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   json.RawMessage
		want string // expected JSON string
	}{
		{name: "nil", in: nil, want: `{}`},
		{name: "empty bytes", in: json.RawMessage{}, want: `{}`},
		{name: "null literal", in: json.RawMessage(`null`), want: `{}`},
		{name: "object preserved", in: json.RawMessage(`{"a":1}`), want: `{"a":1}`},
		{name: "garbage preserved", in: json.RawMessage(`not-json`), want: `not-json`},
		{name: "whitespace_null not normalized", in: json.RawMessage(` null `), want: ` null `},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeMetadata(tc.in)
			if string(got) != tc.want {
				t.Fatalf("NormalizeMetadata(...) = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMetadataJSON_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    interface{}
		want string
	}{
		{name: "nil", v: nil, want: `{}`},
		{name: "empty map", v: map[string]string{}, want: `{}`},
		{name: "map", v: map[string]string{"k": "v"}, want: `{"k":"v"}`},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := MetadataJSON(tc.v)
			if string(got) != tc.want {
				t.Fatalf("MetadataJSON(...) = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestMetadataJSON_MarshalFallbackToEmptyObject(t *testing.T) {
	t.Parallel()
	// Channels are not JSON-marshalable in encoding/json.
	ch := make(chan int)
	got := MetadataJSON(ch)
	if string(got) != `{}` {
		t.Fatalf("MetadataJSON(chan) = %s, want {}", got)
	}
}

func TestNoOpRecorder_Record_NoError(t *testing.T) {
	t.Parallel()
	var r NoOpRecorder
	if err := r.Record(context.Background(), Event{EventType: "test.noop"}); err != nil {
		t.Fatalf("NoOpRecorder.Record err = %v", err)
	}
}

func TestRecordSafe_NilRecorderNoPanic(t *testing.T) {
	t.Parallel()
	RecordSafe(nil, context.Background(), Event{EventType: "x"})
}

func TestRecordSafe_CallsRecorder(t *testing.T) {
	t.Parallel()
	var called bool
	stub := stubRecorder(func(ctx context.Context, e Event) error {
		called = true
		if e.EventType != "typed" {
			t.Fatalf("event type %q", e.EventType)
		}
		return nil
	})
	RecordSafe(stub, context.Background(), Event{EventType: "typed"})
	if !called {
		t.Fatal("expected stub Record called")
	}
}

func TestRecordSafe_SwallowsRecorderError_NoPanic(t *testing.T) {
	t.Parallel()
	stub := stubRecorder(func(ctx context.Context, e Event) error {
		return errors.New("sink failed")
	})
	RecordSafe(stub, context.Background(), Event{EventType: "fail"})
	// If we get here without panic, design is preserved (logs only).
}

type stubRecorder func(ctx context.Context, e Event) error

func (f stubRecorder) Record(ctx context.Context, e Event) error {
	return f(ctx, e)
}
