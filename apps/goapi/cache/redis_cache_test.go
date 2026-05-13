package cache

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"goapi/models"
)

func newTestRedisCache(t *testing.T) (Cache, *miniredis.Miniredis, func()) {
	t.Helper()

	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr:         srv.Addr(),
		DialTimeout:  500 * time.Millisecond,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
	})
	c := NewRedisCache(client)

	cleanup := func() {
		_ = c.Close()
		srv.Close()
	}
	return c, srv, cleanup
}

func TestNewRedisCache_NilClientFallsBackToNoOp(t *testing.T) {
	t.Parallel()

	c := NewRedisCache(nil)
	if c.SupportsDistributedRateLimit() {
		t.Fatal("SupportsDistributedRateLimit() = true, want false for nil client fallback")
	}
}

func TestRedisCache_Get_MissVsRealError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c, srv, cleanup := newTestRedisCache(t)
	defer cleanup()

	if _, err := c.Get(ctx, "does-not-exist"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("Get() miss error = %v, want ErrCacheMiss", err)
	}

	srv.Close()
	_, err := c.Get(ctx, "any-key")
	if err == nil {
		t.Fatal("Get() after redis shutdown = nil, want real redis error")
	}
	if errors.Is(err, ErrCacheMiss) {
		t.Fatalf("Get() after redis shutdown = %v, want non-cache-miss error", err)
	}
}

func TestRedisCache_UserJSONRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c, _, cleanup := newTestRedisCache(t)
	defer cleanup()

	id := uuid.New()
	user := &models.User{
		ID:        id,
		FirstName: "Alice",
		LastName:  "Cache",
		Email:     "alice.cache@example.com",
		Role:      "user",
	}

	if err := c.SetUserByID(ctx, id, user, time.Minute); err != nil {
		t.Fatalf("SetUserByID() error = %v", err)
	}
	gotByID, err := c.GetUserByID(ctx, id)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if gotByID.Email != user.Email || gotByID.FirstName != user.FirstName {
		t.Fatalf("GetUserByID() = %+v, want email=%q first_name=%q", gotByID, user.Email, user.FirstName)
	}

	if err := c.SetUserByEmail(ctx, user.Email, user, time.Minute); err != nil {
		t.Fatalf("SetUserByEmail() error = %v", err)
	}
	gotByEmail, err := c.GetUserByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("GetUserByEmail() error = %v", err)
	}
	if gotByEmail.ID != user.ID {
		t.Fatalf("GetUserByEmail() id = %s, want %s", gotByEmail.ID, user.ID)
	}
}

func TestRedisCache_InvalidJSONDecodeBehavior(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c, srv, cleanup := newTestRedisCache(t)
	defer cleanup()

	id := uuid.New()
	key := UserIDKeyPrefix + id.String()
	srv.Set(key, "{invalid-json")

	_, err := c.GetUserByID(ctx, id)
	if err == nil {
		t.Fatal("GetUserByID() error = nil, want JSON decode error")
	}
	if errors.Is(err, ErrCacheMiss) {
		t.Fatalf("GetUserByID() error = %v, want decode error", err)
	}
	if !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("GetUserByID() error = %v, want JSON parse error", err)
	}
}

func TestRedisCache_DeleteBehavior(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c, _, cleanup := newTestRedisCache(t)
	defer cleanup()

	if err := c.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := c.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := c.Get(ctx, "k"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("Get() after Delete() error = %v, want ErrCacheMiss", err)
	}
}

func TestRedisCache_TTLBehavior(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c, srv, cleanup := newTestRedisCache(t)
	defer cleanup()

	if err := c.Set(ctx, "ttl-key", "ttl-value", 2*time.Second); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	v, err := c.Get(ctx, "ttl-key")
	if err != nil {
		t.Fatalf("Get() before expiry error = %v", err)
	}
	if v != "ttl-value" {
		t.Fatalf("Get() before expiry value = %q, want %q", v, "ttl-value")
	}

	srv.FastForward(3 * time.Second)
	if _, err := c.Get(ctx, "ttl-key"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("Get() after expiry error = %v, want ErrCacheMiss", err)
	}
}

func TestRedisCache_RateLimitIncrementSemantics(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c, srv, cleanup := newTestRedisCache(t)
	defer cleanup()

	key := "client:1"
	first, err := c.IncrementRateLimit(ctx, key, 2*time.Second)
	if err != nil {
		t.Fatalf("IncrementRateLimit(first) error = %v", err)
	}
	if first != 1 {
		t.Fatalf("IncrementRateLimit(first) = %d, want 1", first)
	}

	second, err := c.IncrementRateLimit(ctx, key, 2*time.Second)
	if err != nil {
		t.Fatalf("IncrementRateLimit(second) error = %v", err)
	}
	if second != 2 {
		t.Fatalf("IncrementRateLimit(second) = %d, want 2", second)
	}

	cur, err := c.GetRateLimit(ctx, key)
	if err != nil {
		t.Fatalf("GetRateLimit() error = %v", err)
	}
	if cur != 2 {
		t.Fatalf("GetRateLimit() = %d, want 2", cur)
	}

	srv.FastForward(3 * time.Second)
	cur, err = c.GetRateLimit(ctx, key)
	if err != nil {
		t.Fatalf("GetRateLimit() after window error = %v", err)
	}
	if cur != 0 {
		t.Fatalf("GetRateLimit() after window = %d, want 0", cur)
	}

	if _, err := c.IncrementRateLimit(ctx, key, 2*time.Second); err != nil {
		t.Fatalf("IncrementRateLimit(third) error = %v", err)
	}
	if err := c.ResetRateLimit(ctx, key); err != nil {
		t.Fatalf("ResetRateLimit() error = %v", err)
	}
	cur, err = c.GetRateLimit(ctx, key)
	if err != nil {
		t.Fatalf("GetRateLimit() after reset error = %v", err)
	}
	if cur != 0 {
		t.Fatalf("GetRateLimit() after reset = %d, want 0", cur)
	}
}
