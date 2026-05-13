package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"goapi/internal/events"
	authinfra "goapi/internal/infra/auth"
)

func TestAuthMiddlewareRejectsInvalidAuthorizationHeader(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	manager := authinfra.NewManager("unit-test-secret", 15)
	r := gin.New()
	r.Use(AuthMiddleware(manager))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	tests := []struct {
		name   string
		header string
	}{
		{name: "missing header", header: ""},
		{name: "wrong prefix", header: "Token abc"},
		{name: "empty bearer token", header: "Bearer "},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}

			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestAuthMiddlewareRejectsExpiredToken(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	secret := "unit-test-secret"
	manager := authinfra.NewManager(secret, 15)

	claims := authinfra.Claims{
		ApiKey: "k1",
		Role:   "user",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(-1 * time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	r := gin.New()
	r.Use(AuthMiddleware(manager))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthMiddlewareAcceptsValidTokenAndSetsContext(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	manager := authinfra.NewManager("unit-test-secret", 15)
	userID := uuid.New()
	td, err := manager.CreateToken(userID, "admin")
	if err != nil {
		t.Fatalf("CreateToken() error = %v", err)
	}

	r := gin.New()
	r.Use(AuthMiddleware(manager))
	r.GET("/protected", func(c *gin.Context) {
		apiKey, _ := c.Get("apiKey")
		role, _ := c.Get("role")
		userIDClaim, _ := c.Get("userID")
		actorID, actorSet := events.ActorUserIDFromContext(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{
			"api_key":       apiKey,
			"role":          role,
			"user_id_claim": userIDClaim,
			"actor_id":      actorID.String(),
			"actor_set":     actorSet,
		})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+td.JWTToken)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload["role"] != "admin" {
		t.Fatalf("expected role admin, got %v", payload["role"])
	}
	if payload["user_id_claim"] != userID.String() {
		t.Fatalf("expected user_id_claim %s, got %v", userID, payload["user_id_claim"])
	}
	if payload["actor_id"] != userID.String() {
		t.Fatalf("expected actor_id %s, got %v", userID, payload["actor_id"])
	}
	if payload["actor_set"] != true {
		t.Fatalf("expected actor_set true, got %v", payload["actor_set"])
	}
}

func TestRequireRoleBoundaries(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		setRole      bool
		roleValue    interface{}
		expectedCode int
	}{
		{name: "missing role", setRole: false, expectedCode: http.StatusUnauthorized},
		{name: "invalid role type", setRole: true, roleValue: 123, expectedCode: http.StatusInternalServerError},
		{name: "insufficient role", setRole: true, roleValue: "user", expectedCode: http.StatusForbidden},
		{name: "allowed role", setRole: true, roleValue: "admin", expectedCode: http.StatusOK},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := gin.New()
			r.Use(func(c *gin.Context) {
				if tc.setRole {
					c.Set("role", tc.roleValue)
				}
				c.Next()
			})
			r.Use(RequireRole("admin"))
			r.GET("/admin", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			r.ServeHTTP(w, req)
			if w.Code != tc.expectedCode {
				t.Fatalf("expected %d, got %d body=%s", tc.expectedCode, w.Code, w.Body.String())
			}
		})
	}
}
