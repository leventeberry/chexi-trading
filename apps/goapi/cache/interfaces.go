package cache

import (
	"context"
	"time"

	"github.com/google/uuid"
	"goapi/models"
)

// UserCache defines user-specific cache operations.
type UserCache interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	SetUserByID(ctx context.Context, id uuid.UUID, user *models.User, ttl time.Duration) error
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	SetUserByEmail(ctx context.Context, email string, user *models.User, ttl time.Duration) error
	DeleteUserByID(ctx context.Context, id uuid.UUID) error
	DeleteUserByEmail(ctx context.Context, email string) error
	DeleteUser(ctx context.Context, id uuid.UUID, email string) error
}

// RateLimiterStore defines distributed rate limiting operations.
type RateLimiterStore interface {
	IncrementRateLimit(ctx context.Context, key string, window time.Duration) (int, error)
	GetRateLimit(ctx context.Context, key string) (int, error)
	ResetRateLimit(ctx context.Context, key string) error
	SupportsDistributedRateLimit() bool
}

// KeyValueStore defines generic key/value cache operations.
type KeyValueStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

// HealthChecker defines cache health and lifecycle operations.
type HealthChecker interface {
	Ping(ctx context.Context) error
	Close() error
}

// Cache composes all cache concerns used by the application.
type Cache interface {
	UserCache
	RateLimiterStore
	KeyValueStore
	HealthChecker
}
