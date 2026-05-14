// Package metrics maps normalized Coinbase ticker snapshots into derived market facts.
// It performs no I/O, persistence, WebSocket, HTTP, or strategy scoring.
package metrics

import (
	"fmt"
	"math"

	"goapi/internal/marketdata/coinbase"
)

// Metrics holds derived values from a single MarketTickerEvent.
type Metrics struct {
	ProductID               string  `json:"product_id"`
	CurrentPrice            float64 `json:"current_price"`
	Open24h                 float64 `json:"open_24h"`
	High24h                 float64 `json:"high_24h"`
	Low24h                  float64 `json:"low_24h"`
	Volume24h               float64 `json:"volume_24h"`
	SpreadAmount            float64 `json:"spread_amount"`
	SpreadPercent           float64 `json:"spread_percent"`
	PercentChange24h        float64 `json:"percent_change_24h"`
	IntradayDrawdownPercent float64 `json:"intraday_drawdown_percent"`
	RangePositionPercent    float64 `json:"range_position_percent"`
}

// FromTicker computes metrics from a normalized ticker event.
//
// Definitions:
//   - spreadAmount = bestAsk - bestBid
//   - spreadPercent = spreadAmount / currentPrice * 100
//   - percentChange24h = (currentPrice - open24h) / open24h * 100
//   - intradayDrawdownPercent = (high24h - currentPrice) / high24h * 100
//   - rangePositionPercent = (currentPrice - low24h) / (high24h - low24h) * 100
func FromTicker(in coinbase.MarketTickerEvent) (Metrics, error) {
	if err := validateTicker(in); err != nil {
		return Metrics{}, err
	}
	price := in.Price
	spread := in.BestAsk - in.BestBid
	pctFromOpen := (price - in.Open24h) / in.Open24h * 100
	drawdown := (in.High24h - price) / in.High24h * 100
	rangeSpan := in.High24h - in.Low24h
	rangePos := (price - in.Low24h) / rangeSpan * 100

	return Metrics{
		ProductID:               in.ProductID,
		CurrentPrice:            price,
		Open24h:                 in.Open24h,
		High24h:                 in.High24h,
		Low24h:                  in.Low24h,
		Volume24h:               in.Volume24h,
		SpreadAmount:            spread,
		SpreadPercent:           spread / price * 100,
		PercentChange24h:        pctFromOpen,
		IntradayDrawdownPercent: drawdown,
		RangePositionPercent:    rangePos,
	}, nil
}

func validateTicker(in coinbase.MarketTickerEvent) error {
	if in.ProductID == "" {
		return ErrEmptyProductID
	}
	if !isFinite(in.Price) || in.Price <= 0 {
		return fmt.Errorf("%w", ErrNonPositiveCurrentPrice)
	}
	if !isFinite(in.BestBid) || !isFinite(in.BestAsk) {
		return fmt.Errorf("%w", ErrInvalidSpread)
	}
	if in.BestAsk < in.BestBid {
		return fmt.Errorf("%w", ErrInvalidSpread)
	}
	if in.Open24h == 0 {
		return fmt.Errorf("%w", ErrOpen24hZero)
	}
	if !isFinite(in.Open24h) {
		return fmt.Errorf("%w", ErrInvalidOpen24h)
	}
	if in.High24h <= 0 || !isFinite(in.High24h) {
		return fmt.Errorf("%w", ErrHigh24hNonPositive)
	}
	if !isFinite(in.Low24h) {
		return fmt.Errorf("%w", ErrDegenerateHighLowRange)
	}
	if in.High24h == in.Low24h {
		return fmt.Errorf("%w", ErrDegenerateHighLowRange)
	}
	if !isFinite(in.Volume24h) || in.Volume24h < 0 {
		return fmt.Errorf("%w", ErrInvalidVolume24h)
	}
	return nil
}

func isFinite(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0)
}
