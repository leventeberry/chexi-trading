package repositories

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"goapi/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userSettingsRepository struct {
	db *gorm.DB
}

// NewUserSettingsRepository constructs UserSettingsRepository.
func NewUserSettingsRepository(db *gorm.DB) UserSettingsRepository {
	return &userSettingsRepository{db: db}
}

func (r *userSettingsRepository) FindByUserID(userID uuid.UUID) (*models.UserSettings, error) {
	var s models.UserSettings
	err := r.db.Where("user_id = ?", userID).First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserSettingsNotFound
		}
		return nil, fmt.Errorf("find user settings: %w", err)
	}
	return &s, nil
}

// Upsert inserts or updates settings by primary key user_id.
func (r *userSettingsRepository) Upsert(settings *models.UserSettings) error {
	if settings.NotificationPreferences == nil {
		settings.NotificationPreferences = []byte(`{}`)
	}
	if settings.ExtraSettings == nil {
		settings.ExtraSettings = []byte(`{}`)
	}
	if settings.Theme == "" {
		settings.Theme = "system"
	}

	err := r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"theme",
			"notification_preferences",
			"marketing_email_opt_in",
			"security_notification_opt_in",
			"extra_settings",
			"updated_at",
		}),
	}).Create(settings).Error
	if err != nil {
		return fmt.Errorf("upsert user settings: %w", err)
	}
	return nil
}
