package cache

import (
	"context"
	"time"

	"github.com/google/uuid"
	"goapi/models"
)

// noOpCache implements Cache interface as a no-op (no operation) cache
// Used when Redis is disabled or unavailable
type noOpCache struct{}

// NewNoOpCache creates a new no-op cache implementation
func NewNoOpCache() Cache {
	return &noOpCache{}
}

// GetUserByID always returns cache miss
func (n *noOpCache) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return nil, ErrCacheMiss
}

// SetUserByID does nothing
func (n *noOpCache) SetUserByID(ctx context.Context, id uuid.UUID, user *models.User, ttl time.Duration) error {
	return nil
}

// GetUserByEmail always returns cache miss
func (n *noOpCache) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return nil, ErrCacheMiss
}

// SetUserByEmail does nothing
func (n *noOpCache) SetUserByEmail(ctx context.Context, email string, user *models.User, ttl time.Duration) error {
	return nil
}

// DeleteUserByID does nothing
func (n *noOpCache) DeleteUserByID(ctx context.Context, id uuid.UUID) error {
	return nil
}

// DeleteUserByEmail does nothing
func (n *noOpCache) DeleteUserByEmail(ctx context.Context, email string) error {
	return nil
}

// DeleteUser does nothing
func (n *noOpCache) DeleteUser(ctx context.Context, id uuid.UUID, email string) error {
	return nil
}

// IncrementRateLimit always returns 0
func (n *noOpCache) IncrementRateLimit(ctx context.Context, key string, window time.Duration) (int, error) {
	return 0, ErrCacheDisabled
}

// GetRateLimit always returns 0
func (n *noOpCache) GetRateLimit(ctx context.Context, key string) (int, error) {
	return 0, nil
}

// ResetRateLimit does nothing
func (n *noOpCache) ResetRateLimit(ctx context.Context, key string) error {
	return nil
}

// Get always returns cache miss
func (n *noOpCache) Get(ctx context.Context, key string) (string, error) {
	return "", ErrCacheMiss
}

// Set does nothing
func (n *noOpCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return nil
}

// Delete does nothing
func (n *noOpCache) Delete(ctx context.Context, key string) error {
	return nil
}

// Exists always returns false
func (n *noOpCache) Exists(ctx context.Context, key string) (bool, error) {
	return false, nil
}

// Ping always succeeds (no-op)
func (n *noOpCache) Ping(ctx context.Context) error {
	return nil
}

// SupportsDistributedRateLimit returns false for no-op cache.
func (n *noOpCache) SupportsDistributedRateLimit() bool {
	return false
}

// Close is a no-op for no-op cache.
func (n *noOpCache) Close() error {
	return nil
}
