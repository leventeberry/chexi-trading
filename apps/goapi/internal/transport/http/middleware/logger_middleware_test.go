package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"goapi/logger"
)

func TestRequestLogger_LogsPathWithoutQuery(t *testing.T) {
	// Do not use t.Parallel: this test replaces the global logger, which races with
	// other package tests that log concurrently (e.g. HTTP audit middleware).
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	zl := zerolog.New(&buf).Level(zerolog.InfoLevel).With().Logger()
	old := logger.Log
	logger.Log = zl
	t.Cleanup(func() { logger.Log = old })

	w := httptest.NewRecorder()
	_, eng := gin.CreateTestContext(w)
	eng.Use(RequestLogger())
	eng.GET("/resource", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/resource?token=leak_me&code=abc&foo=bar", nil)
	eng.ServeHTTP(w, req)

	out := buf.String()
	if strings.Contains(out, "leak_me") || strings.Contains(out, "abc") || strings.Contains(out, "foo=bar") || strings.Contains(out, "?token") {
		t.Fatalf("query must not appear in access log output: %q", out)
	}
	if !strings.Contains(out, "/resource") {
		t.Fatalf("expected path in log: %q", out)
	}
	if !strings.Contains(out, `"method":"GET"`) {
		t.Fatalf("expected method in log: %q", out)
	}
	if !strings.Contains(out, `"status_code":200`) {
		t.Fatalf("expected status_code in log: %q", out)
	}
}
