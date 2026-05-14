package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"goapi/internal/marketdata/coinbase"
	"goapi/internal/marketdata/state"
)

// ListMarketTickers returns the latest normalized ticker per product (sorted by product_id).
func ListMarketTickers(store *state.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if store == nil {
			c.JSON(http.StatusOK, []coinbase.MarketTickerEvent{})
			return
		}
		out := store.ListTickers()
		c.JSON(http.StatusOK, out)
	}
}

// GetMarketTicker returns the latest ticker for a single product_id.
func GetMarketTicker(store *state.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if store == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticker not found"})
			return
		}
		pid := c.Param("productID")
		ev, ok := store.GetTicker(pid)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticker not found"})
			return
		}
		c.JSON(http.StatusOK, ev)
	}
}
