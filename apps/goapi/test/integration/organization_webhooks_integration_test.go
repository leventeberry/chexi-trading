//go:build integration

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"goapi/internal/webhooks"
)

func TestOrgWebhooks_CreateReturnsSecretOnce(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "wh-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "Webhook Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/webhooks", map[string]interface{}{
		"url":    "http://localhost:9999/h",
		"events": []string{"organization.updated"},
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("POST webhook = %d body=%s", w.Code, w.Body.String())
	}
	var wh map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &wh); err != nil {
		t.Fatalf("parse: %v", err)
	}
	sec, ok := wh["secret"].(string)
	if !ok || sec == "" {
		t.Fatalf("expected secret in create response, got %#v", wh["secret"])
	}
}

func TestOrgWebhooks_ListNeverReturnsSecret(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "wh-list-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "WH List Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/webhooks", map[string]interface{}{
		"url":    "http://localhost:9998/h",
		"events": []string{"organization.note.created"},
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create wh: %v %d %s", err, w.Code, w.Body.String())
	}

	w, err = makeRequest("GET", "/api/v1/organizations/"+orgID+"/webhooks", nil, userToken)
	if err != nil || w.Code != http.StatusOK {
		t.Fatalf("list: %v %d %s", err, w.Code, w.Body.String())
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 webhook, got %d", len(list))
	}
	if _, has := list[0]["secret"]; has {
		t.Fatal("list must not include secret")
	}
	if _, has := list[0]["secret_ciphertext"]; has {
		t.Fatal("list must not include secret_ciphertext")
	}
}

func TestOrgWebhooks_InvalidURLRejected(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "wh-badurl-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "WH Bad URL",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/webhooks", map[string]interface{}{
		"url":    "http://example.com/nope",
		"events": []string{"organization.updated"},
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestOrgWebhooks_MemberCannotManage(t *testing.T) {
	if userToken == "" || adminToken == "" || userID == "" || testDB == nil {
		t.Skip("tokens or db not available")
	}
	slug := "wh-mem-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "WH Mem Org",
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

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/webhooks", map[string]interface{}{
		"url":    "http://localhost:9997/h",
		"events": []string{"organization.updated"},
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("member POST webhooks = %d, want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestOrgWebhooks_CrossOrgBlocked(t *testing.T) {
	if userToken == "" || adminToken == "" {
		t.Skip("tokens not available")
	}
	slugA := "wh-a-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "WH Org A",
		"slug": slugA,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create A: %v %d %s", err, w.Code, w.Body.String())
	}
	var orgA map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &orgA)
	orgAID := orgA["id"].(string)

	w, err = makeRequest("GET", "/api/v1/organizations/"+orgAID+"/webhooks", nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-org list webhooks = %d, want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestOrgWebhooks_DeliverySignedAndHistoryScoped(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	var received atomic.Int32
	var savedSecret string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		received.Add(1)
		tsStr := r.Header.Get("X-Webhook-Timestamp")
		sig := r.Header.Get("X-Webhook-Signature")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		unix, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		want := webhooks.SignPayload([]byte(savedSecret), unix, body)
		if want != sig {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	slug := "wh-sign-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "WH Sign Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/webhooks", map[string]interface{}{
		"url":    srv.URL + "/hook",
		"events": []string{"organization.updated"},
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create webhook: %v %d %s", err, w.Code, w.Body.String())
	}
	var wh map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &wh)
	whID := wh["id"].(string)
	savedSecret, _ = wh["secret"].(string)
	if savedSecret == "" {
		t.Fatal("missing secret from create")
	}

	w, err = makeRequest("PATCH", "/api/v1/organizations/"+orgID, map[string]interface{}{
		"name": "WH Sign Org Renamed",
	}, userToken)
	if err != nil || w.Code != http.StatusOK {
		t.Fatalf("patch org: %v %d %s", err, w.Code, w.Body.String())
	}

	if received.Load() != 1 {
		t.Fatalf("expected 1 webhook POST, got %d", received.Load())
	}

	w, err = makeRequest("GET", "/api/v1/organizations/"+orgID+"/webhooks/"+whID+"/deliveries", nil, userToken)
	if err != nil || w.Code != http.StatusOK {
		t.Fatalf("deliveries: %v %d %s", err, w.Code, w.Body.String())
	}
	var deliveries []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &deliveries); err != nil {
		t.Fatalf("parse deliveries: %v", err)
	}
	if len(deliveries) < 1 {
		t.Fatalf("expected deliveries, got %d", len(deliveries))
	}
	if deliveries[0]["status"] != "delivered" {
		t.Fatalf("expected delivered, got %#v", deliveries[0]["status"])
	}
}

func TestOrgWebhooks_FailedDeliveryRecordsError(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	slug := "wh-fail-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "WH Fail Org",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/webhooks", map[string]interface{}{
		"url":    srv.URL + "/hook",
		"events": []string{"organization.updated"},
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create webhook: %v %d %s", err, w.Code, w.Body.String())
	}
	var wh map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &wh)
	whID := wh["id"].(string)

	w, err = makeRequest("PATCH", "/api/v1/organizations/"+orgID, map[string]interface{}{
		"name": "WH Fail Org Patch",
	}, userToken)
	if err != nil || w.Code != http.StatusOK {
		t.Fatalf("patch org: %v %d %s", err, w.Code, w.Body.String())
	}

	w, err = makeRequest("GET", "/api/v1/organizations/"+orgID+"/webhooks/"+whID+"/deliveries", nil, userToken)
	if err != nil || w.Code != http.StatusOK {
		t.Fatalf("deliveries: %v %d %s", err, w.Code, w.Body.String())
	}
	var deliveries []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &deliveries); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(deliveries) < 1 {
		t.Fatal("expected a delivery row")
	}
	attempts, _ := deliveries[0]["attempts"].(float64)
	if attempts < 1 {
		t.Fatalf("expected attempts >= 1, got %v", deliveries[0]["attempts"])
	}
	le, ok := deliveries[0]["last_error"].(string)
	if !ok || le == "" {
		t.Fatalf("expected last_error, got %#v", deliveries[0]["last_error"])
	}
	st, _ := deliveries[0]["status"].(string)
	if st != "pending" && st != "failed" {
		t.Fatalf("unexpected status %q", st)
	}
}

