package queue

import (
	"testing"
	"time"
)

func TestNextBackoff_BoundedAndIncreases(t *testing.T) {
	t.Parallel()

	initial := 100 * time.Millisecond
	max := 10 * time.Second

	for attempt := 1; attempt <= 8; attempt++ {
		d := NextBackoff(attempt, initial, max)
		if d < initial {
			t.Fatalf("attempt %d: duration %v below initial %v", attempt, d, initial)
		}
		if d > max {
			t.Fatalf("attempt %d: duration %v above max %v", attempt, d, max)
		}
	}
}

func TestNextBackoff_CapsAtMax(t *testing.T) {
	t.Parallel()

	initial := time.Second
	max := 2 * time.Second
	d := NextBackoff(10, initial, max)
	if d > max {
		t.Fatalf("got %v, want <= %v", d, max)
	}
}
