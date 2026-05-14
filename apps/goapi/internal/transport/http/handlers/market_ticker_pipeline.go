package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"goapi/config"
	"goapi/internal/marketdata/state"
)

// MarketTickerPipelineStatus returns configured Coinbase public WS settings and in-memory ticker cache stats.
// It exposes no secrets (public WSS URL and product IDs only).
func MarketTickerPipelineStatus(store *state.Store, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg == nil {
			cfg = &config.Config{}
		}
		var count int
		var latest time.Time
		if store != nil {
			snap := store.Snapshot()
			count = len(snap)
			for _, ev := range snap {
				if ev.ReceivedAt.After(latest) {
					latest = ev.ReceivedAt
				}
			}
		}
		body := gin.H{
			"coinbase_ws_enabled":  cfg.CoinbaseExchangeWS.Enabled,
			"coinbase_ws_url":      cfg.CoinbaseExchangeWS.URL,
			"coinbase_ws_products": cfg.CoinbaseExchangeWS.Products,
			"cached_ticker_count":  count,
		}
		if !latest.IsZero() {
			body["latest_received_at"] = latest.UTC().Format(time.RFC3339Nano)
		} else {
			body["latest_received_at"] = nil
		}
		c.JSON(http.StatusOK, body)
	}
}
