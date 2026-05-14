package coinbase

import (
	"encoding/json"
	"testing"
)

func TestBuildSubscribeTickerMessage(t *testing.T) {
	t.Parallel()
	raw, err := BuildSubscribeTickerMessage([]string{"BTC-USD", "ETH-USD", "SOL-USD"})
	if err != nil {
		t.Fatalf("BuildSubscribeTickerMessage: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["type"] != "subscribe" {
		t.Fatalf("type=%v", m["type"])
	}
	chs, _ := m["channels"].([]any)
	if len(chs) != 1 {
		t.Fatalf("channels len=%d", len(chs))
	}
	if chs[0] != "ticker" {
		t.Fatalf("channel[0]=%v want ticker", chs[0])
	}
	ids, _ := m["product_ids"].([]any)
	if len(ids) != 3 {
		t.Fatalf("product_ids=%v", ids)
	}
}

func TestNormalizeProductIDs_rejectsInvalid(t *testing.T) {
	t.Parallel()
	if _, err := NormalizeProductIDs([]string{"bad id"}); err == nil {
		t.Fatal("expected error")
	}
}
