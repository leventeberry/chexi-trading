package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"goapi/cache"
	"goapi/config"
	"goapi/container"
	"goapi/internal/events"
	authinfra "goapi/internal/infra/auth"
	"goapi/services"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openSmokeDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func smokeConfig() *config.Config {
	cfg := &config.Config{}
	cfg.RateLimit.RequestsPerMinute = 60
	cfg.RateLimit.BurstSize = 10
	cfg.Audit.HTTPEnabled = false
	cfg.SecurityHeaders.Enabled = true
	cfg.SecurityHeaders.HSTSEnabled = false
	return cfg
}

func TestNewEngine_AppWiringSmoke_NoPanicAndCoreRoutes(t *testing.T) {
	ginCfg := smokeConfig()
	db := openSmokeDB(t)
	cacheClient := cache.NewNoOpCache()

	c := &container.Container{
		DB:            db,
		Cache:         cacheClient,
		JWT:           authinfra.NewManager("0123456789abcdef0123456789abcdef", 15),
		Recorder:      events.NoOpRecorder{},
		HealthService: services.NewHealthService(db, cacheClient),
	}

	var router any
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("NewEngine panicked: %v", r)
			}
		}()
		router = NewEngine(cacheClient, ginCfg, c)
	}()

	engine := router.(*gin.Engine)
	if !hasRoute(engine, http.MethodGet, "/") {
		t.Fatal("expected root route to be registered")
	}
	if !hasRoute(engine, http.MethodGet, "/health") {
		t.Fatal("expected /health route to be registered")
	}
	if !hasRoute(engine, http.MethodPost, "/api/v1/login") {
		t.Fatal("expected /api/v1/login route to be registered")
	}
}

func TestNewEngine_RootAndHealthRespond(t *testing.T) {
	ginCfg := smokeConfig()
	db := openSmokeDB(t)
	cacheClient := cache.NewNoOpCache()

	c := &container.Container{
		DB:            db,
		Cache:         cacheClient,
		JWT:           authinfra.NewManager("0123456789abcdef0123456789abcdef", 15),
		Recorder:      events.NoOpRecorder{},
		HealthService: services.NewHealthService(db, cacheClient),
	}

	engine := NewEngine(cacheClient, ginCfg, c)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200 body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want 200 body=%s", w.Code, w.Body.String())
	}
}
