package overnightmeanreversion

import (
	"slices"
	"testing"
)

func TestScore_highQualityReversalSetup(t *testing.T) {
	t.Parallel()
	in := Input{
		Symbol:                  "ETH-USD",
		CurrentPrice:            3500,
		DailyVolume:             50_000_000,
		PercentChange24h:        5,
		IntradayDrawdownPercent: 4,
		RSI:                     38,
		SupportDistancePercent:  1.0,
		VolumeTrend:             VolumeTrendRising,
		CandleSignal:            CandleSignalBullishReversal,
		HigherLowDetected:       true,
		PlannedEntry:            100,
		StopLoss:                98,
		TargetPrice:             106,
	}
	got := Score(in)
	if got.FinalScore < strongSetupMinScore {
		t.Fatalf("FinalScore=%v want>=%v", got.FinalScore, strongSetupMinScore)
	}
	if got.Label != LabelStrongSetup {
		t.Fatalf("Label=%q want %q", got.Label, LabelStrongSetup)
	}
	if len(got.FailedFilters) != 0 {
		t.Fatalf("FailedFilters=%v want empty", got.FailedFilters)
	}
	if len(got.Reasons) == 0 {
		t.Fatal("expected non-empty Reasons")
	}
}

func TestScore_lowVolumeRejection(t *testing.T) {
	t.Parallel()
	in := Input{
		Symbol:                  "SHIB-USD",
		CurrentPrice:            0.01,
		DailyVolume:             50_000,
		PercentChange24h:        2,
		IntradayDrawdownPercent: 3,
		RSI:                     40,
		SupportDistancePercent:  2,
		VolumeTrend:             VolumeTrendFlat,
		CandleSignal:            CandleSignalNeutral,
		HigherLowDetected:       false,
		PlannedEntry:            10,
		StopLoss:                9,
		TargetPrice:             13,
	}
	got := Score(in)
	if !slices.Contains(got.FailedFilters, FailedInsufficientLiquidity) {
		t.Fatalf("FailedFilters=%v want contains %q", got.FailedFilters, FailedInsufficientLiquidity)
	}
	if got.Label != LabelAvoid {
		t.Fatalf("Label=%q want %q", got.Label, LabelAvoid)
	}
}

func TestScore_catastrophicSelloffRejection(t *testing.T) {
	t.Parallel()
	in := Input{
		Symbol:                  "ALT-USD",
		CurrentPrice:            1.2,
		DailyVolume:             5_000_000,
		PercentChange24h:        -40,
		IntradayDrawdownPercent: 20,
		RSI:                     22,
		SupportDistancePercent:  6,
		VolumeTrend:             VolumeTrendFalling,
		CandleSignal:            CandleSignalBearishContinuation,
		HigherLowDetected:       false,
		PlannedEntry:            1.2,
		StopLoss:                1.0,
		TargetPrice:             1.6,
	}
	got := Score(in)
	if !slices.Contains(got.FailedFilters, FailedCatastrophicSelloff) {
		t.Fatalf("FailedFilters=%v want contains %q", got.FailedFilters, FailedCatastrophicSelloff)
	}
	if got.Label != LabelAvoid {
		t.Fatalf("Label=%q want %q", got.Label, LabelAvoid)
	}
	if got.FinalScore > 40 {
		t.Fatalf("FinalScore=%v want low score for catastrophic context", got.FinalScore)
	}
}

func TestScore_oversoldFarFromSupportWarning(t *testing.T) {
	t.Parallel()
	in := Input{
		Symbol:                  "BTC-USD",
		CurrentPrice:            60_000,
		DailyVolume:             80_000_000,
		PercentChange24h:        1.5,
		IntradayDrawdownPercent: 5,
		RSI:                     22,
		SupportDistancePercent:  8,
		VolumeTrend:             VolumeTrendFlat,
		CandleSignal:            CandleSignalNeutral,
		HigherLowDetected:       false,
		PlannedEntry:            100,
		StopLoss:                97,
		TargetPrice:             109,
	}
	got := Score(in)
	if !slices.Contains(got.Warnings, WarningOversoldFarFromSupport) {
		t.Fatalf("Warnings=%v want contains %q", got.Warnings, WarningOversoldFarFromSupport)
	}
	if got.Label == LabelStrongSetup {
		t.Fatalf("Label=%q should not be strong setup", got.Label)
	}
}

