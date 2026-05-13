package handlers_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"goapi/config"
	"goapi/internal/transport/http/handlers"
)

func TestOAuthStart_InvalidProviderReturns400(t *testing.T) {
	loadTestConfig(t)
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	r := gin.New()
	r.GET("/oauth/:provider/start", handlers.OAuthStart(cfg, &mockAuthService{}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/twitter/start", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("GET oauth/twitter/start = %d, want 400", w.Code)
	}
}

func TestOAuthStart_DisabledProviderReturns503(t *testing.T) {
	loadTestConfig(t)
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	r := gin.New()
	r.GET("/oauth/:provider/start", handlers.OAuthStart(cfg, &mockAuthService{}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/google/start", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET oauth/google/start without OAuth cfg = %d, want 503", w.Code)
	}
}

func TestOAuthComplete_InvalidExchangeMapsTo401(t *testing.T) {
	loadTestConfig(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/oauth/complete", handlers.OAuthComplete(&mockAuthService{}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/oauth/complete", bytes.NewBufferString(`{"oauth_code":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("POST oauth/complete = %d, want 401", w.Code)
	}
}
