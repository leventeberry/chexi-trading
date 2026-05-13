package services

import "goapi/internal/rbac"

// ValidRoles is the list of allowed user roles (single source: internal/rbac).
var ValidRoles = rbac.AllRoleStrings()

// IsValidRole checks if a role is valid.
func IsValidRole(role string) bool {
	return rbac.IsValidRole(role)
}