func TestScore_strongTrendControlledPullback(t *testing.T) {
	t.Parallel()
	in := Input{
		Symbol:                  "SOL-USD",
		CurrentPrice:            140,
		DailyVolume:             40_000_000,
		PercentChange24h:        6,
		IntradayDrawdownPercent: 3,
		RSI:                     44,
		SupportDistancePercent:  2,
		VolumeTrend:             VolumeTrendRising,
		CandleSignal:            CandleSignalNeutral,
		HigherLowDetected:       true,
		PlannedEntry:            100,
		StopLoss:                97,
		TargetPrice:             109,
	}
	got := Score(in)
	if got.CategoryScores.TrendQuality < 70 {
		t.Fatalf("TrendQuality=%v want>=70", got.CategoryScores.TrendQuality)
	}
	if got.CategoryScores.PullbackQuality < 70 {
		t.Fatalf("PullbackQuality=%v want>=70", got.CategoryScores.PullbackQuality)
	}
}

func TestScore_invalidRiskReward(t *testing.T) {
	t.Parallel()
	in := Input{
		Symbol:                  "ETH-USD",
		CurrentPrice:            3000,
		DailyVolume:             30_000_000,
		PercentChange24h:        4,
		IntradayDrawdownPercent: 4,
		RSI:                     36,
		SupportDistancePercent:  1.5,
		VolumeTrend:             VolumeTrendRising,
		CandleSignal:            CandleSignalBullishReversal,
		HigherLowDetected:       true,
		PlannedEntry:            100,
		StopLoss:                98,
		TargetPrice:             100,
	}
	got := Score(in)
	if !slices.Contains(got.FailedFilters, FailedInvalidRiskReward) {
		t.Fatalf("FailedFilters=%v want contains %q", got.FailedFilters, FailedInvalidRiskReward)
	}
	if got.Label != LabelAvoid {
		t.Fatalf("Label=%q want %q", got.Label, LabelAvoid)
	}
}

func TestScore_invalidRiskReward_badRatio(t *testing.T) {
	t.Parallel()
	in := Input{
		Symbol:                  "ETH-USD",
		CurrentPrice:            3000,
		DailyVolume:             30_000_000,
		PercentChange24h:        4,
		IntradayDrawdownPercent: 4,
		RSI:                     36,
		SupportDistancePercent:  1.5,
		VolumeTrend:             VolumeTrendRising,
		CandleSignal:            CandleSignalBullishReversal,
		HigherLowDetected:       true,
		PlannedEntry:            100,
		StopLoss:                99,
		TargetPrice:             100.5,
	}
	got := Score(in)
	if !slices.Contains(got.FailedFilters, FailedInvalidRiskReward) {
		t.Fatalf("FailedFilters=%v want contains %q", got.FailedFilters, FailedInvalidRiskReward)
	}
}

func TestScore_weakCoinHugeDropDoesNotScoreHighly(t *testing.T) {
	t.Parallel()
	in := Input{
		Symbol:                  "MEME-USD",
		CurrentPrice:            0.05,
		DailyVolume:             3_000_000,
		PercentChange24h:        -32,
		IntradayDrawdownPercent: 18,
		RSI:                     24,
		SupportDistancePercent:  5,
		VolumeTrend:             VolumeTrendFalling,
		CandleSignal:            CandleSignalBearishContinuation,
		HigherLowDetected:       false,
		PlannedEntry:            100,
		StopLoss:                95,
		TargetPrice:             115,
	}
	got := Score(in)
	if got.FinalScore >= possibleSetupMinScore {
		t.Fatalf("FinalScore=%v should stay below possible threshold for weak huge drop", got.FinalScore)
	}
	if got.Label == LabelStrongSetup {
		t.Fatalf("Label=%q should not be strong", got.Label)
	}
}
