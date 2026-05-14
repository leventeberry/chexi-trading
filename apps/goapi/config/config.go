package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"goapi/logger"
)

// Config holds all application configuration.
type Config struct {
	Environment string
	Server      struct {
		Port              string
		ReadHeaderTimeout time.Duration
		ReadTimeout       time.Duration
		WriteTimeout      time.Duration
		IdleTimeout       time.Duration
	}
	Database struct {
		User    string
		Pass    string
		Host    string
		Port    string
		Name    string
		SSLMode string
	}
	Redis struct {
		Enabled               bool
		Host                  string
		Port                  string
		Password              string
		TLSEnabled            bool
		TLSServerName         string
		TLSCACert             string
		TLSInsecureSkipVerify bool
	}
	JWT struct {
		Secret                 string
		ExpirationDays         int
		AccessTokenMinutes     int
		RefreshExpirationHours int
	}
	RateLimit struct {
		RequestsPerMinute int
		BurstSize         int
		// RedisFailureMode: fail_closed | local_fallback | fail_open (see RATE_LIMIT_REDIS_FAILURE_MODE).
		RedisFailureMode string
	}
	// SecurityHeaders configures baseline HTTP security headers (see middleware.SecurityHeaders).
	SecurityHeaders struct {
		Enabled               bool
		HSTSEnabled           bool
		HSTSMaxAgeSeconds     int
		HSTSIncludeSubdomains bool
	}
	// AuthResponse controls optional fields in login/register JSON (defaults off if unset).
	AuthResponse struct {
		IncludeAPIKey bool
		IncludeUser   bool
	}
	// Audit controls HTTP request mirroring into event_log (high volume; off by default).
	Audit struct {
		HTTPEnabled      bool
		HTTPMutatingOnly bool
	}
	// Events controls rate limits for admin-only telemetry (POST /api/v1/events) and listing (GET).
	Events struct {
		TelemetryRequestsPerMinute int
		TelemetryBurstSize         int
	}
	// Queue configures Redis-backed background jobs with inline fallback when Redis/async is unavailable.
	Queue struct {
		Enabled          bool
		AsyncEnabled     bool // when false, execute jobs inline even if Redis is connected
		WorkerEnabled    bool // when false, Redis enqueue works but the in-process worker does not run
		Workers          int
		PollInterval     time.Duration
		MaxAttempts      int
		InitialBackoff   time.Duration
		MaxBackoff       time.Duration
		RetryDelayFixed  time.Duration // optional fixed delay between retries; 0 = use exponential backoff
		ShutdownTimeout  time.Duration
		DeadLetterMaxCap int // soft cap on DLQ list length (trim oldest); 0 = default 10000
	}
	Swagger struct {
		Enabled bool
	}
	// Bootstrap admin (optional one-time seed at startup). Password is sensitive — never log it.
	Bootstrap struct {
		Enabled  bool
		Email    string
		Password string
	}
	// Email transactional outbound (verification / password reset).
	Email struct {
		Enabled                        bool
		Provider                       string // log | resend
		From                           string
		AppPublicURL                   string // e.g. https://app.example.com — links built as {AppPublicURL}/verify-email?token=
		VerificationTTLHours           int
		PasswordResetTTLHours          int
		OrganizationInvitationTTLHours int    // pending org invite token lifetime
		ResendMinIntervalSeconds       int    // throttle resend / reset-request per email
		ResendAPIKey                   string // never log
		// RedirectAllTo is development-only: when non-empty and environment is not staging/production,
		// outbound recipients are replaced so all transactional mail lands in one inbox (see EMAIL_REDIRECT_ALL_TO).
		RedirectAllTo string
	}
	// MFA (TOTP). EncryptionKey empty = soft-disable: MFA HTTP routes return 503; login cannot complete MFA step-up.
	MFA struct {
		EncryptionKey       []byte // 32-byte AES key; never log
		ChallengeTTLMinutes int
		RecoveryCodeCount   int // 0 = no recovery codes generated on confirm
		TOTPIssuer          string
	}
	// Webhooks (org outbound). EncryptionKey empty = webhook CRUD returns 503; emits are no-ops.
	Webhooks struct {
		EncryptionKey []byte // 32-byte AES key for secret_ciphertext; never log
	}
	// CoinbaseExchangeWS configures optional public ticker ingestion (no private keys).
	CoinbaseExchangeWS struct {
		Enabled  bool
		URL      string
		Products []string
	}
	// OAuth (Google/GitHub). Secrets never logged.
	OAuth struct {
		GoogleEnabled      bool
		GoogleClientID     string
		GoogleClientSecret string
		GitHubEnabled      bool
		GitHubClientID     string
		GitHubClientSecret string
		RedirectBaseURL    string // API base for authorize redirect_uri + SPA receives oauth_code here
		StateTTLMinutes    int
		ExchangeTTLMinutes int
	}
}

