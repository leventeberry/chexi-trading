package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goapi/internal/marketdata/coinbase"
)

func TestGET_market_tickers_pipeline_status(t *testing.T) {
	t.Parallel()
	router, _, c := buildRouteSmokeRouter(t)
	c.TickerStore.UpsertTicker(coinbase.MarketTickerEvent{
		Source:     coinbase.SourcePublicWS,
		Type:       "ticker",
		ProductID:  "BTC-USD",
		Price:      1,
		Open24h:    1,
		Volume24h:  1,
		Low24h:     1,
		High24h:    1,
		BestBid:    1,
		BestAsk:    1,
		Side:       "buy",
		Time:       "t",
		ReceivedAt: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/market/tickers/status", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["cached_ticker_count"].(float64) != 1 {
		t.Fatalf("cached_ticker_count: %v", body["cached_ticker_count"])
	}
	if body["latest_received_at"] == nil {
		t.Fatalf("want latest_received_at, got %#v", body["latest_received_at"])
	}
}

func TestGET_market_tickers_status_route_not_shadowed_by_productID(t *testing.T) {
	t.Parallel()
	router, _, _ := buildRouteSmokeRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/market/tickers/status", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status route shadowed or missing: status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Smoke router uses zero config: WS disabled, empty URL.
	if got, want := body["coinbase_ws_enabled"].(bool), false; got != want {
		t.Fatalf("coinbase_ws_enabled=%v want %v", got, want)
	}
}

func TestGET_market_tickers_empty(t *testing.T) {
	t.Parallel()
	router, _, c := buildRouteSmokeRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/market/tickers", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var arr []coinbase.MarketTickerEvent
	if err := json.Unmarshal(w.Body.Bytes(), &arr); err != nil || len(arr) != 0 {
		t.Fatalf("want empty json array, got %q err=%v", w.Body.String(), err)
	}
	_ = c
}

func TestGET_market_tickers_seeded(t *testing.T) {
	t.Parallel()
	router, _, c := buildRouteSmokeRouter(t)
	c.TickerStore.UpsertTicker(coinbase.MarketTickerEvent{
		Source:     coinbase.SourcePublicWS,
		Type:       "ticker",
		ProductID:  "BTC-USD",
		Price:      1,
		Open24h:    1,
		Volume24h:  1,
		Low24h:     1,
		High24h:    1,
		BestBid:    1,
		BestAsk:    1,
		Side:       "buy",
		Time:       "t",
		ReceivedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/market/tickers", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var out []coinbase.MarketTickerEvent
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out) != 1 || out[0].ProductID != "BTC-USD" {
		t.Fatalf("unexpected %v", out)
	}
}

func TestGET_market_ticker_by_product_success(t *testing.T) {
	t.Parallel()
	router, _, c := buildRouteSmokeRouter(t)
	c.TickerStore.UpsertTicker(coinbase.MarketTickerEvent{
		Source:     coinbase.SourcePublicWS,
		Type:       "ticker",
		ProductID:  "ETH-USD",
		Price:      42,
		Open24h:    1,
		Volume24h:  1,
		Low24h:     1,
		High24h:    1,
		BestBid:    1,
		BestAsk:    1,
		Side:       "sell",
		Time:       "t",
		ReceivedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/market/tickers/ETH-USD", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var ev coinbase.MarketTickerEvent
	if err := json.Unmarshal(w.Body.Bytes(), &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.ProductID != "ETH-USD" || ev.Price != 42 {
		t.Fatalf("unexpected %+v", ev)
	}
}

func TestGET_market_ticker_by_product_notFound(t *testing.T) {
	t.Parallel()
	router, _, _ := buildRouteSmokeRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/market/tickers/NOPE-USD", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
