//go:build integration

package coinbase

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestIntegrationTickerAdapterReceivesTicker dials the production public feed and waits for at least one ticker.
// Run with: INTEGRATION_COINBASE_WS=1 go test -tags=integration -count=1 ./internal/marketdata/coinbase/...
func TestIntegrationTickerAdapterReceivesTicker(t *testing.T) {
	if os.Getenv("INTEGRATION_COINBASE_WS") != "1" {
		t.Skip("set INTEGRATION_COINBASE_WS=1 to run live Coinbase WebSocket test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	received := make(chan MarketTickerEvent, 1)
	cfg := DefaultTickerAdapterConfig("wss://ws-feed.exchange.coinbase.com", []string{"BTC-USD"})
	ad := NewTickerAdapter(cfg, func(ev MarketTickerEvent) {
		select {
		case received <- ev:
		default:
		}
	})
	ad.SetLogger(t.Logf)

	go ad.Run(ctx)

	select {
	case ev := <-received:
		if ev.ProductID == "" || ev.Price <= 0 {
			t.Fatalf("unexpected event: %+v", ev)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for ticker from Coinbase public feed")
	}
	cancel()
}
