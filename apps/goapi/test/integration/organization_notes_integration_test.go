//go:build integration

package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestTenantOrgNotes_MemberCanCreateAndList(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "notes-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Notes Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/notes", map[string]interface{}{
		"title": "Hello",
		"body":  "World",
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("POST note = %d body=%s", w.Code, w.Body.String())
	}
	var note map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &note); err != nil {
		t.Fatalf("parse note: %v", err)
	}
	if note["organization_id"].(string) != orgID {
		t.Fatalf("tenant organization_id mismatch: %v", note["organization_id"])
	}

	w, err = makeRequest("GET", "/api/v1/organizations/"+orgID+"/notes", nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("GET notes = %d body=%s", w.Code, w.Body.String())
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("parse list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 note, got %d", len(list))
	}
	if list[0]["organization_id"].(string) != orgID {
		t.Fatalf("list tenant org id: %v", list[0]["organization_id"])
	}
}

func TestTenantOrgNotes_NonMemberCannotList(t *testing.T) {
	if userToken == "" || adminToken == "" || testDB == nil {
		t.Skip("tokens or db not available")
	}
	slug := "notes-private-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Private Notes Org",
		"slug": slug,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create: err=%v code=%d body=%s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("GET", "/api/v1/organizations/"+orgID+"/notes", nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-member GET notes = %d, want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestTenantOrgNotes_OrgAdminCanDelete(t *testing.T) {
	if userToken == "" || userID == "" || testDB == nil {
		t.Skip("tokens or db not available")
	}
	slug := "notes-del-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Del Notes Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: err=%v code=%d body=%s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	res := testDB.Exec(`
		UPDATE organization_memberships SET role = 'admin', updated_at = NOW()
		WHERE organization_id = ?::uuid AND user_id = ?::uuid`,
		orgID, userID)
	if res.Error != nil {
		t.Fatalf("promote admin: %v", res.Error)
	}

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/notes", map[string]interface{}{
		"title": "Delme",
		"body":  "x",
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create note: %v %d %s", err, w.Code, w.Body.String())
	}
	var note map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &note)
	noteID := note["id"].(string)

	w, err = makeRequest("DELETE", "/api/v1/organizations/"+orgID+"/notes/"+noteID, nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusNoContent {
		t.Fatalf("DELETE note = %d body=%s", w.Code, w.Body.String())
	}
}

func TestTenantOrgNotes_MemberCannotDelete(t *testing.T) {
	if userToken == "" || adminToken == "" || userID == "" || testDB == nil {
		t.Skip("tokens or db not available")
	}
	slug := "notes-memdel-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Member Del Org",
		"slug": slug,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: err=%v code=%d body=%s", err, w.Code, w.Body.String())
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
		t.Fatalf("membership: %v", res.Error)
	}

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/notes", map[string]interface{}{
		"title": "x",
		"body":  "y",
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("admin create note: %v %d %s", err, w.Code, w.Body.String())
	}
	var note map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &note)
	noteID := note["id"].(string)

	w, err = makeRequest("DELETE", "/api/v1/organizations/"+orgID+"/notes/"+noteID, nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("member DELETE = %d, want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestTenantOrgNotes_CrossOrgNoteIDDoesNotDelete(t *testing.T) {
	if userToken == "" || adminToken == "" || userID == "" || testDB == nil {
		t.Skip("tokens or db not available")
	}
	slugA := "org-a-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Org A",
		"slug": slugA,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create A: err=%v code=%d body=%s", err, w.Code, w.Body.String())
	}
	var orgA map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &orgA)
	orgAID := orgA["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgAID+"/notes", map[string]interface{}{
		"title": "secret",
		"body":  "note",
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("note A: err=%v code=%d body=%s", err, w.Code, w.Body.String())
	}
	var note map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &note)
	noteID := note["id"].(string)

	slugB := "org-b-" + uuid.New().String()[:8]
	w, err = makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Org B",
		"slug": slugB,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create B: err=%v code=%d body=%s", err, w.Code, w.Body.String())
	}
	var orgB map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &orgB)
	orgBID := orgB["id"].(string)

	res := testDB.Exec(`
		INSERT INTO organization_memberships (organization_id, user_id, role, created_at, updated_at)
		VALUES (?::uuid, ?::uuid, 'admin', NOW(), NOW())
		ON CONFLICT (organization_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		orgBID, userID)
	if res.Error != nil {
		t.Fatalf("membership B: %v", res.Error)
	}

	w, err = makeRequest("DELETE", "/api/v1/organizations/"+orgBID+"/notes/"+noteID, nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-org DELETE = %d, want 404 body=%s", w.Code, w.Body.String())
	}
}

func TestTenantOrgNotes_ResolvedBySlug(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "slug-notes-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Slug Notes Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: err=%v code=%d body=%s", err, w.Code, w.Body.String())
	}

	w, err = makeRequest("GET", "/api/v1/organizations/"+slug+"/notes", nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("GET notes by slug = %d body=%s", w.Code, w.Body.String())
	}
}

func TestTenantOrgNotes_ContextAvailableDownstream(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "ctx-notes-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Ctx Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: err=%v code=%d body=%s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/notes", map[string]interface{}{
		"title": "ctx",
		"body":  "proof",
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create note: err=%v code=%d body=%s", err, w.Code, w.Body.String())
	}
	var note map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &note); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if note["organization_id"].(string) != orgID {
		t.Fatal("downstream response should echo resolved organization_id from tenant context")
	}
}