func parseCoinbaseWSProducts(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{"BTC-USD", "ETH-USD", "SOL-USD"}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return []string{"BTC-USD", "ETH-USD", "SOL-USD"}
	}
	return out
}

var AppConfig *Config

// Load reads configuration from environment variables with defaults
func Load() *Config {
	cfg := &Config{}
	resolvedEnvironment, err := ResolveEnvironment()
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Environment resolution failed")
	}
	cfg.Environment = resolvedEnvironment

	// Server configuration
	cfg.Server.Port = getEnv("PORT", "8080")
	cfg.Server.ReadHeaderTimeout = durationSecondsEnv("HTTP_READ_HEADER_TIMEOUT_SEC", 5*time.Second)
	cfg.Server.ReadTimeout = durationSecondsEnv("HTTP_READ_TIMEOUT_SEC", 30*time.Second)
	cfg.Server.WriteTimeout = durationSecondsEnv("HTTP_WRITE_TIMEOUT_SEC", 30*time.Second)
	cfg.Server.IdleTimeout = durationSecondsEnv("HTTP_IDLE_TIMEOUT_SEC", 120*time.Second)

	// Database configuration
	cfg.Database.User = os.Getenv("DB_USER")
	cfg.Database.Pass = os.Getenv("DB_PASS")
	cfg.Database.Host = os.Getenv("DB_HOST")
	cfg.Database.Port = getEnv("DB_PORT", "5432")
	cfg.Database.Name = os.Getenv("DB_NAME")
	if cfg.Database.User == "" || cfg.Database.Pass == "" || cfg.Database.Host == "" || cfg.Database.Name == "" {
		logger.Log.Fatal().Msg("Database configuration is incomplete: DB_USER, DB_PASS, DB_HOST, and DB_NAME are required")
	}
	sslMode, err := resolveDatabaseSSLModeForEnvironment(cfg.Environment, os.Getenv("DB_SSLMODE"))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("DB_SSLMODE validation failed")
	}
	cfg.Database.SSLMode = sslMode

	// Redis configuration
	cfg.Redis.Enabled = os.Getenv("REDIS_ENABLED") == "true"
	cfg.Redis.Host = getEnv("REDIS_HOST", "localhost")
	cfg.Redis.Port = getEnv("REDIS_PORT", "6379")
	cfg.Redis.Password = os.Getenv("REDIS_PASSWORD")
	cfg.Redis.TLSEnabled, _ = optionalBoolEnv("REDIS_TLS_ENABLED")
	cfg.Redis.TLSServerName = strings.TrimSpace(os.Getenv("REDIS_TLS_SERVER_NAME"))
	cfg.Redis.TLSCACert = strings.TrimSpace(os.Getenv("REDIS_TLS_CA_CERT"))
	cfg.Redis.TLSInsecureSkipVerify, _ = optionalBoolEnv("REDIS_TLS_INSECURE_SKIP_VERIFY")
	if err := validateRedisTLSForEnvironment(cfg.Environment, cfg.Redis.Enabled, cfg.Redis.Host, cfg.Redis.TLSEnabled, cfg.Redis.TLSInsecureSkipVerify); err != nil {
		logger.Log.Fatal().Err(err).Msg("Redis TLS validation failed")
	}

	// JWT Configuration
	cfg.JWT.Secret = os.Getenv("JWT_SECRET")
	if cfg.JWT.Secret == "" {
		logger.Log.Fatal().Msg("JWT_SECRET environment variable is required")
	}
	if err := validateJWTSecretForEnvironment(cfg.Environment, cfg.JWT.Secret); err != nil {
		logger.Log.Fatal().Err(err).Msg("JWT_SECRET validation failed")
	}

	expirationDaysStr := os.Getenv("JWT_EXPIRATION_DAYS")
	if expirationDaysStr == "" {
		cfg.JWT.ExpirationDays = 1 // Legacy compatibility fallback
	} else {
		days, err := strconv.Atoi(expirationDaysStr)
		if err != nil || days < 1 {
			logger.Log.Warn().Str("value", expirationDaysStr).Msg("Invalid JWT_EXPIRATION_DAYS, using default 1")
			cfg.JWT.ExpirationDays = 1
		} else {
			cfg.JWT.ExpirationDays = days
		}
	}

	accessTokenMinutesStr := os.Getenv("JWT_ACCESS_TOKEN_MINUTES")
	if accessTokenMinutesStr == "" {
		cfg.JWT.AccessTokenMinutes = 15 // Default: short-lived access tokens
	} else {
		mins, err := strconv.Atoi(accessTokenMinutesStr)
		if err != nil || mins < 1 {
			logger.Log.Warn().Str("value", accessTokenMinutesStr).Msg("Invalid JWT_ACCESS_TOKEN_MINUTES, using default 15")
			cfg.JWT.AccessTokenMinutes = 15
		} else {
			cfg.JWT.AccessTokenMinutes = mins
		}
	}

	refreshHoursStr := os.Getenv("JWT_REFRESH_EXPIRATION_HOURS")
	if refreshHoursStr == "" {
		cfg.JWT.RefreshExpirationHours = 24 * 30 // Default: 30 days
	} else {
		hours, err := strconv.Atoi(refreshHoursStr)
		if err != nil || hours < 1 {
			logger.Log.Warn().Str("value", refreshHoursStr).Msg("Invalid JWT_REFRESH_EXPIRATION_HOURS, using default 720")
			cfg.JWT.RefreshExpirationHours = 24 * 30
		} else {
			cfg.JWT.RefreshExpirationHours = hours
		}
	}

	// Rate Limit Configuration
	requestsPerMinuteStr := os.Getenv("RATE_LIMIT_REQUESTS_PER_MINUTE")
	if requestsPerMinuteStr == "" {
		cfg.RateLimit.RequestsPerMinute = 60 // Default: 60 requests per minute
	} else {
		rpm, err := strconv.Atoi(requestsPerMinuteStr)
		if err != nil || rpm < 1 {
			logger.Log.Warn().Str("value", requestsPerMinuteStr).Msg("Invalid RATE_LIMIT_REQUESTS_PER_MINUTE, using default 60")
			cfg.RateLimit.RequestsPerMinute = 60
		} else {
			cfg.RateLimit.RequestsPerMinute = rpm
		}
	}

	burstSizeStr := os.Getenv("RATE_LIMIT_BURST_SIZE")
	if burstSizeStr == "" {
		cfg.RateLimit.BurstSize = 10 // Default: burst size of 10
	} else {
		burst, err := strconv.Atoi(burstSizeStr)
		if err != nil || burst < 1 {
			logger.Log.Warn().Str("value", burstSizeStr).Msg("Invalid RATE_LIMIT_BURST_SIZE, using default 10")
			cfg.RateLimit.BurstSize = 10
		} else {
			cfg.RateLimit.BurstSize = burst
		}
	}

	rlModeRaw := strings.TrimSpace(os.Getenv("RATE_LIMIT_REDIS_FAILURE_MODE"))
	rlMode, rlWarnInvalid, rlFatal := ResolveRateLimitRedisFailureMode(cfg.Environment, rlModeRaw)
	if rlFatal != nil {
		logger.Log.Fatal().Err(rlFatal).Msg("RATE_LIMIT_REDIS_FAILURE_MODE validation failed")
	}
	if rlWarnInvalid {
		logger.Log.Warn().Str("value", rlModeRaw).Msg("Invalid RATE_LIMIT_REDIS_FAILURE_MODE, using environment default")
	}
	cfg.RateLimit.RedisFailureMode = rlMode

	cfg.SecurityHeaders.Enabled = getenvBoolDefaultTrue("SECURITY_HEADERS_ENABLED")
	cfg.SecurityHeaders.HSTSEnabled = os.Getenv("HSTS_ENABLED") == "true"
	cfg.SecurityHeaders.HSTSMaxAgeSeconds = getenvIntDefault("HSTS_MAX_AGE_SECONDS", 31536000)
	cfg.SecurityHeaders.HSTSIncludeSubdomains = os.Getenv("HSTS_INCLUDE_SUBDOMAINS") == "true"

	// Auth response toggles (same pattern as REDIS_ENABLED: must be "true" to enable)
	cfg.AuthResponse.IncludeAPIKey = os.Getenv("AUTH_RESPONSE_INCLUDE_API_KEY") == "true"
	cfg.AuthResponse.IncludeUser = os.Getenv("AUTH_RESPONSE_INCLUDE_USER") == "true"

	// Audit HTTP → Postgres (AUDIT_HTTP_ENABLED=true)
	cfg.Audit.HTTPEnabled = os.Getenv("AUDIT_HTTP_ENABLED") == "true"
	cfg.Audit.HTTPMutatingOnly = getenvBoolDefaultTrue("AUDIT_HTTP_MUTATING_ONLY")

	// UI / SPA telemetry rate limit (defaults generous for dev)
	cfg.Events.TelemetryRequestsPerMinute = getenvIntDefault("EVENT_TELEMETRY_RATE_LIMIT_RPM", 120)
	cfg.Events.TelemetryBurstSize = getenvIntDefault("EVENT_TELEMETRY_BURST_SIZE", 30)

	// Background queue (Redis primary; inline sync fallback when Redis/async disabled).
	cfg.Queue.Enabled = getenvBoolDefaultTrue("QUEUE_ENABLED")
	cfg.Queue.AsyncEnabled = getenvBoolDefaultTrue("QUEUE_ASYNC_ENABLED")
	if v, ok := optionalBoolEnv("JOB_QUEUE_ENABLED"); ok {
		cfg.Queue.Enabled = v
	}
	cfg.Queue.WorkerEnabled = getenvBoolDefaultTrue("JOB_WORKER_ENABLED")
	cfg.Queue.Workers = getenvIntDefault("QUEUE_WORKERS", 2)
	cfg.Queue.PollInterval = durationMillisEnv("QUEUE_POLL_INTERVAL_MS", 500*time.Millisecond)
	cfg.Queue.MaxAttempts = getenvIntDefault("QUEUE_MAX_ATTEMPTS", 5)
	if s := strings.TrimSpace(os.Getenv("JOB_MAX_ATTEMPTS")); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 1 {
			cfg.Queue.MaxAttempts = n
		} else {
			logger.Log.Warn().Str("key", "JOB_MAX_ATTEMPTS").Str("value", s).Msg("Invalid int env; ignoring")
		}
	}
	cfg.Queue.InitialBackoff = durationMillisEnv("QUEUE_INITIAL_BACKOFF_MS", time.Second)
	cfg.Queue.MaxBackoff = durationMillisEnv("QUEUE_MAX_BACKOFF_MS", 5*time.Minute)
	if s := strings.TrimSpace(os.Getenv("JOB_RETRY_DELAY_SECONDS")); s != "" {
		sec, err := strconv.Atoi(s)
		if err != nil || sec < 0 {
			logger.Log.Warn().Str("key", "JOB_RETRY_DELAY_SECONDS").Str("value", s).Msg("Invalid seconds; ignoring")
		} else if sec > 0 {
			cfg.Queue.RetryDelayFixed = time.Duration(sec) * time.Second
		}
	}
	cfg.Queue.ShutdownTimeout = durationSecondsEnv("QUEUE_SHUTDOWN_TIMEOUT_SEC", 30*time.Second)
	cfg.Queue.DeadLetterMaxCap = getenvIntDefault("QUEUE_DEAD_LETTER_MAX_CAP", 10000)

	cfg.Swagger.Enabled = resolveSwaggerEnabled(cfg.Environment)

	// Bootstrap admin (explicit opt-in; see README). Values validated by bootstrapadmin.EnsureFirstAdmin.
	cfg.Bootstrap.Enabled = os.Getenv("BOOTSTRAP_ADMIN_ENABLED") == "true"
	cfg.Bootstrap.Email = strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL"))
	cfg.Bootstrap.Password = os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	if cfg.Bootstrap.Enabled {
		if cfg.Bootstrap.Email == "" || cfg.Bootstrap.Password == "" {
			logger.Log.Fatal().Msg("BOOTSTRAP_ADMIN_ENABLED=true requires BOOTSTRAP_ADMIN_EMAIL and BOOTSTRAP_ADMIN_PASSWORD")
		}
	}

	// Email (verification / password reset). Default: enabled with log sink in dev/test; off in staging/prod unless EMAIL_ENABLED=true.
	cfg.Email.Provider = strings.TrimSpace(getEnv("EMAIL_PROVIDER", "log"))
	cfg.Email.From = strings.TrimSpace(getEnv("EMAIL_FROM", "noreply@localhost"))
	cfg.Email.AppPublicURL = strings.TrimSpace(os.Getenv("APP_PUBLIC_URL"))
	cfg.Email.VerificationTTLHours = getenvIntDefault("EMAIL_VERIFICATION_TTL_HOURS", 48)
	cfg.Email.PasswordResetTTLHours = getenvIntDefault("EMAIL_PASSWORD_RESET_TTL_HOURS", 1)
	cfg.Email.OrganizationInvitationTTLHours = getenvIntDefault("ORG_INVITATION_TTL_HOURS", 168)
	cfg.Email.ResendMinIntervalSeconds = getenvIntDefault("EMAIL_RESEND_MIN_INTERVAL_SECONDS", 60)
	cfg.Email.ResendAPIKey = os.Getenv("RESEND_API_KEY") // never log
	cfg.Email.RedirectAllTo = strings.TrimSpace(os.Getenv("EMAIL_REDIRECT_ALL_TO"))
	switch strings.ToLower(strings.TrimSpace(os.Getenv("EMAIL_ENABLED"))) {
	case "true":
		cfg.Email.Enabled = true
	case "false":
		cfg.Email.Enabled = false
	default:
		cfg.Email.Enabled = !IsStagingOrProductionEnvironment(cfg.Environment)
	}

	// MFA TOTP (optional encryption key)
	cfg.MFA.ChallengeTTLMinutes = getenvIntDefault("MFA_CHALLENGE_TTL_MINUTES", 5)
	cfg.MFA.RecoveryCodeCount = getenvIntDefault("MFA_RECOVERY_CODE_COUNT", 8)
	cfg.MFA.TOTPIssuer = strings.TrimSpace(getEnv("MFA_TOTP_ISSUER", "goapi"))
	if cfg.MFA.TOTPIssuer == "" {
		cfg.MFA.TOTPIssuer = "goapi"
	}
	if raw := strings.TrimSpace(os.Getenv("MFA_ENCRYPTION_KEY")); raw != "" {
		key, err := decodeMFAEncryptionKey(raw)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("MFA_ENCRYPTION_KEY is invalid; MFA enrollment endpoints will return 503")
		} else {
			cfg.MFA.EncryptionKey = key
		}
	}

	if raw := strings.TrimSpace(os.Getenv("WEBHOOK_ENCRYPTION_KEY")); raw != "" {
		key, err := decodeMFAEncryptionKey(raw)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("WEBHOOK_ENCRYPTION_KEY is invalid; organization webhook endpoints will return 503")
		} else {
			cfg.Webhooks.EncryptionKey = key
		}
	}

	// Coinbase Exchange public WebSocket (optional; disabled by default — see COINBASE_WS_ENABLED).
	cfg.CoinbaseExchangeWS.Enabled = os.Getenv("COINBASE_WS_ENABLED") == "true"
	switch strings.ToLower(strings.TrimSpace(os.Getenv("COINBASE_WS_ENVIRONMENT"))) {
	case "production":
		cfg.CoinbaseExchangeWS.URL = strings.TrimSpace(os.Getenv("COINBASE_WS_URL_PRODUCTION"))
		if cfg.CoinbaseExchangeWS.URL == "" {
			cfg.CoinbaseExchangeWS.URL = "wss://ws-feed.exchange.coinbase.com"
		}
	default:
		cfg.CoinbaseExchangeWS.URL = strings.TrimSpace(os.Getenv("COINBASE_WS_URL_SANDBOX"))
		if cfg.CoinbaseExchangeWS.URL == "" {
			cfg.CoinbaseExchangeWS.URL = "wss://ws-feed-public.sandbox.exchange.coinbase.com"
		}
	}
	// If COINBASE_WS_URL is set manually, it takes full precedence.
	if override := strings.TrimSpace(os.Getenv("COINBASE_WS_URL")); override != "" {
		cfg.CoinbaseExchangeWS.URL = override
	}
	if cfg.CoinbaseExchangeWS.Enabled {
		cfg.CoinbaseExchangeWS.Products = parseCoinbaseWSProducts(os.Getenv("COINBASE_WS_PRODUCTS"))
	}

	// OAuth Google/GitHub (optional per provider)
	cfg.OAuth.GoogleEnabled = os.Getenv("OAUTH_GOOGLE_ENABLED") == "true"
	cfg.OAuth.GitHubEnabled = os.Getenv("OAUTH_GITHUB_ENABLED") == "true"
	cfg.OAuth.GoogleClientID = strings.TrimSpace(os.Getenv("OAUTH_GOOGLE_CLIENT_ID"))
	cfg.OAuth.GoogleClientSecret = os.Getenv("OAUTH_GOOGLE_CLIENT_SECRET")
	cfg.OAuth.GitHubClientID = strings.TrimSpace(os.Getenv("OAUTH_GITHUB_CLIENT_ID"))
	cfg.OAuth.GitHubClientSecret = os.Getenv("OAUTH_GITHUB_CLIENT_SECRET")
	cfg.OAuth.RedirectBaseURL = strings.TrimSpace(os.Getenv("OAUTH_REDIRECT_BASE_URL"))
	cfg.OAuth.StateTTLMinutes = getenvIntDefault("OAUTH_STATE_TTL_MINUTES", 10)
	cfg.OAuth.ExchangeTTLMinutes = getenvIntDefault("OAUTH_EXCHANGE_TTL_MINUTES", 10)
	if cfg.OAuth.GoogleEnabled || cfg.OAuth.GitHubEnabled {
		if cfg.OAuth.RedirectBaseURL == "" {
			logger.Log.Fatal().Msg("OAUTH_REDIRECT_BASE_URL is required when OAuth is enabled")
		}
		if err := validateOAuthRedirectBaseURL(cfg.Environment, cfg.OAuth.RedirectBaseURL); err != nil {
			logger.Log.Fatal().Err(err).Msg("OAUTH_REDIRECT_BASE_URL validation failed")
		}
		if cfg.OAuth.GoogleEnabled {
			if cfg.OAuth.GoogleClientID == "" || cfg.OAuth.GoogleClientSecret == "" {
				logger.Log.Fatal().Msg("OAUTH_GOOGLE_ENABLED=true requires OAUTH_GOOGLE_CLIENT_ID and OAUTH_GOOGLE_CLIENT_SECRET")
			}
		}
		if cfg.OAuth.GitHubEnabled {
			if cfg.OAuth.GitHubClientID == "" || cfg.OAuth.GitHubClientSecret == "" {
				logger.Log.Fatal().Msg("OAUTH_GITHUB_ENABLED=true requires OAUTH_GITHUB_CLIENT_ID and OAUTH_GITHUB_CLIENT_SECRET")
			}
		}
	}

	AppConfig = cfg
	return cfg
}

