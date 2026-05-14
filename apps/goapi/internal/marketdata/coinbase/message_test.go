package coinbase

import (
	"strings"
	"testing"
	"time"
)

func TestHandleRawMessage_ignoresSubscriptions(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"type":"subscriptions","channels":[{"name":"ticker","product_ids":["BTC-USD"]}]}`)
	ev, ignored, err := HandleRawMessage(raw, time.Now())
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !ignored || ev != nil {
		t.Fatalf("ignored=%v ev=%v", ignored, ev)
	}
}

func TestHandleRawMessage_ignoresHeartbeat(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"type":"heartbeat","last_trade_id":0,"product_id":"BTC-USD","sequence":0,"time":"2020-01-01T00:00:00Z"}`)
	ev, ignored, err := HandleRawMessage(raw, time.Now())
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !ignored || ev != nil {
		t.Fatalf("ignored=%v ev=%v", ignored, ev)
	}
}

func TestHandleRawMessage_ignoresUnknownType(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"type":"last_match","trade_id":1}`)
	ev, ignored, err := HandleRawMessage(raw, time.Now())
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !ignored || ev != nil {
		t.Fatalf("ignored=%v ev=%v", ignored, ev)
	}
}

func TestHandleRawMessage_ticker(t *testing.T) {
	t.Parallel()
	ts := time.Now().UTC()
	ev, ignored, err := HandleRawMessage([]byte(validTickerJSON()), ts)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if ignored || ev == nil {
		t.Fatalf("ignored=%v", ignored)
	}
	if ev.ProductID != "BTC-USD" {
		t.Fatalf("product=%q", ev.ProductID)
	}
}

func TestHandleRawMessage_malformedTicker(t *testing.T) {
	t.Parallel()
	bad := strings.Replace(validTickerJSON(), `"BTC-USD"`, `""`, 1)
	ev, ignored, err := HandleRawMessage([]byte(bad), time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
	if ignored || ev != nil {
		t.Fatalf("ignored=%v ev=%v", ignored, ev)
	}
}

func TestHandleRawMessage_invalidJSON(t *testing.T) {
	t.Parallel()
	ev, ignored, err := HandleRawMessage([]byte(`not json`), time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
	if ignored || ev != nil {
		t.Fatalf("ignored=%v ev=%v", ignored, ev)
	}
}
