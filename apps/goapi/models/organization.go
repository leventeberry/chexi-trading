package models

import (
	"time"

	"github.com/google/uuid"
)

// Organization is a tenant/workspace record.
type Organization struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Name            string    `gorm:"size:255;not null" json:"name"`
	Slug            string    `gorm:"size:255;not null" json:"slug"`
	CreatedByUserID uuid.UUID `gorm:"type:uuid;not null;index" json:"created_by_user_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (Organization) TableName() string {
	return "organizations"
}
