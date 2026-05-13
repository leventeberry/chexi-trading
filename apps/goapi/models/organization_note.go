package models

import (
	"time"

	"github.com/google/uuid"
)

// OrganizationNote is a minimal org-scoped sub-resource (tenant pattern example).
type OrganizationNote struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID  uuid.UUID `gorm:"type:uuid;not null;index" json:"organization_id"`
	Title           string    `gorm:"size:255;not null" json:"title"`
	Body            string    `gorm:"type:text;not null" json:"body"`
	CreatedByUserID uuid.UUID `gorm:"type:uuid;not null;index" json:"created_by_user_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (OrganizationNote) TableName() string {
	return "organization_notes"
}