func TestOrgWebhooks_PatchNormalUpdateNoSecretInResponse(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "wh-patch-norm-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "WH Patch Norm",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/webhooks", map[string]interface{}{
		"url":    "http://localhost:9910/h",
		"events": []string{"organization.updated"},
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create webhook: %v %d %s", err, w.Code, w.Body.String())
	}
	var whCreate map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &whCreate)
	whID := whCreate["id"].(string)

	w, err = makeRequest("PATCH", "/api/v1/organizations/"+orgID+"/webhooks/"+whID, map[string]interface{}{
		"url":     "http://localhost:9911/patched",
		"events":  []string{"organization.updated", "organization.note.created"},
		"enabled": false,
	}, userToken)
	if err != nil || w.Code != http.StatusOK {
		t.Fatalf("PATCH webhook: %v %d %s", err, w.Code, w.Body.String())
	}
	var patched map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &patched); err != nil {
		t.Fatalf("parse patch: %v", err)
	}
	if _, has := patched["secret"]; has {
		t.Fatal("normal PATCH must not include secret field")
	}
	if patched["url"] != "http://localhost:9911/patched" {
		t.Fatalf("url not updated: %#v", patched["url"])
	}
	ev, _ := patched["events"].([]interface{})
	if len(ev) != 2 {
		t.Fatalf("events: want 2, got %#v", patched["events"])
	}
	if patched["enabled"] != false {
		t.Fatalf("enabled: want false, got %#v", patched["enabled"])
	}

	w, err = makeRequest("GET", "/api/v1/organizations/"+orgID+"/webhooks", nil, userToken)
	if err != nil || w.Code != http.StatusOK {
		t.Fatalf("list: %v %d %s", err, w.Code, w.Body.String())
	}
	var list []map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("want 1 webhook, got %d", len(list))
	}
	if _, has := list[0]["secret"]; has {
		t.Fatal("list must not include secret after PATCH")
	}
	if list[0]["url"] != "http://localhost:9911/patched" {
		t.Fatalf("list url mismatch: %#v", list[0]["url"])
	}
}

