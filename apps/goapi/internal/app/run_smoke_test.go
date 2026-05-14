package app

import (
	"net/http"
	"testing"
	"time"

	"goapi/cache"
	"goapi/config"
	"goapi/container"
	"goapi/internal/email"
	"goapi/internal/events"
	authinfra "goapi/internal/infra/auth"
	"goapi/internal/marketdata/state"
	"goapi/internal/queue"
	queuejobs "goapi/internal/queue/jobs"
	httpserver "goapi/internal/transport/httpserver"
	"goapi/repositories"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openAppSmokeDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func TestAppWiring_SmokeBuildServerWithoutPanic(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Environment = config.EnvironmentTest
	cfg.Server.Port = "18080"
	cfg.Server.ReadHeaderTimeout = 5 * time.Second
	cfg.Server.ReadTimeout = 30 * time.Second
	cfg.Server.WriteTimeout = 30 * time.Second
	cfg.Server.IdleTimeout = 120 * time.Second
	cfg.RateLimit.RequestsPerMinute = 60
	cfg.RateLimit.BurstSize = 10
	cfg.Audit.HTTPEnabled = false
	cfg.Email.Enabled = false
	cfg.JWT.AccessTokenMinutes = 15
	cfg.JWT.RefreshExpirationHours = 24

	db := openAppSmokeDB(t)
	cacheClient := cache.NewNoOpCache()
	jwtMgr := authinfra.NewManager("0123456789abcdef0123456789abcdef", cfg.JWT.AccessTokenMinutes)
	recorder := events.NoOpRecorder{}
	reg := queue.NewRegistry()
	queuejobs.RegisterEmailHandlers(reg, email.FromConfig(cfg), cfg)
	queuejobs.RegisterWebhookHandlers(reg, repositories.NewOrganizationWebhookRepository(db), cfg)
	jobQ := queue.NewInlineQueue(reg, events.NoOpRecorder{}, cfg)
	appContainer := container.NewContainer(db, cacheClient, jwtMgr, recorder, cfg, jobQ, nil, false, state.New())

	var routerErr interface{}
	var routerHandler http.Handler
	func() {
		defer func() {
			routerErr = recover()
		}()
		routerHandler = httpserver.NewEngine(cacheClient, cfg, appContainer)
	}()

	if routerErr != nil {
		t.Fatalf("NewEngine panicked: %v", routerErr)
	}
	if routerHandler == nil {
		t.Fatal("router handler is nil")
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           routerHandler,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	if srv.Addr != ":18080" {
		t.Fatalf("server addr = %q, want %q", srv.Addr, ":18080")
	}
	if srv.Handler == nil {
		t.Fatal("server handler is nil")
	}
}
