package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"goapi/config"
	authinfra "goapi/internal/infra/auth"
	"goapi/internal/queue"
	queuejobs "goapi/internal/queue/jobs"
	"goapi/internal/rbac"
	"goapi/internal/webhooks"
	"goapi/logger"
	"goapi/models"
	"goapi/repositories"
)

// Webhook event types (v1 allowlist).
const (
	WebhookEventOrganizationUpdated            = "organization.updated"
	WebhookEventOrganizationInvitationCreated  = "organization.invitation.created"
	WebhookEventOrganizationInvitationAccepted = "organization.invitation.accepted"
	WebhookEventOrganizationMemberRemoved      = "organization.member.removed"
	WebhookEventOrganizationNoteCreated        = "organization.note.created"
	WebhookEventOrganizationNoteDeleted        = "organization.note.deleted"
)

var organizationWebhookEventSet = map[string]struct{}{
	WebhookEventOrganizationUpdated:            {},
	WebhookEventOrganizationInvitationCreated:  {},
	WebhookEventOrganizationInvitationAccepted: {},
	WebhookEventOrganizationMemberRemoved:      {},
	WebhookEventOrganizationNoteCreated:        {},
	WebhookEventOrganizationNoteDeleted:        {},
}

// IsValidOrganizationWebhookEvent reports whether eventType is allowed for subscriptions and emits.
func IsValidOrganizationWebhookEvent(eventType string) bool {
	_, ok := organizationWebhookEventSet[strings.TrimSpace(eventType)]
	return ok
}

// webhookEmitter is implemented by OrganizationWebhookService for domain-event fanout.
type webhookEmitter interface {
	EmitOrganizationWebhookEvent(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]interface{})
}

// CreateOrganizationWebhookInput is the body for POST .../webhooks.
type CreateOrganizationWebhookInput struct {
	URL    string
	Events []string
}

// UpdateOrganizationWebhookInput is the body for PATCH .../webhooks/:webhookId.
type UpdateOrganizationWebhookInput struct {
	URL          *string
	Events       *[]string
	Enabled      *bool
	RotateSecret *bool
}

// OrganizationWebhookCreatedDTO includes the plaintext secret once.
type OrganizationWebhookCreatedDTO struct {
	ID               uuid.UUID `json:"id"`
	OrganizationID   uuid.UUID `json:"organization_id"`
	URL              string    `json:"url"`
	Events           []string  `json:"events"`
	Enabled          bool      `json:"enabled"`
	SecretKeyVersion int       `json:"secret_key_version"`
	CreatedByUserID  uuid.UUID `json:"created_by_user_id"`
	CreatedAt        string    `json:"created_at"`
	UpdatedAt        string    `json:"updated_at"`
	Secret           string    `json:"secret"`
}

// OrganizationWebhookListDTO is a safe list/get shape (no secret).
type OrganizationWebhookListDTO struct {
	ID               uuid.UUID `json:"id"`
	OrganizationID   uuid.UUID `json:"organization_id"`
	URL              string    `json:"url"`
	Events           []string  `json:"events"`
	Enabled          bool      `json:"enabled"`
	SecretKeyVersion int       `json:"secret_key_version"`
	CreatedByUserID  uuid.UUID `json:"created_by_user_id"`
	CreatedAt        string    `json:"created_at"`
	UpdatedAt        string    `json:"updated_at"`
}

// OrganizationWebhookPatchDTO may include a new secret when rotated.
type OrganizationWebhookPatchDTO struct {
	ID               uuid.UUID `json:"id"`
	OrganizationID   uuid.UUID `json:"organization_id"`
	URL              string    `json:"url"`
	Events           []string  `json:"events"`
	Enabled          bool      `json:"enabled"`
	SecretKeyVersion int       `json:"secret_key_version"`
	CreatedByUserID  uuid.UUID `json:"created_by_user_id"`
	CreatedAt        string    `json:"created_at"`
	UpdatedAt        string    `json:"updated_at"`
	Secret           *string   `json:"secret,omitempty"`
}

// OrganizationWebhookDeliveryDTO is one delivery row for history APIs.
type OrganizationWebhookDeliveryDTO struct {
	ID                    uuid.UUID       `json:"id"`
	WebhookID             uuid.UUID       `json:"webhook_id"`
	EventType             string          `json:"event_type"`
	Payload               json.RawMessage `json:"payload"`
	Status                string          `json:"status"`
	Attempts              int             `json:"attempts"`
	ResponseStatus        *int            `json:"response_status,omitempty"`
	ResponseBodyTruncated *string         `json:"response_body_truncated,omitempty"`
	LastError             *string         `json:"last_error,omitempty"`
	NextAttemptAt         *string         `json:"next_attempt_at,omitempty"`
	DeliveredAt           *string         `json:"delivered_at,omitempty"`
	CreatedAt             string          `json:"created_at"`
}

