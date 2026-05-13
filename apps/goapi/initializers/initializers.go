package initializers

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"goapi/cache"
	"goapi/config"
	"goapi/internal/bootstrapadmin"
	"goapi/internal/email"
	"goapi/internal/events"
	"goapi/internal/queue"
	queuejobs "goapi/internal/queue/jobs"
	"goapi/logger"
	"goapi/models"
	"goapi/repositories"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// AppDependencies groups initialized infrastructure dependencies.
type AppDependencies struct {
	DB           *gorm.DB
	RedisClient  *redis.Client
	Cache        cache.Cache
	Config       *config.Config
	QueueEnqueue queue.Enqueuer
	RedisQueue   *queue.RedisQueue // nil when inline queue only; used for admin observability
	QueueWorker  *queue.Worker
}

// Bootstrap initializes environment, configuration, and infrastructure.
func Bootstrap() *AppDependencies {
	loadEnv()
	cfg := config.Load()
	db := connectDB(cfg)
	migrateDB(db, cfg)
	if err := bootstrapadmin.EnsureFirstAdmin(context.Background(), db, cfg); err != nil {
		logger.Log.Fatal().Err(err).Msg("Bootstrap admin failed")
	}
	redisClient := connectRedis(cfg)
	cacheClient := GetCacheClientFromRedis(redisClient)

	mailSender := email.FromConfig(cfg)
	jobRegistry := queue.NewRegistry()
	queuejobs.RegisterEmailHandlers(jobRegistry, mailSender, cfg)
	webhookRepo := repositories.NewOrganizationWebhookRepository(db)
	queuejobs.RegisterWebhookHandlers(jobRegistry, webhookRepo, cfg)
	jobRecorder := events.NewPostgresRecorder(db)
	jobEnqueue, jobWorker := queue.NewBundle(redisClient, cfg, jobRegistry, queue.BundleDeps{Recorder: jobRecorder})

	var redisQ *queue.RedisQueue
	if rq, ok := jobEnqueue.(*queue.RedisQueue); ok {
		redisQ = rq
	}

	return &AppDependencies{
		DB:           db,
		RedisClient:  redisClient,
		Cache:        cacheClient,
		Config:       cfg,
		QueueEnqueue: jobEnqueue,
		RedisQueue:   redisQ,
		QueueWorker:  jobWorker,
	}
}

// ValidateConfig validates required application configuration.
func ValidateConfig(cfg *config.Config) {
	if cfg == nil {
		logger.Log.Fatal().Msg("Configuration not loaded")
	}
}

// connectDB opens a PostgreSQL connection using GORM.
func connectDB(cfg *config.Config) *gorm.DB {
	ValidateConfig(cfg)
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		cfg.Database.Host,
		cfg.Database.User,
		cfg.Database.Pass,
		cfg.Database.Name,
		cfg.Database.Port,
		cfg.Database.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	logger.Log.Info().Msg("Database connection established")
	return db
}

// migrateDB runs versioned SQL migrations when USE_VERSIONED_MIGRATIONS=true, otherwise GORM AutoMigrate (local/dev default).
func migrateDB(db *gorm.DB, cfg *config.Config) {
	if db == nil {
		logger.Log.Fatal().Msg("Cannot run migrations: database is nil")
	}
	useVersionedMigrations := os.Getenv("USE_VERSIONED_MIGRATIONS") == "true"
	if config.IsProductionEnvironment(cfg.Environment) && !useVersionedMigrations {
		logger.Log.Fatal().Msg("Production requires USE_VERSIONED_MIGRATIONS=true; refusing AutoMigrate")
	}
	if useVersionedMigrations {
		dir := os.Getenv("MIGRATIONS_DIR")
		if err := RunVersionedMigrations(cfg, dir); err != nil {
			logger.Log.Fatal().Err(err).Msg("Versioned SQL migrations failed")
		}
		logger.Log.Info().Msg("Database schema managed by versioned SQL migrations")
		return
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.EventLog{},
		&models.Session{},
		&models.EmailVerificationToken{},
		&models.PasswordResetToken{},
		&models.UserSettings{},
		&models.UserTOTPFactor{},
		&models.MFAChallenge{},
		&models.UserMFARecoveryCode{},
		&models.UserOAuthAccount{},
		&models.OAuthAuthorizationState{},
		&models.OAuthExchangeCode{},
		&models.Organization{},
		&models.OrganizationMembership{},
		&models.OrganizationInvitation{},
		&models.OrganizationNote{},
		&models.OrganizationAPIKey{},
		&models.OrganizationWebhook{},
		&models.OrganizationWebhookDelivery{},
		// add future models here as needed,
	); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to run database migrations")
	}
	logger.Log.Info().Msg("GORM AutoMigrate completed")
}

// connectRedis opens a Redis connection if Redis is enabled.
func connectRedis(cfg *config.Config) *redis.Client {
	ValidateConfig(cfg)
	if !cfg.Redis.Enabled {
		logger.Log.Info().Msg("Redis is disabled (REDIS_ENABLED != true)")
		return nil
	}

	addr := fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port)
	redisTLSConfig, err := config.BuildRedisTLSConfig(
		cfg.Redis.TLSEnabled,
		cfg.Redis.TLSServerName,
		cfg.Redis.TLSCACert,
		cfg.Redis.TLSInsecureSkipVerify,
	)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Invalid Redis TLS configuration")
	}

	client := redis.NewClient(&redis.Options{
		Addr:      addr,
		Password:  cfg.Redis.Password,
		DB:        0, // Default DB
		TLSConfig: redisTLSConfig,
	})

	// Test connection
	ctx := context.Background()
	_, err = client.Ping(ctx).Result()
	if err != nil {
		logger.Log.Warn().Err(err).Str("address", addr).Msg("Failed to connect to Redis. Application will continue without Redis caching")
		_ = client.Close()
		return nil
	}

	logger.Log.Info().Str("address", addr).Msg("Redis connection established")
	return client
}

// GetCacheClient returns a cache client instance.
// Returns Redis cache if Redis is available, otherwise returns no-op cache.
func GetCacheClientFromRedis(redisClient *redis.Client) cache.Cache {
	if redisClient != nil {
		return cache.NewRedisCache(redisClient)
	}
	return cache.NewNoOpCache()
}

// CloseRedis closes the Redis connection if it exists.
func CloseRedis(redisClient *redis.Client) {
	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			logger.Log.Error().Err(err).Msg("Error closing Redis connection")
		} else {
			logger.Log.Info().Msg("Redis connection closed")
		}
	}
}

// loadEnv reads .env file into environment, if present
func loadEnv() {
	if err := godotenv.Load(); err != nil {
		logger.Log.Info().Msg("No .env file found; relying on environment variables")
	}
}
