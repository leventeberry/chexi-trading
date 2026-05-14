package container

import (
	"goapi/cache"
	"goapi/config"
	"goapi/internal/email"
	"goapi/internal/events"
	authinfra "goapi/internal/infra/auth"
	"goapi/internal/marketdata/state"
	"goapi/internal/queue"
	"goapi/repositories"
	"goapi/services"
	"gorm.io/gorm"
)

// Container holds all application dependencies.
type Container struct {
	DB                            *gorm.DB
	Cache                         cache.Cache
	JWT                           *authinfra.Manager
	Recorder                      events.Recorder
	RedisQueue                    *queue.RedisQueue // nil when jobs run inline (no async Redis queue)
	UserRepository                repositories.UserRepository
	OrganizationRepository        repositories.OrganizationRepository
	OrganizationNoteRepository    repositories.OrganizationNoteRepository
	OrganizationAPIKeyRepository  repositories.OrganizationAPIKeyRepository
	OrganizationWebhookRepository repositories.OrganizationWebhookRepository
	TradePlanRepository           repositories.TradePlanRepository
	UserService                   services.UserService
	OrganizationService           services.OrganizationService
	OrganizationNoteService       services.OrganizationNoteService
	OrganizationAPIKeyService     services.OrganizationAPIKeyService
	OrganizationWebhookService    services.OrganizationWebhookService
	TradePlanService              services.TradePlanService
	AuthService                   services.AuthService
	HealthService                 services.HealthService
	TickerStore                   *state.Store
}

// NewContainer creates and initializes a new dependency injection container.
func NewContainer(db *gorm.DB, cacheClient cache.Cache, jwt *authinfra.Manager, recorder events.Recorder, cfg *config.Config, jobQueue queue.Enqueuer, redisQueue *queue.RedisQueue, webhookSyncFallback bool, tickerStore *state.Store) *Container {
	userRepo := repositories.NewUserRepository(db)
	orgRepo := repositories.NewOrganizationRepository(db)
	noteRepo := repositories.NewOrganizationNoteRepository(db)
	apiKeyRepo := repositories.NewOrganizationAPIKeyRepository(db)
	webhookRepo := repositories.NewOrganizationWebhookRepository(db)
	tradePlanRepo := repositories.NewTradePlanRepository(db)
	settingsRepo := repositories.NewUserSettingsRepository(db)
	tokenRepo := repositories.NewAuthTokenRepository(db)
	userService := services.NewUserService(userRepo, settingsRepo, cacheClient, recorder)
	mail := email.FromConfig(cfg)
	webhookService := services.NewOrganizationWebhookService(webhookRepo, orgRepo, cfg, jobQueue, webhookSyncFallback)
	orgService := services.NewOrganizationService(db, orgRepo, userRepo, cfg, mail, jobQueue, webhookService)
	noteService := services.NewOrganizationNoteService(noteRepo, webhookService)
	apiKeyService := services.NewOrganizationAPIKeyService(apiKeyRepo, orgRepo)
	authService := services.NewAuthService(db, userRepo, tokenRepo, jwt, recorder, cfg.JWT.RefreshExpirationHours, mail, cfg, cacheClient, jobQueue)
	tradePlanService := services.NewTradePlanService(tradePlanRepo)
	healthService := services.NewHealthService(db, cacheClient)

	return &Container{
		DB:                            db,
		Cache:                         cacheClient,
		JWT:                           jwt,
		Recorder:                      recorder,
		RedisQueue:                    redisQueue,
		UserRepository:                userRepo,
		OrganizationRepository:        orgRepo,
		OrganizationNoteRepository:    noteRepo,
		OrganizationAPIKeyRepository:  apiKeyRepo,
		OrganizationWebhookRepository: webhookRepo,
		TradePlanRepository:           tradePlanRepo,
		UserService:                   userService,
		OrganizationService:           orgService,
		OrganizationNoteService:       noteService,
		OrganizationAPIKeyService:     apiKeyService,
		OrganizationWebhookService:    webhookService,
		TradePlanService:              tradePlanService,
		AuthService:                   authService,
		HealthService:                 healthService,
		TickerStore:                   tickerStore,
	}
}
