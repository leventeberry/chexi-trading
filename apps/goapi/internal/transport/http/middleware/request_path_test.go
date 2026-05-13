package middleware

import (
	"strings"
	"testing"
)

func TestRequestPathForAuditMetadata_RedactsSensitiveQueryKeys(t *testing.T) {
	t.Parallel()

	got := requestPathForAuditMetadata("/api/v1/oauth/callback", "code=abc&state=def&foo=bar")
	if got == "" {
		t.Fatal("expected non-empty path")
	}
	if strings.Contains(got, "abc") || strings.Contains(got, "def") {
		t.Fatalf("sensitive values must be redacted: %q", got)
	}
	if !strings.Contains(got, "foo=bar") || !strings.Contains(got, "code=%5Bredacted%5D") || !strings.Contains(got, "state=%5Bredacted%5D") {
		t.Fatalf("unexpected encoded path %q", got)
	}
}

func TestRequestPathForAuditMetadata_InvalidQueryDoesNotLeakRawBytes(t *testing.T) {
	t.Parallel()

	got := requestPathForAuditMetadata("/x", "token=%zz")
	if got != "/x?[invalid_query]" {
		t.Fatalf("got %q", got)
	}
}

func TestRequestPathForAuditMetadata_RedactsTokenAndVerificationParams(t *testing.T) {
	t.Parallel()

	// Use distinctive secret values (not single letters) so we do not false-positive on substrings of "[redacted]" / percent-encoding.
	got := requestPathForAuditMetadata("/cb", "token=supersecret&reset_token=r1&verification_token=v1&api_key=keyval99&key=keyid77&secret=sec88&ok=yes")
	for _, leak := range []string{"supersecret", "r1", "v1", "keyval99", "keyid77", "sec88"} {
		if strings.Contains(got, leak) {
			t.Fatalf("value %q leaked in %q", leak, got)
		}
	}
	if !strings.Contains(got, "ok=yes") {
		t.Fatalf("non-sensitive param missing: %q", got)
	}
}
