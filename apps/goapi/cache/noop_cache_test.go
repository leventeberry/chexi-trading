package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"goapi/models"
)

func TestNoOpCache_GetReturnsCacheMiss(t *testing.T) {
	t.Parallel()

	c := NewNoOpCache()
	ctx := context.Background()

	if _, err := c.Get(ctx, "k"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("Get() error = %v, want ErrCacheMiss", err)
	}

	if _, err := c.GetUserByID(ctx, uuid.New()); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("GetUserByID() error = %v, want ErrCacheMiss", err)
	}

	if _, err := c.GetUserByEmail(ctx, "missing@example.com"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("GetUserByEmail() error = %v, want ErrCacheMiss", err)
	}
}

func TestNoOpCache_SetDeleteAreSafeNoOps(t *testing.T) {
	t.Parallel()

	c := NewNoOpCache()
	ctx := context.Background()
	id := uuid.New()
	user := &models.User{ID: id, Email: "noop@example.com"}

	if err := c.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("Set() error = %v, want nil", err)
	}
	if err := c.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}
	if err := c.SetUserByID(ctx, id, user, time.Minute); err != nil {
		t.Fatalf("SetUserByID() error = %v, want nil", err)
	}
	if err := c.SetUserByEmail(ctx, user.Email, user, time.Minute); err != nil {
		t.Fatalf("SetUserByEmail() error = %v, want nil", err)
	}
	if err := c.DeleteUserByID(ctx, id); err != nil {
		t.Fatalf("DeleteUserByID() error = %v, want nil", err)
	}
	if err := c.DeleteUserByEmail(ctx, user.Email); err != nil {
		t.Fatalf("DeleteUserByEmail() error = %v, want nil", err)
	}
	if err := c.DeleteUser(ctx, id, user.Email); err != nil {
		t.Fatalf("DeleteUser() error = %v, want nil", err)
	}
}

func TestNoOpCache_RateLimitBehavior(t *testing.T) {
	t.Parallel()

	c := NewNoOpCache()
	ctx := context.Background()

	count, err := c.IncrementRateLimit(ctx, "ip:1.2.3.4", time.Minute)
	if !errors.Is(err, ErrCacheDisabled) {
		t.Fatalf("IncrementRateLimit() error = %v, want ErrCacheDisabled", err)
	}
	if count != 0 {
		t.Fatalf("IncrementRateLimit() count = %d, want 0", count)
	}

	got, err := c.GetRateLimit(ctx, "ip:1.2.3.4")
	if err != nil {
		t.Fatalf("GetRateLimit() error = %v, want nil", err)
	}
	if got != 0 {
		t.Fatalf("GetRateLimit() count = %d, want 0", got)
	}

	if err := c.ResetRateLimit(ctx, "ip:1.2.3.4"); err != nil {
		t.Fatalf("ResetRateLimit() error = %v, want nil", err)
	}
	if c.SupportsDistributedRateLimit() {
		t.Fatal("SupportsDistributedRateLimit() = true, want false")
	}
}
