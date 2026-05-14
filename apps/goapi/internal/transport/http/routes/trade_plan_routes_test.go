package routes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"goapi/container"
	"goapi/internal/infra/auth"
	"goapi/models"
)

func seedTradePlanTestUser(t *testing.T, c *container.Container, userID uuid.UUID) {
	t.Helper()
	if err := c.DB.AutoMigrate(&models.User{}, &models.TradePlan{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now().UTC()
	email := fmt.Sprintf("trade-plan-%s@example.com", userID.String()[:8])
	u := models.User{
		ID:        userID,
		FirstName: "Trade",
		LastName:  "Tester",
		Email:     email,
		PassHash:  "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		Role:      "user",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := c.DB.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func tradePlanAuthHeader(t *testing.T, jwt *auth.Manager, userID uuid.UUID) string {
	t.Helper()
	tok, err := jwt.CreateToken(userID, "user")
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return "Bearer " + tok.JWTToken
}

func TestPOST_trade_plans_valid(t *testing.T) {
	t.Parallel()
	router, jwt, c := buildRouteSmokeRouter(t)
	uid := uuid.New()
	seedTradePlanTestUser(t, c, uid)
	h := tradePlanAuthHeader(t, jwt, uid)

	body := map[string]any{
		"symbol":          "BTC-USD",
		"strategy_name":   "overnight_mean_reversion",
		"direction":       "LONG",
		"thesis":          "Pullback into support after washout.",
		"planned_entry":   100.0,
		"stop_loss":       97.0,
		"target_price":    109.0,
		"position_size":   2.0,
		"max_risk_amount": 50.0,
		"source_score":    71.2,
		"source_label":    "WATCH",
		"notes":           "Paper only.",
	}
	raw, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trade-plans", bytes.NewReader(raw))
	req.Header.Set("Authorization", h)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["status"] != "PLANNED" {
		t.Fatalf("status=%v", out["status"])
	}
	if out["risk_per_unit"].(float64) != 3 {
		t.Fatalf("risk_per_unit=%v", out["risk_per_unit"])
	}
	if out["reward_per_unit"].(float64) != 9 {
		t.Fatalf("reward_per_unit=%v", out["reward_per_unit"])
	}
	if out["max_loss"].(float64) != 6 {
		t.Fatalf("max_loss=%v", out["max_loss"])
	}
	if out["risk_reward_ratio"].(float64) != 3 {
		t.Fatalf("rr=%v", out["risk_reward_ratio"])
	}
}

func TestPOST_trade_plans_invalidSymbol(t *testing.T) {
	t.Parallel()
	router, jwt, c := buildRouteSmokeRouter(t)
	uid := uuid.New()
	seedTradePlanTestUser(t, c, uid)
	h := tradePlanAuthHeader(t, jwt, uid)
	body := map[string]any{
		"symbol": "@@@", "strategy_name": "s", "direction": "LONG", "thesis": "x",
		"planned_entry": 100.0, "stop_loss": 97.0, "target_price": 109.0,
		"position_size": 1.0, "max_risk_amount": 1.0,
	}
	raw, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trade-plans", bytes.NewReader(raw))
	req.Header.Set("Authorization", h)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestPOST_trade_plans_invalidDirection(t *testing.T) {
	t.Parallel()
	router, jwt, c := buildRouteSmokeRouter(t)
	uid := uuid.New()
	seedTradePlanTestUser(t, c, uid)
	h := tradePlanAuthHeader(t, jwt, uid)
	body := map[string]any{
		"symbol": "ETH-USD", "strategy_name": "s", "direction": "SIDEWAYS", "thesis": "x",
		"planned_entry": 100.0, "stop_loss": 97.0, "target_price": 109.0,
		"position_size": 1.0, "max_risk_amount": 1.0,
	}
	raw, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trade-plans", bytes.NewReader(raw))
	req.Header.Set("Authorization", h)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestPOST_trade_plans_invalidGeometry(t *testing.T) {
	t.Parallel()
	router, jwt, c := buildRouteSmokeRouter(t)
	uid := uuid.New()
	seedTradePlanTestUser(t, c, uid)
	h := tradePlanAuthHeader(t, jwt, uid)
	body := map[string]any{
		"symbol": "BTC-USD", "strategy_name": "s", "direction": "LONG", "thesis": "x",
		"planned_entry": 100.0, "stop_loss": 101.0, "target_price": 109.0,
		"position_size": 1.0, "max_risk_amount": 1.0,
	}
	raw, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trade-plans", bytes.NewReader(raw))
	req.Header.Set("Authorization", h)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestGET_trade_plans_listAndGet(t *testing.T) {
	t.Parallel()
	router, jwt, c := buildRouteSmokeRouter(t)
	uid := uuid.New()
	seedTradePlanTestUser(t, c, uid)
	h := tradePlanAuthHeader(t, jwt, uid)

	body := map[string]any{
		"symbol": "SOL-USD", "strategy_name": "omr", "direction": "LONG", "thesis": "setup",
		"planned_entry": 50.0, "stop_loss": 48.0, "target_price": 55.0,
		"position_size": 1.0, "max_risk_amount": 5.0,
	}
	raw, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trade-plans", bytes.NewReader(raw))
	req.Header.Set("Authorization", h)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id := created["id"].(string)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/trade-plans", nil)
	req.Header.Set("Authorization", h)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("len=%d", len(list))
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/trade-plans/"+id, nil)
	req.Header.Set("Authorization", h)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get: %d %s", w.Code, w.Body.String())
	}
}

func TestGET_trade_plans_missing404(t *testing.T) {
	t.Parallel()
	router, jwt, c := buildRouteSmokeRouter(t)
	uid := uuid.New()
	seedTradePlanTestUser(t, c, uid)
	h := tradePlanAuthHeader(t, jwt, uid)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trade-plans/"+uuid.New().String(), nil)
	req.Header.Set("Authorization", h)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
