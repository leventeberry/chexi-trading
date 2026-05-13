//go:build integration

package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"goapi/services"
)

func TestOrgAPIKeys_CreateReturnsFullKeyOnce(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "apikey-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "API Key Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/api-keys", map[string]interface{}{
		"name":   "Integration key",
		"scopes": []string{"org:read", "org:write"},
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("POST api-key = %d body=%s", w.Code, w.Body.String())
	}
	var keyResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &keyResp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	secret, ok := keyResp["api_key"].(string)
	if !ok || secret == "" || len(secret) < 8 {
		t.Fatalf("expected api_key in create response, got %#v", keyResp["api_key"])
	}
	if secret[:5] != "orgk_" {
		t.Fatalf("expected orgk_ prefix, got %q", secret[:5])
	}
}

func TestOrgAPIKeys_ListNeverReturnsSecretOrHash(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "apikey-list-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "List Keys Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/api-keys", map[string]interface{}{
		"name":   "k1",
		"scopes": []string{"org:read"},
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create key: %v %d %s", err, w.Code, w.Body.String())
	}

	w, err = makeRequest("GET", "/api/v1/organizations/"+orgID+"/api-keys", nil, userToken)
	if err != nil || w.Code != http.StatusOK {
		t.Fatalf("list keys: %v %d %s", err, w.Code, w.Body.String())
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("parse list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 key, got %d", len(list))
	}
	if _, has := list[0]["api_key"]; has {
		t.Fatal("list must not include api_key")
	}
	if _, has := list[0]["key_hash"]; has {
		t.Fatal("list must not include key_hash")
	}
	if list[0]["key_prefix"] == nil || list[0]["key_prefix"] == "" {
		t.Fatal("expected key_prefix for display")
	}
}

func TestOrgAPIKeys_MemberCannotCreateOrRevoke(t *testing.T) {
	if userToken == "" || adminToken == "" || userID == "" || testDB == nil {
		t.Skip("tokens or db not available")
	}
	slug := "apikey-mem-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Member Key Org",
		"slug": slug,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
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

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/api-keys", map[string]interface{}{
		"name":   "nope",
		"scopes": []string{"org:read"},
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("member POST api-keys = %d, want 403 body=%s", w.Code, w.Body.String())
	}

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/api-keys", map[string]interface{}{
		"name":   "admin key",
		"scopes": []string{"org:read"},
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("admin create key: %v %d %s", err, w.Code, w.Body.String())
	}
	var keyResp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &keyResp)
	keyID := keyResp["id"].(string)

	w, err = makeRequest("DELETE", "/api/v1/organizations/"+orgID+"/api-keys/"+keyID, nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("member DELETE api-keys = %d, want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestOrgAPIKeys_CrossOrgBlocked(t *testing.T) {
	if userToken == "" || adminToken == "" {
		t.Skip("tokens or db not available")
	}
	slugA := "apikey-a-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Org A Keys",
		"slug": slugA,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create A: %v %d %s", err, w.Code, w.Body.String())
	}
	var orgA map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &orgA)
	orgAID := orgA["id"].(string)

	// userToken is not a member of org A — must not list API keys.
	w, err = makeRequest("GET", "/api/v1/organizations/"+orgAID+"/api-keys", nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-org list keys = %d, want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestOrgAPIKeys_RevokedKeyRejected(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "apikey-rev-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Revoke Key Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/api-keys", map[string]interface{}{
		"name":   "to-revoke",
		"scopes": []string{"org:read"},
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create key: %v %d %s", err, w.Code, w.Body.String())
	}
	var keyResp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &keyResp)
	secret := keyResp["api_key"].(string)
	keyID := keyResp["id"].(string)

	w, err = makeRequestAuth("GET", "/api/v1/organizations/"+orgID+"/notes", nil, "", secret)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("GET notes with key before revoke = %d body=%s", w.Code, w.Body.String())
	}

	w, err = makeRequest("DELETE", "/api/v1/organizations/"+orgID+"/api-keys/"+keyID, nil, userToken)
	if err != nil || w.Code != http.StatusNoContent {
		t.Fatalf("revoke: %v %d %s", err, w.Code, w.Body.String())
	}

	w, err = makeRequestAuth("GET", "/api/v1/organizations/"+orgID+"/notes", nil, "", secret)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET notes with revoked key = %d, want 401 body=%s", w.Code, w.Body.String())
	}
}

