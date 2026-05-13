package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"goapi/internal/rbac"
	"goapi/models"
	"goapi/repositories"
)

const organizationAPIKeyPrefix = "orgk_"

// CreateOrganizationAPIKeyInput is the payload for POST .../api-keys.
type CreateOrganizationAPIKeyInput struct {
	Name      string
	Scopes    []string
	ExpiresAt *time.Time
}

// OrganizationAPIKeyCreatedDTO is returned once on create (includes full secret).
type OrganizationAPIKeyCreatedDTO struct {
	ID              uuid.UUID `json:"id"`
	OrganizationID  uuid.UUID `json:"organization_id"`
	Name            string    `json:"name"`
	KeyPrefix       string    `json:"key_prefix"`
	Scopes          []string  `json:"scopes"`
	CreatedByUserID uuid.UUID `json:"created_by_user_id"`
	CreatedAt       string    `json:"created_at"`
	UpdatedAt       string    `json:"updated_at"`
	ExpiresAt       *string   `json:"expires_at,omitempty"`
	APIKey          string    `json:"api_key"`
}

// OrganizationAPIKeyListDTO is returned by list (no secret or hash).
type OrganizationAPIKeyListDTO struct {
	ID              uuid.UUID `json:"id"`
	OrganizationID  uuid.UUID `json:"organization_id"`
	Name            string    `json:"name"`
	KeyPrefix       string    `json:"key_prefix"`
	Scopes          []string  `json:"scopes"`
	CreatedByUserID uuid.UUID `json:"created_by_user_id"`
	LastUsedAt      *string   `json:"last_used_at,omitempty"`
	RevokedAt       *string   `json:"revoked_at,omitempty"`
	ExpiresAt       *string   `json:"expires_at,omitempty"`
	CreatedAt       string    `json:"created_at"`
	UpdatedAt       string    `json:"updated_at"`
}

// OrganizationAPIKeyPrincipal is the verified API key identity for request context.
type OrganizationAPIKeyPrincipal struct {
	OrganizationID  uuid.UUID
	KeyID           uuid.UUID
	Scopes          []string
	CreatedByUserID uuid.UUID // user who created the key (audit attribution for API-key writes)
}

type organizationAPIKeyService struct {
	keyRepo repositories.OrganizationAPIKeyRepository
	orgRepo repositories.OrganizationRepository
}

// NewOrganizationAPIKeyService constructs OrganizationAPIKeyService.
func NewOrganizationAPIKeyService(keyRepo repositories.OrganizationAPIKeyRepository, orgRepo repositories.OrganizationRepository) OrganizationAPIKeyService {
	return &organizationAPIKeyService{keyRepo: keyRepo, orgRepo: orgRepo}
}

func normalizeOrgAPIScopes(in []string) ([]string, error) {
	if len(in) == 0 {
		return nil, ErrInvalidOrganizationAPIKeyScopes
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !rbac.IsValidOrgAPIScope(s) {
			return nil, ErrInvalidOrganizationAPIKeyScopes
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, ErrInvalidOrganizationAPIKeyScopes
	}
	return out, nil
}

func (s *organizationAPIKeyService) requireOrgManager(orgID, actorID uuid.UUID) error {
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

func generateRawOrganizationAPIKey() (raw string, prefix string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	secret := base64.RawURLEncoding.EncodeToString(b)
	raw = organizationAPIKeyPrefix + secret
	if len(raw) > 16 {
		prefix = raw[:16]
	} else {
		prefix = raw
	}
	return raw, prefix, nil
}

// HashOrganizationAPIKey returns the hex SHA-256 of the full raw key (for storage / lookup).
func HashOrganizationAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *organizationAPIKeyService) CreateOrganizationAPIKey(_ context.Context, orgID uuid.UUID, actorID uuid.UUID, input *CreateOrganizationAPIKeyInput) (*OrganizationAPIKeyCreatedDTO, error) {
	if input == nil {
		return nil, ErrInvalidOrganizationAPIKeyName
	}
	if err := s.requireOrgManager(orgID, actorID); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrInvalidOrganizationAPIKeyName
	}
	scopes, err := normalizeOrgAPIScopes(input.Scopes)
	if err != nil {
		return nil, err
	}
	raw, prefix, err := generateRawOrganizationAPIKey()
	if err != nil {
		return nil, err
	}
	hash := HashOrganizationAPIKey(raw)
	row := &models.OrganizationAPIKey{
		OrganizationID:  orgID,
		Name:            name,
		KeyPrefix:       prefix,
		KeyHash:         hash,
		Scopes:          pq.StringArray(scopes),
		CreatedByUserID: actorID,
		ExpiresAt:       input.ExpiresAt,
	}
	if err := s.keyRepo.Create(row); err != nil {
		return nil, err
	}
	return createdDTOFromModel(row, raw), nil
}

