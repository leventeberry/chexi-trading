package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"goapi/config"
	"goapi/internal/queue"
)

func TestAdminJobsHealth_WithRedisQueue(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
		srv.Close()
	})
	cfg := &config.Config{}
	cfg.Queue.Enabled = true
	cfg.Queue.WorkerEnabled = true
	cfg.Queue.DeadLetterMaxCap = 100
	rq := queue.NewRedisQueue(client, cfg)

	ctx := context.Background()
	_ = rq.Enqueue(ctx, "t", json.RawMessage(`{}`), queue.EnqueueOptions{})
	job := queue.Job{
		ID:          "dlq-1",
		Type:        "t2",
		Payload:     json.RawMessage(`{}`),
		Status:      queue.StatusDeadLetter,
		Attempts:    1,
		MaxAttempts: 3,
		LastError:   "handler failed",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	raw, _ := json.Marshal(job)
	_ = rq.PushDeadLetter(ctx, raw)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/jobs/health", nil)
	AdminJobsHealth(AdminJobsDeps{Cfg: cfg, RedisQueue: rq})(c)
	if w.Code != http.StatusOK {
		t.Fatalf("code %d body %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["pending_count"].(float64) < 1 {
		t.Fatalf("pending_count = %v", resp["pending_count"])
	}
	if resp["failed_count"].(float64) < 1 {
		t.Fatalf("failed_count = %v", resp["failed_count"])
	}
	if resp["last_error"] != "handler failed" {
		t.Fatalf("last_error = %v", resp["last_error"])
	}
}

func TestAdminJobsFailed_RedactsPayload(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
		srv.Close()
	})
	cfg := &config.Config{}
	cfg.Queue.DeadLetterMaxCap = 100
	rq := queue.NewRedisQueue(client, cfg)
	job := queue.Job{
		ID:          "j1",
		Type:        "email.send_verification",
		Payload:     json.RawMessage(`{"to":"u@e.com","raw_token":"super-secret"}`),
		Status:      queue.StatusDeadLetter,
		Attempts:    2,
		MaxAttempts: 5,
		LastError:   "send failed",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	raw, _ := json.Marshal(job)
	_ = rq.PushDeadLetter(context.Background(), raw)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/jobs/failed", nil)
	AdminJobsFailed(AdminJobsDeps{Cfg: cfg, RedisQueue: rq})(c)
	if w.Code != http.StatusOK {
		t.Fatalf("code %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("data len %d", len(resp.Data))
	}
	payloadStr := string(mustJSON(t, resp.Data[0]["payload"]))
	if strings.Contains(payloadStr, "super-secret") {
		t.Fatalf("secret leaked: %s", payloadStr)
	}
}

func TestAdminJobsRetry_SuccessFromDLQ(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
		srv.Close()
	})
	cfg := &config.Config{}
	cfg.Queue.DeadLetterMaxCap = 100
	rq := queue.NewRedisQueue(client, cfg)
	job := queue.Job{
		ID:          "retry-me",
		Type:        "t",
		Payload:     json.RawMessage(`{}`),
		Status:      queue.StatusDeadLetter,
		Attempts:    2,
		MaxAttempts: 3,
		LastError:   "x",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	raw, _ := json.Marshal(job)
	_ = rq.PushDeadLetter(context.Background(), raw)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/jobs/retry-me/retry", nil)
	c.Params = gin.Params{{Key: "id", Value: "retry-me"}}
	AdminJobsRetry(AdminJobsDeps{Cfg: cfg, RedisQueue: rq})(c)
	if w.Code != http.StatusOK {
		t.Fatalf("code %d %s", w.Code, w.Body.String())
	}
	depth, _ := rq.DeadLetterDepth(context.Background())
	if depth != 0 {
		t.Fatalf("dlq depth = %d", depth)
	}
}

func TestAdminJobsRetry_InlineModeConflict(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/jobs/x/retry", nil)
	c.Params = gin.Params{{Key: "id", Value: "x"}}
	AdminJobsRetry(AdminJobsDeps{Cfg: cfg, RedisQueue: nil})(c)
	if w.Code != http.StatusConflict {
		t.Fatalf("code %d", w.Code)
	}
}

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
