package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goapi/internal/marketdata/coinbase"
	"goapi/internal/strategy/overnightmeanreversion"
)

func validSeededTicker() coinbase.MarketTickerEvent {
	return coinbase.MarketTickerEvent{
		Source:     coinbase.SourcePublicWS,
		Type:       "ticker",
		ProductID:  "BTC-USD",
		Price:      100,
		Open24h:    80,
		Volume24h:  50_000_000,
		Low24h:     70,
		High24h:    120,
		BestBid:    99.5,
		BestAsk:    100.5,
		Side:       "buy",
		Time:       "2026-01-01T00:00:00Z",
		ReceivedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func validOMRBody() map[string]any {
	return map[string]any{
		"rsi":                      38.0,
		"support_distance_percent": 1.5,
		"volume_trend":             "rising",
		"candle_signal":            "bullish_reversal",
		"higher_low_detected":      true,
		"planned_entry":            100.0,
		"stop_loss":                97.0,
		"target_price":             109.0,
	}
}

func TestPOST_market_ticker_OMR_score_success(t *testing.T) {
	t.Parallel()
	router, _, c := buildRouteSmokeRouter(t)
	c.TickerStore.UpsertTicker(validSeededTicker())

	body, _ := json.Marshal(validOMRBody())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/market/tickers/BTC-USD/overnight-mean-reversion/score", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var out overnightmeanreversion.Result
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Label == "" {
		t.Fatal("expected label")
	}
}

func TestPOST_market_ticker_OMR_score_missingTicker(t *testing.T) {
	t.Parallel()
	router, _, _ := buildRouteSmokeRouter(t)

	body, _ := json.Marshal(validOMRBody())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/market/tickers/MISSING-USD/overnight-mean-reversion/score", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestPOST_market_ticker_OMR_score_invalidRSI(t *testing.T) {
	t.Parallel()
	router, _, c := buildRouteSmokeRouter(t)
	c.TickerStore.UpsertTicker(validSeededTicker())

	b := validOMRBody()
	b["rsi"] = 150.0
	body, _ := json.Marshal(b)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/market/tickers/BTC-USD/overnight-mean-reversion/score", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestPOST_market_ticker_OMR_score_invalidPlannedTrade(t *testing.T) {
	t.Parallel()
	router, _, c := buildRouteSmokeRouter(t)
	c.TickerStore.UpsertTicker(validSeededTicker())

	b := validOMRBody()
	b["stop_loss"] = 100.0
	b["planned_entry"] = 99.0
	b["target_price"] = 105.0
	body, _ := json.Marshal(b)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/market/tickers/BTC-USD/overnight-mean-reversion/score", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestPOST_market_ticker_OMR_score_badTickerMetrics(t *testing.T) {
	t.Parallel()
	router, _, c := buildRouteSmokeRouter(t)
	bad := validSeededTicker()
	bad.BestBid = 101
	bad.BestAsk = 99
	c.TickerStore.UpsertTicker(bad)

	body, _ := json.Marshal(validOMRBody())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/market/tickers/BTC-USD/overnight-mean-reversion/score", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
