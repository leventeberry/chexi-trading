package events

import (
	"context"
	"errors"
	"testing"
)

func TestMultiRecorder_AllSuccess_ReturnsNil(t *testing.T) {
	t.Parallel()
	var order []string
	a := stubRecorder(func(ctx context.Context, e Event) error {
		order = append(order, "a")
		return nil
	})
	b := stubRecorder(func(ctx context.Context, e Event) error {
		order = append(order, "b")
		return nil
	})
	m := MultiRecorder{a, b}
	if err := m.Record(context.Background(), Event{EventType: "fanout"}); err != nil {
		t.Fatalf("Record err = %v", err)
	}
	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Fatalf("call order %v", order)
	}
}

func TestMultiRecorder_NilSinkSkipped(t *testing.T) {
	t.Parallel()
	var calls int
	one := stubRecorder(func(ctx context.Context, e Event) error {
		calls++
		return nil
	})
	m := MultiRecorder{one, nil, one}
	if err := m.Record(context.Background(), Event{EventType: "x"}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestMultiRecorder_FirstErrorReturned_SecondStillRuns(t *testing.T) {
	t.Parallel()
	var order []string
	errA := errors.New("a fails")
	first := stubRecorder(func(ctx context.Context, e Event) error {
		order = append(order, "1")
		return errA
	})
	second := stubRecorder(func(ctx context.Context, e Event) error {
		order = append(order, "2")
		return errors.New("ignored")
	})
	m := MultiRecorder{first, second}
	err := m.Record(context.Background(), Event{EventType: "x"})
	if !errors.Is(err, errA) {
		t.Fatalf("err = %v, want %v", err, errA)
	}
	if len(order) != 2 || order[0] != "1" || order[1] != "2" {
		t.Fatalf("expected both sinks invoked in order, got %v", order)
	}
}

func TestMultiRecorder_SecondErrorWhenFirstNil(t *testing.T) {
	t.Parallel()
	errB := errors.New("b")
	first := stubRecorder(func(ctx context.Context, e Event) error { return nil })
	second := stubRecorder(func(ctx context.Context, e Event) error { return errB })
	m := MultiRecorder{first, second}
	if err := m.Record(context.Background(), Event{}); !errors.Is(err, errB) {
		t.Fatalf("err = %v, want %v", err, errB)
	}
}

func TestMultiRecorder_Empty(t *testing.T) {
	t.Parallel()
	var m MultiRecorder
	if err := m.Record(context.Background(), Event{}); err != nil {
		t.Fatalf("empty MultiRecorder.Record err = %v", err)
	}
}

func TestMultiRecorder_FirstErrNotOverwrittenByLaterSuccess(t *testing.T) {
	t.Parallel()
	err1 := errors.New("first")
	first := stubRecorder(func(ctx context.Context, e Event) error { return err1 })
	second := stubRecorder(func(ctx context.Context, e Event) error { return nil })
	m := MultiRecorder{first, second}
	if err := m.Record(context.Background(), Event{}); !errors.Is(err, err1) {
		t.Fatalf("err = %v", err)
	}
}
