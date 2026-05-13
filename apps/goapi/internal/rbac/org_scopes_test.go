package rbac

import "testing"

func TestAPIScopesContainRead(t *testing.T) {
	t.Parallel()
	if !APIScopesContainRead([]string{OrgScopeRead}) {
		t.Fatal("read scope should imply read access")
	}
	if !APIScopesContainRead([]string{OrgScopeWrite}) {
		t.Fatal("write scope should imply read access")
	}
	if APIScopesContainRead([]string{}) {
		t.Fatal("empty scopes should not grant read")
	}
}

func TestAPIScopesContainWrite(t *testing.T) {
	t.Parallel()
	if !APIScopesContainWrite([]string{OrgScopeWrite}) {
		t.Fatal("write scope should grant write")
	}
	if APIScopesContainWrite([]string{OrgScopeRead}) {
		t.Fatal("read-only should not grant write")
	}
}
