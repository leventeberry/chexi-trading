package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestNewManager_DefaultAccessTokenMinutes(t *testing.T) {
	m := NewManager("secret", 0)
	if m.accessTokenMinutes != 15 {
		t.Fatalf("expected default accessTokenMinutes=15, got %d", m.accessTokenMinutes)
	}
}

func TestManagerCreateAndParseToken(t *testing.T) {
	t.Parallel()

	m := NewManager("unit-test-secret", 5)
	userID := uuid.New()

	td, err := m.CreateToken(userID, "admin")
	if err != nil {
		t.Fatalf("CreateToken() error = %v", err)
	}
	if td.JWTToken == "" || td.ApiKey == "" {
		t.Fatalf("CreateToken() returned empty token details: %+v", td)
	}

	claims, err := m.ParseToken(td.JWTToken)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if claims.Subject != userID.String() {
		t.Fatalf("expected subject %q, got %q", userID.String(), claims.Subject)
	}
	if claims.Role != "admin" {
		t.Fatalf("expected role admin, got %q", claims.Role)
	}
	if claims.ApiKey != td.ApiKey {
		t.Fatalf("expected api key %q, got %q", td.ApiKey, claims.ApiKey)
	}
	if claims.ExpiresAt == nil || time.Until(claims.ExpiresAt.Time) <= 0 {
		t.Fatalf("expected non-expired token, claims exp=%v", claims.ExpiresAt)
	}
}

func TestManagerParseTokenRejectsInvalidCases(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	now := time.Now().UTC()
	secret := "unit-test-secret"
	m := NewManager(secret, 5)

	expiredClaims := Claims{
		ApiKey: "k-expired",
		Role:   "user",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
		},
	}
	expiredToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to create expired token: %v", err)
	}

	otherSecretClaims := Claims{
		ApiKey: "k-wrong-secret",
		Role:   "user",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}
	wrongSecretToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, otherSecretClaims).SignedString([]byte("different-secret"))
	if err != nil {
		t.Fatalf("failed to create wrong-secret token: %v", err)
	}

	noneAlgClaims := Claims{
		ApiKey: "k-none",
		Role:   "user",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}
	noneAlgToken, err := jwt.NewWithClaims(jwt.SigningMethodNone, noneAlgClaims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("failed to create none-alg token: %v", err)
	}

	tests := []struct {
		name  string
		token string
	}{
		{name: "malformed token", token: "not-a-jwt"},
		{name: "expired token", token: expiredToken},
		{name: "wrong secret", token: wrongSecretToken},
		{name: "unsupported signing method", token: noneAlgToken},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := m.ParseToken(tc.token); err == nil {
				t.Fatalf("expected ParseToken() error for %q", tc.name)
			}
		})
	}
}

func TestParseTokenRejectsMFAChallengeJWT(t *testing.T) {
	t.Parallel()
	m := NewManager("unit-test-secret", 5)
	userID := uuid.New()
	tok, err := m.CreateMFAChallengeToken(userID, "jti-test-123", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.ParseToken(tok); err == nil {
		t.Fatal("expected ParseToken to reject MFA challenge JWT")
	}
}

func TestParseTokenRejectsMissingApiKeyOrRole(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	now := time.Now().UTC()
	secret := "unit-test-secret"
	m := NewManager(secret, 5)

	badClaims := Claims{
		ApiKey: "",
		Role:   "user",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}
	badTok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, badClaims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.ParseToken(badTok); err == nil {
		t.Fatal("expected error for empty api_key")
	}
}
