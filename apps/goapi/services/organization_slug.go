package services

import (
	"regexp"
	"strings"
)

// MaxOrganizationSlugLen is the maximum stored slug length (matches VARCHAR in migrations).
const MaxOrganizationSlugLen = 128

var organizationSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// NormalizeOrganizationSlug lowercases, trims, and validates slug format (safe URL segment).
func NormalizeOrganizationSlug(raw string) (string, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return "", ErrInvalidOrganizationSlug
	}
	if len(s) > MaxOrganizationSlugLen {
		return "", ErrInvalidOrganizationSlug
	}
	if !organizationSlugPattern.MatchString(s) {
		return "", ErrInvalidOrganizationSlug
	}
	return s, nil
}

// GenerateOrganizationSlugFromName derives a slug from a display name when the client omits slug.
func GenerateOrganizationSlugFromName(name string) string {
	var b strings.Builder
	lastWasSep := false
	for _, r := range strings.TrimSpace(strings.ToLower(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastWasSep = false
		case r == ' ', r == '-', r == '_', r == '.':
			if b.Len() > 0 && !lastWasSep {
				b.WriteRune('-')
				lastWasSep = true
			}
		default:
			if b.Len() > 0 && !lastWasSep {
				b.WriteRune('-')
				lastWasSep = true
			}
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "org"
	}
	if len(s) > MaxOrganizationSlugLen {
		s = strings.TrimRight(s[:MaxOrganizationSlugLen], "-")
	}
	return s
}

// appendSlugSuffix appends "-suffix" to base, trimming base so the result fits MaxOrganizationSlugLen.
func appendSlugSuffix(base, suffix string) string {
	tail := "-" + suffix
	maxBase := MaxOrganizationSlugLen - len(tail)
	if maxBase < 1 {
		return suffix
	}
	b := base
	if len(b) > maxBase {
		b = strings.TrimRight(b[:maxBase], "-")
	}
	return b + tail
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