func TestOrgWebhooks_PatchRotateSecretReturnsOnceListOmitsSecret(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "wh-patch-rot-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "WH Patch Rotate",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/webhooks", map[string]interface{}{
		"url":    "http://localhost:9920/h",
		"events": []string{"organization.updated"},
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create webhook: %v %d %s", err, w.Code, w.Body.String())
	}
	var whCreate map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &whCreate)
	whID := whCreate["id"].(string)
	origSecret, _ := whCreate["secret"].(string)
	if origSecret == "" {
		t.Fatal("missing create secret")
	}

	w, err = makeRequest("PATCH", "/api/v1/organizations/"+orgID+"/webhooks/"+whID, map[string]interface{}{
		"rotate_secret": true,
	}, userToken)
	if err != nil || w.Code != http.StatusOK {
		t.Fatalf("PATCH rotate: %v %d %s", err, w.Code, w.Body.String())
	}
	var rotated map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &rotated); err != nil {
		t.Fatalf("parse: %v", err)
	}
	newSec, ok := rotated["secret"].(string)
	if !ok || newSec == "" {
		t.Fatalf("rotate PATCH must return new secret, got %#v", rotated["secret"])
	}
	if newSec == origSecret {
		t.Fatal("rotated secret should differ from original")
	}

	w, err = makeRequest("GET", "/api/v1/organizations/"+orgID+"/webhooks", nil, userToken)
	if err != nil || w.Code != http.StatusOK {
		t.Fatalf("list: %v %d %s", err, w.Code, w.Body.String())
	}
	var list []map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("want 1 webhook, got %d", len(list))
	}
	if _, has := list[0]["secret"]; has {
		t.Fatal("list must not include secret after rotate")
	}
	if _, has := list[0]["secret_ciphertext"]; has {
		t.Fatal("list must not include secret_ciphertext")
	}
	if int(list[0]["secret_key_version"].(float64)) < 2 {
		t.Fatalf("expected secret_key_version >= 2, got %#v", list[0]["secret_key_version"])
	}

	w, err = makeRequest("PATCH", "/api/v1/organizations/"+orgID+"/webhooks/"+whID, map[string]interface{}{
		"enabled": true,
	}, userToken)
	if err != nil || w.Code != http.StatusOK {
		t.Fatalf("PATCH after rotate: %v %d %s", err, w.Code, w.Body.String())
	}
	var after map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &after)
	if _, has := after["secret"]; has {
		t.Fatal("non-rotate PATCH must not return secret")
	}
}

