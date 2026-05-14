package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goapi/internal/strategy/overnightmeanreversion"
)

func TestPOST_StrategyOvernightMeanReversionScore_StrongSetup(t *testing.T) {
	t.Parallel()
	router, _, _ := buildRouteSmokeRouter(t)

	body := map[string]interface{}{
		"symbol":                    "ETH-USD",
		"current_price":             3500,
		"daily_volume":              50_000_000,
		"percent_change_24h":        5,
		"intraday_drawdown_percent": 4,
		"rsi":                       38,
		"support_distance_percent":  1.0,
		"volume_trend":              "rising",
		"candle_signal":             "bullish_reversal",
		"higher_low_detected":       true,
		"planned_entry":             100,
		"stop_loss":                 98,
		"target_price":              106,
	}
	raw, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/strategy/overnight-mean-reversion/score", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var out overnightmeanreversion.Result
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if out.Label != overnightmeanreversion.LabelStrongSetup {
		t.Fatalf("label=%q want STRONG_SETUP", out.Label)
	}
	if out.FinalScore < 80 {
		t.Fatalf("final_score=%v want>=80", out.FinalScore)
	}
	if len(out.FailedFilters) != 0 {
		t.Fatalf("failed_filters=%v want empty", out.FailedFilters)
	}
	if out.CategoryScores.Liquidity <= 0 || out.CategoryScores.TrendQuality <= 0 {
		t.Fatalf("unexpected category scores: %+v", out.CategoryScores)
	}
}

func TestPOST_StrategyOvernightMeanReversionScore_LowVolumeAvoid(t *testing.T) {
	t.Parallel()
	router, _, _ := buildRouteSmokeRouter(t)

	body := map[string]interface{}{
		"symbol":                    "SHIB-USD",
		"current_price":             0.01,
		"daily_volume":              50_000,
		"percent_change_24h":        2,
		"intraday_drawdown_percent": 3,
		"rsi":                       40,
		"support_distance_percent":  2,
		"volume_trend":              "flat",
		"candle_signal":             "neutral",
		"higher_low_detected":       false,
		"planned_entry":             10,
		"stop_loss":                 9,
		"target_price":              13,
	}
	raw, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/strategy/overnight-mean-reversion/score", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var out overnightmeanreversion.Result
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Label != overnightmeanreversion.LabelAvoid {
		t.Fatalf("label=%q want AVOID", out.Label)
	}
	if !containsString(out.FailedFilters, overnightmeanreversion.FailedInsufficientLiquidity) {
		t.Fatalf("failed_filters=%v want %q", out.FailedFilters, overnightmeanreversion.FailedInsufficientLiquidity)
	}
}

func TestPOST_StrategyOvernightMeanReversionScore_InvalidRiskReward(t *testing.T) {
	t.Parallel()
	router, _, _ := buildRouteSmokeRouter(t)

	body := map[string]interface{}{
		"symbol":                    "ETH-USD",
		"current_price":             3000,
		"daily_volume":              30_000_000,
		"percent_change_24h":        4,
		"intraday_drawdown_percent": 4,
		"rsi":                       36,
		"support_distance_percent":  1.5,
		"volume_trend":              "rising",
		"candle_signal":             "bullish_reversal",
		"higher_low_detected":       true,
		"planned_entry":             100,
		"stop_loss":                 98,
		"target_price":              100,
	}
	raw, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/strategy/overnight-mean-reversion/score", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var out overnightmeanreversion.Result
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !containsString(out.FailedFilters, overnightmeanreversion.FailedInvalidRiskReward) {
		t.Fatalf("failed_filters=%v want %q", out.FailedFilters, overnightmeanreversion.FailedInvalidRiskReward)
	}
	if out.Label != overnightmeanreversion.LabelAvoid {
		t.Fatalf("label=%q want AVOID", out.Label)
	}
}

func TestPOST_StrategyOvernightMeanReversionScore_MalformedJSON(t *testing.T) {
	t.Parallel()
	router, _, _ := buildRouteSmokeRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/strategy/overnight-mean-reversion/score", bytes.NewReader([]byte(`{"symbol":`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"error"`) {
		t.Fatalf("expected error field: %s", w.Body.String())
	}
}

func TestPOST_StrategyOvernightMeanReversionScore_InvalidNumericRSI(t *testing.T) {
	t.Parallel()
	router, _, _ := buildRouteSmokeRouter(t)

	body := map[string]interface{}{
		"symbol":                    "ETH-USD",
		"current_price":             3000,
		"daily_volume":              30_000_000,
		"percent_change_24h":        4,
		"intraday_drawdown_percent": 4,
		"rsi":                       101,
		"support_distance_percent":  1.5,
		"volume_trend":              "rising",
		"candle_signal":             "bullish_reversal",
		"higher_low_detected":       true,
		"planned_entry":             100,
		"stop_loss":                 98,
		"target_price":              106,
	}
	raw, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/strategy/overnight-mean-reversion/score", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "rsi must be between 0 and 100") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestSetupRoutes_RegistersStrategyOMRScoreRoute(t *testing.T) {
	t.Parallel()
	router, _, _ := buildRouteSmokeRouter(t)
	if !hasRoute(router, http.MethodPost, "/api/v1/strategy/overnight-mean-reversion/score") {
		t.Fatal("missing POST /api/v1/strategy/overnight-mean-reversion/score")
	}
}

func containsString(slice []string, want string) bool {
	for _, s := range slice {
		if s == want {
			return true
		}
	}
	return false
}
