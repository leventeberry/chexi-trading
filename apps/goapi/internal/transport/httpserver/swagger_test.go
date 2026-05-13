package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"goapi/config"
	authinfra "goapi/internal/infra/auth"
)

func TestMountSwaggerRoutes_DevelopmentEnabled_IsAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	cfg := &config.Config{Environment: "development"}
	cfg.Swagger.Enabled = true

	MountSwaggerRoutes(router, cfg, nil)

	if !hasRoute(router, http.MethodGet, "/swagger/*any") {
		t.Fatal("expected swagger route to be mounted")
	}

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	router.ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized {
		t.Fatalf("expected swagger to be accessible without auth in development, got %d", w.Code)
	}
}

func TestMountSwaggerRoutes_ProductionDisabled_IsNotMounted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	cfg := &config.Config{Environment: "production"}
	cfg.Swagger.Enabled = false

	MountSwaggerRoutes(router, cfg, authinfra.NewManager("test-secret", 15))

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestMountSwaggerRoutes_ProductionEnabled_RequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	cfg := &config.Config{Environment: "production"}
	cfg.Swagger.Enabled = true

	MountSwaggerRoutes(router, cfg, authinfra.NewManager("test-secret", 15))

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d", w.Code)
	}
}

func hasRoute(router *gin.Engine, method, path string) bool {
	for _, route := range router.Routes() {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}