// OAuthGoogleConfigured reports whether Google OAuth is enabled with credentials.
func OAuthGoogleConfigured(cfg *Config) bool {
	return cfg != nil && cfg.OAuth.GoogleEnabled && cfg.OAuth.GoogleClientID != "" && cfg.OAuth.GoogleClientSecret != ""
}

// OAuthGitHubConfigured reports whether GitHub OAuth is enabled with credentials.
func OAuthGitHubConfigured(cfg *Config) bool {
	return cfg != nil && cfg.OAuth.GitHubEnabled && cfg.OAuth.GitHubClientID != "" && cfg.OAuth.GitHubClientSecret != ""
}

// OAuthProviderCallbackURL builds the redirect_uri registered at the IdP (must match exactly).
func OAuthProviderCallbackURL(cfg *Config, provider string) string {
	if cfg == nil {
		return ""
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.OAuth.RedirectBaseURL), "/")
	return base + "/api/v1/oauth/" + provider + "/callback"
}

// OAuthStateTTL returns TTL for authorization state rows.
func OAuthStateTTL(cfg *Config) time.Duration {
	if cfg == nil || cfg.OAuth.StateTTLMinutes < 1 {
		return 10 * time.Minute
	}
	if cfg.OAuth.StateTTLMinutes > 60 {
		return 60 * time.Minute
	}
	return time.Duration(cfg.OAuth.StateTTLMinutes) * time.Minute
}

