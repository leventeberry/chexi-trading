// Package state holds the latest in-memory market snapshots (e.g. per-product tickers).
// It does not persist data or perform scoring.
package state

import (
	"slices"
	"sync"

	"goapi/internal/marketdata/coinbase"
)

// Store is a concurrency-safe in-memory map of the latest ticker per product ID.
type Store struct {
	mu sync.RWMutex
	m  map[string]coinbase.MarketTickerEvent
}

// New returns an empty store.
func New() *Store {
	return &Store{m: make(map[string]coinbase.MarketTickerEvent)}
}

// UpsertTicker replaces the stored ticker for ev.ProductID.
// Events with an empty ProductID are ignored.
func (s *Store) UpsertTicker(ev coinbase.MarketTickerEvent) {
	pid := ev.ProductID
	if pid == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = make(map[string]coinbase.MarketTickerEvent)
	}
	s.m[pid] = ev
}

// GetTicker returns the latest ticker for productID and whether it was found.
func (s *Store) GetTicker(productID string) (coinbase.MarketTickerEvent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ev, ok := s.m[productID]
	return ev, ok
}

// ListTickers returns a new slice of ticker copies sorted by product ID (stable ordering).
func (s *Store) ListTickers() []coinbase.MarketTickerEvent {
	snap := s.Snapshot()
	keys := make([]string, 0, len(snap))
	for k := range snap {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	out := make([]coinbase.MarketTickerEvent, 0, len(keys))
	for _, k := range keys {
		out = append(out, snap[k])
	}
	return out
}

// Snapshot returns a shallow copy of the internal map: new map, same struct values
// (MarketTickerEvent is a value type — safe to read without holding the lock).
func (s *Store) Snapshot() map[string]coinbase.MarketTickerEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]coinbase.MarketTickerEvent, len(s.m))
	for k, v := range s.m {
		out[k] = v
	}
	return out
}
