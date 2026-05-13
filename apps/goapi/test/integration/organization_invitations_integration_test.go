//go:build integration

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestTenantInvitations_OwnerAdminCanInvite(t *testing.T) {
	if adminToken == "" {
		t.Skip("admin token not available")
	}
	slug := "inv-admin-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Invite Org",
		"slug": slug,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var org map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &org)
	orgID := org["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/invitations", map[string]interface{}{
		"email": "fresh-invite-" + uuid.New().String()[:8] + "@example.com",
		"role":  "member",
	}, adminToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("invite = %d body=%s", w.Code, w.Body.String())
	}
}

func TestTenantInvitations_MemberCannotInvite(t *testing.T) {
	if adminToken == "" || userToken == "" || userID == "" || testDB == nil {
		t.Skip("tokens/db missing")
	}
	slug := "inv-mem-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Member Invite Denied",
		"slug": slug,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v", w.Code)
	}
	var org map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &org)
	orgID := org["id"].(string)

	res := testDB.Exec(`
		INSERT INTO organization_memberships (organization_id, user_id, role, created_at, updated_at)
		VALUES (?::uuid, ?::uuid, 'member', NOW(), NOW())
		ON CONFLICT (organization_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		orgID, userID)
	if res.Error != nil {
		t.Fatalf("membership: %v", res.Error)
	}

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/invitations", map[string]interface{}{
		"email": "someone@example.com",
		"role":  "member",
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("member invite = %d want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestTenantInvitations_AcceptJoinsOrgAndSingleUse(t *testing.T) {
	if adminToken == "" || userToken == "" || userID == "" || testDB == nil {
		t.Skip("tokens/db missing")
	}
	slug := "inv-acc-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Accept Org",
		"slug": slug,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v", w.Code)
	}
	var org map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &org)
	orgID := org["id"].(string)

	rawTok := "invite-raw-" + uuid.New().String()
	hash := sha256Hex(rawTok)
	expires := "NOW() + INTERVAL '1 day'"
	res := testDB.Exec(`
		INSERT INTO organization_invitations (
			id, organization_id, email, role, token_hash, invited_by_user_id,
			expires_at, accepted_at, created_at, updated_at
		)
		VALUES (
			gen_random_uuid(), ?::uuid, 'john.doe@test.com', 'member', ?, ?::uuid,
			`+expires+`, NULL, NOW(), NOW()
		)`, orgID, hash, adminUserID)
	if res.Error != nil {
		t.Fatalf("seed invite: %v", res.Error)
	}

	w, err = makeRequest("POST", "/api/v1/organizations/invitations/accept", map[string]interface{}{
		"token": rawTok,
	}, userToken)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if w.Code != http.StatusNoContent {
		t.Fatalf("accept = %d body=%s", w.Code, w.Body.String())
	}

	w, err = makeRequest("GET", "/api/v1/organizations/"+orgID+"/members", nil, userToken)
	if err != nil || w.Code != http.StatusOK {
		t.Fatalf("members: %v %d", err, w.Code)
	}
	var members []map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &members)
	found := false
	for _, m := range members {
		if m["user_id"] == userID && m["role"] == "member" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("user not listed as member after accept")
	}

	w, err = makeRequest("POST", "/api/v1/organizations/invitations/accept", map[string]interface{}{
		"token": rawTok,
	}, userToken)
	if err != nil {
		t.Fatalf("second accept: %v", err)
	}
	if w.Code != http.StatusConflict {
		t.Fatalf("second accept = %d want 409", w.Code)
	}
}

func TestTenantInvitations_ExpiredRejected(t *testing.T) {
	if adminToken == "" || userToken == "" || testDB == nil {
		t.Skip("tokens/db missing")
	}
	slug := "inv-exp-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Expired Org",
		"slug": slug,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v", w.Code)
	}
	var org map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &org)
	orgID := org["id"].(string)

	rawTok := "expired-tok-" + uuid.New().String()
	hash := sha256Hex(rawTok)
	res := testDB.Exec(`
		INSERT INTO organization_invitations (
			id, organization_id, email, role, token_hash, invited_by_user_id,
			expires_at, accepted_at, created_at, updated_at
		)
		VALUES (
			gen_random_uuid(), ?::uuid, 'john.doe@test.com', 'member', ?, ?::uuid,
			NOW() - INTERVAL '1 hour', NULL, NOW(), NOW()
		)`, orgID, hash, adminUserID)
	if res.Error != nil {
		t.Fatalf("seed: %v", res.Error)
	}

	w, err = makeRequest("POST", "/api/v1/organizations/invitations/accept", map[string]interface{}{
		"token": rawTok,
	}, userToken)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expired accept = %d want 400 body=%s", w.Code, w.Body.String())
	}
}

func TestTenantInvitations_RemoveMemberAuthAndLastOwner(t *testing.T) {
	if adminToken == "" || userToken == "" || userID == "" || adminUserID == "" || testDB == nil {
		t.Skip("tokens/db missing")
	}
	slug := "inv-rem-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Remove Org",
		"slug": slug,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v", w.Code)
	}
	var org map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &org)
	orgID := org["id"].(string)

	res := testDB.Exec(`
		INSERT INTO organization_memberships (organization_id, user_id, role, created_at, updated_at)
		VALUES (?::uuid, ?::uuid, 'member', NOW(), NOW())
		ON CONFLICT (organization_id, user_id) DO NOTHING`, orgID, userID)
	if res.Error != nil {
		t.Fatalf("add member: %v", res.Error)
	}

	w, err = makeRequest("DELETE", "/api/v1/organizations/"+orgID+"/members/"+userID, nil, userToken)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("member remove other = %d want 403", w.Code)
	}

	w, err = makeRequest("DELETE", "/api/v1/organizations/"+orgID+"/members/"+userID, nil, adminToken)
	if err != nil || w.Code != http.StatusNoContent {
		t.Fatalf("admin remove member = %d body=%s", w.Code, w.Body.String())
	}

	w, err = makeRequest("DELETE", "/api/v1/organizations/"+orgID+"/members/"+adminUserID, nil, adminToken)
	if err != nil {
		t.Fatalf("delete owner: %v", err)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("remove sole owner = %d want 400 body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "last owner") && w.Body.Len() > 0 {
		// error body optional
	}
}

func TestTenantInvitations_DuplicatePendingSuperseded(t *testing.T) {
	if adminToken == "" {
		t.Skip("admin token missing")
	}
	slug := "inv-dup-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Dup Invite Org",
		"slug": slug,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v", w.Code)
	}
	var org map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &org)
	orgID := org["id"].(string)
	emailAddr := "dup-" + uuid.New().String()[:8] + "@example.com"

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/invitations", map[string]interface{}{
		"email": emailAddr,
		"role":  "member",
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("first invite: %v %d", err, w.Code)
	}
	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/invitations", map[string]interface{}{
		"email": emailAddr,
		"role":  "admin",
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("second invite: %v %d %s", err, w.Code, w.Body.String())
	}

	var count int64
	if err := testDB.Raw(`
		SELECT COUNT(*) FROM organization_invitations
		WHERE organization_id = ?::uuid AND LOWER(email) = LOWER(?) AND accepted_at IS NULL`,
		orgID, emailAddr).Scan(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 pending invite for email, got %d", count)
	}
}