// OAuthExchangeTTL returns TTL for oauth_code exchange rows.
func OAuthExchangeTTL(cfg *Config) time.Duration {
	if cfg == nil || cfg.OAuth.ExchangeTTLMinutes < 1 {
		return 10 * time.Minute
	}
	if cfg.OAuth.ExchangeTTLMinutes > 60 {
		return 60 * time.Minute
	}
	return time.Duration(cfg.OAuth.ExchangeTTLMinutes) * time.Minute
}

func validateOAuthRedirectBaseURL(env, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("OAUTH_REDIRECT_BASE_URL must use http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("OAUTH_REDIRECT_BASE_URL must include host")
	}
	if u.User != nil {
		return fmt.Errorf("OAUTH_REDIRECT_BASE_URL must not include userinfo")
	}
	normalizedEnv := NormalizeEnvironment(env)
	if normalizedEnv == EnvironmentStaging || normalizedEnv == EnvironmentProduction {
		host := strings.ToLower(strings.TrimSpace(u.Hostname()))
		isLocal := host == "localhost" || host == "127.0.0.1"
		if u.Scheme != "https" && !isLocal {
			return fmt.Errorf("OAUTH_REDIRECT_BASE_URL must use https in %s", normalizedEnv)
		}
	}
	return nil
}