func createdDTOFromModel(row *models.OrganizationAPIKey, rawOnce string) *OrganizationAPIKeyCreatedDTO {
	dto := &OrganizationAPIKeyCreatedDTO{
		ID:              row.ID,
		OrganizationID:  row.OrganizationID,
		Name:            row.Name,
		KeyPrefix:       row.KeyPrefix,
		Scopes:          []string(row.Scopes),
		CreatedByUserID: row.CreatedByUserID,
		CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       row.UpdatedAt.UTC().Format(time.RFC3339),
		APIKey:          rawOnce,
	}
	if row.ExpiresAt != nil {
		s := row.ExpiresAt.UTC().Format(time.RFC3339)
		dto.ExpiresAt = &s
	}
	return dto
}

func (s *organizationAPIKeyService) ListOrganizationAPIKeys(_ context.Context, orgID uuid.UUID, actorID uuid.UUID) ([]OrganizationAPIKeyListDTO, error) {
	if err := s.requireOrgManager(orgID, actorID); err != nil {
		return nil, err
	}
	rows, err := s.keyRepo.ListByOrganization(orgID)
	if err != nil {
		return nil, err
	}
	out := make([]OrganizationAPIKeyListDTO, 0, len(rows))
	for i := range rows {
		out = append(out, listDTOFromModel(&rows[i]))
	}
	return out, nil
}

func listDTOFromModel(row *models.OrganizationAPIKey) OrganizationAPIKeyListDTO {
	dto := OrganizationAPIKeyListDTO{
		ID:              row.ID,
		OrganizationID:  row.OrganizationID,
		Name:            row.Name,
		KeyPrefix:       row.KeyPrefix,
		Scopes:          []string(row.Scopes),
		CreatedByUserID: row.CreatedByUserID,
		CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       row.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if row.LastUsedAt != nil {
		s := row.LastUsedAt.UTC().Format(time.RFC3339)
		dto.LastUsedAt = &s
	}
	if row.RevokedAt != nil {
		s := row.RevokedAt.UTC().Format(time.RFC3339)
		dto.RevokedAt = &s
	}
	if row.ExpiresAt != nil {
		s := row.ExpiresAt.UTC().Format(time.RFC3339)
		dto.ExpiresAt = &s
	}
	return dto
}

func (s *organizationAPIKeyService) RevokeOrganizationAPIKey(_ context.Context, orgID uuid.UUID, keyID uuid.UUID, actorID uuid.UUID) error {
	if err := s.requireOrgManager(orgID, actorID); err != nil {
		return err
	}
	err := s.keyRepo.Revoke(orgID, keyID, time.Now().UTC())
	if err != nil {
		if errors.Is(err, repositories.ErrOrganizationAPIKeyNotFound) {
			return ErrOrganizationAPIKeyNotFound
		}
		return err
	}
	return nil
}

func (s *organizationAPIKeyService) AuthenticateOrganizationAPIKey(_ context.Context, rawKey string) (*OrganizationAPIKeyPrincipal, error) {
	rawKey = strings.TrimSpace(rawKey)
	if rawKey == "" || !strings.HasPrefix(rawKey, organizationAPIKeyPrefix) {
		return nil, ErrInvalidOrganizationAPIKey
	}
	hash := HashOrganizationAPIKey(rawKey)
	row, err := s.keyRepo.FindActiveByKeyHash(hash)
	if err != nil {
		if errors.Is(err, repositories.ErrOrganizationAPIKeyNotFound) {
			return nil, ErrInvalidOrganizationAPIKey
		}
		return nil, err
	}
	// Defensive: repository already filters expiry/revocation.
	principal := &OrganizationAPIKeyPrincipal{
		OrganizationID:  row.OrganizationID,
		KeyID:           row.ID,
		Scopes:          []string(row.Scopes),
		CreatedByUserID: row.CreatedByUserID,
	}
	now := time.Now().UTC()
	go func(id uuid.UUID, at time.Time) {
		_ = s.keyRepo.TouchLastUsed(id, at)
	}(row.ID, now)
	return principal, nil
}
