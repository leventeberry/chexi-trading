package webhooks

import (
	"encoding/hex"
	"testing"
)

func TestSignPayload_Deterministic(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	body := []byte(`{"event":"organization.updated"}`)
	ts := int64(1700000000)
	sig := SignPayload(secret, ts, body)
	if sig == "" {
		t.Fatal("empty signature")
	}
	if _, err := hex.DecodeString(sig); err != nil {
		t.Fatalf("signature not hex: %v", err)
	}
	sig2 := SignPayload(secret, ts, body)
	if sig != sig2 {
		t.Fatalf("signatures differ: %q vs %q", sig, sig2)
	}
}

func TestSignPayload_ChangesWithTimestampOrBody(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	body := []byte(`{"a":1}`)
	s1 := SignPayload(secret, 1, body)
	s2 := SignPayload(secret, 2, body)
	s3 := SignPayload(secret, 1, []byte(`{"a":2}`))
	if s1 == s2 || s1 == s3 {
		t.Fatal("expected different signatures")
	}
}
