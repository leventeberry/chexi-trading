package queue

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

var sensitivePayloadKeys = map[string]struct{}{
	"raw_token":     {},
	"token":         {},
	"password":      {},
	"refresh_token": {},
	"api_key":       {},
	"authorization": {},
	"secret":        {},
	"jwt_token":     {},
}

const maxLastErrorRunes = 256

// RedactPayloadJSON returns a copy of payload with sensitive keys redacted (recursive for objects).
func RedactPayloadJSON(payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 {
		return payload
	}
	var v interface{}
	if err := json.Unmarshal(payload, &v); err != nil {
		return json.RawMessage(`"[unreadable_payload]"`)
	}
	redactValue(v)
	out, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`"[unreadable_payload]"`)
	}
	return out
}

func redactValue(v interface{}) {
	switch x := v.(type) {
	case map[string]interface{}:
		for k, val := range x {
			if _, sensitive := sensitivePayloadKeys[strings.ToLower(k)]; sensitive {
				x[k] = "[redacted]"
				continue
			}
			redactValue(val)
		}
	case []interface{}:
		for i := range x {
			redactValue(x[i])
		}
	default:
		// scalars unchanged
	}
}

// SanitizeLastError truncates and avoids echoing obvious high-entropy secrets in error strings.
func SanitizeLastError(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !utf8.ValidString(s) {
		return "[invalid_utf8]"
	}
	runes := []rune(s)
	if len(runes) > maxLastErrorRunes {
		s = string(runes[:maxLastErrorRunes]) + "…"
	}
	// Heuristic: long base64-ish segments often indicate tokens
	if len(s) > 120 && strings.Contains(s, "eyJ") {
		return "[error_redacted_possible_token]"
	}
	return s
}
