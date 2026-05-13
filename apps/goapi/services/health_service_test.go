package services

import (
	"context"
	"errors"
	"testing"

	"goapi/cache"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type failingPingCache struct {
	cache.Cache
}

func (f failingPingCache) Ping(ctx context.Context) error {
	return errors.New("redis unavailable")
}

func openSQLiteHealthDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func TestHealthService_Check_HealthyDBAndCache(t *testing.T) {
	t.Parallel()

	db := openSQLiteHealthDB(t)
	svc := NewHealthService(db, cache.NewNoOpCache())

	health, code := svc.Check(context.Background())
	if code != 200 {
		t.Fatalf("status code = %d, want 200", code)
	}
	if health["status"] != "healthy" {
		t.Fatalf("health status = %v, want healthy", health["status"])
	}

	database, ok := health["database"].(map[string]string)
	if !ok || database["status"] != "healthy" {
		t.Fatalf("database health = %#v, want status=healthy", health["database"])
	}

	cacheSection, ok := health["cache"].(map[string]string)
	if !ok || cacheSection["status"] != "healthy" {
		t.Fatalf("cache health = %#v, want status=healthy", health["cache"])
	}
}

func TestHealthService_Check_CacheOptionalDisabled(t *testing.T) {
	t.Parallel()

	db := openSQLiteHealthDB(t)
	svc := NewHealthService(db, nil)

	health, code := svc.Check(context.Background())
	if code != 200 {
		t.Fatalf("status code = %d, want 200", code)
	}

	cacheSection, ok := health["cache"].(map[string]string)
	if !ok || cacheSection["status"] != "disabled" {
		t.Fatalf("cache health = %#v, want status=disabled", health["cache"])
	}
}

func TestHealthService_Check_DBUnavailable(t *testing.T) {
	t.Parallel()

	db := openSQLiteHealthDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sql db: %v", err)
	}

	svc := NewHealthService(db, cache.NewNoOpCache())
	health, code := svc.Check(context.Background())
	if code != 503 {
		t.Fatalf("status code = %d, want 503", code)
	}
	if health["status"] != "unhealthy" {
		t.Fatalf("health status = %v, want unhealthy", health["status"])
	}

	database, ok := health["database"].(map[string]string)
	if !ok || database["status"] != "unhealthy" {
		t.Fatalf("database health = %#v, want status=unhealthy", health["database"])
	}
}

func TestHealthService_Check_DegradedCacheDoesNotFailEndpoint(t *testing.T) {
	t.Parallel()

	db := openSQLiteHealthDB(t)
	svc := NewHealthService(db, failingPingCache{Cache: cache.NewNoOpCache()})

	health, code := svc.Check(context.Background())
	if code != 200 {
		t.Fatalf("status code = %d, want 200", code)
	}

	cacheSection, ok := health["cache"].(map[string]string)
	if !ok || cacheSection["status"] != "unhealthy" {
		t.Fatalf("cache health = %#v, want status=unhealthy", health["cache"])
	}
}
