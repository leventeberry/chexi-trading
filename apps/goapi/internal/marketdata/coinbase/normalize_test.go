package coinbase

import (
	"strings"
	"testing"
	"time"
)

func validTickerJSON() string {
	return `{
  "type": "ticker",
  "sequence": 1,
  "product_id": "BTC-USD",
  "price": "50000.12",
  "open_24h": "49000.00",
  "volume_24h": "1234.56",
  "low_24h": "48000.00",
  "high_24h": "51000.00",
  "best_bid": "49999.00",
  "best_ask": "50000.50",
  "side": "buy",
  "time": "2022-01-02T15:04:05Z"
}`
}

func TestNormalizeTickerJSON_valid(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	ev, err := NormalizeTickerJSON([]byte(validTickerJSON()), ts)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if ev.Source != SourcePublicWS {
		t.Fatalf("source=%q", ev.Source)
	}
	if ev.ProductID != "BTC-USD" {
		t.Fatalf("product=%q", ev.ProductID)
	}
	if ev.Price != 50000.12 || ev.Open24h != 49000 || ev.Volume24h != 1234.56 {
		t.Fatalf("prices %+v", ev)
	}
	if ev.ReceivedAt != ts.UTC() {
		t.Fatalf("receivedAt=%v", ev.ReceivedAt)
	}
}

func TestNormalizeTickerJSON_malformedJSON(t *testing.T) {
	t.Parallel()
	_, err := NormalizeTickerJSON([]byte(`{`), time.Now())
	if err == nil || !strings.Contains(err.Error(), "ticker json") {
		t.Fatalf("err=%v", err)
	}
}

func TestNormalizeTickerJSON_invalidNumeric(t *testing.T) {
	t.Parallel()
	j := strings.Replace(validTickerJSON(), `"50000.12"`, `"not-a-number"`, 1)
	_, err := NormalizeTickerJSON([]byte(j), time.Now())
	if err == nil || !strings.Contains(err.Error(), "price") {
		t.Fatalf("err=%v", err)
	}
}

func TestNormalizeTickerJSON_negativePrice(t *testing.T) {
	t.Parallel()
	j := strings.Replace(validTickerJSON(), `"50000.12"`, `"-1"`, 1)
	_, err := NormalizeTickerJSON([]byte(j), time.Now())
	if err == nil || !strings.Contains(err.Error(), "non-negative") {
		t.Fatalf("err=%v", err)
	}
}
