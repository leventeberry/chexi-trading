package rbac

// Organization API key scopes (v1).
const (
	OrgScopeRead  = "org:read"
	OrgScopeWrite = "org:write"
)

// IsValidOrgAPIScope reports whether s is a known org API key scope.
func IsValidOrgAPIScope(s string) bool {
	switch s {
	case OrgScopeRead, OrgScopeWrite:
		return true
	default:
		return false
	}
}

// OrgScopeImpliesRead returns true if the scope grants read access (read or write).
func OrgScopeImpliesRead(scope string) bool {
	switch scope {
	case OrgScopeRead, OrgScopeWrite:
		return true
	default:
		return false
	}
}

// OrgScopeImpliesWrite returns true if the scope grants write access.
func OrgScopeImpliesWrite(scope string) bool {
	return scope == OrgScopeWrite
}

// APIScopesContainRead reports whether scopes allow read-level org access.
func APIScopesContainRead(scopes []string) bool {
	for _, s := range scopes {
		if OrgScopeImpliesRead(s) {
			return true
		}
	}
	return false
}

// APIScopesContainWrite reports whether scopes allow write-level org access.
func APIScopesContainWrite(scopes []string) bool {
	for _, s := range scopes {
		if OrgScopeImpliesWrite(s) {
			return true
		}
	}
	return false
}
