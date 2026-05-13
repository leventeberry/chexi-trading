package repositories

import (
	"fmt"

	"github.com/google/uuid"
	"goapi/models"
	"gorm.io/gorm"
)

type organizationWebhookRepository struct {
	db *gorm.DB
}

// NewOrganizationWebhookRepository constructs OrganizationWebhookRepository.
func NewOrganizationWebhookRepository(db *gorm.DB) OrganizationWebhookRepository {
	return &organizationWebhookRepository{db: db}
}

func (r *organizationWebhookRepository) CreateWebhook(row *models.OrganizationWebhook) error {
	if row.ID == uuid.Nil {
		row.ID = uuid.New()
	}
	if err := r.db.Create(row).Error; err != nil {
		return fmt.Errorf("create organization webhook: %w", err)
	}
	return nil
}

func (r *organizationWebhookRepository) UpdateWebhook(row *models.OrganizationWebhook) error {
	res := r.db.Model(&models.OrganizationWebhook{}).
		Where("id = ? AND organization_id = ?", row.ID, row.OrganizationID).
		Updates(map[string]interface{}{
			"url":                row.URL,
			"secret_ciphertext":  row.SecretCiphertext,
			"secret_key_version": row.SecretKeyVersion,
			"events":             row.Events,
			"enabled":            row.Enabled,
			"updated_at":         row.UpdatedAt,
		})
	if res.Error != nil {
		return fmt.Errorf("update organization webhook: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrOrganizationWebhookNotFound
	}
	return nil
}

func (r *organizationWebhookRepository) DeleteWebhook(orgID, webhookID uuid.UUID) error {
	res := r.db.Where("organization_id = ? AND id = ?", orgID, webhookID).Delete(&models.OrganizationWebhook{})
	if res.Error != nil {
		return fmt.Errorf("delete organization webhook: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrOrganizationWebhookNotFound
	}
	return nil
}

func (r *organizationWebhookRepository) ListWebhooksByOrganization(orgID uuid.UUID) ([]models.OrganizationWebhook, error) {
	var rows []models.OrganizationWebhook
	err := r.db.Where("organization_id = ?", orgID).Order("created_at DESC").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list organization webhooks: %w", err)
	}
	return rows, nil
}

func (r *organizationWebhookRepository) FindWebhookByOrganizationAndID(orgID, webhookID uuid.UUID) (*models.OrganizationWebhook, error) {
	var row models.OrganizationWebhook
	err := r.db.Where("organization_id = ? AND id = ?", orgID, webhookID).First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrOrganizationWebhookNotFound
		}
		return nil, fmt.Errorf("find organization webhook: %w", err)
	}
	return &row, nil
}

func (r *organizationWebhookRepository) FindWebhookByID(webhookID uuid.UUID) (*models.OrganizationWebhook, error) {
	var row models.OrganizationWebhook
	err := r.db.Where("id = ?", webhookID).First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrOrganizationWebhookNotFound
		}
		return nil, fmt.Errorf("find organization webhook by id: %w", err)
	}
	return &row, nil
}

func (r *organizationWebhookRepository) ListEnabledWebhooksForEvent(orgID uuid.UUID, eventType string) ([]models.OrganizationWebhook, error) {
	var rows []models.OrganizationWebhook
	err := r.db.Where("organization_id = ? AND enabled = ? AND ? = ANY(events)", orgID, true, eventType).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list webhooks for event: %w", err)
	}
	return rows, nil
}

func (r *organizationWebhookRepository) CreateDelivery(row *models.OrganizationWebhookDelivery) error {
	if row.ID == uuid.Nil {
		row.ID = uuid.New()
	}
	if err := r.db.Create(row).Error; err != nil {
		return fmt.Errorf("create webhook delivery: %w", err)
	}
	return nil
}

func (r *organizationWebhookRepository) UpdateDelivery(row *models.OrganizationWebhookDelivery) error {
	if err := r.db.Save(row).Error; err != nil {
		return fmt.Errorf("update webhook delivery: %w", err)
	}
	return nil
}

func (r *organizationWebhookRepository) FindDeliveryByID(id uuid.UUID) (*models.OrganizationWebhookDelivery, error) {
	var row models.OrganizationWebhookDelivery
	err := r.db.Where("id = ?", id).First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrOrganizationWebhookDeliveryNotFound
		}
		return nil, fmt.Errorf("find webhook delivery: %w", err)
	}
	return &row, nil
}

func (r *organizationWebhookRepository) ListDeliveriesByWebhook(orgID, webhookID uuid.UUID, limit int) ([]models.OrganizationWebhookDelivery, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var rows []models.OrganizationWebhookDelivery
	err := r.db.Table("organization_webhook_deliveries AS d").
		Select("d.*").
		Joins("INNER JOIN organization_webhooks w ON w.id = d.webhook_id").
		Where("w.organization_id = ? AND d.webhook_id = ?", orgID, webhookID).
		Order("d.created_at DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list webhook deliveries: %w", err)
	}
	return rows, nil
}
