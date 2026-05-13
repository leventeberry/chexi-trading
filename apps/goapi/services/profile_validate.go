package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxDisplayNameLen = 120
	maxNameLen        = 50
	maxPhoneLen       = 20
	maxAvatarURLLen   = 2048
	maxTimezoneLen    = 64
	maxLocaleLen      = 16
	maxJSONBBytes     = 16384 // cap flexible JSON payloads
)

// ValidThemes lists allowed UX theme preferences.
var ValidThemes = map[string]struct{}{
	"light":  {},
	"dark":   {},
	"system": {},
}

// SanitizeString trims UTF-8 safe whitespace-only edges; rejects interior control chars.
func SanitizeString(s string, maxRunes int) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > maxRunes {
		return "", fmt.Errorf("exceeds max length (%d)", maxRunes)
	}
	for _, r := range s {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return "", errors.New("contains invalid control characters")
		}
	}
	return s, nil
}

// ValidateHTTPSURL rejects non-http(s) URLs for avatar_url.
func ValidateHTTPSURL(raw string, maxLen int) error {
	if raw == "" {
		return errors.New("empty url")
	}
	raw = strings.TrimSpace(raw)
	if len(raw) > maxLen {
		return errors.New("url too long")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("avatar_url must use http or https")
	}
	if u.Host == "" {
		return errors.New("avatar_url missing host")
	}
	return nil
}

// ValidateIANATimezone uses time.LoadLocation (IANA) with "UTC" special-case.
func ValidateIANATimezone(tz string) error {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return errors.New("empty timezone")
	}
	if utf8.RuneCountInString(tz) > maxTimezoneLen {
		return ErrInvalidTimezone
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return ErrInvalidTimezone
	}
	return nil
}

// ValidateLocaleFormat accepts BCP47-like xx or xx-YY (2–5 char subtags).
func ValidateLocaleFormat(locale string) error {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return errors.New("empty locale")
	}
	if len(locale) > maxLocaleLen {
		return ErrInvalidLocale
	}
	parts := strings.Split(locale, "-")
	if len(parts) == 1 {
		p := parts[0]
		if len(p) != 2 || !isASCIILetter(p[0]) || !isASCIILetter(p[1]) {
			return ErrInvalidLocale
		}
		return nil
	}
	if len(parts) == 2 {
		p0, p1 := parts[0], parts[1]
		if len(p0) != 2 || len(p1) < 2 || len(p1) > 8 {
			return ErrInvalidLocale
		}
		if !isASCIILetter(p0[0]) || !isASCIILetter(p0[1]) {
			return ErrInvalidLocale
		}
		for i := 0; i < len(p1); i++ {
			c := p1[i]
			if !isASCIILetter(c) && !isDigit(c) {
				return ErrInvalidLocale
			}
		}
		return nil
	}
	return ErrInvalidLocale
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// ValidateJSONObjectBytes ensures JSON is an object and within size cap.
func ValidateJSONObjectBytes(b []byte) error {
	if len(b) == 0 {
		return errors.New("empty json")
	}
	if len(b) > maxJSONBBytes {
		return errors.New("json payload too large")
	}
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	if _, ok := v.(map[string]interface{}); !ok {
		return errors.New("must be a JSON object")
	}
	return nil
}
