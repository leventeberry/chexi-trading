package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"goapi/container"
)

type fakeHealthService struct {
	payload map[string]interface{}
	code    int
}

func (f fakeHealthService) Check(ctx context.Context) (map[string]interface{}, int) {
	return f.payload, f.code
}

func TestHealthCheckHandler_ResponseShapeAndStatus(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health", healthCheckHandler(&container.Container{
		HealthService: fakeHealthService{
			payload: map[string]interface{}{
				"status":   "healthy",
				"database": map[string]string{"status": "healthy"},
				"cache":    map[string]string{"status": "healthy"},
			},
			code: http.StatusOK,
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, expected := range []string{`"status":"healthy"`, `"database":{"status":"healthy"}`, `"cache":{"status":"healthy"}`, `"timestamp":`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("response missing %q: %s", expected, body)
		}
	}
}

func TestHealthCheckHandler_DBUnavailableReturns503(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health", healthCheckHandler(&container.Container{
		HealthService: fakeHealthService{
			payload: map[string]interface{}{
				"status":   "unhealthy",
				"database": map[string]string{"status": "unhealthy", "error": "db down"},
				"cache":    map[string]string{"status": "healthy"},
			},
			code: http.StatusServiceUnavailable,
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"status":"unhealthy"`) {
		t.Fatalf("expected unhealthy payload, got: %s", w.Body.String())
	}
}