func TestOrgAPIKeys_ExpiredKeyRejected(t *testing.T) {
	if userToken == "" || userID == "" || testDB == nil {
		t.Skip("user token or db not available")
	}
	slug := "apikey-exp-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Expire Key Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	future := time.Now().UTC().Add(24 * time.Hour)
	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/api-keys", map[string]interface{}{
		"name":       "expires-soon",
		"scopes":     []string{"org:read"},
		"expires_at": future.Format(time.RFC3339Nano),
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create key: %v %d %s", err, w.Code, w.Body.String())
	}
	var keyResp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &keyResp)
	secret := keyResp["api_key"].(string)
	keyID := keyResp["id"].(string)

	res := testDB.Exec(`UPDATE organization_api_keys SET expires_at = NOW() - INTERVAL '1 hour', updated_at = NOW() WHERE id = ?::uuid`, keyID)
	if res.Error != nil {
		t.Fatalf("expire key: %v", res.Error)
	}

	w, err = makeRequestAuth("GET", "/api/v1/organizations/"+orgID+"/notes", nil, "", secret)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET notes with expired key = %d, want 401 body=%s", w.Code, w.Body.String())
	}
}

func TestOrgAPIKeys_InvalidScopesRejected(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "apikey-badscope-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Bad Scope Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/api-keys", map[string]interface{}{
		"name":   "bad",
		"scopes": []string{"org:read", "superuser"},
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid scopes = %d, want 400 body=%s", w.Code, w.Body.String())
	}

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/api-keys", map[string]interface{}{
		"name":   "empty",
		"scopes": []string{},
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty scopes = %d, want 400 body=%s", w.Code, w.Body.String())
	}
}

func TestOrgAPIKeys_ReadScopeCannotWrite(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "apikey-ro-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "RO Key Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/api-keys", map[string]interface{}{
		"name":   "read-only",
		"scopes": []string{"org:read"},
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create key: %v %d %s", err, w.Code, w.Body.String())
	}
	var keyResp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &keyResp)
	secret := keyResp["api_key"].(string)

	w, err = makeRequestAuth("POST", "/api/v1/organizations/"+orgID+"/notes", map[string]interface{}{
		"title": "x",
		"body":  "y",
	}, "", secret)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("POST note read-only key = %d, want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestOrgAPIKeys_WriteScopeCanUseNotes(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "apikey-rw-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "RW Key Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/api-keys", map[string]interface{}{
		"name":   "rw",
		"scopes": []string{"org:write"},
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create key: %v %d %s", err, w.Code, w.Body.String())
	}
	var keyResp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &keyResp)
	secret := keyResp["api_key"].(string)

	w, err = makeRequestAuth("GET", "/api/v1/organizations/"+orgID+"/notes", nil, "", secret)
	if err != nil || w.Code != http.StatusOK {
		t.Fatalf("GET notes: %v %d %s", err, w.Code, w.Body.String())
	}

	w, err = makeRequestAuth("POST", "/api/v1/organizations/"+orgID+"/notes", map[string]interface{}{
		"title": "via key",
		"body":  "body",
	}, "", secret)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("POST note: %v %d %s", err, w.Code, w.Body.String())
	}
}

func TestOrgAPIKeys_BearerAndXAPIKeyHeaders(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "apikey-hdr-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Header Key Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/api-keys", map[string]interface{}{
		"name":   "hdr",
		"scopes": []string{"org:read"},
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create key: %v %d %s", err, w.Code, w.Body.String())
	}
	var keyResp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &keyResp)
	secret := keyResp["api_key"].(string)

	w, err = makeRequestAuth("GET", "/api/v1/organizations/"+orgID+"/notes", nil, secret, "")
	if err != nil || w.Code != http.StatusOK {
		t.Fatalf("Bearer api key: %v %d %s", err, w.Code, w.Body.String())
	}

	w, err = makeRequestAuth("GET", "/api/v1/organizations/"+orgID+"/notes", nil, "", secret)
	if err != nil || w.Code != http.StatusOK {
		t.Fatalf("X-API-Key: %v %d %s", err, w.Code, w.Body.String())
	}
}

func TestOrgAPIKeys_CannotManageKeysWithKeyAuth(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "apikey-nomanage-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "No Manage Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/api-keys", map[string]interface{}{
		"name":   "k",
		"scopes": []string{"org:write"},
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create key: %v %d %s", err, w.Code, w.Body.String())
	}
	var keyResp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &keyResp)
	secret := keyResp["api_key"].(string)

	w, err = makeRequestAuth("GET", "/api/v1/organizations/"+orgID+"/api-keys", nil, "", secret)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("API key list keys = %d, want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestOrgAPIKeys_HashStableForLookup(t *testing.T) {
	raw := "orgk_testintegrationdummy"
	h := services.HashOrganizationAPIKey(raw)
	if len(h) != 64 {
		t.Fatalf("expected 64-char hex hash, got len=%d", len(h))
	}
}
