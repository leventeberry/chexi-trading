package omrinput

import (
	"errors"
	"math"
	"testing"

	"goapi/internal/marketdata/metrics"
	"goapi/internal/strategy/overnightmeanreversion"
)

func sampleMetrics() metrics.Metrics {
	return metrics.Metrics{
		ProductID:               "BTC-USD",
		CurrentPrice:            100,
		Open24h:                 90,
		High24h:                 110,
		Low24h:                  85,
		Volume24h:               5_000_000,
		SpreadAmount:            0.5,
		SpreadPercent:           0.5,
		PercentChange24h:        (100 - 90) / 90 * 100,
		IntradayDrawdownPercent: (110 - 100) / 110 * 100,
		RangePositionPercent:    50,
	}
}

func sampleManual() ManualFields {
	return ManualFields{
		RSI:                    40,
		SupportDistancePercent: 2.5,
		VolumeTrend:            overnightmeanreversion.VolumeTrendRising,
		CandleSignal:           overnightmeanreversion.CandleSignalNeutral,
		HigherLowDetected:      true,
		PlannedEntry:           100,
		StopLoss:               95,
		TargetPrice:            112,
	}
}

func TestInputFromMetrics_success(t *testing.T) {
	t.Parallel()
	m := sampleMetrics()
	man := sampleManual()
	in, err := InputFromMetrics(m, man)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if in.Symbol != "BTC-USD" || in.CurrentPrice != 100 || in.DailyVolume != 5_000_000 {
		t.Fatalf("market fields %+v", in)
	}
	if in.PercentChange24h != m.PercentChange24h || in.IntradayDrawdownPercent != m.IntradayDrawdownPercent {
		t.Fatalf("derived fields wrong")
	}
	if in.RSI != 40 || in.SupportDistancePercent != 2.5 {
		t.Fatalf("manual fields %+v", in)
	}
	if in.VolumeTrend != man.VolumeTrend || in.CandleSignal != man.CandleSignal {
		t.Fatalf("enum passthrough wrong")
	}
	if !in.HigherLowDetected {
		t.Fatal("higher low flag")
	}
	if in.PlannedEntry != 100 || in.StopLoss != 95 || in.TargetPrice != 112 {
		t.Fatalf("plan %+v", in)
	}
}

func TestInputFromMetrics_invalidRSI(t *testing.T) {
	t.Parallel()
	man := sampleManual()
	man.RSI = 101
	_, err := InputFromMetrics(sampleMetrics(), man)
	if !errors.Is(err, ErrInvalidRSI) {
		t.Fatalf("err=%v", err)
	}
}

func TestInputFromMetrics_negativeSupportDistance(t *testing.T) {
	t.Parallel()
	man := sampleManual()
	man.SupportDistancePercent = -0.1
	_, err := InputFromMetrics(sampleMetrics(), man)
	if !errors.Is(err, ErrInvalidSupportDistance) {
		t.Fatalf("err=%v", err)
	}
}

func TestInputFromMetrics_invalidPlannedTrade(t *testing.T) {
	t.Parallel()
	man := sampleManual()
	man.StopLoss = 100
	man.PlannedEntry = 99
	man.TargetPrice = 110
	_, err := InputFromMetrics(sampleMetrics(), man)
	if !errors.Is(err, ErrInvalidPlannedTrade) {
		t.Fatalf("err=%v", err)
	}
}

func TestInputFromMetrics_volumeTrendPassthrough(t *testing.T) {
	t.Parallel()
	for _, vt := range []overnightmeanreversion.VolumeTrend{
		overnightmeanreversion.VolumeTrendRising,
		overnightmeanreversion.VolumeTrendFlat,
		overnightmeanreversion.VolumeTrendFalling,
	} {
		man := sampleManual()
		man.VolumeTrend = vt
		in, err := InputFromMetrics(sampleMetrics(), man)
		if err != nil {
			t.Fatalf("%s err=%v", vt, err)
		}
		if in.VolumeTrend != vt {
			t.Fatalf("want %s got %s", vt, in.VolumeTrend)
		}
	}
}

func TestInputFromMetrics_candleSignalPassthrough(t *testing.T) {
	t.Parallel()
	for _, cs := range []overnightmeanreversion.CandleSignal{
		overnightmeanreversion.CandleSignalBullishReversal,
		overnightmeanreversion.CandleSignalNeutral,
		overnightmeanreversion.CandleSignalBearishContinuation,
	} {
		man := sampleManual()
		man.CandleSignal = cs
		in, err := InputFromMetrics(sampleMetrics(), man)
		if err != nil {
			t.Fatalf("%s err=%v", cs, err)
		}
		if in.CandleSignal != cs {
			t.Fatalf("want %s got %s", cs, in.CandleSignal)
		}
	}
}

func TestInputFromMetrics_invalidVolumeTrend(t *testing.T) {
	t.Parallel()
	man := sampleManual()
	man.VolumeTrend = overnightmeanreversion.VolumeTrend("sideways")
	_, err := InputFromMetrics(sampleMetrics(), man)
	if !errors.Is(err, ErrInvalidVolumeTrend) {
		t.Fatalf("err=%v", err)
	}
}

func TestInputFromMetrics_invalidCandleSignal(t *testing.T) {
	t.Parallel()
	man := sampleManual()
	man.CandleSignal = overnightmeanreversion.CandleSignal("doji")
	_, err := InputFromMetrics(sampleMetrics(), man)
	if !errors.Is(err, ErrInvalidCandleSignal) {
		t.Fatalf("err=%v", err)
	}
}

func TestInputFromMetrics_metricsDrawdownNegativeRejected(t *testing.T) {
	t.Parallel()
	m := sampleMetrics()
	m.IntradayDrawdownPercent = -1
	_, err := InputFromMetrics(m, sampleManual())
	if !errors.Is(err, ErrInvalidMetricsDrawdown) {
		t.Fatalf("err=%v", err)
	}
}

func TestInputFromMetrics_trimSymbol(t *testing.T) {
	t.Parallel()
	m := sampleMetrics()
	m.ProductID = "  ETH-USD  "
	man := sampleManual()
	in, err := InputFromMetrics(m, man)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if in.Symbol != "ETH-USD" {
		t.Fatalf("symbol=%q", in.Symbol)
	}
}

func TestInputFromMetrics_nanRSI(t *testing.T) {
	t.Parallel()
	man := sampleManual()
	man.RSI = math.NaN()
	_, err := InputFromMetrics(sampleMetrics(), man)
	if !errors.Is(err, ErrInvalidRSI) {
		t.Fatalf("err=%v", err)
	}
}
