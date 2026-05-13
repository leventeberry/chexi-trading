//go:build integration

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestGetMeUnauthenticated(t *testing.T) {
	w, err := makeRequest("GET", "/api/v1/users/me", nil, "")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUsersAuthz_GetAdminUsersForbiddenForRegularUser(t *testing.T) {
	if userToken == "" {
		t.Skip("user token not available")
	}
	w, err := makeRequest("GET", "/api/v1/admin/users", nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUsersAuthz_UserCannotReadAnotherUserViaAdmin(t *testing.T) {
	if userToken == "" || adminUserID == "" {
		t.Skip("tokens or admin id not available")
	}
	url := fmt.Sprintf("/api/v1/admin/users/%s", adminUserID)
	w, err := makeRequest("GET", url, nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cross-user read, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUsersAuthz_UserCannotUpdateAnotherUserViaAdmin(t *testing.T) {
	if userToken == "" || adminUserID == "" {
		t.Skip("tokens or admin id not available")
	}
	url := fmt.Sprintf("/api/v1/admin/users/%s", adminUserID)
	w, err := makeRequest("PUT", url, map[string]interface{}{
		"first_name": "Hax",
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cross-user update, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUsersAuthz_AdminCanReadAnotherUser(t *testing.T) {
	if adminToken == "" || userID == "" {
		t.Skip("admin token or user id not available")
	}
	url := fmt.Sprintf("/api/v1/admin/users/%s", userID)
	w, err := makeRequest("GET", url, nil, adminToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
		return
	}
	var u map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &u); err != nil {
		t.Fatalf("json: %v", err)
	}
	if u["email"] != "john.doe@test.com" {
		t.Errorf("expected john.doe@test.com, got %v", u["email"])
	}
}

func TestUsersAuthz_MalformedAdminUserIDReturns400(t *testing.T) {
	if adminToken == "" {
		t.Skip("admin token not available")
	}
	w, err := makeRequest("GET", "/api/v1/admin/users/not-a-uuid", nil, adminToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUsersAuthz_RegularUserUnknownUsersSubpathIs404(t *testing.T) {
	if userToken == "" {
		t.Skip("user token not available")
	}
	// Legacy /users/:id removed; self route has no id param.
	w, err := makeRequest("GET", "/api/v1/users/not-a-uuid", nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown route, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUsersAuthz_AdminUpdateNonExistentUserReturns404(t *testing.T) {
	if adminToken == "" {
		t.Skip("admin token not available")
	}
	url := "/api/v1/admin/users/00000000-0000-4000-8000-00000000beef"
	w, err := makeRequest("PUT", url, map[string]interface{}{"first_name": "Nope"}, adminToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUsersAuthz_EventsListForbiddenForRegularUser(t *testing.T) {
	if userToken == "" {
		t.Skip("user token not available")
	}
	w, err := makeRequest("GET", "/api/v1/events", nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUsersAuthz_EventsListOKForAdmin(t *testing.T) {
	if adminToken == "" {
		t.Skip("admin token not available")
	}
	w, err := makeRequest("GET", "/api/v1/events?limit=5", nil, adminToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUsersAuthz_EventsIngestForbiddenForRegularUser(t *testing.T) {
	if userToken == "" {
		t.Skip("user token not available")
	}
	w, err := makeRequest("POST", "/api/v1/events", map[string]interface{}{
		"event_type": "test.event",
	}, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUsersAuthz_EventsIngestAcceptedForAdmin(t *testing.T) {
	if adminToken == "" {
		t.Skip("admin token not available")
	}
	w, err := makeRequest("POST", "/api/v1/events", map[string]interface{}{
		"event_type": "test.event",
		"metadata":   map[string]interface{}{},
	}, adminToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d body=%s", w.Code, w.Body.String())
	}
}
