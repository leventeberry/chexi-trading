package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"goapi/config"
	"goapi/logger"
)

const defaultResendAPIURL = "https://api.resend.com/emails"

// ResendSender sends transactional mail via the Resend HTTP API (https://resend.com/docs/api-reference/emails/send-email).
type ResendSender struct {
	apiKey        string
	defaultFrom   string
	env           string
	redirectAllTo string
	httpClient    *http.Client
	apiURL        string // override for tests
}

// NewResendSender returns a sender that POSTs to Resend. redirectAllTo applies only outside staging/production.
func NewResendSender(apiKey, defaultFrom, env, redirectAllTo string) *ResendSender {
	return &ResendSender{
		apiKey:        strings.TrimSpace(apiKey),
		defaultFrom:   strings.TrimSpace(defaultFrom),
		env:           strings.TrimSpace(env),
		redirectAllTo: strings.TrimSpace(redirectAllTo),
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		apiURL:        defaultResendAPIURL,
	}
}

type resendEmailPayload struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

// Send implements Sender.
func (s *ResendSender) Send(ctx context.Context, msg Message) error {
	to := strings.TrimSpace(msg.To)
	if to == "" {
		return fmt.Errorf("resend: empty recipient")
	}
	if s.redirectAllTo != "" && !config.IsStagingOrProductionEnvironment(s.env) {
		logger.Log.Debug().Str("kind", msg.Kind).Str("original_to", to).Str("redirect_to", s.redirectAllTo).Msg("email recipient redirect (non-production only)")
		to = s.redirectAllTo
	}

	from := strings.TrimSpace(msg.From)
	if from == "" {
		from = s.defaultFrom
	}
	if from == "" {
		return fmt.Errorf("resend: empty from (set EMAIL_FROM)")
	}

	body := map[string]any{
		"from":    from,
		"to":      []string{to},
		"subject": strings.TrimSpace(msg.Subject),
		"text":    msg.TextBody,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("resend: marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("resend: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("resend: request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.Log.Info().
			Str("kind", msg.Kind).
			Str("to", to).
			Int("status", resp.StatusCode).
			Msg("email sent via Resend")
		// Match LogSender behavior in development: echo body to logs so local docker logs / E2E scripts can recover tokens.
		if !config.IsStagingOrProductionEnvironment(s.env) {
			logger.Log.Info().
				Str("kind", msg.Kind).
				Str("to", to).
				Str("body", msg.TextBody).
				Msg("email dev log echo (contains tokens; non-production only)")
		}
		return nil
	}

	logger.Log.Warn().
		Str("kind", msg.Kind).
		Int("status", resp.StatusCode).
		Bytes("body", respBody).
		Msg("Resend API error")
	return fmt.Errorf("resend: HTTP %d", resp.StatusCode)
}
