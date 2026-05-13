package middleware

import (
	"net/url"
	"strings"
)

var sensitiveQueryKeys = map[string]struct{}{
	"token":              {},
	"code":               {},
	"state":              {},
	"oauth_code":         {},
	"refresh_token":      {},
	"reset_token":        {},
	"verification_token": {},
	"access_token":       {},
	"id_token":           {},
	"api_key":            {},
	"key":                {},
	"secret":             {},
	"authorization":      {},
}

const redactedQueryValue = "[redacted]"

// requestPathForAuditMetadata returns path with query string for persisted audit events,
// redacting sensitive parameter values. Malformed queries never fall back to raw bytes.
func requestPathForAuditMetadata(path, rawQuery string) string {
	if rawQuery == "" {
		return path
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		// On malformed queries, avoid logging raw query bytes.
		return path + "?[invalid_query]"
	}
	for key, vals := range values {
		if _, sensitive := sensitiveQueryKeys[strings.ToLower(strings.TrimSpace(key))]; !sensitive {
			continue
		}
		for i := range vals {
			vals[i] = redactedQueryValue
		}
		values[key] = vals
	}
	return path + "?" + values.Encode()
}
