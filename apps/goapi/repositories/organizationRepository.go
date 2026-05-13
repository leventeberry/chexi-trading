package repositories

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"goapi/models"
	"gorm.io/gorm"
)

type organizationRepository struct {
	db *gorm.DB
}

// NewOrganizationRepository constructs an OrganizationRepository.
func NewOrganizationRepository(db *gorm.DB) OrganizationRepository {
	return &organizationRepository{db: db}
}

func (r *organizationRepository) CreateWithOwnerMembership(org *models.Organization, membership *models.OrganizationMembership) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if org.ID == uuid.Nil {
			org.ID = uuid.New()
		}
		if err := tx.Create(org).Error; err != nil {
			return fmt.Errorf("create organization: %w", err)
		}
		membership.OrganizationID = org.ID
		if membership.UserID == uuid.Nil {
			return fmt.Errorf("membership user id required")
		}
		if err := tx.Create(membership).Error; err != nil {
			return fmt.Errorf("create owner membership: %w", err)
		}
		return nil
	})
}

func (r *organizationRepository) FindByID(id uuid.UUID) (*models.Organization, error) {
	var org models.Organization
	err := r.db.First(&org, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrganizationNotFound
		}
		return nil, fmt.Errorf("find organization: %w", err)
	}
	return &org, nil
}

func (r *organizationRepository) FindBySlugLower(slug string) (*models.Organization, error) {
	s := strings.TrimSpace(slug)
	if s == "" {
		return nil, ErrOrganizationNotFound
	}
	var org models.Organization
	err := r.db.Where("LOWER(slug) = LOWER(?)", s).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrganizationNotFound
		}
		return nil, fmt.Errorf("find organization by slug: %w", err)
	}
	return &org, nil
}

func (r *organizationRepository) ExistsSlugLower(slug string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Organization{}).
		Where("LOWER(slug) = LOWER(?)", slug).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("slug existence check: %w", err)
	}
	return count > 0, nil
}

func (r *organizationRepository) ExistsSlugLowerForOtherOrg(slug string, excludeOrgID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Model(&models.Organization{}).
		Where("LOWER(slug) = LOWER(?) AND id <> ?", slug, excludeOrgID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("slug existence check: %w", err)
	}
	return count > 0, nil
}

func (r *organizationRepository) ListByUserID(userID uuid.UUID) ([]models.Organization, error) {
	var orgs []models.Organization
	err := r.db.Model(&models.Organization{}).
		Joins("JOIN organization_memberships om ON om.organization_id = organizations.id").
		Where("om.user_id = ?", userID).
		Order("organizations.name ASC").
		Find(&orgs).Error
	if err != nil {
		return nil, fmt.Errorf("list organizations for user: %w", err)
	}
	return orgs, nil
}

func (r *organizationRepository) Update(org *models.Organization) error {
	if err := r.db.Save(org).Error; err != nil {
		return fmt.Errorf("update organization: %w", err)
	}
	return nil
}

func (r *organizationRepository) FindMembership(orgID, userID uuid.UUID) (*models.OrganizationMembership, error) {
	var m models.OrganizationMembership
	err := r.db.Where("organization_id = ? AND user_id = ?", orgID, userID).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrganizationMembershipNotFound
		}
		return nil, fmt.Errorf("find membership: %w", err)
	}
	return &m, nil
}

func (r *organizationRepository) ListMembershipsForOrg(orgID uuid.UUID) ([]models.OrganizationMembership, error) {
	var rows []models.OrganizationMembership
	err := r.db.Where("organization_id = ?", orgID).
		Order("CASE WHEN role = 'owner' THEN 0 WHEN role = 'admin' THEN 1 ELSE 2 END, user_id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}
	return rows, nil
}

func normalizeInviteEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (r *organizationRepository) DeletePendingInvitationsForEmail(orgID uuid.UUID, email string) error {
	e := normalizeInviteEmail(email)
	res := r.db.Where("organization_id = ? AND LOWER(email) = ? AND accepted_at IS NULL", orgID, e).
		Delete(&models.OrganizationInvitation{})
	if res.Error != nil {
		return fmt.Errorf("delete pending invitations: %w", res.Error)
	}
	return nil
}

func (r *organizationRepository) CreateInvitation(inv *models.OrganizationInvitation) error {
	if inv.ID == uuid.Nil {
		inv.ID = uuid.New()
	}
	inv.Email = normalizeInviteEmail(inv.Email)
	if err := r.db.Create(inv).Error; err != nil {
		return fmt.Errorf("create invitation: %w", err)
	}
	return nil
}

func (r *organizationRepository) FindInvitationByTokenHash(tokenHash string) (*models.OrganizationInvitation, error) {
	var inv models.OrganizationInvitation
	err := r.db.Where("token_hash = ?", tokenHash).First(&inv).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrganizationInvitationNotFound
		}
		return nil, fmt.Errorf("find invitation: %w", err)
	}
	return &inv, nil
}

func (r *organizationRepository) ListInvitationsByOrg(orgID uuid.UUID) ([]models.OrganizationInvitation, error) {
	var rows []models.OrganizationInvitation
	err := r.db.Where("organization_id = ?", orgID).Order("created_at DESC").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	return rows, nil
}

func (r *organizationRepository) MarkInvitationAccepted(id uuid.UUID, at time.Time) error {
	res := r.db.Model(&models.OrganizationInvitation{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"accepted_at": at,
			"updated_at":  at,
		})
	if res.Error != nil {
		return fmt.Errorf("mark invitation accepted: %w", res.Error)
	}
	return nil
}

func (r *organizationRepository) DeleteMembership(orgID, userID uuid.UUID) error {
	res := r.db.Where("organization_id = ? AND user_id = ?", orgID, userID).Delete(&models.OrganizationMembership{})
	if res.Error != nil {
		return fmt.Errorf("delete membership: %w", res.Error)
	}
	return nil
}

func (r *organizationRepository) CountMembersWithRole(orgID uuid.UUID, role string) (int64, error) {
	var n int64
	err := r.db.Model(&models.OrganizationMembership{}).
		Where("organization_id = ? AND role = ?", orgID, role).
		Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count members by role: %w", err)
	}
	return n, nil
}
