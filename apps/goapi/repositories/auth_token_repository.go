package repositories

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"goapi/models"
	"gorm.io/gorm"
)

// AuthTokenRepository persists hashed email verification and password reset tokens.
type AuthTokenRepository interface {
	CreateEmailVerification(tx *gorm.DB, token *models.EmailVerificationToken) error
	DeleteUnusedEmailVerificationsForUser(tx *gorm.DB, userID uuid.UUID) error
	FindActiveEmailVerificationByHash(tx *gorm.DB, hash string) (*models.EmailVerificationToken, error)
	MarkEmailVerificationUsed(tx *gorm.DB, id uuid.UUID, usedAt time.Time) error
	LatestEmailVerificationCreatedAt(userID uuid.UUID) (time.Time, bool, error)

	CreatePasswordReset(tx *gorm.DB, token *models.PasswordResetToken) error
	DeleteUnusedPasswordResetsForUser(tx *gorm.DB, userID uuid.UUID) error
	FindActivePasswordResetByHash(tx *gorm.DB, hash string) (*models.PasswordResetToken, error)
	MarkPasswordResetUsed(tx *gorm.DB, id uuid.UUID, usedAt time.Time) error
	LatestPasswordResetCreatedAt(userID uuid.UUID) (time.Time, bool, error)
}

type authTokenRepository struct {
	db *gorm.DB
}

// NewAuthTokenRepository constructs AuthTokenRepository.
func NewAuthTokenRepository(db *gorm.DB) AuthTokenRepository {
	return &authTokenRepository{db: db}
}

func (r *authTokenRepository) dbTx(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.db
}

func (r *authTokenRepository) CreateEmailVerification(tx *gorm.DB, token *models.EmailVerificationToken) error {
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	if err := r.dbTx(tx).Create(token).Error; err != nil {
		return fmt.Errorf("create email verification token: %w", err)
	}
	return nil
}

func (r *authTokenRepository) DeleteUnusedEmailVerificationsForUser(tx *gorm.DB, userID uuid.UUID) error {
	res := r.dbTx(tx).Where("user_id = ? AND used_at IS NULL", userID).Delete(&models.EmailVerificationToken{})
	if res.Error != nil {
		return fmt.Errorf("delete unused verification tokens: %w", res.Error)
	}
	return nil
}

func (r *authTokenRepository) FindActiveEmailVerificationByHash(tx *gorm.DB, hash string) (*models.EmailVerificationToken, error) {
	var t models.EmailVerificationToken
	err := r.dbTx(tx).Where("token_hash = ? AND used_at IS NULL", hash).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *authTokenRepository) MarkEmailVerificationUsed(tx *gorm.DB, id uuid.UUID, usedAt time.Time) error {
	res := r.dbTx(tx).Model(&models.EmailVerificationToken{}).
		Where("id = ? AND used_at IS NULL", id).
		Updates(map[string]interface{}{
			"used_at": usedAt,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrTokenNotFound
	}
	return nil
}

func (r *authTokenRepository) LatestEmailVerificationCreatedAt(userID uuid.UUID) (time.Time, bool, error) {
	var t models.EmailVerificationToken
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return t.CreatedAt, true, nil
}

func (r *authTokenRepository) CreatePasswordReset(tx *gorm.DB, token *models.PasswordResetToken) error {
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	if err := r.dbTx(tx).Create(token).Error; err != nil {
		return fmt.Errorf("create password reset token: %w", err)
	}
	return nil
}

func (r *authTokenRepository) DeleteUnusedPasswordResetsForUser(tx *gorm.DB, userID uuid.UUID) error {
	res := r.dbTx(tx).Where("user_id = ? AND used_at IS NULL", userID).Delete(&models.PasswordResetToken{})
	if res.Error != nil {
		return fmt.Errorf("delete unused password reset tokens: %w", res.Error)
	}
	return nil
}

func (r *authTokenRepository) FindActivePasswordResetByHash(tx *gorm.DB, hash string) (*models.PasswordResetToken, error) {
	var t models.PasswordResetToken
	err := r.dbTx(tx).Where("token_hash = ? AND used_at IS NULL", hash).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *authTokenRepository) MarkPasswordResetUsed(tx *gorm.DB, id uuid.UUID, usedAt time.Time) error {
	res := r.dbTx(tx).Model(&models.PasswordResetToken{}).
		Where("id = ? AND used_at IS NULL", id).
		Updates(map[string]interface{}{
			"used_at": usedAt,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrTokenNotFound
	}
	return nil
}

func (r *authTokenRepository) LatestPasswordResetCreatedAt(userID uuid.UUID) (time.Time, bool, error) {
	var t models.PasswordResetToken
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return t.CreatedAt, true, nil
}
