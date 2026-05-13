package services

import (
	"errors"
	"strings"
	"testing"
)

func TestSanitizeString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       string
		maxRunes int
		wantOut  string
		wantErr  string
	}{
		{name: "trims whitespace", in: "  hello  ", maxRunes: 10, wantOut: "hello"},
		{name: "rejects over max", in: "abcdef", maxRunes: 3, wantErr: "exceeds max length"},
		{name: "rejects control chars", in: "ab\x01cd", maxRunes: 10, wantErr: "contains invalid control characters"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := SanitizeString(tc.in, tc.maxRunes)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantOut {
				t.Fatalf("SanitizeString(%q) = %q, want %q", tc.in, got, tc.wantOut)
			}
		})
	}
}

func TestValidateHTTPSURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{name: "empty", url: "", wantErr: "empty url"},
		{name: "missing host", url: "https://", wantErr: "avatar_url missing host"},
		{name: "invalid scheme", url: "ftp://example.com/avatar.png", wantErr: "avatar_url must use http or https"},
		{name: "valid https", url: "https://example.com/avatar.png", wantErr: ""},
		{name: "valid http", url: "http://example.com/avatar.png", wantErr: ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateHTTPSURL(tc.url, 2048)
			if tc.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr != "" && (err == nil || err.Error() != tc.wantErr) {
				t.Fatalf("expected error %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateIANATimezone(t *testing.T) {
	t.Parallel()

	if err := ValidateIANATimezone("UTC"); err != nil {
		t.Fatalf("expected UTC timezone to be valid, got %v", err)
	}
	if !errors.Is(ValidateIANATimezone("Invalid/Timezone"), ErrInvalidTimezone) {
		t.Fatalf("expected ErrInvalidTimezone for invalid timezone")
	}
}

func TestValidateLocaleFormat(t *testing.T) {
	t.Parallel()

	valid := []string{"en", "en-US", "zh-CN", "es-419"}
	for _, loc := range valid {
		if err := ValidateLocaleFormat(loc); err != nil {
			t.Fatalf("expected locale %q to be valid, got %v", loc, err)
		}
	}

	invalid := []string{"", "english", "e", "en_", "en-USA$", "en-US-extra"}
	for _, loc := range invalid {
		if !errors.Is(ValidateLocaleFormat(loc), ErrInvalidLocale) && loc != "" {
			t.Fatalf("expected ErrInvalidLocale for %q", loc)
		}
		if loc == "" && ValidateLocaleFormat(loc) == nil {
			t.Fatal("expected empty locale to fail")
		}
	}
}

func TestValidateJSONObjectBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload []byte
		wantErr string
	}{
		{name: "empty", payload: []byte(""), wantErr: "empty json"},
		{name: "invalid json", payload: []byte("{invalid"), wantErr: "invalid character"},
		{name: "not object", payload: []byte(`["x"]`), wantErr: "must be a JSON object"},
		{name: "valid object", payload: []byte(`{"k":"v"}`), wantErr: ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateJSONObjectBytes(tc.payload)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if tc.wantErr == "invalid character" {
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error to contain %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err.Error() != tc.wantErr {
				t.Fatalf("expected error %q, got %v", tc.wantErr, err)
			}
		})
	}
}
