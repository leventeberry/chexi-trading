//go:build integration

package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestTenantOrganizations_UnauthenticatedCannotList(t *testing.T) {
	w, err := makeRequest("GET", "/api/v1/organizations", nil, "")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET /organizations without auth = %d, want 401", w.Code)
	}
}

func TestTenantOrganizations_CreateSetsOwnerMembership(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "ownercheck-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Owner Check Inc",
		"slug": slug,
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("POST organizations = %d, body=%s", w.Code, w.Body.String())
	}
	var created map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("parse: %v", err)
	}
	orgID, _ := created["id"].(string)
	if orgID == "" {
		t.Fatal("missing org id")
	}

	w, err = makeRequest("GET", "/api/v1/organizations/"+orgID+"/members", nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("GET members = %d, body=%s", w.Code, w.Body.String())
	}
	var members []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &members); err != nil {
		t.Fatalf("parse members: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("want 1 member, got %d", len(members))
	}
	if members[0]["role"] != "owner" {
		t.Fatalf("want owner role, got %v", members[0]["role"])
	}
	if members[0]["user_id"] != userID {
		t.Fatalf("want owner user_id=%s, got %v", userID, members[0]["user_id"])
	}
}

func TestTenantOrganizations_UserSeesOnlyMemberOrgs(t *testing.T) {
	if userToken == "" || adminToken == "" || userID == "" || adminUserID == "" || testDB == nil {
		t.Skip("tokens or db not available")
	}
	slugUser := "list-user-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "User Listed Org",
		"slug": slugUser,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create user org: %v code=%d body=%s", err, w.Code, w.Body.String())
	}

	slugAdmin := "list-admin-" + uuid.New().String()[:8]
	w, err = makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Admin Only Org",
		"slug": slugAdmin,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create admin org: %v code=%d body=%s", err, w.Code, w.Body.String())
	}

	w, err = makeRequest("GET", "/api/v1/organizations", nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("list = %d", w.Code)
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("parse: %v", err)
	}
	foundUser := false
	foundAdmin := false
	for _, o := range list {
		if s, ok := o["slug"].(string); ok && s == slugUser {
			foundUser = true
		}
		if s, ok := o["slug"].(string); ok && s == slugAdmin {
			foundAdmin = true
		}
	}
	if !foundUser {
		t.Fatal("user should see their org")
	}
	if foundAdmin {
		t.Fatal("user must not see admin org without membership")
	}
}

func TestTenantOrganizations_NonMemberCannotViewOrg(t *testing.T) {
	if userToken == "" || adminToken == "" || testDB == nil {
		t.Skip("tokens or db not available")
	}
	slug := "idor-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Secret Org",
		"slug": slug,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("GET", "/api/v1/organizations/"+orgID, nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-member GET org = %d, want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestTenantOrganizations_MemberCanViewOrg(t *testing.T) {
	if userToken == "" || adminToken == "" || userID == "" || testDB == nil {
		t.Skip("tokens or db not available")
	}
	slug := "memview-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Shared Org",
		"slug": slug,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create: err=%v code=%d body=%s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	res := testDB.Exec(`
		INSERT INTO organization_memberships (organization_id, user_id, role, created_at, updated_at)
		VALUES (?::uuid, ?::uuid, 'member', NOW(), NOW())
		ON CONFLICT (organization_id, user_id) DO NOTHING`,
		orgID, userID)
	if res.Error != nil {
		t.Fatalf("insert membership: %v", res.Error)
	}

	w, err = makeRequest("GET", "/api/v1/organizations/"+orgID, nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("member GET org = %d, body=%s", w.Code, w.Body.String())
	}
}

func TestTenantOrganizations_OrgAdminCanUpdateOrg(t *testing.T) {
	if userToken == "" || adminToken == "" || userID == "" || testDB == nil {
		t.Skip("tokens or db not available")
	}
	slug := "orgadmin-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Org With Admin",
		"slug": slug,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create: err=%v code=%d body=%s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	res := testDB.Exec(`
		INSERT INTO organization_memberships (organization_id, user_id, role, created_at, updated_at)
		VALUES (?::uuid, ?::uuid, 'admin', NOW(), NOW())
		ON CONFLICT (organization_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		orgID, userID)
	if res.Error != nil {
		t.Fatalf("insert admin membership: %v", res.Error)
	}

	newName := "Admin Updated " + uuid.New().String()[:6]
	w, err = makeRequest("PATCH", "/api/v1/organizations/"+orgID, map[string]interface{}{
		"name": newName,
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("org admin PATCH = %d, body=%s", w.Code, w.Body.String())
	}
}

func TestTenantOrganizations_MemberCannotUpdateOrg(t *testing.T) {
	if userToken == "" || adminToken == "" || userID == "" || testDB == nil {
		t.Skip("tokens or db not available")
	}
	slug := "memupd-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Patch Org",
		"slug": slug,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create: err=%v code=%d body=%s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	res := testDB.Exec(`
		INSERT INTO organization_memberships (organization_id, user_id, role, created_at, updated_at)
		VALUES (?::uuid, ?::uuid, 'member', NOW(), NOW())
		ON CONFLICT (organization_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		orgID, userID)
	if res.Error != nil {
		t.Fatalf("insert membership: %v", res.Error)
	}

	w, err = makeRequest("PATCH", "/api/v1/organizations/"+orgID, map[string]interface{}{
		"name": "Should Fail",
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("member PATCH = %d, want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestTenantOrganizations_AdminOrOwnerCanUpdateOrg(t *testing.T) {
	if adminToken == "" {
		t.Skip("admin token not available")
	}
	slug := "adminpatch-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Patchable Org",
		"slug": slug,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create: err=%v code=%d body=%s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	newName := "Patched Name " + uuid.New().String()[:6]
	w, err = makeRequest("PATCH", "/api/v1/organizations/"+orgID, map[string]interface{}{
		"name": newName,
	}, adminToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("owner PATCH = %d, body=%s", w.Code, w.Body.String())
	}
	var out map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if out["name"] != newName {
		t.Fatalf("name not updated: %v", out["name"])
	}
}

func TestTenantOrganizations_SlugUniquenessCaseInsensitive(t *testing.T) {
	if userToken == "" {
		t.Skip("user token not available")
	}
	base := "slug-uni-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "First",
		"slug": base,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("first create: %v %d %s", err, w.Code, w.Body.String())
	}

	w, err = makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Second",
		"slug": strings.ToUpper(base),
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate slug = %d, want 409 body=%s", w.Code, w.Body.String())
	}
}
