package bootstrapadmin

import (
	"context"
	"path/filepath"
	"testing"

	"goapi/config"
	"goapi/internal/rbac"
	"goapi/models"
	"goapi/repositories"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bootstrap_test.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestValidateSettings_DisabledNoError(t *testing.T) {
	err := ValidateSettings(config.EnvironmentProduction, false, "", "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateSettings_EnabledMissingFields(t *testing.T) {
	err := ValidateSettings(config.EnvironmentDevelopment, true, "", "Password123!")
	if err == nil {
		t.Fatal("expected error")
	}
	err = ValidateSettings(config.EnvironmentDevelopment, true, "a@b.com", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateSettings_WeakPasswordStagingRejected(t *testing.T) {
	err := ValidateSettings(config.EnvironmentStaging, true, "admin@example.com", "short")
	if err == nil {
		t.Fatal("expected error")
	}
	err = ValidateSettings(config.EnvironmentProduction, true, "admin@example.com", "password123!")
	if err == nil {
		t.Fatal("expected error")
	}
	err = ValidateSettings(config.EnvironmentStaging, true, "admin@example.com", "Password123!")
	if err == nil {
		t.Fatal("expected denylist error")
	}
}

func TestValidateSettings_DevAllowsMediumPassword(t *testing.T) {
	err := ValidateSettings(config.EnvironmentDevelopment, true, "admin@example.com", "longenough")
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnsureFirstAdmin_Disabled(t *testing.T) {
	db := setupTestDB(t)
	cfg := &config.Config{}
	cfg.Environment = config.EnvironmentDevelopment
	cfg.Bootstrap.Enabled = false
	if err := EnsureFirstAdmin(context.Background(), db, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureFirstAdmin_CreatesAdmin(t *testing.T) {
	db := setupTestDB(t)
	cfg := &config.Config{}
	cfg.Environment = config.EnvironmentDevelopment
	cfg.Bootstrap.Enabled = true
	cfg.Bootstrap.Email = "first-admin@example.com"
	cfg.Bootstrap.Password = "longenough"

	if err := EnsureFirstAdmin(context.Background(), db, cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Bootstrap.Password != "" {
		t.Fatal("password should be cleared from config")
	}

	repo := repositories.NewUserRepository(db)
	u, err := repo.FindByEmail("first-admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != rbac.RoleAdmin.String() {
		t.Fatalf("role: got %q want admin", u.Role)
	}
}

func TestEnsureFirstAdmin_SkipsWhenAdminExists(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewUserRepository(db)
	if err := repo.Create(&models.User{
		ID:        uuid.New(),
		FirstName: "A",
		LastName:  "B",
		Email:     "existing@example.com",
		PassHash:  "x",
		Role:      rbac.RoleAdmin.String(),
	}); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.Environment = config.EnvironmentDevelopment
	cfg.Bootstrap.Enabled = true
	cfg.Bootstrap.Email = "bootstrap@example.com"
	cfg.Bootstrap.Password = "longenough"

	if err := EnsureFirstAdmin(context.Background(), db, cfg); err != nil {
		t.Fatal(err)
	}
	count := int64(0)
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count: got %d want 1", count)
	}
}

func TestEnsureFirstAdmin_SkipsWhenEmailExists(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewUserRepository(db)
	if err := repo.Create(&models.User{
		ID:        uuid.New(),
		FirstName: "U",
		LastName:  "Ser",
		Email:     "same@example.com",
		PassHash:  "x",
		Role:      rbac.RoleUser.String(),
	}); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.Environment = config.EnvironmentDevelopment
	cfg.Bootstrap.Enabled = true
	cfg.Bootstrap.Email = "same@example.com"
	cfg.Bootstrap.Password = "longenough"

	if err := EnsureFirstAdmin(context.Background(), db, cfg); err != nil {
		t.Fatal(err)
	}
	u, err := repo.FindByEmail("same@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != rbac.RoleUser.String() {
		t.Fatalf("role unchanged: got %q", u.Role)
	}
}
