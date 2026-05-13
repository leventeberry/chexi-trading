package rbac

// Organization-scoped roles (tenant RBAC, distinct from global User.Role).
const (
	OrgRoleOwner  = "owner"
	OrgRoleAdmin  = "admin"
	OrgRoleMember = "member"
)

// IsValidOrgRole reports whether s is a known organization membership role.
func IsValidOrgRole(s string) bool {
	switch s {
	case OrgRoleOwner, OrgRoleAdmin, OrgRoleMember:
		return true
	default:
		return false
	}
}

// OrgRoleCanManageOrganization reports whether the membership role may PATCH organization metadata.
func OrgRoleCanManageOrganization(role string) bool {
	switch role {
	case OrgRoleOwner, OrgRoleAdmin:
		return true
	default:
		return false
	}
}

// IsValidOrgInviteRole reports roles allowed on invitations (not owner).
func IsValidOrgInviteRole(s string) bool {
	switch s {
	case OrgRoleAdmin, OrgRoleMember:
		return true
	default:
		return false
	}
}

// OrgInviteRank compares invite-target roles for upgrades (admin > member).
func OrgInviteRank(role string) int {
	switch role {
	case OrgRoleAdmin:
		return 2
	case OrgRoleMember:
		return 1
	default:
		return 0
	}
}
