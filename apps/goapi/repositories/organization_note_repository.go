package repositories

import (
	"fmt"

	"github.com/google/uuid"
	"goapi/models"
	"gorm.io/gorm"
)

type organizationNoteRepository struct {
	db *gorm.DB
}

// NewOrganizationNoteRepository constructs OrganizationNoteRepository.
func NewOrganizationNoteRepository(db *gorm.DB) OrganizationNoteRepository {
	return &organizationNoteRepository{db: db}
}

func (r *organizationNoteRepository) Create(note *models.OrganizationNote) error {
	if note.ID == uuid.Nil {
		note.ID = uuid.New()
	}
	if err := r.db.Create(note).Error; err != nil {
		return fmt.Errorf("create organization note: %w", err)
	}
	return nil
}

func (r *organizationNoteRepository) ListByOrganization(orgID uuid.UUID) ([]models.OrganizationNote, error) {
	var rows []models.OrganizationNote
	err := r.db.Where("organization_id = ?", orgID).Order("created_at DESC").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list organization notes: %w", err)
	}
	return rows, nil
}

func (r *organizationNoteRepository) DeleteByOrganizationAndID(orgID, noteID uuid.UUID) error {
	res := r.db.Where("organization_id = ? AND id = ?", orgID, noteID).Delete(&models.OrganizationNote{})
	if res.Error != nil {
		return fmt.Errorf("delete organization note: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrOrganizationNoteNotFound
	}
	return nil
}
