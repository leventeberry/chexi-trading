package queue

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactPayloadJSON_StripsSensitiveKeys(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{"to":"a@b.c","raw_token":"secret-token-value"}`)
	out := RedactPayloadJSON(raw)
	var m map[string]interface{}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["to"] != "a@b.c" {
		t.Fatalf("to = %v", m["to"])
	}
	if m["raw_token"] != "[redacted]" {
		t.Fatalf("raw_token = %v", m["raw_token"])
	}
	if strings.Contains(string(out), "secret-token-value") {
		t.Fatalf("secret leaked in %s", out)
	}
}

func TestSanitizeLastError_TruncatesLongString(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 400)
	s := SanitizeLastError(long)
	if len([]rune(s)) > maxLastErrorRunes+2 {
		t.Fatalf("expected truncation, len runes %d", len([]rune(s)))
	}
}