func TestOrgWebhooks_PatchMemberForbidden(t *testing.T) {
	if userToken == "" || adminToken == "" || userID == "" || testDB == nil {
		t.Skip("tokens or db not available")
	}
	slug := "wh-patch-mem-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "WH Patch Mem",
		"slug": slug,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/webhooks", map[string]interface{}{
		"url":    "http://localhost:9930/h",
		"events": []string{"organization.updated"},
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("admin create webhook: %v %d %s", err, w.Code, w.Body.String())
	}
	var wh map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &wh)
	whID := wh["id"].(string)

	res := testDB.Exec(`
		INSERT INTO organization_memberships (organization_id, user_id, role, created_at, updated_at)
		VALUES (?::uuid, ?::uuid, 'member', NOW(), NOW())
		ON CONFLICT (organization_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		orgID, userID)
	if res.Error != nil {
		t.Fatalf("membership: %v", res.Error)
	}

	w, err = makeRequest("PATCH", "/api/v1/organizations/"+orgID+"/webhooks/"+whID, map[string]interface{}{
		"url": "http://localhost:9931/h",
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("member PATCH webhook = %d, want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestOrgWebhooks_PatchCrossOrgNonMemberForbidden(t *testing.T) {
	if userToken == "" || adminToken == "" {
		t.Skip("tokens not available")
	}
	slugA := "wh-patch-xo-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "WH Patch OrgA",
		"slug": slugA,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create A: %v %d %s", err, w.Code, w.Body.String())
	}
	var orgA map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &orgA)
	orgAID := orgA["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgAID+"/webhooks", map[string]interface{}{
		"url":    "http://localhost:9940/h",
		"events": []string{"organization.updated"},
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create wh: %v %d %s", err, w.Code, w.Body.String())
	}
	var wh map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &wh)
	whID := wh["id"].(string)

	w, err = makeRequest("PATCH", "/api/v1/organizations/"+orgAID+"/webhooks/"+whID, map[string]interface{}{
		"url": "http://localhost:9941/h",
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-org PATCH = %d, want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestOrgWebhooks_PatchForeignWebhookIDNotFound(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "wh-patch-404-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "WH Patch 404",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	fakeWhID := uuid.New().String()
	w, err = makeRequest("PATCH", "/api/v1/organizations/"+orgID+"/webhooks/"+fakeWhID, map[string]interface{}{
		"url": "http://localhost:9950/h",
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("PATCH foreign webhook id = %d, want 404 body=%s", w.Code, w.Body.String())
	}
}

func TestOrgWebhooks_DeleteNoContentAndRemovedFromList(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "wh-del-ok-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "WH Delete OK",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/webhooks", map[string]interface{}{
		"url":    "http://localhost:9960/h",
		"events": []string{"organization.updated"},
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create webhook: %v %d %s", err, w.Code, w.Body.String())
	}
	var wh map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &wh)
	whID := wh["id"].(string)

	w, err = makeRequest("DELETE", "/api/v1/organizations/"+orgID+"/webhooks/"+whID, nil, userToken)
	if err != nil || w.Code != http.StatusNoContent {
		t.Fatalf("DELETE: %v %d %s", err, w.Code, w.Body.String())
	}

	w, err = makeRequest("GET", "/api/v1/organizations/"+orgID+"/webhooks", nil, userToken)
	if err != nil || w.Code != http.StatusOK {
		t.Fatalf("list: %v %d %s", err, w.Code, w.Body.String())
	}
	var list []map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatalf("want 0 webhooks after delete, got %d", len(list))
	}
}

func TestOrgWebhooks_DeleteMemberForbidden(t *testing.T) {
	if userToken == "" || adminToken == "" || userID == "" || testDB == nil {
		t.Skip("tokens or db not available")
	}
	slug := "wh-del-mem-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "WH Delete Mem",
		"slug": slug,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgID+"/webhooks", map[string]interface{}{
		"url":    "http://localhost:9970/h",
		"events": []string{"organization.updated"},
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create webhook: %v %d %s", err, w.Code, w.Body.String())
	}
	var wh map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &wh)
	whID := wh["id"].(string)

	res := testDB.Exec(`
		INSERT INTO organization_memberships (organization_id, user_id, role, created_at, updated_at)
		VALUES (?::uuid, ?::uuid, 'member', NOW(), NOW())
		ON CONFLICT (organization_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		orgID, userID)
	if res.Error != nil {
		t.Fatalf("membership: %v", res.Error)
	}

	w, err = makeRequest("DELETE", "/api/v1/organizations/"+orgID+"/webhooks/"+whID, nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("member DELETE = %d, want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestOrgWebhooks_DeleteCrossOrgNonMemberForbidden(t *testing.T) {
	if userToken == "" || adminToken == "" {
		t.Skip("tokens not available")
	}
	slugA := "wh-del-xo-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "WH Del OrgA",
		"slug": slugA,
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create A: %v %d %s", err, w.Code, w.Body.String())
	}
	var orgA map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &orgA)
	orgAID := orgA["id"].(string)

	w, err = makeRequest("POST", "/api/v1/organizations/"+orgAID+"/webhooks", map[string]interface{}{
		"url":    "http://localhost:9980/h",
		"events": []string{"organization.updated"},
	}, adminToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create wh: %v %d %s", err, w.Code, w.Body.String())
	}
	var wh map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &wh)
	whID := wh["id"].(string)

	w, err = makeRequest("DELETE", "/api/v1/organizations/"+orgAID+"/webhooks/"+whID, nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-org DELETE = %d, want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestOrgWebhooks_DeleteForeignWebhookIDNotFound(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("user token not seeded")
	}
	slug := "wh-del-404-" + uuid.New().String()[:8]
	w, err := makeRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name": "WH Del 404",
		"slug": slug,
	}, userToken)
	if err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create org: %v %d %s", err, w.Code, w.Body.String())
	}
	var created map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	orgID := created["id"].(string)

	fakeWhID := uuid.New().String()
	w, err = makeRequest("DELETE", "/api/v1/organizations/"+orgID+"/webhooks/"+fakeWhID, nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("DELETE foreign webhook = %d, want 404 body=%s", w.Code, w.Body.String())
	}
}
