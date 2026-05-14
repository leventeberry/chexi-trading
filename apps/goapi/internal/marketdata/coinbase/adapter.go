package coinbase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

// TickerAdapterConfig configures the public WebSocket ticker client.
type TickerAdapterConfig struct {
	URL              string
	ProductIDs       []string
	InitialReconnect time.Duration
	MaxReconnect     time.Duration
	ReadDeadline     time.Duration
}

// DefaultTickerAdapterConfig returns sensible defaults for production Coinbase Exchange ws-feed.
func DefaultTickerAdapterConfig(url string, productIDs []string) TickerAdapterConfig {
	return TickerAdapterConfig{
		URL:              url,
		ProductIDs:       productIDs,
		InitialReconnect: time.Second,
		MaxReconnect:     30 * time.Second,
		ReadDeadline:     60 * time.Second,
	}
}

// TickerAdapter maintains a WebSocket connection and forwards normalized tickers.
// It is safe for one goroutine to call Run.
type TickerAdapter struct {
	cfg    TickerAdapterConfig
	onTick func(MarketTickerEvent)
	logf   func(string, ...any)
}

// NewTickerAdapter builds an adapter. onTick is invoked synchronously from the read loop (keep it fast).
func NewTickerAdapter(cfg TickerAdapterConfig, onTick func(MarketTickerEvent)) *TickerAdapter {
	if onTick == nil {
		onTick = func(MarketTickerEvent) {}
	}
	return &TickerAdapter{cfg: cfg, onTick: onTick, logf: func(string, ...any) {}}
}

// SetLogger sets an optional printf-style logger for reconnect and parse warnings.
func (a *TickerAdapter) SetLogger(logf func(string, ...any)) {
	if logf != nil {
		a.logf = logf
	}
}

// Run connects, subscribes to ticker for cfg.ProductIDs, and processes messages until ctx is cancelled.
// It reconnects with exponential backoff on connection or read failures.
func (a *TickerAdapter) Run(ctx context.Context) {
	if a.cfg.URL == "" {
		a.logf("coinbase ws: empty url, exiting")
		return
	}
	attempt := 0
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		if err := a.runOnce(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			d := reconnectBackoff(attempt, a.cfg.InitialReconnect, a.cfg.MaxReconnect)
			a.logf("coinbase ws: session ended: %v; reconnecting in %s", err, d)
			attempt++
			if err := sleepCtx(ctx, d); err != nil {
				return
			}
			continue
		}
		return
	}
}

func (a *TickerAdapter) runOnce(ctx context.Context) error {
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.DialContext(ctx, a.cfg.URL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	sub, err := BuildSubscribeTickerMessage(a.cfg.ProductIDs)
	if err != nil {
		return fmt.Errorf("subscribe message: %w", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, sub); err != nil {
		return fmt.Errorf("write subscribe: %w", err)
	}

	rd := a.cfg.ReadDeadline
	if rd <= 0 {
		rd = 60 * time.Second
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := conn.SetReadDeadline(time.Now().Add(rd)); err != nil {
			return fmt.Errorf("set read deadline: %w", err)
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		now := time.Now().UTC()
		ev, ignored, herr := HandleRawMessage(data, now)
		if herr != nil {
			a.logf("coinbase ws: drop malformed message: %v", herr)
			continue
		}
		if ignored || ev == nil {
			continue
		}
		a.onTick(*ev)
	}
}
