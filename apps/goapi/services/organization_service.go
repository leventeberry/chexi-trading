package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"goapi/config"
	"goapi/internal/email"
	"goapi/internal/queue"
	"goapi/internal/rbac"
	"goapi/models"
	"goapi/repositories"
	"gorm.io/gorm"
)

// CreateOrganizationInput is the payload for POST /organizations.
type CreateOrganizationInput struct {
	Name string
	Slug *string
}

// UpdateOrganizationInput is the payload for PATCH /organizations/:id.
type UpdateOrganizationInput struct {
	Name *string
	Slug *string
}

// OrganizationDTO is returned by organization APIs.
type OrganizationDTO struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Slug            string    `json:"slug"`
	CreatedByUserID uuid.UUID `json:"created_by_user_id"`
	CreatedAt       string    `json:"created_at"`
	UpdatedAt       string    `json:"updated_at"`
}

// OrganizationMemberDTO is one row in GET /organizations/:id/members.
type OrganizationMemberDTO struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	UserID         uuid.UUID `json:"user_id"`
	Role           string    `json:"role"`
	CreatedAt      string    `json:"created_at"`
	UpdatedAt      string    `json:"updated_at"`
}

type organizationService struct {
	db       *gorm.DB
	orgRepo  repositories.OrganizationRepository
	userRepo repositories.UserRepository
	cfg      *config.Config
	mail     email.Sender
	jobQueue queue.Enqueuer
	webhooks webhookEmitter
}

// NewOrganizationService constructs OrganizationService.
func NewOrganizationService(db *gorm.DB, orgRepo repositories.OrganizationRepository, userRepo repositories.UserRepository, cfg *config.Config, mail email.Sender, jobQueue queue.Enqueuer, webhooks webhookEmitter) OrganizationService {
	return &organizationService{
		db:       db,
		orgRepo:  orgRepo,
		userRepo: userRepo,
		cfg:      cfg,
		mail:     mail,
		jobQueue: jobQueue,
		webhooks: webhooks,
	}
}

func (s *organizationService) emitOrgWebhook(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]interface{}) {
	if s == nil || s.webhooks == nil {
		return
	}
	s.webhooks.EmitOrganizationWebhookEvent(ctx, orgID, eventType, payload)
}

func (s *organizationService) ResolveOrganizationRouteID(_ context.Context, raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.Nil, ErrInvalidOrganizationSlug
	}
	if id, err := uuid.Parse(raw); err == nil {
		return id, nil
	}
	org, err := s.orgRepo.FindBySlugLower(raw)
	if err != nil {
		if errors.Is(err, repositories.ErrOrganizationNotFound) {
			return uuid.Nil, ErrOrganizationNotFound
		}
		return uuid.Nil, err
	}
	return org.ID, nil
}

func (s *organizationService) CreateOrganization(_ context.Context, actorID uuid.UUID, input *CreateOrganizationInput) (*OrganizationDTO, error) {
	if input == nil {
		return nil, ErrInvalidOrganizationName
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrInvalidOrganizationName
	}

	slug, err := s.resolveCreateSlug(name, input.Slug)
	if err != nil {
		return nil, err
	}

	org := &models.Organization{
		Name:            name,
		Slug:            slug,
		CreatedByUserID: actorID,
	}
	membership := &models.OrganizationMembership{
		UserID: actorID,
		Role:   rbac.OrgRoleOwner,
	}

	if err := s.orgRepo.CreateWithOwnerMembership(org, membership); err != nil {
		if isLikelyUniqueViolation(err) {
			return nil, ErrOrganizationSlugExists
		}
		return nil, err
	}

	return orgToDTO(org), nil
}

func (s *organizationService) ListOrganizations(_ context.Context, actorID uuid.UUID) ([]OrganizationDTO, error) {
	orgs, err := s.orgRepo.ListByUserID(actorID)
	if err != nil {
		return nil, err
	}
	out := make([]OrganizationDTO, 0, len(orgs))
	for i := range orgs {
		out = append(out, *orgToDTO(&orgs[i]))
	}
	return out, nil
}

func (s *organizationService) GetOrganization(_ context.Context, id uuid.UUID, actorID uuid.UUID) (*OrganizationDTO, error) {
	org, err := s.orgRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repositories.ErrOrganizationNotFound) {
			return nil, ErrOrganizationNotFound
		}
		return nil, err
	}

	if _, err := s.orgRepo.FindMembership(id, actorID); err != nil {
		if errors.Is(err, repositories.ErrOrganizationMembershipNotFound) {
			return nil, ErrInsufficientPrivileges
		}
		return nil, err
	}

	return orgToDTO(org), nil
}

