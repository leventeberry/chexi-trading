package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"goapi/models"
	"goapi/repositories"
)

// CreateOrganizationNoteInput is the payload for POST .../notes.
type CreateOrganizationNoteInput struct {
	Title string
	Body  string
}

// OrganizationNoteDTO is returned by organization note APIs.
type OrganizationNoteDTO struct {
	ID              uuid.UUID `json:"id"`
	OrganizationID  uuid.UUID `json:"organization_id"`
	Title           string    `json:"title"`
	Body            string    `json:"body"`
	CreatedByUserID uuid.UUID `json:"created_by_user_id"`
	CreatedAt       string    `json:"created_at"`
	UpdatedAt       string    `json:"updated_at"`
}

type organizationNoteService struct {
	noteRepo repositories.OrganizationNoteRepository
	webhooks webhookEmitter
}

// NewOrganizationNoteService constructs OrganizationNoteService.
func NewOrganizationNoteService(noteRepo repositories.OrganizationNoteRepository, webhooks webhookEmitter) OrganizationNoteService {
	return &organizationNoteService{noteRepo: noteRepo, webhooks: webhooks}
}

func (s *organizationNoteService) CreateOrganizationNote(ctx context.Context, orgID uuid.UUID, actorID uuid.UUID, input *CreateOrganizationNoteInput) (*OrganizationNoteDTO, error) {
	if input == nil {
		return nil, ErrInvalidOrganizationNoteBody
	}
	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)
	if title == "" || body == "" {
		return nil, ErrInvalidOrganizationNoteBody
	}

	note := &models.OrganizationNote{
		OrganizationID:  orgID,
		Title:           title,
		Body:            body,
		CreatedByUserID: actorID,
	}
	if err := s.noteRepo.Create(note); err != nil {
		return nil, err
	}
	if s.webhooks != nil {
		s.webhooks.EmitOrganizationWebhookEvent(ctx, orgID, WebhookEventOrganizationNoteCreated, map[string]interface{}{
			"note_id": note.ID.String(),
			"title":   note.Title,
		})
	}
	return noteToDTO(note), nil
}

func (s *organizationNoteService) ListOrganizationNotes(_ context.Context, orgID uuid.UUID) ([]OrganizationNoteDTO, error) {
	rows, err := s.noteRepo.ListByOrganization(orgID)
	if err != nil {
		return nil, err
	}
	out := make([]OrganizationNoteDTO, 0, len(rows))
	for i := range rows {
		out = append(out, *noteToDTO(&rows[i]))
	}
	return out, nil
}

func (s *organizationNoteService) DeleteOrganizationNote(ctx context.Context, orgID uuid.UUID, noteID uuid.UUID) error {
	err := s.noteRepo.DeleteByOrganizationAndID(orgID, noteID)
	if err != nil {
		if errors.Is(err, repositories.ErrOrganizationNoteNotFound) {
			return ErrOrganizationNoteNotFound
		}
		return err
	}
	if s.webhooks != nil {
		s.webhooks.EmitOrganizationWebhookEvent(ctx, orgID, WebhookEventOrganizationNoteDeleted, map[string]interface{}{
			"note_id": noteID.String(),
		})
	}
	return nil
}

func noteToDTO(n *models.OrganizationNote) *OrganizationNoteDTO {
	return &OrganizationNoteDTO{
		ID:              n.ID,
		OrganizationID:  n.OrganizationID,
		Title:           n.Title,
		Body:            n.Body,
		CreatedByUserID: n.CreatedByUserID,
		CreatedAt:       n.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       n.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