type organizationWebhookService struct {
	repo                repositories.OrganizationWebhookRepository
	orgRepo             repositories.OrganizationRepository
	cfg                 *config.Config
	jobQueue            queue.Enqueuer
	webhookSyncFallback bool // Redis async without in-process worker: deliver synchronously (no duplicate with inline queue).
}

// NewOrganizationWebhookService constructs OrganizationWebhookService.
func NewOrganizationWebhookService(
	repo repositories.OrganizationWebhookRepository,
	orgRepo repositories.OrganizationRepository,
	cfg *config.Config,
	jobQueue queue.Enqueuer,
	webhookSyncFallback bool,
) OrganizationWebhookService {
	return &organizationWebhookService{
		repo:                repo,
		orgRepo:             orgRepo,
		cfg:                 cfg,
		jobQueue:            jobQueue,
		webhookSyncFallback: webhookSyncFallback,
	}
}

func (s *organizationWebhookService) requireOrgManager(orgID, actorID uuid.UUID) error {
	m, err := s.orgRepo.FindMembership(orgID, actorID)
	if err != nil {
		if errors.Is(err, repositories.ErrOrganizationMembershipNotFound) {
			return ErrInsufficientPrivileges
		}
		return err
	}
	if !rbac.OrgRoleCanManageOrganization(m.Role) {
		return ErrInsufficientPrivileges
	}
	return nil
}

func generateWebhookPlainSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *organizationWebhookService) encryptSecret(plain string) ([]byte, error) {
	if !config.WebhooksEncryptionConfigured(s.cfg) {
		return nil, ErrWebhooksUnavailable
	}
	return authinfra.EncryptAESGCM([]byte(plain), s.cfg.Webhooks.EncryptionKey)
}

func normalizeWebhookEvents(events []string) ([]string, error) {
	if len(events) == 0 {
		return nil, ErrInvalidWebhookEvents
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(events))
	for _, e := range events {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if !IsValidOrganizationWebhookEvent(e) {
			return nil, ErrInvalidWebhookEvents
		}
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, ErrInvalidWebhookEvents
	}
	sort.Strings(out)
	return out, nil
}

func validateWebhookURL(ctx context.Context, cfg *config.Config, raw string) error {
	if err := webhooks.ValidateOutboundWebhookURL(ctx, cfg, raw, webhooks.DefaultLookupHost); err != nil {
		return ErrInvalidWebhookURL
	}
	return nil
}

