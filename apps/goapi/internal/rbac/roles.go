// Package rbac defines application role identifiers and validation (enum-like).
// Gin binding tags use the same string values as RoleUser / RoleAdmin.
package rbac

// Role is a persisted user role string (JWT "role" claim matches).
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// String returns the wire / DB value.
func (r Role) String() string {
	return string(r)
}

// AllRoleStrings is the canonical list of assignable roles (order stable for docs).
func AllRoleStrings() []string {
	return []string{string(RoleUser), string(RoleAdmin)}
}

// IsValidRole reports whether s is a known role.
func IsValidRole(s string) bool {
	switch Role(s) {
	case RoleUser, RoleAdmin:
		return true
	default:
		return false
	}
}

// IsAdminRole reports whether s is the admin role.
func IsAdminRole(s string) bool {
	return Role(s) == RoleAdmin
}
