// Package coinbase implements a public Coinbase Exchange WebSocket market-data adapter.
// It does not use private API credentials, does not place orders, and does not persist data.
package coinbase

import "time"

// SourcePublicWS identifies normalized events from the public WebSocket feed.
const SourcePublicWS = "coinbase_exchange_ws_public"

// MarketTickerEvent is a normalized ticker snapshot derived from Coinbase ticker channel messages.
type MarketTickerEvent struct {
	Source     string    `json:"source"`
	Type       string    `json:"type"`
	ProductID  string    `json:"product_id"`
	Price      float64   `json:"price"`
	Open24h    float64   `json:"open_24h"`
	Volume24h  float64   `json:"volume_24h"`
	Low24h     float64   `json:"low_24h"`
	High24h    float64   `json:"high_24h"`
	BestBid    float64   `json:"best_bid"`
	BestAsk    float64   `json:"best_ask"`
	Side       string    `json:"side"`
	Time       string    `json:"time"`
	ReceivedAt time.Time `json:"received_at"`
}
