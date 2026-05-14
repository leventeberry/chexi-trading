package coinbase

import (
	"encoding/json"
	"fmt"
	"strings"
)

const defaultTickerChannel = "ticker"

// SubscribeTickerPayload is the Coinbase Exchange subscribe message for the ticker channel.
// See https://docs.cdp.coinbase.com/exchange/websocket-feed/channels — channels must be string names, e.g. ["ticker"].
type SubscribeTickerPayload struct {
	Type       string   `json:"type"`
	ProductIDs []string `json:"product_ids"`
	Channels   []string `json:"channels"`
}

// BuildSubscribeTickerMessage returns JSON for subscribing to the ticker channel for the given products.
func BuildSubscribeTickerMessage(productIDs []string) ([]byte, error) {
	ids, err := NormalizeProductIDs(productIDs)
	if err != nil {
		return nil, err
	}
	msg := SubscribeTickerPayload{
		Type:       "subscribe",
		ProductIDs: ids,
		Channels:   []string{defaultTickerChannel},
	}
	return json.Marshal(msg)
}

// NormalizeProductIDs trims, de-duplicates, and validates Coinbase-style product IDs (e.g. BTC-USD).
func NormalizeProductIDs(productIDs []string) ([]string, error) {
	if len(productIDs) == 0 {
		return nil, fmt.Errorf("at least one product_id is required")
	}
	seen := make(map[string]struct{}, len(productIDs))
	out := make([]string, 0, len(productIDs))
	for _, raw := range productIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		if err := validateProductID(id); err != nil {
			return nil, err
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one non-empty product_id is required")
	}
	return out, nil
}

func validateProductID(id string) error {
	if len(id) < 3 || len(id) > 32 {
		return fmt.Errorf("invalid product_id %q", id)
	}
	for i, r := range id {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-':
			if i == 0 || i == len(id)-1 {
				return fmt.Errorf("invalid product_id %q", id)
			}
		default:
			return fmt.Errorf("invalid product_id %q", id)
		}
	}
	return nil
}
