package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// SignPayload returns HMAC-SHA256 hex of (timestamp + "." + body) using secret as key.
func SignPayload(secret []byte, timestampUnix int64, body []byte) string {
	msg := strconv.FormatInt(timestampUnix, 10) + "." + string(body)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}
