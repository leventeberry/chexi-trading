package state

import (
	"sync"
	"testing"
	"time"

	"goapi/internal/marketdata/coinbase"
)

func sampleTicker(productID string, price float64) coinbase.MarketTickerEvent {
	return coinbase.MarketTickerEvent{
		Source:     coinbase.SourcePublicWS,
		Type:       "ticker",
		ProductID:  productID,
		Price:      price,
		Open24h:    1,
		Volume24h:  2,
		Low24h:     3,
		High24h:    4,
		BestBid:    5,
		BestAsk:    6,
		Side:       "buy",
		Time:       "t",
		ReceivedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

func TestUpsertTicker_storesTicker(t *testing.T) {
	t.Parallel()
	s := New()
	ev := sampleTicker("BTC-USD", 100)
	s.UpsertTicker(ev)
	got, ok := s.GetTicker("BTC-USD")
	if !ok || got.Price != 100 {
		t.Fatalf("GetTicker=%v ok=%v", got, ok)
	}
}

func TestGetTicker_notFound(t *testing.T) {
	t.Parallel()
	s := New()
	_, ok := s.GetTicker("MISSING-USD")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestUpsertTicker_ignoresEmptyProductID(t *testing.T) {
	t.Parallel()
	s := New()
	s.UpsertTicker(coinbase.MarketTickerEvent{ProductID: ""})
	if len(s.Snapshot()) != 0 {
		t.Fatalf("expected empty store")
	}
}

func TestListTickers_sortedStableCopies(t *testing.T) {
	t.Parallel()
	s := New()
	s.UpsertTicker(sampleTicker("ETH-USD", 2))
	s.UpsertTicker(sampleTicker("BTC-USD", 1))
	s.UpsertTicker(sampleTicker("SOL-USD", 3))
	list := s.ListTickers()
	if len(list) != 3 {
		t.Fatalf("len=%d", len(list))
	}
	if list[0].ProductID != "BTC-USD" || list[1].ProductID != "ETH-USD" || list[2].ProductID != "SOL-USD" {
		t.Fatalf("order wrong: %#v", list)
	}
	list[0].Price = 99999
	got, _ := s.GetTicker("BTC-USD")
	if got.Price != 1 {
		t.Fatalf("mutating list leaked: price=%v", got.Price)
	}
}

func TestSnapshot_notMutableInternalMap(t *testing.T) {
	t.Parallel()
	s := New()
	s.UpsertTicker(sampleTicker("BTC-USD", 50))
	snap := s.Snapshot()
	snap["BTC-USD"] = sampleTicker("BTC-USD", 777)
	snap["XRP-USD"] = sampleTicker("XRP-USD", 1)
	got, _ := s.GetTicker("BTC-USD")
	if got.Price != 50 {
		t.Fatalf("internal mutated: %v", got.Price)
	}
	if _, ok := s.GetTicker("XRP-USD"); ok {
		t.Fatal("phantom product from snapshot mutation")
	}
}

func TestConcurrentUpsertAndRead(t *testing.T) {
	t.Parallel()
	s := New()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pid := "AAA-USD"
			if i%2 == 0 {
				pid = "BBB-USD"
			}
			s.UpsertTicker(sampleTicker(pid, float64(i)))
			_, _ = s.GetTicker(pid)
			_ = s.ListTickers()
			_ = s.Snapshot()
		}(i)
	}
	wg.Wait()
	if _, ok := s.GetTicker("AAA-USD"); !ok {
		t.Fatal("missing AAA-USD")
	}
	if _, ok := s.GetTicker("BBB-USD"); !ok {
		t.Fatal("missing BBB-USD")
	}
}
