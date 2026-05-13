package rbac

import (
	"reflect"
	"testing"
)

func TestIsValidRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role string
		want bool
	}{
		{name: "user is valid", role: "user", want: true},
		{name: "admin is valid", role: "admin", want: true},
		{name: "empty invalid", role: "", want: false},
		{name: "unknown invalid", role: "superadmin", want: false},
		{name: "case sensitive invalid", role: "Admin", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsValidRole(tc.role); got != tc.want {
				t.Fatalf("IsValidRole(%q) = %v, want %v", tc.role, got, tc.want)
			}
		})
	}
}

func TestIsAdminRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		role string
		want bool
	}{
		{role: "admin", want: true},
		{role: "user", want: false},
		{role: "ADMIN", want: false},
		{role: "", want: false},
	}

	for _, tc := range tests {
		if got := IsAdminRole(tc.role); got != tc.want {
			t.Fatalf("IsAdminRole(%q) = %v, want %v", tc.role, got, tc.want)
		}
	}
}

func TestAllRoleStringsIsStableAndDefensiveCopy(t *testing.T) {
	t.Parallel()

	want := []string{"user", "admin"}
	got := AllRoleStrings()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AllRoleStrings() = %v, want %v", got, want)
	}

	got[0] = "mutated"
	gotAgain := AllRoleStrings()
	if !reflect.DeepEqual(gotAgain, want) {
		t.Fatalf("AllRoleStrings() should return fresh stable values, got %v want %v", gotAgain, want)
	}
}
