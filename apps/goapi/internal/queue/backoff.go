package queue

import (
	crand "crypto/rand"
	"encoding/binary"
	"math"
	"time"
)

// NextBackoff returns exponential backoff with jitter in [initial, max].
// attempt is 1-based (first retry uses attempt=1).
func NextBackoff(attempt int, initial, max time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if initial < time.Millisecond {
		initial = time.Millisecond
	}
	if max < initial {
		max = initial
	}
	// Factor 2^(attempt-1), capped to avoid overflow.
	exp := float64(initial) * math.Pow(2, float64(attempt-1))
	if exp > float64(max) {
		exp = float64(max)
	}
	d := time.Duration(exp)
	maxJitter := d / 4
	if maxJitter > 0 {
		var buf [8]byte
		if _, err := crand.Read(buf[:]); err == nil {
			u := binary.BigEndian.Uint64(buf[:])
			mj := maxJitter.Nanoseconds()
			if mj < 1 {
				mj = 1
			}
			// mj is a positive nanosecond bound derived from Duration; safe for modulus.
			mod := u % uint64(mj) // #nosec G115
			if mod <= uint64(math.MaxInt64) {
				d += time.Duration(int64(mod))
				if d > max {
					d = max
				}
			}
		}
	}
	if d < initial {
		d = initial
	}
	if d > max {
		d = max
	}
	return d
}
