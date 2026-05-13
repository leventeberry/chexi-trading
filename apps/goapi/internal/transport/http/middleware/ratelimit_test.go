package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"goapi/cache"
	"goapi/config"
)

var errSimulatedRedisFailure = errors.New("simulated redis failure")

// errIncrementCache implements Cache with distributed rate-limit support but fails every IncrementRateLimit.
type errIncrementCache struct {
	cache.Cache
}

func newErrIncrementCache() cache.Cache {
	return &errIncrementCache{Cache: cache.NewNoOpCache()}
}

func (e *errIncrementCache) SupportsDistributedRateLimit() bool { return true }

func (e *errIncrementCache) IncrementRateLimit(ctx context.Context, key string, window time.Duration) (int, error) {
	return 0, errSimulatedRedisFailure
}

func TestRateLimiterAllowBurstAndRefill(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiter(RateLimiterConfig{
		RequestsPerMinute: 60,
		BurstSize:         2,
	})
	defer rl.cleanupTick.Stop()

	ip := "127.0.0.1"
	if !rl.allow(ip) {
		t.Fatal("first request should be allowed")
	}
	if !rl.allow(ip) {
		t.Fatal("second request should be allowed")
	}
	if rl.allow(ip) {
		t.Fatal("third immediate request should be blocked")
	}

	rl.mu.RLock()
	entry := rl.entries[ip]
	rl.mu.RUnlock()
	entry.mu.Lock()
	entry.lastUpdate = time.Now().Add(-time.Minute)
	entry.mu.Unlock()

	if !rl.allow(ip) {
		t.Fatal("request should be allowed after simulated refill")
	}
}

func TestRateLimitMiddlewareWithCacheReturns429WhenExceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetRateLimiterGlobalsForTest()
	defer resetRateLimiterGlobalsForTest()

	cfg := &config.Config{}
	cfg.RateLimit.RequestsPerMinute = 1
	cfg.RateLimit.BurstSize = 1
	cfg.RateLimit.RedisFailureMode = config.RateLimitRedisFailureLocalFallback

	r := gin.New()
	r.Use(RateLimitMiddlewareWithCache(nil, cfg))
	r.GET("/limited", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req1 := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req1.RemoteAddr = "198.51.100.10:12345"
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request expected 200, got %d", w1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req2.RemoteAddr = "198.51.100.10:12345"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request expected 429, got %d body=%s", w2.Code, w2.Body.String())
	}
}

func TestRedisRateLimiterReturnsErrorOnIncrementFailure(t *testing.T) {
	t.Parallel()

	limiter := NewRedisRateLimiter(newErrIncrementCache(), RateLimiterConfig{
		RequestsPerMinute: 1,
		BurstSize:         1,
	})
	ok, err := limiter.allow(context.Background(), "ip-key")
	if err == nil {
		t.Fatal("expected redis error from increment failure")
	}
	if ok {
		t.Fatal("expected ok=false when store returns error")
	}
}

func TestRateLimitMiddleware_RedisFailClosedReturns503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetRateLimiterGlobalsForTest()
	defer resetRateLimiterGlobalsForTest()

	cfg := &config.Config{}
	cfg.Environment = config.EnvironmentProduction
	cfg.RateLimit.RequestsPerMinute = 100
	cfg.RateLimit.BurstSize = 100
	cfg.RateLimit.RedisFailureMode = config.RateLimitRedisFailureFailClosed

	r := gin.New()
	r.Use(RateLimitMiddlewareWithCache(newErrIncrementCache(), cfg))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "198.51.100.20:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "unavailable") {
		t.Fatalf("expected unavailable message, got %q", w.Body.String())
	}
}

func TestRateLimitMiddleware_RedisLocalFallbackUsesMemory429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetRateLimiterGlobalsForTest()
	defer resetRateLimiterGlobalsForTest()

	cfg := &config.Config{}
	cfg.Environment = config.EnvironmentProduction
	cfg.RateLimit.RequestsPerMinute = 1
	cfg.RateLimit.BurstSize = 1
	cfg.RateLimit.RedisFailureMode = config.RateLimitRedisFailureLocalFallback

	r := gin.New()
	r.Use(RateLimitMiddlewareWithCache(newErrIncrementCache(), cfg))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	addr := "198.51.100.30:12345"
	req1 := httptest.NewRequest(http.MethodGet, "/x", nil)
	req1.RemoteAddr = addr
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request expected 200, got %d", w1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	req2.RemoteAddr = addr
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request expected 429, got %d body=%s", w2.Code, w2.Body.String())
	}
}

func TestRateLimitMiddleware_RedisFailOpenAllowsWhenRedisErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetRateLimiterGlobalsForTest()
	defer resetRateLimiterGlobalsForTest()

	cfg := &config.Config{}
	cfg.Environment = config.EnvironmentDevelopment
	cfg.RateLimit.RequestsPerMinute = 2
	cfg.RateLimit.BurstSize = 2
	cfg.RateLimit.RedisFailureMode = config.RateLimitRedisFailureFailOpen

	r := gin.New()
	r.Use(RateLimitMiddlewareWithCache(newErrIncrementCache(), cfg))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	addr := "198.51.100.40:12345"
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = addr
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 with fail_open, got %d", i, w.Code)
		}
	}
}

func resetRateLimiterGlobalsForTest() {
	ShutdownRateLimiter()
	globalRateLimiter = nil
	globalRedisRateLimiter = nil
	useRedis = false
	rateLimiterOnce = sync.Once{}
}
