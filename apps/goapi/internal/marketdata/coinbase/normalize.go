package coinbase

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// coinbaseTickerWire matches Coinbase Exchange WebSocket ticker channel JSON.
type coinbaseTickerWire struct {
	Type      string `json:"type"`
	ProductID string `json:"product_id"`
	Price     string `json:"price"`
	Open24h   string `json:"open_24h"`
	Volume24h string `json:"volume_24h"`
	Low24h    string `json:"low_24h"`
	High24h   string `json:"high_24h"`
	BestBid   string `json:"best_bid"`
	BestAsk   string `json:"best_ask"`
	Side      string `json:"side"`
	Time      string `json:"time"`
}

// NormalizeTickerJSON parses a single JSON object as a Coinbase ticker message and returns a MarketTickerEvent.
func NormalizeTickerJSON(raw []byte, receivedAt time.Time) (MarketTickerEvent, error) {
	var w coinbaseTickerWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return MarketTickerEvent{}, fmt.Errorf("ticker json: %w", err)
	}
	if strings.TrimSpace(w.Type) != "ticker" {
		return MarketTickerEvent{}, fmt.Errorf("ticker json: expected type ticker, got %q", w.Type)
	}
	if strings.TrimSpace(w.ProductID) == "" {
		return MarketTickerEvent{}, fmt.Errorf("ticker json: product_id is required")
	}

	price, err := parsePositiveOrZeroFloat("price", w.Price)
	if err != nil {
		return MarketTickerEvent{}, err
	}
	open24, err := parsePositiveOrZeroFloat("open_24h", w.Open24h)
	if err != nil {
		return MarketTickerEvent{}, err
	}
	vol24, err := parsePositiveOrZeroFloat("volume_24h", w.Volume24h)
	if err != nil {
		return MarketTickerEvent{}, err
	}
	low24, err := parsePositiveOrZeroFloat("low_24h", w.Low24h)
	if err != nil {
		return MarketTickerEvent{}, err
	}
	high24, err := parsePositiveOrZeroFloat("high_24h", w.High24h)
	if err != nil {
		return MarketTickerEvent{}, err
	}
	bid, err := parsePositiveOrZeroFloat("best_bid", w.BestBid)
	if err != nil {
		return MarketTickerEvent{}, err
	}
	ask, err := parsePositiveOrZeroFloat("best_ask", w.BestAsk)
	if err != nil {
		return MarketTickerEvent{}, err
	}

	return MarketTickerEvent{
		Source:     SourcePublicWS,
		Type:       "ticker",
		ProductID:  strings.TrimSpace(w.ProductID),
		Price:      price,
		Open24h:    open24,
		Volume24h:  vol24,
		Low24h:     low24,
		High24h:    high24,
		BestBid:    bid,
		BestAsk:    ask,
		Side:       strings.TrimSpace(w.Side),
		Time:       strings.TrimSpace(w.Time),
		ReceivedAt: receivedAt.UTC(),
	}, nil
}

func parsePositiveOrZeroFloat(field, s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("ticker json: %s is required", field)
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("ticker json: invalid %s: %w", field, err)
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("ticker json: %s must be finite", field)
	}
	if v < 0 {
		return 0, fmt.Errorf("ticker json: %s must be non-negative", field)
	}
	return v, nil
}
