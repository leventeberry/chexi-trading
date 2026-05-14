package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/cache"
	"goapi/config"
	"goapi/container"
	"goapi/internal/email"
	"goapi/internal/events"
	authinfra "goapi/internal/infra/auth"
	"goapi/internal/marketdata/state"
	"goapi/internal/queue"
	queuejobs "goapi/internal/queue/jobs"
	"goapi/repositories"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type routeTestHealthService struct{}

func (routeTestHealthService) Check(ctx context.Context) (map[string]interface{}, int) {
	return map[string]interface{}{
		"status":   "healthy",
		"database": map[string]string{"status": "healthy"},
		"cache":    map[string]string{"status": "disabled"},
	}, http.StatusOK
}

func openRouteSmokeDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func buildRouteSmokeRouter(t *testing.T) (*gin.Engine, *authinfra.Manager, *container.Container) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	cfg := &config.Config{}
	cfg.RateLimit.RequestsPerMinute = 60
	cfg.RateLimit.BurstSize = 10
	cfg.Audit.HTTPEnabled = false
	cfg.Email.Enabled = false
	cfg.Environment = config.EnvironmentTest

	jwt := authinfra.NewManager("0123456789abcdef0123456789abcdef", 15)
	db := openRouteSmokeDB(t)
	reg := queue.NewRegistry()
	queuejobs.RegisterEmailHandlers(reg, email.FromConfig(cfg), cfg)
	queuejobs.RegisterWebhookHandlers(reg, repositories.NewOrganizationWebhookRepository(db), cfg)
	jobQ := queue.NewInlineQueue(reg, events.NoOpRecorder{}, cfg)
	c := container.NewContainer(
		db,
		cache.NewNoOpCache(),
		jwt,
		events.NoOpRecorder{},
		cfg,
		jobQ,
		nil,
		false,
		state.New(),
	)
	c.HealthService = routeTestHealthService{}

	SetupRoutes(router, c, cfg)
	return router, jwt, c
}

func TestSetupRoutes_RegistersCorePublicRoutes(t *testing.T) {
	t.Parallel()

	router, _, _ := buildRouteSmokeRouter(t)

	want := [][2]string{
		{http.MethodGet, "/"},
		{http.MethodGet, "/health"},
		{http.MethodPost, "/api/v1/login"},
		{http.MethodPost, "/api/v1/login/verify-mfa"},
		{http.MethodPost, "/api/v1/register"},
		{http.MethodPost, "/api/v1/strategy/overnight-mean-reversion/score"},
		{http.MethodGet, "/api/v1/market/tickers/status"},
		{http.MethodGet, "/api/v1/market/tickers"},
		{http.MethodGet, "/api/v1/market/tickers/:productID"},
		{http.MethodPost, "/api/v1/market/tickers/:productID/overnight-mean-reversion/score"},
		{http.MethodPost, "/api/v1/oauth/complete"},
		{http.MethodGet, "/api/v1/oauth/:provider/start"},
		{http.MethodGet, "/api/v1/oauth/:provider/callback"},
		{http.MethodPost, "/api/v1/oauth/:provider/link"},
	}
	for _, r := range want {
		if !hasRoute(router, r[0], r[1]) {
			t.Fatalf("missing route %s %s", r[0], r[1])
		}
	}
}

func TestSetupRoutes_ProtectedUserRoutesRequireAuth(t *testing.T) {
	t.Parallel()

	router, _, _ := buildRouteSmokeRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/users/me without auth = %d, want 401", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/organizations", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/organizations without auth = %d, want 401", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/trade-plans", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/trade-plans without auth = %d, want 401", w.Code)
	}
}

func TestSetupRoutes_AdminRoutesRequireAdminAndNotPublic(t *testing.T) {
	t.Parallel()

	router, jwt, _ := buildRouteSmokeRouter(t)

	// Not publicly exposed: unauthenticated request must be rejected.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/admin/users without auth = %d, want 401", w.Code)
	}

	// Authenticated regular user still forbidden.
	token, err := jwt.CreateToken(uuid.New(), "user")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+token.JWTToken)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("GET /api/v1/admin/users as non-admin = %d, want 403", w.Code)
	}

	// Admin jobs observability: same authz as other admin routes.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/jobs/health", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/admin/jobs/health without auth = %d, want 401", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/jobs/health", nil)
	req.Header.Set("Authorization", "Bearer "+token.JWTToken)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("GET /api/v1/admin/jobs/health as non-admin = %d, want 403", w.Code)
	}

	adminToken, err := jwt.CreateToken(uuid.New(), "admin")
	if err != nil {
		t.Fatalf("create admin token: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/jobs/health", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken.JWTToken)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/admin/jobs/health as admin = %d, want 200, body %s", w.Code, w.Body.String())
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