// MFAEncryptionConfigured reports whether AES-256-GCM key material is present (MFA routes may operate).
func MFAEncryptionConfigured(cfg *Config) bool {
	return cfg != nil && len(cfg.MFA.EncryptionKey) == 32
}

// WebhooksEncryptionConfigured reports whether webhook secrets can be encrypted at rest.
func WebhooksEncryptionConfigured(cfg *Config) bool {
	return cfg != nil && len(cfg.Webhooks.EncryptionKey) == 32
}

// MFAChallengeTTL returns the lifetime of login step-up challenge tokens.
func MFAChallengeTTL(cfg *Config) time.Duration {
	if cfg == nil || cfg.MFA.ChallengeTTLMinutes < 1 {
		return 5 * time.Minute
	}
	if cfg.MFA.ChallengeTTLMinutes > 30 {
		return 30 * time.Minute
	}
	return time.Duration(cfg.MFA.ChallengeTTLMinutes) * time.Minute
}

func decodeMFAEncryptionKey(raw string) ([]byte, error) {
	// Prefer standard base64
	if b, err := base64.StdEncoding.DecodeString(raw); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(raw); err == nil && len(b) == 32 {
		return b, nil
	}
	b, err := hex.DecodeString(strings.TrimPrefix(raw, "0x"))
	if err != nil {
		return nil, fmt.Errorf("MFA_ENCRYPTION_KEY must be 32 bytes as base64 or 64 hex chars")
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("MFA_ENCRYPTION_KEY decoded length %d, want 32", len(b))
	}
	return b, nil
}

