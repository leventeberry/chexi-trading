package services

import (
	"context"
	"fmt"

	"goapi/cache"
	"gorm.io/gorm"
)

// HealthService exposes infrastructure health checks to transport handlers.
type HealthService interface {
	Check(ctx context.Context) (map[string]interface{}, int)
}

type healthService struct {
	db    *gorm.DB
	cache cache.HealthChecker
}

// NewHealthService creates a health-check service backed by DB and cache.
func NewHealthService(db *gorm.DB, c cache.Cache) HealthService {
	var checker cache.HealthChecker
	if c != nil {
		checker = c
	}
	return &healthService{db: db, cache: checker}
}

// Check evaluates core dependencies and returns payload/status code.
func (s *healthService) Check(ctx context.Context) (map[string]interface{}, int) {
	health := map[string]interface{}{
		"status": "healthy",
	}

	sqlDB, err := s.db.DB()
	if err != nil {
		health["database"] = map[string]string{"status": "unhealthy", "error": "failed to get database connection"}
		health["status"] = "unhealthy"
		return health, 503
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		health["database"] = map[string]string{"status": "unhealthy", "error": err.Error()}
		health["status"] = "unhealthy"
		return health, 503
	}
	health["database"] = map[string]string{"status": "healthy"}

	if s.cache == nil {
		health["cache"] = map[string]string{"status": "disabled"}
		return health, 200
	}
	if err := s.cache.Ping(ctx); err != nil {
		health["cache"] = map[string]string{"status": "unhealthy", "error": fmt.Sprintf("cache ping failed: %v", err)}
		return health, 200
	}
	health["cache"] = map[string]string{"status": "healthy"}
	return health, 200
}
