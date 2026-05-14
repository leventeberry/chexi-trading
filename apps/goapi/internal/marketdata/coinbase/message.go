package coinbase

import (
	"encoding/json"
	"strings"
	"time"
)

type messageHead struct {
	Type string `json:"type"`
}

// HandleRawMessage classifies a single WebSocket text payload.
// It returns (nil, true, nil) for subscription acks, heartbeats, and unknown types.
// For ticker messages it returns a normalized event, ignored=false, and a non-nil error if the ticker is malformed.
func HandleRawMessage(raw []byte, receivedAt time.Time) (*MarketTickerEvent, bool, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, true, nil
	}

	var head messageHead
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, false, err
	}

	switch strings.TrimSpace(head.Type) {
	case "subscriptions", "subscription":
		return nil, true, nil
	case "heartbeat":
		return nil, true, nil
	case "ticker":
		ev, err := NormalizeTickerJSON(raw, receivedAt)
		if err != nil {
			return nil, false, err
		}
		return &ev, false, nil
	default:
		if head.Type == "" {
			return nil, true, nil
		}
		return nil, true, nil
	}
}