func (s *organizationService) UpdateOrganization(ctx context.Context, id uuid.UUID, actorID uuid.UUID, input *UpdateOrganizationInput) (*OrganizationDTO, error) {
	if input == nil || (input.Name == nil && input.Slug == nil) {
		return nil, ErrNoFieldsToUpdate
	}

	org, err := s.orgRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repositories.ErrOrganizationNotFound) {
			return nil, ErrOrganizationNotFound
		}
		return nil, err
	}

	m, err := s.orgRepo.FindMembership(id, actorID)
	if err != nil {
		if errors.Is(err, repositories.ErrOrganizationMembershipNotFound) {
			return nil, ErrInsufficientPrivileges
		}
		return nil, err
	}
	if !rbac.OrgRoleCanManageOrganization(m.Role) {
		return nil, ErrInsufficientPrivileges
	}

	if input.Name != nil {
		n := strings.TrimSpace(*input.Name)
		if n == "" {
			return nil, ErrInvalidOrganizationName
		}
		org.Name = n
	}

	if input.Slug != nil {
		raw := strings.TrimSpace(*input.Slug)
		normalized, err := NormalizeOrganizationSlug(raw)
		if err != nil {
			return nil, err
		}
		if normalized != org.Slug {
			taken, err := s.orgRepo.ExistsSlugLowerForOtherOrg(normalized, org.ID)
			if err != nil {
				return nil, err
			}
			if taken {
				return nil, ErrOrganizationSlugExists
			}
			org.Slug = normalized
		}
	}

	if err := s.orgRepo.Update(org); err != nil {
		if isLikelyUniqueViolation(err) {
			return nil, ErrOrganizationSlugExists
		}
		return nil, err
	}

	out := orgToDTO(org)
	s.emitOrgWebhook(ctx, id, WebhookEventOrganizationUpdated, map[string]interface{}{
		"organization_id": id.String(),
		"name":            out.Name,
		"slug":            out.Slug,
	})
	return out, nil
}

func (s *organizationService) ListOrganizationMembers(_ context.Context, id uuid.UUID, actorID uuid.UUID) ([]OrganizationMemberDTO, error) {
	if _, err := s.orgRepo.FindByID(id); err != nil {
		if errors.Is(err, repositories.ErrOrganizationNotFound) {
			return nil, ErrOrganizationNotFound
		}
		return nil, err
	}

	if _, err := s.orgRepo.FindMembership(id, actorID); err != nil {
		if errors.Is(err, repositories.ErrOrganizationMembershipNotFound) {
			return nil, ErrInsufficientPrivileges
		}
		return nil, err
	}

	rows, err := s.orgRepo.ListMembershipsForOrg(id)
	if err != nil {
		return nil, err
	}
	out := make([]OrganizationMemberDTO, 0, len(rows))
	for i := range rows {
		out = append(out, membershipToDTO(&rows[i]))
	}
	return out, nil
}

func (s *organizationService) resolveCreateSlug(name string, slugInput *string) (string, error) {
	if slugInput != nil && strings.TrimSpace(*slugInput) != "" {
		normalized, err := NormalizeOrganizationSlug(strings.TrimSpace(*slugInput))
		if err != nil {
			return "", err
		}
		taken, err := s.orgRepo.ExistsSlugLower(normalized)
		if err != nil {
			return "", err
		}
		if taken {
			return "", ErrOrganizationSlugExists
		}
		return normalized, nil
	}

	base := GenerateOrganizationSlugFromName(name)
	candidate := base
	for range 32 {
		taken, err := s.orgRepo.ExistsSlugLower(candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
		candidate = appendSlugSuffix(base, uuid.New().String()[:8])
	}
	return "", ErrOrganizationSlugExists
}

func orgToDTO(o *models.Organization) *OrganizationDTO {
	return &OrganizationDTO{
		ID:              o.ID,
		Name:            o.Name,
		Slug:            o.Slug,
		CreatedByUserID: o.CreatedByUserID,
		CreatedAt:       o.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       o.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func membershipToDTO(m *models.OrganizationMembership) OrganizationMemberDTO {
	return OrganizationMemberDTO{
		OrganizationID: m.OrganizationID,
		UserID:         m.UserID,
		Role:           m.Role,
		CreatedAt:      m.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      m.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func isLikelyUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "duplicate") ||
		strings.Contains(s, "unique constraint") ||
		strings.Contains(s, "violates unique constraint")
}
