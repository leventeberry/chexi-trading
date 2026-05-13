package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// UserSettings holds per-user preferences (1:1 with users.id).
type UserSettings struct {
	UserID                    uuid.UUID       `gorm:"type:uuid;primaryKey" json:"user_id"`
	Theme                     string          `gorm:"size:32;not null;default:system" json:"theme"`
	NotificationPreferences   json.RawMessage `gorm:"type:jsonb;not null" json:"notification_preferences"`
	MarketingEmailOptIn       bool            `gorm:"not null;default:false" json:"marketing_email_opt_in"`
	SecurityNotificationOptIn bool            `gorm:"not null;default:true" json:"security_notification_opt_in"`
	ExtraSettings             json.RawMessage `gorm:"type:jsonb;not null" json:"extra_settings"`
	CreatedAt                 time.Time       `json:"created_at"`
	UpdatedAt                 time.Time       `json:"updated_at"`
}

// TableName overrides default pluralization.
func (UserSettings) TableName() string {
	return "user_settings"
}