func (s *organizationWebhookService) CreateOrganizationWebhook(ctx context.Context, orgID uuid.UUID, actorID uuid.UUID, input *CreateOrganizationWebhookInput) (*OrganizationWebhookCreatedDTO, error) {
	if !config.WebhooksEncryptionConfigured(s.cfg) {
		return nil, ErrWebhooksUnavailable
	}
	if input == nil {
		return nil, ErrInvalidWebhookURL
	}
	if err := s.requireOrgManager(orgID, actorID); err != nil {
		return nil, err
	}
	if err := validateWebhookURL(ctx, s.cfg, input.URL); err != nil {
		return nil, err
	}
	ev, err := normalizeWebhookEvents(input.Events)
	if err != nil {
		return nil, err
	}
	plainSecret, err := generateWebhookPlainSecret()
	if err != nil {
		return nil, err
	}
	cipher, err := s.encryptSecret(plainSecret)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	row := &models.OrganizationWebhook{
		OrganizationID:   orgID,
		URL:              strings.TrimSpace(input.URL),
		SecretCiphertext: cipher,
		SecretKeyVersion: 1,
		Events:           pq.StringArray(ev),
		Enabled:          true,
		CreatedByUserID:  actorID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.repo.CreateWebhook(row); err != nil {
		return nil, err
	}
	return &OrganizationWebhookCreatedDTO{
		ID:               row.ID,
		OrganizationID:   row.OrganizationID,
		URL:              row.URL,
		Events:           ev,
		Enabled:          row.Enabled,
		SecretKeyVersion: row.SecretKeyVersion,
		CreatedByUserID:  row.CreatedByUserID,
		CreatedAt:        row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        row.UpdatedAt.UTC().Format(time.RFC3339),
		Secret:           plainSecret,
	}, nil
}

func (s *organizationWebhookService) ListOrganizationWebhooks(_ context.Context, orgID uuid.UUID, actorID uuid.UUID) ([]OrganizationWebhookListDTO, error) {
	if !config.WebhooksEncryptionConfigured(s.cfg) {
		return nil, ErrWebhooksUnavailable
	}
	if err := s.requireOrgManager(orgID, actorID); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListWebhooksByOrganization(orgID)
	if err != nil {
		return nil, err
	}
	out := make([]OrganizationWebhookListDTO, 0, len(rows))
	for i := range rows {
		out = append(out, webhookToListDTO(&rows[i]))
	}
	return out, nil
}

func webhookToListDTO(row *models.OrganizationWebhook) OrganizationWebhookListDTO {
	return OrganizationWebhookListDTO{
		ID:               row.ID,
		OrganizationID:   row.OrganizationID,
		URL:              row.URL,
		Events:           []string(row.Events),
		Enabled:          row.Enabled,
		SecretKeyVersion: row.SecretKeyVersion,
		CreatedByUserID:  row.CreatedByUserID,
		CreatedAt:        row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        row.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (s *organizationWebhookService) UpdateOrganizationWebhook(ctx context.Context, orgID uuid.UUID, webhookID uuid.UUID, actorID uuid.UUID, input *UpdateOrganizationWebhookInput) (*OrganizationWebhookPatchDTO, error) {
	if !config.WebhooksEncryptionConfigured(s.cfg) {
		return nil, ErrWebhooksUnavailable
	}
	if input == nil {
		return nil, ErrNoFieldsToUpdate
	}
	if input.URL == nil && input.Events == nil && input.Enabled == nil && (input.RotateSecret == nil || !*input.RotateSecret) {
		return nil, ErrNoFieldsToUpdate
	}
	if err := s.requireOrgManager(orgID, actorID); err != nil {
		return nil, err
	}
	row, err := s.repo.FindWebhookByOrganizationAndID(orgID, webhookID)
	if err != nil {
		if errors.Is(err, repositories.ErrOrganizationWebhookNotFound) {
			return nil, ErrOrganizationWebhookNotFound
		}
		return nil, err
	}
	if input.URL != nil {
		if err := validateWebhookURL(ctx, s.cfg, *input.URL); err != nil {
			return nil, err
		}
		row.URL = strings.TrimSpace(*input.URL)
	}
	if input.Events != nil {
		ev, err := normalizeWebhookEvents(*input.Events)
		if err != nil {
			return nil, err
		}
		row.Events = pq.StringArray(ev)
	}
	if input.Enabled != nil {
		row.Enabled = *input.Enabled
	}
	var newSecret *string
	if input.RotateSecret != nil && *input.RotateSecret {
		plain, err := generateWebhookPlainSecret()
		if err != nil {
			return nil, err
		}
		cipher, err := s.encryptSecret(plain)
		if err != nil {
			return nil, err
		}
		row.SecretCiphertext = cipher
		row.SecretKeyVersion++
		newSecret = &plain
	}
	row.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateWebhook(row); err != nil {
		if errors.Is(err, repositories.ErrOrganizationWebhookNotFound) {
			return nil, ErrOrganizationWebhookNotFound
		}
		return nil, err
	}
	dto := OrganizationWebhookPatchDTO{
		ID:               row.ID,
		OrganizationID:   row.OrganizationID,
		URL:              row.URL,
		Events:           []string(row.Events),
		Enabled:          row.Enabled,
		SecretKeyVersion: row.SecretKeyVersion,
		CreatedByUserID:  row.CreatedByUserID,
		CreatedAt:        row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        row.UpdatedAt.UTC().Format(time.RFC3339),
		Secret:           newSecret,
	}
	return &dto, nil
}

func (s *organizationWebhookService) DeleteOrganizationWebhook(_ context.Context, orgID uuid.UUID, webhookID uuid.UUID, actorID uuid.UUID) error {
	if !config.WebhooksEncryptionConfigured(s.cfg) {
		return ErrWebhooksUnavailable
	}
	if err := s.requireOrgManager(orgID, actorID); err != nil {
		return err
	}
	if err := s.repo.DeleteWebhook(orgID, webhookID); err != nil {
		if errors.Is(err, repositories.ErrOrganizationWebhookNotFound) {
			return ErrOrganizationWebhookNotFound
		}
		return err
	}
	return nil
}

func (s *organizationWebhookService) ListOrganizationWebhookDeliveries(_ context.Context, orgID uuid.UUID, webhookID uuid.UUID, actorID uuid.UUID, limit int) ([]OrganizationWebhookDeliveryDTO, error) {
	if !config.WebhooksEncryptionConfigured(s.cfg) {
		return nil, ErrWebhooksUnavailable
	}
	if err := s.requireOrgManager(orgID, actorID); err != nil {
		return nil, err
	}
	if _, err := s.repo.FindWebhookByOrganizationAndID(orgID, webhookID); err != nil {
		if errors.Is(err, repositories.ErrOrganizationWebhookNotFound) {
			return nil, ErrOrganizationWebhookNotFound
		}
		return nil, err
	}
	rows, err := s.repo.ListDeliveriesByWebhook(orgID, webhookID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]OrganizationWebhookDeliveryDTO, 0, len(rows))
	for i := range rows {
		out = append(out, deliveryToDTO(&rows[i]))
	}
	return out, nil
}

func deliveryToDTO(d *models.OrganizationWebhookDelivery) OrganizationWebhookDeliveryDTO {
	dto := OrganizationWebhookDeliveryDTO{
		ID:                    d.ID,
		WebhookID:             d.WebhookID,
		EventType:             d.EventType,
		Payload:               json.RawMessage(d.Payload),
		Status:                d.Status,
		Attempts:              d.Attempts,
		ResponseStatus:        d.ResponseStatus,
		ResponseBodyTruncated: d.ResponseBodyTruncated,
		LastError:             d.LastError,
		CreatedAt:             d.CreatedAt.UTC().Format(time.RFC3339),
	}
	if d.NextAttemptAt != nil {
		s := d.NextAttemptAt.UTC().Format(time.RFC3339)
		dto.NextAttemptAt = &s
	}
	if d.DeliveredAt != nil {
		s := d.DeliveredAt.UTC().Format(time.RFC3339)
		dto.DeliveredAt = &s
	}
	return dto
}

func (s *organizationWebhookService) EmitOrganizationWebhookEvent(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]interface{}) {
	if s == nil || s.repo == nil || s.cfg == nil || s.jobQueue == nil {
		return
	}
	if !config.WebhooksEncryptionConfigured(s.cfg) {
		return
	}
	eventType = strings.TrimSpace(eventType)
	if !IsValidOrganizationWebhookEvent(eventType) {
		logger.Log.Warn().Str("event", eventType).Msg("webhook emit: unknown event type skipped")
		return
	}
	hooks, err := s.repo.ListEnabledWebhooksForEvent(orgID, eventType)
	if err != nil {
		logger.Log.Warn().Err(err).Str("org_id", orgID.String()).Msg("webhook emit: list subscriptions failed")
		return
	}
	for i := range hooks {
		s.enqueueOne(ctx, &hooks[i], eventType, payload)
	}
}

func (s *organizationWebhookService) enqueueOne(ctx context.Context, hook *models.OrganizationWebhook, eventType string, payload map[string]interface{}) {
	envelope := map[string]interface{}{
		"event":           eventType,
		"organization_id": hook.OrganizationID.String(),
		"payload":         payload,
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("webhook: marshal envelope failed")
		return
	}
	d := &models.OrganizationWebhookDelivery{
		WebhookID: hook.ID,
		EventType: eventType,
		Payload:   raw,
		Status:    models.OrganizationWebhookDeliveryStatusPending,
	}
	if err := s.repo.CreateDelivery(d); err != nil {
		logger.Log.Warn().Err(err).Msg("webhook: create delivery failed")
		return
	}
	jobPayload, err := json.Marshal(map[string]string{"delivery_id": d.ID.String()})
	if err != nil {
		logger.Log.Warn().Err(err).Msg("webhook: marshal job payload failed")
		return
	}
	if s.webhookSyncFallback {
		if err := queuejobs.HandleWebhookDelivery(ctx, s.repo, s.cfg, d.ID); err != nil {
			logger.Log.Warn().Err(err).Str("delivery_id", d.ID.String()).Msg("webhook: synchronous delivery failed")
		}
		return
	}
	opts := queue.EnqueueOptions{}
	if s.cfg.Queue.MaxAttempts > 0 {
		opts.MaxAttempts = s.cfg.Queue.MaxAttempts
	}
	if err := s.jobQueue.Enqueue(ctx, queuejobs.WebhookDeliver, jobPayload, opts); err != nil {
		logger.Log.Warn().Err(err).Str("delivery_id", d.ID.String()).Msg("webhook: enqueue delivery job failed")
	}
}
