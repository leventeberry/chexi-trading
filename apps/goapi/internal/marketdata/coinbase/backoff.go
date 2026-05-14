package coinbase

import (
	"context"
	"time"
)

// reconnectBackoff returns a simple exponential delay capped at maxDelay.
func reconnectBackoff(attempt int, initial, maxDelay time.Duration) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	d := initial
	for i := 0; i < attempt && d < maxDelay; i++ {
		d *= 2
		if d > maxDelay {
			return maxDelay
		}
	}
	if d > maxDelay {
		return maxDelay
	}
	return d
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
