package overnightmeanreversion

import "math"

const (
	weightLiquidity       = 0.15
	weightTrend           = 0.20
	weightPullback        = 0.20
	weightExhaustion      = 0.15
	weightRisk            = 0.15
	weightFinalReversal   = 0.15
	strongSetupMinScore   = 80.0
	possibleSetupMinScore = 60.0
	watchMinScore         = 40.0
)

// Score evaluates the overnight mean-reversion setup from a single Input snapshot.
// It is deterministic: no I/O, time, or randomness.
func Score(in Input) Result {
	var reasons, warnings, failed []string

	if insufficientLiquidity(in) {
		failed = append(failed, FailedInsufficientLiquidity)
	}
	if catastrophicSelloff(in) {
		failed = append(failed, FailedCatastrophicSelloff)
	}
	if invalidRiskReward(in) {
		failed = append(failed, FailedInvalidRiskReward)
	}

	lq, rLq, wLq := liquidityScore(in)
	tr, rTr, wTr := trendQualityScore(in)
	pb, rPb, wPb := pullbackQualityScore(in)
	ex, rEx, wEx := exhaustionScore(in)
	rk, rRk, wRk := riskSetupScore(in)
	fr, rFr, wFr := finalReversalScore(in)

	reasons = append(reasons, rLq...)
	reasons = append(reasons, rTr...)
	reasons = append(reasons, rPb...)
	reasons = append(reasons, rEx...)
	reasons = append(reasons, rRk...)
	reasons = append(reasons, rFr...)

	warnings = append(warnings, wLq...)
	warnings = append(warnings, wTr...)
	warnings = append(warnings, wPb...)
	warnings = append(warnings, wEx...)
	warnings = append(warnings, wRk...)
	warnings = append(warnings, wFr...)

	// Oversold far from support: explicit cross-field warning when RSI very low.
	if in.RSI < rsiOversold && in.SupportDistancePercent > supportFarPct {
		warnings = append(warnings, WarningOversoldFarFromSupport)
	}

	cat := CategoryScores{
		Liquidity:       lq,
		TrendQuality:    tr,
		PullbackQuality: pb,
		Exhaustion:      ex,
		RiskSetup:       rk,
		FinalReversal:   fr,
	}

	raw := weightLiquidity*lq +
		weightTrend*tr +
		weightPullback*pb +
		weightExhaustion*ex +
		weightRisk*rk +
		weightFinalReversal*fr

	// Hard situations: pull composite down even if some components are high.
	if catastrophicSelloff(in) {
		raw = math.Min(raw, 32)
	}
	if invalidRiskReward(in) {
		raw = math.Min(raw, 25)
	}
	if insufficientLiquidity(in) {
		raw = math.Min(raw, 22)
	}

	final := clamp(raw, 0, 100)

	label := labelFromScoreAndFilters(final, failed)

	return Result{
		FinalScore:     final,
		Label:          label,
		CategoryScores: cat,
		Reasons:        dedupeStrings(reasons),
		Warnings:       dedupeStrings(warnings),
		FailedFilters:  dedupeStrings(failed),
	}
}

func labelFromScoreAndFilters(final float64, failed []string) Label {
	if len(failed) > 0 {
		return LabelAvoid
	}
	switch {
	case final >= strongSetupMinScore:
		return LabelStrongSetup
	case final >= possibleSetupMinScore:
		return LabelPossibleSetup
	case final >= watchMinScore:
		return LabelWatch
	default:
		return LabelAvoid
	}
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
