//go:build integration

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Must stay aligned with handlers.msgPasswordResetRequestAck / msgResendVerificationAck.
const (
	wantPasswordResetRequestAck = "If an account exists for that email, password reset instructions were sent."
	wantResendVerificationAck   = "If an account exists for that email and verification is required, instructions were sent."
)

func sha256HexString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func mustUserIDByEmail(t *testing.T, email string) string {
	t.Helper()
	var id string
	if err := testDB.Raw(`SELECT id::text FROM users WHERE LOWER(email) = LOWER(?)`, email).Scan(&id).Error; err != nil {
		t.Fatalf("lookup user: %v", err)
	}
	if id == "" {
		t.Fatalf("no user for email %s", email)
	}
	return id
}

func replaceEmailVerificationToken(t *testing.T, userID, raw string, expiresAt time.Time) {
	t.Helper()
	h := sha256HexString(raw)
	res := testDB.Exec(`DELETE FROM email_verification_tokens WHERE user_id = ?`, userID)
	if res.Error != nil {
		t.Fatalf("delete verification tokens: %v", res.Error)
	}
	res = testDB.Exec(
		`INSERT INTO email_verification_tokens (user_id, token_hash, expires_at) VALUES (?, ?, ?)`,
		userID, h, expiresAt,
	)
	if res.Error != nil {
		t.Fatalf("insert verification token: %v", res.Error)
	}
}

func replacePasswordResetToken(t *testing.T, userID, raw string, expiresAt time.Time) {
	t.Helper()
	h := sha256HexString(raw)
	res := testDB.Exec(`DELETE FROM password_reset_tokens WHERE user_id = ?`, userID)
	if res.Error != nil {
		t.Fatalf("delete reset tokens: %v", res.Error)
	}
	res = testDB.Exec(
		`INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES (?, ?, ?)`,
		userID, h, expiresAt,
	)
	if res.Error != nil {
		t.Fatalf("insert reset token: %v", res.Error)
	}
}

func registerVerificationFlowUser(t *testing.T) string {
	t.Helper()
	email := fmt.Sprintf("verify-flow-%d@test.com", time.Now().UnixNano())
	w, err := makeRequest("POST", "/api/v1/register", map[string]interface{}{
		"first_name": "VF",
		"last_name":  "User",
		"email":      email,
		"password":   "Password123!",
		"role":       "user",
	}, "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("register expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	return email
}

func TestEmailVerification_RegisterCreatesVerificationRow(t *testing.T) {
	if testDB == nil {
		t.Fatal("testDB not initialized")
	}
	email := registerVerificationFlowUser(t)
	uid := mustUserIDByEmail(t, email)

	var n int64
	if err := testDB.Table("email_verification_tokens").
		Where("user_id = ? AND used_at IS NULL", uid).
		Count(&n).Error; err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly one active verification token row, got %d", n)
	}
}

func TestEmailVerification_VerifySuccessThenReuseFails(t *testing.T) {
	if testDB == nil {
		t.Fatal("testDB not initialized")
	}
	email := registerVerificationFlowUser(t)
	uid := mustUserIDByEmail(t, email)

	raw := "integration-verify-token-raw"
	expires := time.Now().UTC().Add(time.Hour)
	replaceEmailVerificationToken(t, uid, raw, expires)

	w, err := makeRequest("POST", "/api/v1/verify-email", map[string]interface{}{"token": raw}, "")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("verify expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var verifiedCount int64
	if err := testDB.Raw(`SELECT COUNT(*) FROM users WHERE id = ? AND email_verified_at IS NOT NULL`, uid).Scan(&verifiedCount).Error; err != nil {
		t.Fatalf("check verified_at: %v", err)
	}
	if verifiedCount != 1 {
		t.Fatal("expected email_verified_at to be set after verify")
	}

	w2, err := makeRequest("POST", "/api/v1/verify-email", map[string]interface{}{"token": raw}, "")
	if err != nil {
		t.Fatalf("verify again: %v", err)
	}
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("second verify expected 400, got %d body=%s", w2.Code, w2.Body.String())
	}
}

