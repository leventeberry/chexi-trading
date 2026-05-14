package coinbase

import (
	"testing"
	"time"
)

func TestReconnectBackoff(t *testing.T) {
	t.Parallel()
	if d := reconnectBackoff(0, time.Second, 10*time.Second); d != time.Second {
		t.Fatalf("0: %v", d)
	}
	if d := reconnectBackoff(1, time.Second, 10*time.Second); d != 2*time.Second {
		t.Fatalf("1: %v", d)
	}
	if d := reconnectBackoff(5, time.Second, 10*time.Second); d != 10*time.Second {
		t.Fatalf("5: %v", d)
	}
}
