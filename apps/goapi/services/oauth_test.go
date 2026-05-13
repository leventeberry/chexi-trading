package services

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"goapi/cache"
	"goapi/config"
	"goapi/internal/email"
	"goapi/internal/events"
	authinfra "goapi/internal/infra/auth"
	"goapi/internal/queue"
	queuejobs "goapi/internal/queue/jobs"
	"goapi/models"
	"goapi/repositories"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func openOAuthIsolatedDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "oauth.sqlite")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func oauthTestCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.Environment = config.EnvironmentDevelopment
	// Non-hex placeholder avoids gitleaks generic-api-key on unit-test fixtures (still 32+ chars for HMAC).
	cfg.JWT.Secret = "unit-test-jwt-hmac-secret-not-real-x"
	cfg.JWT.AccessTokenMinutes = 15
	cfg.JWT.RefreshExpirationHours = 24 * 7
	cfg.OAuth.GoogleEnabled = true
	cfg.OAuth.GoogleClientID = "oauth-unit-google-client-id"
	cfg.OAuth.GoogleClientSecret = "oauth-unit-google-secret"
	cfg.OAuth.RedirectBaseURL = "http://127.0.0.1:3000"
	cfg.OAuth.StateTTLMinutes = 10
	cfg.OAuth.ExchangeTTLMinutes = 10
	return cfg
}

func migrateOAuthRelated(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(
		&models.User{},
		&models.Session{},
		&models.EmailVerificationToken{},
		&models.PasswordResetToken{},
		&models.UserOAuthAccount{},
		&models.OAuthAuthorizationState{},
		&models.OAuthExchangeCode{},
		&models.UserTOTPFactor{},
		&models.MFAChallenge{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
}

func newOAuthAuthService(t *testing.T, db *gorm.DB, cfg *config.Config) AuthService {
	t.Helper()
	jwt := authinfra.NewManager(cfg.JWT.Secret, cfg.JWT.AccessTokenMinutes)
	userRepo := repositories.NewUserRepository(db)
	tokenRepo := repositories.NewAuthTokenRepository(db)
	reg := queue.NewRegistry()
	queuejobs.RegisterEmailHandlers(reg, email.FromConfig(cfg), cfg)
	jobQ := queue.NewInlineQueue(reg, events.NoOpRecorder{}, cfg)
	return NewAuthService(db, userRepo, tokenRepo, jwt, events.NoOpRecorder{}, cfg.JWT.RefreshExpirationHours, email.FromConfig(cfg), cfg, cache.NewNoOpCache(), jobQ)
}

func TestOAuthAuthorizeURL_InvalidProvider(t *testing.T) {
	db := openOAuthIsolatedDB(t)
	migrateOAuthRelated(t, db)
	svc := newOAuthAuthService(t, db, oauthTestCfg(t))

	_, err := svc.OAuthAuthorizeURL(context.Background(), "twitter", nil)
	if err != ErrOAuthInvalidProvider {
		t.Fatalf("OAuthAuthorizeURL twitter = %v, want %v", err, ErrOAuthInvalidProvider)
	}
}

func TestOAuthAuthorizeURL_PersistsStateAndReturnsGoogleAuthorizeURL(t *testing.T) {
	db := openOAuthIsolatedDB(t)
	migrateOAuthRelated(t, db)
	svc := newOAuthAuthService(t, db, oauthTestCfg(t))

	urlStr, err := svc.OAuthAuthorizeURL(context.Background(), OAuthProviderGoogle, nil)
	if err != nil {
		t.Fatalf("OAuthAuthorizeURL: %v", err)
	}
	if !strings.Contains(urlStr, "accounts.google.com") && !strings.Contains(urlStr, "google") {
		t.Fatalf("unexpected authorize URL: %s", urlStr)
	}
	if !strings.Contains(urlStr, "state=") || !strings.Contains(urlStr, "code_challenge=") {
		t.Fatalf("authorize URL missing PKCE/state params: %s", urlStr)
	}

	var count int64
	if err := db.Model(&models.OAuthAuthorizationState{}).Count(&count).Error; err != nil {
		t.Fatalf("count states: %v", err)
	}
	if count != 1 {
		t.Fatalf("oauth states count = %d, want 1", count)
	}
}

func TestOAuthAuthorizeURL_WhenProviderDisabled(t *testing.T) {
	db := openOAuthIsolatedDB(t)
	migrateOAuthRelated(t, db)
	cfg := oauthTestCfg(t)
	cfg.OAuth.GoogleEnabled = false
	cfg.OAuth.GoogleClientID = ""
	cfg.OAuth.GoogleClientSecret = ""
	svc := newOAuthAuthService(t, db, cfg)

	_, err := svc.OAuthAuthorizeURL(context.Background(), OAuthProviderGoogle, nil)
	if err != ErrOAuthProviderDisabled {
		t.Fatalf("error = %v, want %v", err, ErrOAuthProviderDisabled)
	}
}

func TestOAuthCompleteExchange_RejectsEmptyAndUnknownCode(t *testing.T) {
	db := openOAuthIsolatedDB(t)
	migrateOAuthRelated(t, db)
	svc := newOAuthAuthService(t, db, oauthTestCfg(t))

	_, err := svc.OAuthCompleteExchange(context.Background(), "")
	if err != ErrOAuthExchangeInvalid {
		t.Fatalf("empty code: got %v, want %v", err, ErrOAuthExchangeInvalid)
	}

	_, err = svc.OAuthCompleteExchange(context.Background(), "not-a-real-exchange-code")
	if err != ErrOAuthExchangeInvalid {
		t.Fatalf("unknown code: got %v, want %v", err, ErrOAuthExchangeInvalid)
	}
}
