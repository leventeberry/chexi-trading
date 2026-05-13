package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/internal/events"
)

func TestRequestID_PreservesIncomingHeader(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	_, eng := gin.CreateTestContext(w)
	eng.Use(RequestID())
	ridEcho := ""
	eng.GET("/", func(c *gin.Context) {
		ridEcho = events.RequestIDFromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "client-correlation-id-xyz")
	eng.ServeHTTP(w, req)

	if got := w.Header().Get("X-Request-ID"); got != "client-correlation-id-xyz" {
		t.Fatalf("response X-Request-ID = %q, want preserved", got)
	}
	if ridEcho != "client-correlation-id-xyz" {
		t.Fatalf("context request id = %q", ridEcho)
	}
}

func TestRequestID_GeneratesWhenMissing_UUIDAndHeaderSet(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	_, eng := gin.CreateTestContext(w)
	eng.Use(RequestID())
	ridEcho := ""
	eng.GET("/", func(c *gin.Context) {
		ridEcho = events.RequestIDFromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	eng.ServeHTTP(w, req)

	hdr := w.Header().Get("X-Request-ID")
	if hdr == "" {
		t.Fatal("expected X-Request-ID header on response")
	}
	if _, err := uuid.Parse(hdr); err != nil {
		t.Fatalf("generated X-Request-ID not a valid UUID: %q (%v)", hdr, err)
	}
	if hdr != ridEcho {
		t.Fatalf("header %q != context %q", hdr, ridEcho)
	}
}
