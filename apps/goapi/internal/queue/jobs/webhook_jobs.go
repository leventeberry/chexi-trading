package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"goapi/config"
	authinfra "goapi/internal/infra/auth"
	"goapi/internal/queue"
	"goapi/internal/webhooks"
	"goapi/models"
	"goapi/repositories"
)

const (
	// WebhookDeliver posts one organization webhook delivery (retry via queue worker).
	WebhookDeliver = "webhook.deliver"
)

const (
	webhookHTTPTimeout       = 30 * time.Second
	webhookMaxResponseStored = 4096
)

var defaultWebhookHTTPClient = &http.Client{
	Timeout: webhookHTTPTimeout,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// HandleWebhookDelivery runs a single webhook delivery (used by the queue worker and synchronous fallback).
func HandleWebhookDelivery(ctx context.Context, repo repositories.OrganizationWebhookRepository, cfg *config.Config, deliveryID uuid.UUID) error {
	return handleWebhookDeliver(ctx, defaultWebhookHTTPClient, repo, cfg, deliveryID)
}

type webhookDeliverPayload struct {
	DeliveryID string `json:"delivery_id"`
}

// RegisterWebhookHandlers wires outbound webhook delivery jobs.
func RegisterWebhookHandlers(reg *queue.Registry, repo repositories.OrganizationWebhookRepository, cfg *config.Config) {
	reg.Register(WebhookDeliver, func(ctx context.Context, payload json.RawMessage) error {
		var p webhookDeliverPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("%w: %v", queue.ErrInvalidPayload, err)
		}
		deliveryID, err := uuid.Parse(strings.TrimSpace(p.DeliveryID))
		if err != nil || deliveryID == uuid.Nil {
			return fmt.Errorf("%w: invalid delivery_id", queue.ErrInvalidPayload)
		}
		return handleWebhookDeliver(ctx, defaultWebhookHTTPClient, repo, cfg, deliveryID)
	})
}

func handleWebhookDeliver(ctx context.Context, client *http.Client, repo repositories.OrganizationWebhookRepository, cfg *config.Config, deliveryID uuid.UUID) error {
	if cfg == nil || !config.WebhooksEncryptionConfigured(cfg) {
		return nil
	}
	maxAttempts := cfg.Queue.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 5
	}

	delivery, err := repo.FindDeliveryByID(deliveryID)
	if err != nil {
		if err == repositories.ErrOrganizationWebhookDeliveryNotFound {
			return nil
		}
		return err
	}
	if delivery.Status == models.OrganizationWebhookDeliveryStatusDelivered ||
		delivery.Status == models.OrganizationWebhookDeliveryStatusFailed {
		return nil
	}

	hook, err := repo.FindWebhookByID(delivery.WebhookID)
	if err != nil {
		if err == repositories.ErrOrganizationWebhookNotFound {
			return nil
		}
		return err
	}
	if !hook.Enabled {
		delivery.Status = models.OrganizationWebhookDeliveryStatusFailed
		msg := "webhook disabled"
		delivery.LastError = &msg
		_ = repo.UpdateDelivery(delivery)
		return nil
	}

	plainSecret, err := authinfra.DecryptAESGCM(hook.SecretCiphertext, cfg.Webhooks.EncryptionKey)
	if err != nil {
		delivery.Attempts++
		errMsg := "decrypt webhook secret failed"
		delivery.LastError = &errMsg
		delivery.Status = models.OrganizationWebhookDeliveryStatusFailed
		_ = repo.UpdateDelivery(delivery)
		return nil
	}

	delivery.Attempts++
	if err := webhooks.ValidateOutboundWebhookURL(ctx, cfg, hook.URL, webhooks.DefaultLookupHost); err != nil {
		return failDelivery(repo, delivery, maxAttempts, "webhook url blocked by security policy")
	}

	body := delivery.Payload
	ts := time.Now().UTC().Unix()
	sig := webhooks.SignPayload(plainSecret, ts, body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL, bytes.NewReader(body))
	if err != nil {
		return failDelivery(repo, delivery, maxAttempts, fmt.Sprintf("build request: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)
	req.Header.Set("X-Webhook-Timestamp", strconv.FormatInt(ts, 10))

	resp, err := client.Do(req)
	if err != nil {
		return failDelivery(repo, delivery, maxAttempts, err.Error())
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, webhookMaxResponseStored+1))
	trunc := truncateForStore(string(respBody), webhookMaxResponseStored)
	code := resp.StatusCode
	delivery.ResponseStatus = &code
	if trunc != "" {
		delivery.ResponseBodyTruncated = &trunc
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("http status %d", resp.StatusCode)
		delivery.LastError = &msg
		if err := repo.UpdateDelivery(delivery); err != nil {
			return err
		}
		if delivery.Attempts >= maxAttempts {
			delivery.Status = models.OrganizationWebhookDeliveryStatusFailed
			_ = repo.UpdateDelivery(delivery)
			return nil
		}
		return fmt.Errorf("webhook delivery failed: status %d", resp.StatusCode)
	}

	now := time.Now().UTC()
	delivery.Status = models.OrganizationWebhookDeliveryStatusDelivered
	delivery.DeliveredAt = &now
	delivery.LastError = nil
	if err := repo.UpdateDelivery(delivery); err != nil {
		return err
	}
	return nil
}

func failDelivery(repo repositories.OrganizationWebhookRepository, delivery *models.OrganizationWebhookDelivery, maxAttempts int, msg string) error {
	delivery.LastError = &msg
	if err := repo.UpdateDelivery(delivery); err != nil {
		return err
	}
	if delivery.Attempts >= maxAttempts {
		delivery.Status = models.OrganizationWebhookDeliveryStatusFailed
		_ = repo.UpdateDelivery(delivery)
		return nil
	}
	return fmt.Errorf("webhook delivery failed: %s", msg)
}

func truncateForStore(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
