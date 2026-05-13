package container

import (
	"testing"

	"goapi/cache"
	"goapi/config"
	"goapi/internal/email"
	"goapi/internal/events"
	authinfra "goapi/internal/infra/auth"
	"goapi/internal/queue"
	queuejobs "goapi/internal/queue/jobs"
	"goapi/repositories"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openContainerSmokeDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func TestNewContainer_SmokeInitializesDependencies(t *testing.T) {
	t.Parallel()

	db := openContainerSmokeDB(t)
	cfg := &config.Config{}
	cfg.JWT.RefreshExpirationHours = 24
	cfg.Email.Enabled = false
	cfg.Environment = config.EnvironmentTest

	reg := queue.NewRegistry()
	queuejobs.RegisterEmailHandlers(reg, email.FromConfig(cfg), cfg)
	queuejobs.RegisterWebhookHandlers(reg, repositories.NewOrganizationWebhookRepository(db), cfg)
	jobQ := queue.NewInlineQueue(reg, events.NoOpRecorder{}, cfg)

	c := NewContainer(
		db,
		cache.NewNoOpCache(),
		authinfra.NewManager("0123456789abcdef0123456789abcdef", 15),
		events.NoOpRecorder{},
		cfg,
		jobQ,
		nil,
		false,
	)

	if c == nil {
		t.Fatal("NewContainer() returned nil")
	}
	if c.UserRepository == nil {
		t.Fatal("UserRepository not initialized")
	}
	if c.UserService == nil {
		t.Fatal("UserService not initialized")
	}
	if c.AuthService == nil {
		t.Fatal("AuthService not initialized")
	}
	if c.HealthService == nil {
		t.Fatal("HealthService not initialized")
	}
}
