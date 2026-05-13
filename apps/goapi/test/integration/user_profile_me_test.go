//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// makeRawRequest sends a pre-serialized JSON body (for unknown-field / strict JSON cases).
func makeRawRequest(method, path string, rawJSON []byte, token string) (*httptest.ResponseRecorder, error) {
	req, err := http.NewRequest(method, path, bytes.NewReader(rawJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	return w, nil
}

func TestUserProfileMe_GetHasProfileAndSettings(t *testing.T) {
	if userToken == "" {
		t.Skip("user token not available")
	}
	w, err := makeRequest("GET", "/api/v1/users/me", nil, userToken)
	if err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var envelope map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	prof, ok := envelope["profile"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected profile object, got %#v", envelope["profile"])
	}
	if prof["email"] == nil {
		t.Fatal("expected profile.email")
	}
	settings, ok := envelope["settings"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected settings object, got %#v", envelope["settings"])
	}
	if settings["theme"] == nil {
		t.Fatal("expected settings.theme")
	}
}

func TestUserProfileMe_PatchUpdatesProfileAndSettings(t *testing.T) {
	if userToken == "" {
		t.Skip("user token not available")
	}
	body := map[string]interface{}{
		"profile": map[string]interface{}{
			"display_name": "Johnny D",
			"timezone":     "America/New_York",
			"locale":       "en-US",
		},
		"settings": map[string]interface{}{
			"theme":                    "dark",
			"marketing_email_opt_in":   true,
			"notification_preferences": map[string]interface{}{"digest": "weekly"},
			"extra_settings":           map[string]interface{}{"foo": "bar"},
		},
	}
	w, err := makeRequest(http.MethodPatch, "/api/v1/users/me", body, userToken)
	if err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var out map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	prof := out["profile"].(map[string]interface{})
	if prof["display_name"] != "Johnny D" {
		t.Fatalf("display_name: %v", prof["display_name"])
	}
	if prof["timezone"] != "America/New_York" {
		t.Fatalf("timezone: %v", prof["timezone"])
	}
	if prof["locale"] != "en-US" {
		t.Fatalf("locale: %v", prof["locale"])
	}
	st := out["settings"].(map[string]interface{})
	if st["theme"] != "dark" {
		t.Fatalf("theme: %v", st["theme"])
	}
}

func TestUserProfileMe_PatchRejectsUnknownRootField(t *testing.T) {
	if userToken == "" {
		t.Skip("user token not available")
	}
	w, err := makeRawRequest(http.MethodPatch, "/api/v1/users/me", []byte(`{"profile":{"display_name":"X"},"role":"admin"}`), userToken)
	if err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUserProfileMe_PatchRejectsEmailInProfile(t *testing.T) {
	if userToken == "" {
		t.Skip("user token not available")
	}
	w, err := makeRawRequest(http.MethodPatch, "/api/v1/users/me", []byte(`{"profile":{"email":"hijack@test.com"}}`), userToken)
	if err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUserProfileMe_PatchInvalidAvatarURL(t *testing.T) {
	if userToken == "" {
		t.Skip("user token not available")
	}
	w, err := makeRawRequest(http.MethodPatch, "/api/v1/users/me", []byte(`{"profile":{"avatar_url":"ftp://example.com/x.png"}}`), userToken)
	if err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUserProfileMe_PatchInvalidTimezone(t *testing.T) {
	if userToken == "" {
		t.Skip("user token not available")
	}
	w, err := makeRawRequest(http.MethodPatch, "/api/v1/users/me", []byte(`{"profile":{"timezone":"Not/A/Zone"}}`), userToken)
	if err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUserProfileMe_PatchInvalidLocale(t *testing.T) {
	if userToken == "" {
		t.Skip("user token not available")
	}
	w, err := makeRawRequest(http.MethodPatch, "/api/v1/users/me", []byte(`{"profile":{"locale":"123"}}`), userToken)
	if err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUserProfileMe_PatchEmptyObjectRejected(t *testing.T) {
	if userToken == "" {
		t.Skip("user token not available")
	}
	w, err := makeRawRequest(http.MethodPatch, "/api/v1/users/me", []byte(`{}`), userToken)
	if err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// Users/me is always the JWT subject; a second user sees only their own record.
func TestUserProfileMe_IsolatedPerUser(t *testing.T) {
	email := fmt.Sprintf("other-me-%d@test.com", time.Now().UnixNano())
	regBody := map[string]interface{}{
		"first_name": "Other", "last_name": "User", "email": email,
		"password": "Password123!", "role": "user",
	}
	wReg, err := makeRequest("POST", "/api/v1/register", regBody, "")
	if err != nil {
		t.Fatal(err)
	}
	if wReg.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", wReg.Code, wReg.Body.String())
	}
	verifyEmailForUser(t, email)
	wLogin, err := makeRequest("POST", "/api/v1/login", map[string]interface{}{
		"email": email, "password": "Password123!",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if wLogin.Code != http.StatusOK {
		t.Fatalf("login: %d %s", wLogin.Code, wLogin.Body.String())
	}
	var loginResp map[string]interface{}
	_ = json.Unmarshal(wLogin.Body.Bytes(), &loginResp)
	tok := loginResp["token"].(map[string]interface{})["jwt_token"].(string)

	wMe, err := makeRequest("GET", "/api/v1/users/me", nil, tok)
	if err != nil {
		t.Fatal(err)
	}
	if wMe.Code != http.StatusOK {
		t.Fatalf("GET /me: %d %s", wMe.Code, wMe.Body.String())
	}
	var me map[string]interface{}
	if err := json.Unmarshal(wMe.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	prof := me["profile"].(map[string]interface{})
	if prof["email"] != email {
		t.Fatalf("expected me email %s, got %v", email, prof["email"])
	}
}
