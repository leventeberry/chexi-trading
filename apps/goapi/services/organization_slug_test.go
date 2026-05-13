package services

import "testing"

func TestGenerateOrganizationSlugFromName(t *testing.T) {
	if g := GenerateOrganizationSlugFromName("  Acme  Corp  "); g != "acme-corp" {
		t.Fatalf("got %q", g)
	}
	if g := GenerateOrganizationSlugFromName("___"); g != "org" {
		t.Fatalf("empty name fallback: got %q", g)
	}
}

func TestNormalizeOrganizationSlug(t *testing.T) {
	s, err := NormalizeOrganizationSlug("  Good-Slug  ")
	if err != nil || s != "good-slug" {
		t.Fatalf("got %q err=%v", s, err)
	}
	_, err = NormalizeOrganizationSlug("bad slug")
	if err == nil {
		t.Fatal("expected error for space in slug")
	}
}
