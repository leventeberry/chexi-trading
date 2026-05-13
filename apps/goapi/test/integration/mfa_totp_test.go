//go:build integration

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestMFA_TOTP_SetupConfirmLoginDisable(t *testing.T) {
	email := fmt.Sprintf("mfa.%d@test.com", time.Now().UnixNano())
	password := "Password123!"

	reg := map[string]interface{}{
		"first_name": "M",
		"last_name":  "FA",
		"email":      email,
		"password":   password,
	}
	w, err := makeRequest("POST", "/api/v1/register", reg, "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("register status %d body %s", w.Code, w.Body.String())
	}
	verifyEmailForUser(t, email)
	w, err = makeRequest("POST", "/api/v1/login", map[string]interface{}{"email": email, "password": password}, "")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("login after verify status %d body %s", w.Code, w.Body.String())
	}
	var login0 map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &login0); err != nil {
		t.Fatalf("parse login: %v", err)
	}
	tokenObj := login0["token"].(map[string]interface{})
	jwt0 := tokenObj["jwt_token"].(string)

	w, err = makeRequest("POST", "/api/v1/mfa/totp/setup", nil, jwt0)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("setup status %d body %s", w.Code, w.Body.String())
	}
	var setup map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &setup); err != nil {
		t.Fatalf("parse setup: %v", err)
	}
	secret, _ := setup["secret"].(string)
	uri, _ := setup["uri"].(string)
	if secret == "" || uri == "" {
		t.Fatalf("expected secret and otpauth uri in setup response, got %#v", setup)
	}

	w, _ = makeRequest("POST", "/api/v1/mfa/totp/confirm", map[string]string{"code": "000000"}, jwt0)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong confirm code want 401 got %d %s", w.Code, w.Body.String())
	}

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("totp code: %v", err)
	}
	w, err = makeRequest("POST", "/api/v1/mfa/totp/confirm", map[string]string{"code": code}, jwt0)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("confirm status %d body %s", w.Code, w.Body.String())
	}
	var conf map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &conf); err != nil {
		t.Fatalf("parse confirm: %v", err)
	}
	rc, _ := conf["recovery_codes"].([]interface{})
	if len(rc) < 1 {
		t.Fatalf("expected recovery_codes in confirm response, got %#v", conf)
	}

	w, _ = makeRequest("POST", "/api/v1/mfa/totp/setup", nil, jwt0)
	if w.Code != http.StatusConflict {
		t.Fatalf("setup when MFA enabled want 409 got %d %s", w.Code, w.Body.String())
	}

	w, err = makeRequest("POST", "/api/v1/login", map[string]string{"email": email, "password": password}, "")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("login status %d body %s", w.Code, w.Body.String())
	}
	var login1 map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &login1); err != nil {
		t.Fatalf("parse login: %v", err)
	}
	if v, ok := login1["mfa_required"].(bool); !ok || !v {
		t.Fatalf("expected mfa_required true, got %#v", login1)
	}
	if login1["token"] != nil {
		t.Fatalf("did not expect access token before MFA, got %#v", login1)
	}
	mfaTok, _ := login1["mfa_challenge_token"].(string)
	if mfaTok == "" {
		t.Fatalf("missing mfa_challenge_token")
	}

	w, _ = makeRequest("POST", "/api/v1/login/verify-mfa", map[string]string{
		"mfa_challenge_token": mfaTok,
		"code":                "000000",
	}, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("verify-mfa wrong code want 401 got %d %s", w.Code, w.Body.String())
	}

	okCode, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("totp: %v", err)
	}
	w, err = makeRequest("POST", "/api/v1/login/verify-mfa", map[string]string{
		"mfa_challenge_token": mfaTok,
		"code":                okCode,
	}, "")
	if err != nil {
		t.Fatalf("verify-mfa: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("verify-mfa status %d body %s", w.Code, w.Body.String())
	}
	var verifyResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &verifyResp); err != nil {
		t.Fatalf("parse verify: %v", err)
	}
	tok2 := verifyResp["token"].(map[string]interface{})
	jwt1 := tok2["jwt_token"].(string)
	if jwt1 == "" {
		t.Fatal("expected jwt_token after MFA verify")
	}

	w, _ = makeRequest("POST", "/api/v1/login/verify-mfa", map[string]string{
		"mfa_challenge_token": mfaTok,
		"code":                okCode,
	}, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("replay challenge want 401 got %d %s", w.Code, w.Body.String())
	}

	w, err = makeRequest("POST", "/api/v1/mfa/totp/disable", map[string]string{"password": "WrongPassword123!"}, jwt1)
	if err != nil {
		t.Fatalf("disable wrong pw: %v", err)
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("disable wrong password want 401 got %d %s", w.Code, w.Body.String())
	}

	w, err = makeRequest("POST", "/api/v1/mfa/totp/disable", map[string]string{"password": password}, jwt1)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("disable status %d body %s", w.Code, w.Body.String())
	}

	w, err = makeRequest("POST", "/api/v1/login", map[string]string{"email": email, "password": password}, "")
	if err != nil {
		t.Fatalf("login after disable: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("login status %d body %s", w.Code, w.Body.String())
	}
	var login2 map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &login2); err != nil {
		t.Fatalf("parse login2: %v", err)
	}
	if login2["mfa_required"] == true {
		t.Fatalf("expected MFA off after disable, got %#v", login2)
	}
	if login2["token"] == nil {
		t.Fatalf("expected token object after login without MFA, got %#v", login2)
	}
}
