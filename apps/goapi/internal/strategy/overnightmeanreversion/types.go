// Package overnightmeanreversion provides deterministic pure functions for scoring
// overnight mean-reversion setups. It performs no I/O, persistence, or trading.
package overnightmeanreversion

// VolumeTrend describes recent volume direction for the symbol.
type VolumeTrend string

const (
	VolumeTrendRising  VolumeTrend = "rising"
	VolumeTrendFlat    VolumeTrend = "flat"
	VolumeTrendFalling VolumeTrend = "falling"
)

// CandleSignal summarizes the latest candle structure for the setup.
type CandleSignal string

const (
	CandleSignalBullishReversal     CandleSignal = "bullish_reversal"
	CandleSignalNeutral             CandleSignal = "neutral"
	CandleSignalBearishContinuation CandleSignal = "bearish_continuation"
)

// Label is the discrete recommendation tier derived from scores and failed filters.
type Label string

const (
	LabelAvoid         Label = "AVOID"
	LabelWatch         Label = "WATCH"
	LabelPossibleSetup Label = "POSSIBLE_SETUP"
	LabelStrongSetup   Label = "STRONG_SETUP"
)

// Input holds all fields required to score an OMR candidate in a single snapshot.
//
// Conventions (long-only overnight mean reversion):
//   - PercentChange24h: signed 24h return in percent (e.g. -2.5 means down 2.5%).
//   - IntradayDrawdownPercent: non-negative magnitude, percent off the session high
//     (e.g. 4 means price is 4% below the intraday high).
//   - SupportDistancePercent: non-negative distance to the nearest support below
//     CurrentPrice (smaller means closer support).
//   - PlannedEntry, StopLoss, TargetPrice: for a valid long RR geometry callers
//     should use StopLoss < PlannedEntry < TargetPrice; invalid geometry is handled
//     without panics and surfaces as failed filters / low scores.
type Input struct {
	Symbol                  string
	CurrentPrice            float64
	DailyVolume             float64
	PercentChange24h        float64
	IntradayDrawdownPercent float64
	RSI                     float64
	SupportDistancePercent  float64
	VolumeTrend             VolumeTrend
	CandleSignal            CandleSignal
	HigherLowDetected       bool
	PlannedEntry            float64
	StopLoss                float64
	TargetPrice             float64
}

// CategoryScores holds the six component scores, each in [0, 100].
type CategoryScores struct {
	Liquidity       float64 `json:"liquidity"`
	TrendQuality    float64 `json:"trend_quality"`
	PullbackQuality float64 `json:"pullback_quality"`
	Exhaustion      float64 `json:"exhaustion"`
	RiskSetup       float64 `json:"risk_setup"`
	FinalReversal   float64 `json:"final_reversal"`
}

// Result is the structured output of Score.
//
// Reasons, Warnings, and FailedFilters use stable machine-oriented codes
// (see score.go and categories.go constants) for tests and downstream logic.
type Result struct {
	FinalScore     float64        `json:"final_score"`
	Label          Label          `json:"label"`
	CategoryScores CategoryScores `json:"category_scores"`
	Reasons        []string       `json:"reasons"`
	Warnings       []string       `json:"warnings"`
	FailedFilters  []string       `json:"failed_filters"`
}
