package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/config"
	"goapi/internal/transport/http/handlers"
	"goapi/models"
	"goapi/services"
)

func loadTestConfig(t *testing.T) {
	t.Helper()
	t.Setenv("DB_USER", "test")
	t.Setenv("DB_PASS", "test")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_NAME", "test")
	t.Setenv("JWT_SECRET", "unit-test-jwt-secret-minimum-32-chars-ok")
	t.Setenv("AUTH_RESPONSE_INCLUDE_API_KEY", "false")
	t.Setenv("AUTH_RESPONSE_INCLUDE_USER", "false")
	config.Load()
}

type mockAuthService struct {
	registerFn      func(*services.RegisterInput) (*models.User, *services.Authentication, error)
	verifyEmailErr  error
	confirmResetErr error
	resendErr       error
	requestResetErr error
}

func (m *mockAuthService) Login(ctx context.Context, email, password string) (*services.LoginResult, error) {
	_ = email
	_ = password
	return nil, services.ErrInvalidCredentials
}

func (m *mockAuthService) SetupTOTP(ctx context.Context, userID uuid.UUID) (*services.TOTPSetupResult, error) {
	_ = ctx
	_ = userID
	return nil, services.ErrMFAUnavailable
}

func (m *mockAuthService) ConfirmTOTP(ctx context.Context, userID uuid.UUID, code string) (*services.TOTPConfirmResult, error) {
	_ = ctx
	_ = userID
	_ = code
	return nil, services.ErrMFAInvalidCode
}

func (m *mockAuthService) DisableTOTP(ctx context.Context, userID uuid.UUID, password string) error {
	_ = ctx
	_ = userID
	_ = password
	return nil
}

func (m *mockAuthService) VerifyMFALogin(ctx context.Context, challengeToken, code string) (*models.User, *services.Authentication, error) {
	_ = ctx
	_ = challengeToken
	_ = code
	return nil, nil, services.ErrMFAInvalidCode
}

func (m *mockAuthService) OAuthAuthorizeURL(ctx context.Context, provider string, linkUserID *uuid.UUID) (string, error) {
	_ = ctx
	_ = provider
	_ = linkUserID
	return "https://idp.example/authorize", nil
}

func (m *mockAuthService) OAuthHandleCallback(ctx context.Context, provider, authCode, rawState string) (*services.OAuthCallbackResult, error) {
	_ = ctx
	_ = provider
	_ = authCode
	_ = rawState
	return &services.OAuthCallbackResult{RedirectURL: "https://app.example/cb?ok=1"}, nil
}

func (m *mockAuthService) OAuthCompleteExchange(ctx context.Context, oauthCode string) (*services.LoginResult, error) {
	_ = ctx
	_ = oauthCode
	return nil, services.ErrOAuthExchangeInvalid
}

func (m *mockAuthService) Register(ctx context.Context, input *services.RegisterInput) (*models.User, *services.Authentication, error) {
	if m.registerFn != nil {
		return m.registerFn(input)
	}
	return &models.User{ID: uuid.MustParse("11111111-1111-4111-8111-111111111111"), Email: input.Email}, nil, nil
}

func (m *mockAuthService) RefreshToken(ctx context.Context, refreshToken string) (*services.Authentication, error) {
	return &services.Authentication{JWTToken: "jwt-new", RefreshToken: "refresh-new"}, nil
}

func (m *mockAuthService) Logout(ctx context.Context, refreshToken string) error {
	return nil
}

func (m *mockAuthService) ValidateCredentials(email, password string) (*models.User, error) {
	return nil, services.ErrInvalidCredentials
}

func (m *mockAuthService) VerifyEmail(ctx context.Context, token string) error {
	_ = ctx
	_ = token
	return m.verifyEmailErr
}

func (m *mockAuthService) ResendVerificationEmail(ctx context.Context, email string) error {
	_ = ctx
	_ = email
	return m.resendErr
}

func (m *mockAuthService) RequestPasswordReset(ctx context.Context, email string) error {
	_ = ctx
	_ = email
	return m.requestResetErr
}

func (m *mockAuthService) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	_ = ctx
	_ = token
	_ = newPassword
	return m.confirmResetErr
}

func TestSignupUser_StatusCreated(t *testing.T) {
	loadTestConfig(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/register", handlers.SignupUser(&mockAuthService{}))

	body := map[string]interface{}{
		"first_name": "A",
		"last_name":  "B",
		"email":      "signup-unit@test.com",
		"password":   "Password123!",
		"role":       "user",
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["token"] != nil {
		t.Fatalf("expected no token until email verified, got %#v", resp["token"])
	}
	if resp["message"] == nil || resp["message"] == "" {
		t.Fatal("expected registration message")
	}
	userObj, ok := resp["user"].(map[string]interface{})
	if !ok {
		t.Fatal("expected user object with id and email")
	}
	if userObj["email"] != "signup-unit@test.com" {
		t.Fatalf("user email: %v", userObj["email"])
	}
}

func TestSignupUser_ResponseIncludesOptionalFields(t *testing.T) {
	t.Setenv("DB_USER", "test")
	t.Setenv("DB_PASS", "test")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_NAME", "test")
	t.Setenv("JWT_SECRET", "unit-test-jwt-secret-minimum-32-chars-ok")
	t.Setenv("AUTH_RESPONSE_INCLUDE_API_KEY", "true")
	t.Setenv("AUTH_RESPONSE_INCLUDE_USER", "true")
	config.Load()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	uid := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	r.POST("/register", handlers.SignupUser(&mockAuthService{
		registerFn: func(input *services.RegisterInput) (*models.User, *services.Authentication, error) {
			return &models.User{ID: uid, Email: input.Email}, &services.Authentication{ApiKey: "k", JWTToken: "jwt", RefreshToken: "rt"}, nil
		},
	}))

	body := map[string]interface{}{
		"first_name": "A",
		"last_name":  "B",
		"email":      "signup-flags@test.com",
		"password":   "Password123!",
		"role":       "user",
	}
	payload, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	tokenObj := resp["token"].(map[string]interface{})
	if _, ok := tokenObj["api_key"]; !ok {
		t.Fatal("expected api_key when AUTH_RESPONSE_INCLUDE_API_KEY=true")
	}
	userObj, ok := resp["user"].(map[string]interface{})
	if !ok {
		t.Fatal("expected user when AUTH_RESPONSE_INCLUDE_USER=true")
	}
	if userObj["email"] != "signup-flags@test.com" {
		t.Fatalf("user email: got %v", userObj["email"])
	}
}

func TestVerifyEmail_InvalidTokenReturns400(t *testing.T) {
	loadTestConfig(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/verify-email", handlers.VerifyEmail(&mockAuthService{verifyEmailErr: services.ErrInvalidVerificationToken}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/verify-email", bytes.NewReader([]byte(`{"token":"bad"}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestVerifyEmail_QueryTokenSuccess(t *testing.T) {
	loadTestConfig(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/verify-email", handlers.VerifyEmail(&mockAuthService{}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/verify-email?token=abc", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestConfirmPasswordReset_WeakPasswordRejectedBeforeService(t *testing.T) {
	loadTestConfig(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/password-reset/confirm", handlers.ConfirmPasswordReset(&mockAuthService{}))

	w := httptest.NewRecorder()
	payload := []byte(`{"token":"x","password":"short"}`)
	req := httptest.NewRequest(http.MethodPost, "/password-reset/confirm", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for weak password, got %d body=%s", w.Code, w.Body.String())
	}
}
