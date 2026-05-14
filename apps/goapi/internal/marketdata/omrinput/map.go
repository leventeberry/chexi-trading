// Package omrinput maps market metrics plus manual analysis fields into overnight mean reversion scoring input.
// It performs no I/O, HTTP, persistence, WebSocket, or scoring.
package omrinput

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"goapi/internal/marketdata/metrics"
	"goapi/internal/strategy/overnightmeanreversion"
)

// ManualFields holds analyst-supplied inputs not derivable from ticker metrics alone.
type ManualFields struct {
	RSI                    float64
	SupportDistancePercent float64
	VolumeTrend            overnightmeanreversion.VolumeTrend
	CandleSignal           overnightmeanreversion.CandleSignal
	HigherLowDetected      bool
	PlannedEntry           float64
	StopLoss               float64
	TargetPrice            float64
}

// Sentinel errors for invalid manual inputs or unusable metrics slices.
var (
	ErrInvalidMetricsProductID = errors.New("omrinput: metrics product_id is required")
	ErrInvalidMetricsPrice     = errors.New("omrinput: metrics current_price must be finite and positive")
	ErrInvalidMetricsVolume    = errors.New("omrinput: metrics volume_24h must be finite and non-negative")
	ErrInvalidMetricsPercent   = errors.New("omrinput: metrics percent_change_24h must be finite")
	ErrInvalidMetricsDrawdown  = errors.New("omrinput: metrics intraday_drawdown_percent must be finite and non-negative")
	ErrInvalidRSI              = errors.New("omrinput: rsi must be between 0 and 100")
	ErrInvalidSupportDistance  = errors.New("omrinput: support_distance_percent must be finite and non-negative")
	ErrInvalidVolumeTrend      = errors.New("omrinput: volume_trend must be rising, flat, or falling")
	ErrInvalidCandleSignal     = errors.New("omrinput: candle_signal must be bullish_reversal, neutral, or bearish_continuation")
	ErrInvalidPlannedTrade     = errors.New("omrinput: require stop_loss < planned_entry < target_price with all prices finite and positive")
)

// InputFromMetrics builds an overnightmeanreversion.Input from derived metrics and manual fields.
func InputFromMetrics(m metrics.Metrics, manual ManualFields) (overnightmeanreversion.Input, error) {
	if err := validateMetrics(m); err != nil {
		return overnightmeanreversion.Input{}, err
	}
	if err := validateManual(manual); err != nil {
		return overnightmeanreversion.Input{}, err
	}

	return overnightmeanreversion.Input{
		Symbol:                  strings.TrimSpace(m.ProductID),
		CurrentPrice:            m.CurrentPrice,
		DailyVolume:             m.Volume24h,
		PercentChange24h:        m.PercentChange24h,
		IntradayDrawdownPercent: m.IntradayDrawdownPercent,
		RSI:                     manual.RSI,
		SupportDistancePercent:  manual.SupportDistancePercent,
		VolumeTrend:             manual.VolumeTrend,
		CandleSignal:            manual.CandleSignal,
		HigherLowDetected:       manual.HigherLowDetected,
		PlannedEntry:            manual.PlannedEntry,
		StopLoss:                manual.StopLoss,
		TargetPrice:             manual.TargetPrice,
	}, nil
}

func validateMetrics(m metrics.Metrics) error {
	if strings.TrimSpace(m.ProductID) == "" {
		return ErrInvalidMetricsProductID
	}
	if !isFinite(m.CurrentPrice) || m.CurrentPrice <= 0 {
		return fmt.Errorf("%w", ErrInvalidMetricsPrice)
	}
	if !isFinite(m.Volume24h) || m.Volume24h < 0 {
		return fmt.Errorf("%w", ErrInvalidMetricsVolume)
	}
	if !isFinite(m.PercentChange24h) {
		return fmt.Errorf("%w", ErrInvalidMetricsPercent)
	}
	if !isFinite(m.IntradayDrawdownPercent) || m.IntradayDrawdownPercent < 0 {
		return fmt.Errorf("%w", ErrInvalidMetricsDrawdown)
	}
	return nil
}

func validateManual(m ManualFields) error {
	if !isFinite(m.RSI) || m.RSI < 0 || m.RSI > 100 {
		return fmt.Errorf("%w", ErrInvalidRSI)
	}
	if !isFinite(m.SupportDistancePercent) || m.SupportDistancePercent < 0 {
		return fmt.Errorf("%w", ErrInvalidSupportDistance)
	}
	switch m.VolumeTrend {
	case overnightmeanreversion.VolumeTrendRising,
		overnightmeanreversion.VolumeTrendFlat,
		overnightmeanreversion.VolumeTrendFalling:
	default:
		return fmt.Errorf("%w", ErrInvalidVolumeTrend)
	}
	switch m.CandleSignal {
	case overnightmeanreversion.CandleSignalBullishReversal,
		overnightmeanreversion.CandleSignalNeutral,
		overnightmeanreversion.CandleSignalBearishContinuation:
	default:
		return fmt.Errorf("%w", ErrInvalidCandleSignal)
	}
	if !isFinite(m.PlannedEntry) || m.PlannedEntry <= 0 ||
		!isFinite(m.StopLoss) || m.StopLoss <= 0 ||
		!isFinite(m.TargetPrice) || m.TargetPrice <= 0 {
		return fmt.Errorf("%w", ErrInvalidPlannedTrade)
	}
	if !(m.StopLoss < m.PlannedEntry && m.PlannedEntry < m.TargetPrice) {
		return fmt.Errorf("%w", ErrInvalidPlannedTrade)
	}
	return nil
}

func isFinite(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0)
}