func resolveSwaggerEnabled(env string) bool {
	if enabled, ok := optionalBoolEnv("SWAGGER_ENABLED"); ok {
		return enabled
	}
	return !IsStagingOrProductionEnvironment(env)
}

func optionalBoolEnv(key string) (bool, bool) {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if raw == "" {
		return false, false
	}
	switch raw {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	default:
		logger.Log.Warn().Str("key", key).Str("value", raw).Msg("Invalid bool env; using default")
		return false, false
	}
}

var jwtSecretDenylist = map[string]struct{}{
	"secret":                            {},
	"changeme":                          {},
	"change-me":                         {},
	"default":                           {},
	"jwt_secret":                        {},
	"your-secret-key":                   {},
	"replace-with-strong-random-secret": {},
	"dev-secret":                        {},
	"test-secret":                       {},
	"admin":                             {},
}

const jwtSecretMinLenStagingProd = 32

const (
	dbSSLModeDisable    = "disable"
	dbSSLModeAllow      = "allow"
	dbSSLModePrefer     = "prefer"
	dbSSLModeRequire    = "require"
	dbSSLModeVerifyCA   = "verify-ca"
	dbSSLModeVerifyFull = "verify-full"
)

var allowedDBSSLModes = map[string]struct{}{
	dbSSLModeDisable:    {},
	dbSSLModeAllow:      {},
	dbSSLModePrefer:     {},
	dbSSLModeRequire:    {},
	dbSSLModeVerifyCA:   {},
	dbSSLModeVerifyFull: {},
}

