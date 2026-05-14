package overnightmeanreversion

import "math"

// Stable codes for Reasons, Warnings, and FailedFilters.
const (
	ReasonLiquidityStrong       = "liquidity_strong"
	ReasonTrendStrong           = "trend_strong_24h"
	ReasonPullbackControlled    = "pullback_controlled"
	ReasonVolumeTrendRising     = "volume_trend_rising"
	ReasonHigherLow             = "higher_low_detected"
	ReasonBullishReversalCandle = "bullish_reversal_candle"
	ReasonRiskRewardFavorable   = "risk_reward_favorable"
	ReasonSupportNearby         = "support_nearby"
	ReasonExhaustionMild        = "exhaustion_mild"

	WarningOversoldFarFromSupport = "oversold_far_from_support"
	WarningDeepPullback           = "deep_pullback_vs_trend"
	WarningVolumeFalling          = "volume_trend_falling"

	FailedInsufficientLiquidity = "insufficient_liquidity"
	FailedCatastrophicSelloff   = "catastrophic_selloff"
	FailedInvalidRiskReward     = "invalid_risk_reward"
)

const (
	minDailyVolumeQuote     = 1_000_000.0
	catastrophic24hHard     = -28.0
	catastrophic24hSoft     = -18.0
	catastrophicVolCeiling  = 1_500_000.0
	catastrophicDrawdownMin = 14.0

	minRewardRiskRatio = 1.0

	rsiConstructiveMin = 28.0
	rsiConstructiveMax = 48.0
	rsiOversold        = 32.0
	supportFarPct      = 4.0

	pullbackIdealMin = 2.0
	pullbackIdealMax = 9.0
)

func clamp(x, lo, hi float64) float64 {
	return math.Min(hi, math.Max(lo, x))
}

func longRiskReward(in Input) (rewardRisk float64, valid bool) {
	if in.PlannedEntry <= in.StopLoss || in.TargetPrice <= in.PlannedEntry {
		return 0, false
	}
	risk := in.PlannedEntry - in.StopLoss
	reward := in.TargetPrice - in.PlannedEntry
	if risk <= 0 {
		return 0, false
	}
	return reward / risk, true
}

func liquidityScore(in Input) (float64, []string, []string) {
	var reasons, warnings []string
	v := in.DailyVolume
	if v < minDailyVolumeQuote {
		return 0, reasons, warnings
	}
	// Log-scaled score from min threshold to ~100M quote volume.
	const refHigh = 80_000_000.0
	t := (math.Log10(math.Max(v, minDailyVolumeQuote)) - math.Log10(minDailyVolumeQuote)) /
		math.Max(1e-9, math.Log10(refHigh)-math.Log10(minDailyVolumeQuote))
	score := clamp(t*100, 0, 100)
	if score >= 70 {
		reasons = append(reasons, ReasonLiquidityStrong)
	}
	return score, reasons, warnings
}

func trendQualityScore(in Input) (float64, []string, []string) {
	var reasons, warnings []string
	p := in.PercentChange24h
	// Strong positive 24h is best; mild positive good; mild negative acceptable;
	// deep negative scores low (catastrophic handled separately via failed filter).
	var score float64
	switch {
	case p >= 8:
		score = 100
	case p >= 4:
		score = 85
	case p >= 1:
		score = 72
	case p >= -1:
		score = 58
	case p >= -4:
		score = 42
	case p >= -8:
		score = 28
	default:
		score = 12
	}
	if p >= 3 {
		reasons = append(reasons, ReasonTrendStrong)
	}
	return score, reasons, warnings
}

func pullbackQualityScore(in Input) (float64, []string, []string) {
	var reasons, warnings []string
	d := in.IntradayDrawdownPercent
	p24 := in.PercentChange24h

	var score float64
	switch {
	case d < pullbackIdealMin:
		score = 35 // shallow dip — weak mean-reversion context
	case d <= pullbackIdealMax:
		score = 90
		reasons = append(reasons, ReasonPullbackControlled)
	case d <= 14:
		score = 65
		reasons = append(reasons, ReasonPullbackControlled)
		warnings = append(warnings, WarningDeepPullback)
	default:
		score = 30
		warnings = append(warnings, WarningDeepPullback)
	}

	// Penalize large intraday drawdown when the coin is already very weak on the day.
	if p24 < -6 && d > 10 {
		score = math.Min(score, 25)
	}
	return score, reasons, warnings
}

func exhaustionScore(in Input) (float64, []string, []string) {
	var reasons, warnings []string
	r := in.RSI
	sup := in.SupportDistancePercent

	var score float64
	switch {
	case r >= rsiConstructiveMin && r <= rsiConstructiveMax:
		score = 85
		reasons = append(reasons, ReasonExhaustionMild)
	case r < rsiConstructiveMin:
		score = 45
		if r < rsiOversold && sup > supportFarPct {
			warnings = append(warnings, WarningOversoldFarFromSupport)
		}
	case r <= 65:
		score = 70
	default:
		score = 50
	}
	return score, reasons, warnings
}

func riskSetupScore(in Input) (float64, []string, []string) {
	var reasons, warnings []string
	rr, ok := longRiskReward(in)
	if !ok || rr < minRewardRiskRatio {
		return 0, reasons, warnings
	}
	score := clamp((rr-1)*35+55, 0, 100)
	if rr >= 1.8 {
		reasons = append(reasons, ReasonRiskRewardFavorable)
	}
	return score, reasons, warnings
}

func finalReversalScore(in Input) (float64, []string, []string) {
	var reasons, warnings []string
	score := 45.0

	switch in.VolumeTrend {
	case VolumeTrendRising:
		score += 18
		reasons = append(reasons, ReasonVolumeTrendRising)
	case VolumeTrendFlat:
		score += 6
	case VolumeTrendFalling:
		score -= 12
		warnings = append(warnings, WarningVolumeFalling)
	}

	switch in.CandleSignal {
	case CandleSignalBullishReversal:
		score += 22
		reasons = append(reasons, ReasonBullishReversalCandle)
	case CandleSignalNeutral:
		score += 4
	case CandleSignalBearishContinuation:
		score -= 25
	}

	if in.HigherLowDetected {
		score += 15
		reasons = append(reasons, ReasonHigherLow)
	}

	if in.SupportDistancePercent <= 1.5 {
		score += 12
		reasons = append(reasons, ReasonSupportNearby)
	} else if in.SupportDistancePercent <= 3 {
		score += 6
	}

	// RSI constructive band helps reversal quality without double-counting exhaustion too harshly.
	if in.RSI >= rsiConstructiveMin && in.RSI <= 45 {
		score += 6
	}

	return clamp(score, 0, 100), reasons, warnings
}

func catastrophicSelloff(in Input) bool {
	if in.PercentChange24h <= catastrophic24hHard {
		return true
	}
	if in.PercentChange24h <= catastrophic24hSoft &&
		in.DailyVolume < catastrophicVolCeiling &&
		in.IntradayDrawdownPercent >= catastrophicDrawdownMin {
		return true
	}
	return false
}

func insufficientLiquidity(in Input) bool {
	return in.DailyVolume < minDailyVolumeQuote
}

func invalidRiskReward(in Input) bool {
	rr, ok := longRiskReward(in)
	if !ok {
		return true
	}
	return rr < minRewardRiskRatio
}
