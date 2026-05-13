//go:build integration

package main

import (
	"net/http"
	"testing"
)

func TestOAuth_InvalidProviderReturns400(t *testing.T) {
	w, err := makeRequest(http.MethodGet, "/api/v1/oauth/twitter/start", nil, "")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("GET /api/v1/oauth/twitter/start = %d, want 400", w.Code)
	}
}

func TestOAuth_Start_WhenProviderDisabled_Returns503(t *testing.T) {
	w, err := makeRequest(http.MethodGet, "/api/v1/oauth/google/start", nil, "")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /api/v1/oauth/google/start = %d, want 503 (OAuth not enabled in test env)", w.Code)
	}
}

func TestOAuth_Complete_InvalidCodeReturns401(t *testing.T) {
	body := map[string]string{"oauth_code": "not-valid-exchange-code"}
	w, err := makeRequest(http.MethodPost, "/api/v1/oauth/complete", body, "")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("POST /api/v1/oauth/complete = %d, want 401", w.Code)
	}
}