func validateJWTSecretForEnvironment(env, secret string) error {
	normalizedEnv := NormalizeEnvironment(env)
	if normalizedEnv != EnvironmentStaging && normalizedEnv != EnvironmentProduction {
		return nil
	}
	s := strings.TrimSpace(secret)
	if len(s) < jwtSecretMinLenStagingProd {
		return fmt.Errorf("JWT_SECRET must be at least %d characters in %s", jwtSecretMinLenStagingProd, normalizedEnv)
	}
	key := strings.ToLower(s)
	if _, banned := jwtSecretDenylist[key]; banned {
		return fmt.Errorf("JWT_SECRET is a forbidden placeholder value in %s", normalizedEnv)
	}
	return nil
}

func resolveDatabaseSSLModeForEnvironment(env, rawSSLMode string) (string, error) {
	sslMode := strings.ToLower(strings.TrimSpace(rawSSLMode))
	if sslMode == "" {
		switch NormalizeEnvironment(env) {
		case EnvironmentDevelopment, EnvironmentTest:
			return dbSSLModeDisable, nil
		default:
			return "", fmt.Errorf("DB_SSLMODE is required in %s and must be one of: require, verify-ca, verify-full", NormalizeEnvironment(env))
		}
	}
	if _, ok := allowedDBSSLModes[sslMode]; !ok {
		return "", fmt.Errorf("invalid DB_SSLMODE %q: expected one of disable, allow, prefer, require, verify-ca, verify-full", rawSSLMode)
	}
	if err := validateDatabaseSSLModeForEnvironment(env, sslMode); err != nil {
		return "", err
	}
	return sslMode, nil
}

