package repositories

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"goapi/models"
	"gorm.io/gorm"
)

type organizationAPIKeyRepository struct {
	db *gorm.DB
}

// NewOrganizationAPIKeyRepository constructs OrganizationAPIKeyRepository.
func NewOrganizationAPIKeyRepository(db *gorm.DB) OrganizationAPIKeyRepository {
	return &organizationAPIKeyRepository{db: db}
}

func (r *organizationAPIKeyRepository) Create(row *models.OrganizationAPIKey) error {
	if row.ID == uuid.Nil {
		row.ID = uuid.New()
	}
	if err := r.db.Create(row).Error; err != nil {
		return fmt.Errorf("create organization api key: %w", err)
	}
	return nil
}

func (r *organizationAPIKeyRepository) ListByOrganization(orgID uuid.UUID) ([]models.OrganizationAPIKey, error) {
	var rows []models.OrganizationAPIKey
	err := r.db.Where("organization_id = ?", orgID).Order("created_at DESC").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list organization api keys: %w", err)
	}
	return rows, nil
}

func (r *organizationAPIKeyRepository) FindActiveByKeyHash(keyHash string) (*models.OrganizationAPIKey, error) {
	var row models.OrganizationAPIKey
	now := time.Now().UTC()
	err := r.db.Where("key_hash = ? AND revoked_at IS NULL", keyHash).
		Where("expires_at IS NULL OR expires_at > ?", now).
		First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrOrganizationAPIKeyNotFound
		}
		return nil, fmt.Errorf("find organization api key: %w", err)
	}
	return &row, nil
}

func (r *organizationAPIKeyRepository) Revoke(orgID, keyID uuid.UUID, at time.Time) error {
	res := r.db.Model(&models.OrganizationAPIKey{}).
		Where("organization_id = ? AND id = ? AND revoked_at IS NULL", orgID, keyID).
		Updates(map[string]interface{}{
			"revoked_at": at,
			"updated_at": at,
		})
	if res.Error != nil {
		return fmt.Errorf("revoke organization api key: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrOrganizationAPIKeyNotFound
	}
	return nil
}

func (r *organizationAPIKeyRepository) TouchLastUsed(id uuid.UUID, at time.Time) error {
	res := r.db.Model(&models.OrganizationAPIKey{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("last_used_at", at)
	if res.Error != nil {
		return fmt.Errorf("touch organization api key last_used_at: %w", res.Error)
	}
	return nil
}
