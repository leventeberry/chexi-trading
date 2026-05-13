//go:build integration

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

func registerSessionTestUser(t *testing.T) (string, string) {
	t.Helper()
	email := fmt.Sprintf("session-%d@test.com", time.Now().UnixNano())
	password := "Password123!"
	w, err := makeRequest("POST", "/api/v1/register", map[string]interface{}{
		"first_name": "Session",
		"last_name":  "Tester",
		"email":      email,
		"password":   password,
		"role":       "user",
	}, "")
	if err != nil {
		t.Fatalf("register request failed: %v", err)
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("register expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	verifyEmailForUser(t, email)
	return email, password
}

func loginSessionUser(t *testing.T, email, password string) (string, string) {
	t.Helper()
	w, err := makeRequest("POST", "/api/v1/login", map[string]interface{}{
		"email":    email,
		"password": password,
	}, "")
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("login expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("parse login response: %v", err)
	}
	tokenObj, ok := response["token"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected token object in login response, got %#v", response["token"])
	}
	access, _ := tokenObj["jwt_token"].(string)
	refresh, _ := tokenObj["refresh_token"].(string)
	if access == "" || refresh == "" {
		t.Fatalf("expected jwt_token and refresh_token, got %#v", tokenObj)
	}
	return access, refresh
}

func refreshSessionToken(t *testing.T, refreshToken string) (int, map[string]interface{}) {
	t.Helper()
	w, err := makeRequest("POST", "/api/v1/refresh", map[string]interface{}{
		"refresh_token": refreshToken,
	}, "")
	if err != nil {
		t.Fatalf("refresh request failed: %v", err)
	}
	var response map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	return w.Code, response
}

func logoutSessionToken(t *testing.T, refreshToken string) int {
	t.Helper()
	w, err := makeRequest("POST", "/api/v1/logout", map[string]interface{}{
		"refresh_token": refreshToken,
	}, "")
	if err != nil {
		t.Fatalf("logout request failed: %v", err)
	}
	return w.Code
}

func hashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func TestAuthSession_LoginCreatesSessionAndRefreshToken(t *testing.T) {
	email, password := registerSessionTestUser(t)
	_, refresh := loginSessionUser(t, email, password)

	var count int64
	if err := testDB.Table("auth_sessions").Where("token_hash = ?", hashRefreshToken(refresh)).Count(&count).Error; err != nil {
		t.Fatalf("count auth sessions: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 session row for refresh token hash, got %d", count)
	}
}

func TestAuthSession_RefreshRotatesTokenAndRejectsOldToken(t *testing.T) {
	email, password := registerSessionTestUser(t)
	_, refresh := loginSessionUser(t, email, password)

	code, response := refreshSessionToken(t, refresh)
	if code != http.StatusOK {
		t.Fatalf("refresh expected 200, got %d body=%v", code, response)
	}
	tokenObj, ok := response["token"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected token object from refresh, got %#v", response["token"])
	}
	newRefresh, _ := tokenObj["refresh_token"].(string)
	if newRefresh == "" {
		t.Fatal("expected rotated refresh token")
	}
	if newRefresh == refresh {
		t.Fatal("expected refresh token rotation to produce a different token")
	}

	oldCode, _ := refreshSessionToken(t, refresh)
	if oldCode != http.StatusUnauthorized {
		t.Fatalf("expected old refresh token reuse to be rejected with 401, got %d", oldCode)
	}
}

func TestAuthSession_LogoutRevokesSession(t *testing.T) {
	email, password := registerSessionTestUser(t)
	_, refresh := loginSessionUser(t, email, password)

	logoutCode := logoutSessionToken(t, refresh)
	if logoutCode != http.StatusOK {
		t.Fatalf("logout expected 200, got %d", logoutCode)
	}

	refreshCode, _ := refreshSessionToken(t, refresh)
	if refreshCode != http.StatusUnauthorized {
		t.Fatalf("expected revoked refresh token to be rejected with 401, got %d", refreshCode)
	}
}

// Logout with any known refresh token (including already-rotated) revokes all sessions for the user.
func TestAuthSession_LogoutRevokesAllSessionsIncludingRotatedSuccessor(t *testing.T) {
	email, password := registerSessionTestUser(t)
	_, refresh := loginSessionUser(t, email, password)

	code, response := refreshSessionToken(t, refresh)
	if code != http.StatusOK {
		t.Fatalf("refresh expected 200, got %d body=%v", code, response)
	}
	tokenObj := response["token"].(map[string]interface{})
	newRefresh, _ := tokenObj["refresh_token"].(string)
	if newRefresh == "" {
		t.Fatal("expected rotated refresh token")
	}

	// Log out using the OLD (already rotated) refresh token — must invalidate successor too.
	if logoutSessionToken(t, refresh) != http.StatusOK {
		t.Fatal("logout with stale refresh token expected 200")
	}
	if code2, _ := refreshSessionToken(t, newRefresh); code2 != http.StatusUnauthorized {
		t.Fatalf("successor refresh token must be invalid after logout_all, got %d", code2)
	}
	if code3, _ := refreshSessionToken(t, refresh); code3 != http.StatusUnauthorized {
		t.Fatalf("stale refresh must stay invalid, got %d", code3)
	}

	var uid string
	if err := testDB.Raw(`SELECT id::text FROM users WHERE LOWER(email) = LOWER(?)`, email).Scan(&uid).Error; err != nil || uid == "" {
		t.Fatalf("lookup user id: %v uid=%q", err, uid)
	}
	var active int64
	if err := testDB.Table("auth_sessions").Where("user_id = ? AND revoked_at IS NULL", uid).Count(&active).Error; err != nil {
		t.Fatalf("count active sessions: %v", err)
	}
	if active != 0 {
		t.Fatalf("expected zero active sessions after logout_all, got %d", active)
	}
}

func TestAuthSession_ConcurrentRefreshSameToken(t *testing.T) {
	email, password := registerSessionTestUser(t)
	_, refresh := loginSessionUser(t, email, password)

	ready := make(chan struct{})
	var wg sync.WaitGroup
	codes := make([]int, 2)
	wg.Add(2)
	for i := range 2 {
		i := i
		go func() {
			defer wg.Done()
			<-ready
			w, err := makeRequest("POST", "/api/v1/refresh", map[string]interface{}{
				"refresh_token": refresh,
			}, "")
			if err != nil {
				codes[i] = -1
				return
			}
			codes[i] = w.Code
		}()
	}
	close(ready)
	wg.Wait()

	var okCount, unauthorizedCount int
	for _, c := range codes {
		switch c {
		case http.StatusOK:
			okCount++
		case http.StatusUnauthorized:
			unauthorizedCount++
		default:
			t.Fatalf("unexpected status code %d (expected 200 or 401)", c)
		}
	}
	if okCount != 1 || unauthorizedCount != 1 {
		t.Fatalf("expected exactly one 200 and one 401, got codes=%v", codes)
	}

	if code, _ := refreshSessionToken(t, refresh); code != http.StatusUnauthorized {
		t.Fatalf("original refresh token must not work after concurrent rotation, got %d", code)
	}

	var uid string
	if err := testDB.Raw(`SELECT id::text FROM users WHERE LOWER(email) = LOWER(?)`, email).Scan(&uid).Error; err != nil || uid == "" {
		t.Fatalf("lookup user id: %v", err)
	}
	var active int64
	if err := testDB.Table("auth_sessions").Where("user_id = ? AND revoked_at IS NULL", uid).Count(&active).Error; err != nil {
		t.Fatalf("count active sessions: %v", err)
	}
	// Winner rotates once; loser may hit rotated-token reuse handling (revoke-all). Either one active session or zero remains — never two.
	if active != 0 && active != 1 {
		t.Fatalf("expected at most one active session after rotation race, got %d", active)
	}
}

func TestAuthSession_ExpiredRefreshTokenRejected(t *testing.T) {
	email, password := registerSessionTestUser(t)
	_, refresh := loginSessionUser(t, email, password)
	refreshHash := hashRefreshToken(refresh)

	past := time.Now().UTC().Add(-1 * time.Hour)
	if err := testDB.Exec("UPDATE auth_sessions SET expires_at = ? WHERE token_hash = ?", past, refreshHash).Error; err != nil {
		t.Fatalf("expire session row: %v", err)
	}

	code, _ := refreshSessionToken(t, refresh)
	if code != http.StatusUnauthorized {
		t.Fatalf("expected expired refresh token to be rejected with 401, got %d", code)
	}
}
