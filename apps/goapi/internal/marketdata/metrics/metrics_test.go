package metrics

import (
	"errors"
	"math"
	"testing"
	"time"

	"goapi/internal/marketdata/coinbase"
)

func baseTicker() coinbase.MarketTickerEvent {
	return coinbase.MarketTickerEvent{
		Source:     coinbase.SourcePublicWS,
		Type:       "ticker",
		ProductID:  "BTC-USD",
		Price:      100,
		Open24h:    80,
		Volume24h:  1_000_000,
		Low24h:     70,
		High24h:    120,
		BestBid:    99.5,
		BestAsk:    100.5,
		Side:       "buy",
		Time:       "t",
		ReceivedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestFromTicker_normal(t *testing.T) {
	t.Parallel()
	m, err := FromTicker(baseTicker())
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if m.ProductID != "BTC-USD" || m.CurrentPrice != 100 {
		t.Fatalf("identity %+v", m)
	}
	if m.SpreadAmount != 1 || math.Abs(m.SpreadPercent-1) > 1e-9 {
		t.Fatalf("spread %+v", m)
	}
	if math.Abs(m.PercentChange24h-25) > 1e-9 {
		t.Fatalf("pct24h=%v want 25", m.PercentChange24h)
	}
	if math.Abs(m.IntradayDrawdownPercent-(20.0/120.0*100)) > 1e-9 {
		t.Fatalf("drawdown=%v", m.IntradayDrawdownPercent)
	}
	var wantRange = (100.0 - 70.0) / (120.0 - 70.0) * 100.0
	if math.Abs(m.RangePositionPercent-wantRange) > 1e-9 {
		t.Fatalf("rangePos=%v want %v", m.RangePositionPercent, wantRange)
	}
	if m.Volume24h != 1_000_000 {
		t.Fatalf("volume=%v", m.Volume24h)
	}
}

func TestFromTicker_zeroOpen24h(t *testing.T) {
	t.Parallel()
	in := baseTicker()
	in.Open24h = 0
	_, err := FromTicker(in)
	if !errors.Is(err, ErrOpen24hZero) {
		t.Fatalf("err=%v", err)
	}
}

func TestFromTicker_zeroCurrentPrice(t *testing.T) {
	t.Parallel()
	in := baseTicker()
	in.Price = 0
	_, err := FromTicker(in)
	if !errors.Is(err, ErrNonPositiveCurrentPrice) {
		t.Fatalf("err=%v", err)
	}
}

func TestFromTicker_highEqualsLow(t *testing.T) {
	t.Parallel()
	in := baseTicker()
	in.High24h = 100
	in.Low24h = 100
	_, err := FromTicker(in)
	if !errors.Is(err, ErrDegenerateHighLowRange) {
		t.Fatalf("err=%v", err)
	}
}

func TestFromTicker_invalidSpreadAskBelowBid(t *testing.T) {
	t.Parallel()
	in := baseTicker()
	in.BestBid = 101
	in.BestAsk = 99
	_, err := FromTicker(in)
	if !errors.Is(err, ErrInvalidSpread) {
		t.Fatalf("err=%v", err)
	}
}

func TestFromTicker_volumeZeroAllowed(t *testing.T) {
	t.Parallel()
	in := baseTicker()
	in.Volume24h = 0
	m, err := FromTicker(in)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if m.Volume24h != 0 {
		t.Fatalf("volume=%v", m.Volume24h)
	}
}
