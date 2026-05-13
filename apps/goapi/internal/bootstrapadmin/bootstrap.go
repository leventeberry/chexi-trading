// Package bootstrapadmin provides a one-time, idempotent first-admin creation at startup
// when BOOTSTRAP_ADMIN_ENABLED=true. Never log bootstrap passwords.
package bootstrapadmin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"goapi/config"
	"goapi/internal/infra/auth"
	"goapi/internal/rbac"
	"goapi/logger"
	"goapi/models"
	"goapi/repositories"
	"goapi/services"

	"gorm.io/gorm"
)

// Weak/default passwords rejected when staging or production (additional layer on top of ValidatePasswordStrength).
var bootstrapPasswordDenylist = map[string]struct{}{
	"password":                       {},
	"password123":                    {},
	"password123!":                   {},
	"admin123":                       {},
	"admin123!":                      {},
	"changeme":                       {},
	"bootstrap":                      {},
	"bootstrapadmin":                 {},
	"bootstrapadmin123!":             {},
	"your-strong-bootstrap-password": {},
}

// ValidateSettings checks bootstrap env consistency before touching the database.
func ValidateSettings(env string, enabled bool, email, password string) error {
	if !enabled {
		return nil
	}
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return fmt.Errorf("bootstrap admin: email and password required when bootstrap is enabled")
	}
	if !strings.Contains(email, "@") {
		return fmt.Errorf("bootstrap admin: BOOTSTRAP_ADMIN_EMAIL must be a valid email address")
	}
	return validateBootstrapPassword(env, password)
}

func validateBootstrapPassword(env string, password string) error {
	if config.IsStagingOrProductionEnvironment(env) {
		if err := services.ValidatePasswordStrength(password); err != nil {
			return fmt.Errorf("bootstrap admin password rejected for %s: %w", config.NormalizeEnvironment(env), err)
		}
		key := strings.ToLower(strings.TrimSpace(password))
		if _, banned := bootstrapPasswordDenylist[key]; banned {
			return fmt.Errorf("bootstrap admin password is a forbidden default in staging/production")
		}
		return nil
	}
	// Development/test: require minimum length only (staging/production rules apply above).
	if len(strings.TrimSpace(password)) < 8 {
		return fmt.Errorf("bootstrap admin password must be at least 8 characters in non-production environments")
	}
	return nil
}

func clearBootstrapPassword(cfg *config.Config) {
	if cfg == nil {
		return
	}
	cfg.Bootstrap.Password = ""
}

// EnsureFirstAdmin creates the first admin user when enabled and safe to do so.
func EnsureFirstAdmin(ctx context.Context, db *gorm.DB, cfg *config.Config) error {
	_ = ctx
	if cfg == nil || !cfg.Bootstrap.Enabled {
		return nil
	}
	if err := ValidateSettings(cfg.Environment, cfg.Bootstrap.Enabled, cfg.Bootstrap.Email, cfg.Bootstrap.Password); err != nil {
		return err
	}

	repo := repositories.NewUserRepository(db)

	hasAdmin, err := repo.ExistsAnyAdmin()
	if err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	if hasAdmin {
		logger.Log.Info().Msg("Bootstrap admin skipped: an admin user already exists")
		clearBootstrapPassword(cfg)
		return nil
	}

	email := normalizeEmail(cfg.Bootstrap.Email)
	exists, err := repo.ExistsByEmail(email)
	if err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	if exists {
		logger.Log.Info().Str("email", email).Msg("Bootstrap admin skipped: email already registered (password not changed)")
		clearBootstrapPassword(cfg)
		return nil
	}

	passHash, err := auth.HashPassword(cfg.Bootstrap.Password)
	if err != nil {
		return fmt.Errorf("bootstrap admin: hash password: %w", err)
	}

	now := time.Now().UTC()
	u := &models.User{
		FirstName:       "Bootstrap",
		LastName:        "Admin",
		Email:           email,
		PassHash:        passHash,
		PhoneNum:        "",
		Role:            rbac.RoleAdmin.String(),
		EmailVerifiedAt: &now,
	}
	if err := repo.Create(u); err != nil {
		clearBootstrapPassword(cfg)
		return fmt.Errorf("bootstrap admin: create user: %w", err)
	}

	logger.Log.Info().Str("email", email).Msg("Bootstrap admin user created")
	clearBootstrapPassword(cfg)
	return nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
