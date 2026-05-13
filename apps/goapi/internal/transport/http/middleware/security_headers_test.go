package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"goapi/config"
)

func TestSecurityHeaders_DefaultPlainHTTP_NoHSTS(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.SecurityHeaders.Enabled = true
	cfg.SecurityHeaders.HSTSEnabled = false

	r := gin.New()
	r.Use(SecurityHeaders(cfg))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	assertSecurityBaseline(t, w)
	if w.Header().Get("Strict-Transport-Security") != "" {
		t.Fatalf("unexpected Strict-Transport-Security on plain HTTP with HSTS disabled")
	}
}

func TestSecurityHeaders_HSTSForwardedHTTPS(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.SecurityHeaders.Enabled = true
	cfg.SecurityHeaders.HSTSEnabled = true
	cfg.SecurityHeaders.HSTSMaxAgeSeconds = 63072000

	r := gin.New()
	r.Use(SecurityHeaders(cfg))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	r.ServeHTTP(w, req)

	sts := w.Header().Get("Strict-Transport-Security")
	if !strings.Contains(sts, "max-age=63072000") {
		t.Fatalf("Strict-Transport-Security = %q, want max-age=63072000", sts)
	}
}

func TestSecurityHeaders_HSTSRequestTLS(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.SecurityHeaders.Enabled = true
	cfg.SecurityHeaders.HSTSEnabled = true
	cfg.SecurityHeaders.HSTSMaxAgeSeconds = 31536000

	r := gin.New()
	r.Use(SecurityHeaders(cfg))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.TLS = &tls.ConnectionState{}
	r.ServeHTTP(w, req)

	sts := w.Header().Get("Strict-Transport-Security")
	if !strings.Contains(sts, "max-age=31536000") {
		t.Fatalf("Strict-Transport-Security = %q", sts)
	}
}

func TestSecurityHeaders_Disabled(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.SecurityHeaders.Enabled = false

	r := gin.New()
	r.Use(SecurityHeaders(cfg))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Content-Type-Options") != "" {
		t.Fatal("expected no X-Content-Type-Options when middleware disabled")
	}
	if w.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("expected no HSTS when middleware disabled")
	}
}

func TestSecurityHeaders_HandlerOverridesXFrameOptions(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.SecurityHeaders.Enabled = true
	cfg.SecurityHeaders.HSTSEnabled = false

	r := gin.New()
	r.Use(SecurityHeaders(cfg))
	r.GET("/", func(c *gin.Context) {
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if got := w.Header().Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Fatalf("X-Frame-Options = %q, want SAMEORIGIN (handler should win)", got)
	}
}

func TestSecurityHeaders_NilConfigNoOp(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(SecurityHeaders(nil))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Frame-Options") != "" {
		t.Fatal("expected no security headers when cfg is nil")
	}
}

func assertSecurityBaseline(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := w.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q", got)
	}
	if got := w.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q", got)
	}
	pp := w.Header().Get("Permissions-Policy")
	if !strings.Contains(pp, "camera=()") || !strings.Contains(pp, "geolocation=()") {
		t.Fatalf("Permissions-Policy = %q", pp)
	}
	if got := w.Header().Get("Cross-Origin-Opener-Policy"); got != "same-origin" {
		t.Fatalf("Cross-Origin-Opener-Policy = %q", got)
	}
}