func TestEmailVerification_ExpiredTokenRejected(t *testing.T) {
	if testDB == nil {
		t.Fatal("testDB not initialized")
	}
	email := registerVerificationFlowUser(t)
	uid := mustUserIDByEmail(t, email)
	raw := "expired-verify-token"
	replaceEmailVerificationToken(t, uid, raw, time.Now().UTC().Add(-time.Hour))

	w, err := makeRequest("POST", "/api/v1/verify-email", map[string]interface{}{"token": raw}, "")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for expired token, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPasswordReset_GenericResponseKnownAndUnknownEmail(t *testing.T) {
	emailKnown := registerVerificationFlowUser(t)
	emailUnknown := fmt.Sprintf("unknown-%d@nobody.test", time.Now().UnixNano())

	w1, err := makeRequest("POST", "/api/v1/password-reset/request", map[string]interface{}{
		"email": emailKnown,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	w2, err := makeRequest("POST", "/api/v1/password-reset/request", map[string]interface{}{
		"email": emailUnknown,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if w1.Code != http.StatusOK || w2.Code != http.StatusOK {
		t.Fatalf("expected 200 both, got %d and %d", w1.Code, w2.Code)
	}
	if w1.Body.String() != w2.Body.String() {
		t.Fatalf("bodies differ:\n%s\nvs\n%s", w1.Body.String(), w2.Body.String())
	}
	var m map[string]interface{}
	if err := json.Unmarshal(w1.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["message"] != wantPasswordResetRequestAck {
		t.Fatalf("unexpected message: %#v", m["message"])
	}
}

func TestPasswordReset_TokenStoredHashedNotPlaintext(t *testing.T) {
	if testDB == nil {
		t.Fatal("testDB not initialized")
	}
	email := registerVerificationFlowUser(t)
	uid := mustUserIDByEmail(t, email)

	raw := "plaintext-reset-token-test"
	expires := time.Now().UTC().Add(time.Hour)
	replacePasswordResetToken(t, uid, raw, expires)

	var stored string
	if err := testDB.Raw(
		`SELECT token_hash FROM password_reset_tokens WHERE user_id = ? AND used_at IS NULL ORDER BY created_at DESC LIMIT 1`,
		uid,
	).Scan(&stored).Error; err != nil {
		t.Fatalf("query hash: %v", err)
	}
	if stored == raw {
		t.Fatal("token_hash must not equal raw token")
	}
	if len(stored) != 64 {
		t.Fatalf("expected sha256 hex length 64, got %d", len(stored))
	}
	if stored != sha256HexString(raw) {
		t.Fatal("token_hash must be SHA256 hex of raw token")
	}
}

func TestPasswordReset_ConfirmWeakPasswordRejected(t *testing.T) {
	if testDB == nil {
		t.Fatal("testDB not initialized")
	}
	email := registerVerificationFlowUser(t)
	uid := mustUserIDByEmail(t, email)
	raw := "confirm-weak-pass-token"
	replacePasswordResetToken(t, uid, raw, time.Now().UTC().Add(time.Hour))

	w, err := makeRequest("POST", "/api/v1/password-reset/confirm", map[string]interface{}{
		"token":    raw,
		"password": "short",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPasswordReset_ConfirmRevokesRefreshSessions(t *testing.T) {
	if testDB == nil {
		t.Fatal("testDB not initialized")
	}
	email := fmt.Sprintf("reset-session-%d@test.com", time.Now().UnixNano())
	pass := "Password123!"
	wReg, err := makeRequest("POST", "/api/v1/register", map[string]interface{}{
		"first_name": "RS",
		"last_name":  "User",
		"email":      email,
		"password":   pass,
		"role":       "user",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if wReg.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", wReg.Code, wReg.Body.String())
	}

	verifyEmailForUser(t, email)

	_, refresh := loginSessionUser(t, email, pass)
	if refresh == "" {
		t.Fatal("expected refresh token")
	}

	uid := mustUserIDByEmail(t, email)
	raw := "reset-session-token"
	replacePasswordResetToken(t, uid, raw, time.Now().UTC().Add(time.Hour))

	newPass := "Newpassword456!"
	wConf, err := makeRequest("POST", "/api/v1/password-reset/confirm", map[string]interface{}{
		"token":    raw,
		"password": newPass,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if wConf.Code != http.StatusOK {
		t.Fatalf("confirm reset expected 200, got %d body=%s", wConf.Code, wConf.Body.String())
	}

	code, _ := refreshSessionToken(t, refresh)
	if code != http.StatusUnauthorized {
		t.Fatalf("refresh after password reset expected 401, got %d", code)
	}

	wLogin, err := makeRequest("POST", "/api/v1/login", map[string]interface{}{
		"email":    email,
		"password": newPass,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if wLogin.Code != http.StatusOK {
		t.Fatalf("login with new password expected 200, got %d body=%s", wLogin.Code, wLogin.Body.String())
	}
}

func TestPasswordReset_ConfirmSecondUseFails(t *testing.T) {
	if testDB == nil {
		t.Fatal("testDB not initialized")
	}
	email := registerVerificationFlowUser(t)
	uid := mustUserIDByEmail(t, email)
	raw := "one-time-reset-token"
	replacePasswordResetToken(t, uid, raw, time.Now().UTC().Add(time.Hour))

	newPass := "Newpassword456!"
	w1, err := makeRequest("POST", "/api/v1/password-reset/confirm", map[string]interface{}{
		"token":    raw,
		"password": newPass,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if w1.Code != http.StatusOK {
		t.Fatalf("first confirm expected 200, got %d body=%s", w1.Code, w1.Body.String())
	}

	w2, err := makeRequest("POST", "/api/v1/password-reset/confirm", map[string]interface{}{
		"token":    raw,
		"password": "Otherpass789!",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("reuse token expected 400, got %d body=%s", w2.Code, w2.Body.String())
	}
}

func TestResendVerification_GenericAck(t *testing.T) {
	w, err := makeRequest("POST", "/api/v1/resend-verification", map[string]interface{}{
		"email": fmt.Sprintf("who-knows-%d@test.com", time.Now().UnixNano()),
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var m map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["message"] != wantResendVerificationAck {
		t.Fatalf("unexpected message: %#v", m["message"])
	}
}