func validateDatabaseSSLModeForEnvironment(env, sslMode string) error {
	normalizedEnv := NormalizeEnvironment(env)
	switch normalizedEnv {
	case EnvironmentProduction:
		if sslMode != dbSSLModeVerifyFull {
			return fmt.Errorf("DB_SSLMODE=%s is not allowed in production; only verify-full is permitted", sslMode)
		}
	case EnvironmentStaging:
		switch sslMode {
		case dbSSLModeRequire, dbSSLModeVerifyCA, dbSSLModeVerifyFull:
			return nil
		default:
			return fmt.Errorf("DB_SSLMODE=%s is not allowed in staging; allowed values: require, verify-ca, verify-full", sslMode)
		}
	}
	return nil
}

func validateRedisTLSForEnvironment(env string, redisEnabled bool, redisHost string, redisTLSEnabled bool, redisTLSInsecureSkipVerify bool) error {
	normalizedEnv := NormalizeEnvironment(env)
	if !redisEnabled {
		return nil
	}
	if (normalizedEnv == EnvironmentStaging || normalizedEnv == EnvironmentProduction) && redisTLSInsecureSkipVerify {
		return fmt.Errorf("REDIS_TLS_INSECURE_SKIP_VERIFY=true is not allowed in %s", normalizedEnv)
	}
	if (normalizedEnv == EnvironmentStaging || normalizedEnv == EnvironmentProduction) && !isLocalRedisHost(redisHost) && !redisTLSEnabled {
		return fmt.Errorf("REDIS_TLS_ENABLED=true is required in %s when REDIS_ENABLED=true and REDIS_HOST=%q is non-local", normalizedEnv, redisHost)
	}
	return nil
}

func isLocalRedisHost(host string) bool {
	normalized := strings.TrimSpace(strings.ToLower(host))
	if normalized == "" {
		return false
	}
	switch normalized {
	case "localhost":
		return true
	}
	ip := net.ParseIP(normalized)
	return ip != nil && ip.IsLoopback()
}

// BuildRedisTLSConfig creates a Redis TLS config when TLS is enabled.
func BuildRedisTLSConfig(tlsEnabled bool, serverName, caCertPEM string, insecureSkipVerify bool) (*tls.Config, error) {
	if !tlsEnabled {
		return nil, nil
	}
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: strings.TrimSpace(serverName),
		// #nosec G402 -- insecureSkipVerify is driven by REDIS_TLS_INSECURE_SKIP_VERIFY; staging/production forbid true in validateRedisTLSForEnvironment.
		InsecureSkipVerify: insecureSkipVerify,
	}
	caCertPEM = strings.TrimSpace(caCertPEM)
	if caCertPEM == "" {
		return tlsConfig, nil
	}
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM([]byte(caCertPEM)); !ok {
		return nil, fmt.Errorf("REDIS_TLS_CA_CERT is not valid PEM")
	}
	tlsConfig.RootCAs = pool
	return tlsConfig, nil
}

func getenvIntDefault(key string, fallback int) int {
	s := os.Getenv(key)
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		logger.Log.Warn().Str("key", key).Str("value", s).Msg("Invalid int env; using default")
		return fallback
	}
	return n
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func durationSecondsEnv(key string, fallback time.Duration) time.Duration {
	s := os.Getenv(key)
	if s == "" {
		return fallback
	}
	sec, err := strconv.Atoi(s)
	if err != nil || sec < 0 {
		logger.Log.Warn().Str("key", key).Str("value", s).Msg("Invalid duration seconds; using default")
		return fallback
	}
	return time.Duration(sec) * time.Second
}

func getenvBoolDefaultTrue(key string) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if raw == "" {
		return true
	}
	switch raw {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		logger.Log.Warn().Str("key", key).Str("value", raw).Msg("Invalid bool env; using default true")
		return true
	}
}

func durationMillisEnv(key string, fallback time.Duration) time.Duration {
	s := os.Getenv(key)
	if s == "" {
		return fallback
	}
	ms, err := strconv.Atoi(s)
	if err != nil || ms < 1 {
		logger.Log.Warn().Str("key", key).Str("value", s).Msg("Invalid duration milliseconds; using default")
		return fallback
	}
	return time.Duration(ms) * time.Millisecond
}

// PostgresURL returns a postgres connection URL suitable for golang-migrate (password/user are URL-escaped).
func PostgresURL(cfg *Config) string {
	if cfg == nil {
		return ""
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		url.QueryEscape(cfg.Database.User),
		url.QueryEscape(cfg.Database.Pass),
		cfg.Database.Host,
		cfg.Database.Port,
		url.PathEscape(cfg.Database.Name),
		url.QueryEscape(cfg.Database.SSLMode),
	)
}

// Get returns the global configuration instance
func Get() *Config {
	if AppConfig == nil {
		logger.Log.Fatal().Msg("Configuration not loaded. Call config.Load() first.")
	}
	return AppConfig
}
