package services

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	authinfra "goapi/internal/infra/auth"
	"goapi/internal/queue"
	queuejobs "goapi/internal/queue/jobs"
	"goapi/logger"
	"goapi/models"
	"goapi/repositories"
	"gorm.io/gorm"
)

// VerifyEmail validates a single-use verification token and marks the user verified.
func (s *authService) VerifyEmail(ctx context.Context, rawToken string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" || s.tokenRepo == nil || s.db == nil {
		return ErrInvalidVerificationToken
	}
	hash := tokenHash(rawToken)

	var verifiedUID uuid.UUID
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		vt, err := s.tokenRepo.FindActiveEmailVerificationByHash(tx, hash)
		if err != nil {
			if err == repositories.ErrTokenNotFound {
				return ErrInvalidVerificationToken
			}
			return err
		}
		now := time.Now().UTC()
		if now.After(vt.ExpiresAt) {
			return ErrInvalidVerificationToken
		}
		var u models.User
		if err := tx.First(&u, "id = ?", vt.UserID).Error; err != nil {
			return ErrInvalidVerificationToken
		}
		verifiedUID = vt.UserID
		if u.EmailVerifiedAt == nil {
			if err := tx.Model(&models.User{}).Where("id = ?", u.ID).Updates(map[string]interface{}{
				"email_verified_at": now,
				"updated_at":        now,
			}).Error; err != nil {
				return err
			}
		}
		return s.tokenRepo.MarkEmailVerificationUsed(tx, vt.ID, now)
	})
	if err != nil {
		return err
	}
	s.invalidateUserCache(ctx, verifiedUID)
	return nil
}

// ResendVerificationEmail issues a new verification token if the email exists and throttle allows.
func (s *authService) ResendVerificationEmail(ctx context.Context, emailAddr string) error {
	if s.tokenRepo == nil || s.db == nil || s.jobQueue == nil || s.cfg == nil {
		return nil
	}
	emailAddr = strings.TrimSpace(emailAddr)
	if emailAddr == "" || !strings.Contains(emailAddr, "@") {
		return nil
	}
	user, err := s.userRepo.FindByEmail(emailAddr)
	if err != nil {
		return nil
	}
	if user.EmailVerifiedAt != nil {
		return nil
	}
	minGap := time.Duration(s.cfg.Email.ResendMinIntervalSeconds) * time.Second
	if minGap < time.Second {
		minGap = time.Second
	}
	if last, ok, err := s.tokenRepo.LatestEmailVerificationCreatedAt(user.ID); err == nil && ok {
		if time.Since(last) < minGap {
			return nil
		}
	}
	rawToken, err := newOpaqueToken()
	if err != nil {
		logger.Log.Warn().Err(err).Msg("resend verification: token generation failed")
		return nil
	}
	hash := tokenHash(rawToken)
	ttlHours := s.cfg.Email.VerificationTTLHours
	if ttlHours < 1 {
		ttlHours = 48
	}
	expires := time.Now().UTC().Add(time.Duration(ttlHours) * time.Hour)

	rec := &models.EmailVerificationToken{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: expires,
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.tokenRepo.DeleteUnusedEmailVerificationsForUser(tx, user.ID); err != nil {
			return err
		}
		return s.tokenRepo.CreateEmailVerification(tx, rec)
	})
	if err != nil {
		logger.Log.Warn().Err(err).Msg("resend verification persistence failed")
		return nil
	}
	s.enqueueVerificationEmail(ctx, user.Email, rawToken)
	return nil
}

// RequestPasswordReset creates a reset token when the user exists (caller returns generic HTTP body either way).
func (s *authService) RequestPasswordReset(ctx context.Context, emailAddr string) error {
	if s.tokenRepo == nil || s.db == nil || s.jobQueue == nil || s.cfg == nil {
		return nil
	}
	emailAddr = strings.TrimSpace(emailAddr)
	if emailAddr == "" || !strings.Contains(emailAddr, "@") {
		return nil
	}
	user, err := s.userRepo.FindByEmail(emailAddr)
	if err != nil {
		return nil
	}
	minGap := time.Duration(s.cfg.Email.ResendMinIntervalSeconds) * time.Second
	if minGap < time.Second {
		minGap = time.Second
	}
	if last, ok, err := s.tokenRepo.LatestPasswordResetCreatedAt(user.ID); err == nil && ok {
		if time.Since(last) < minGap {
			return nil
		}
	}
	rawToken, err := newOpaqueToken()
	if err != nil {
		logger.Log.Warn().Err(err).Msg("password reset: token generation failed")
		return nil
	}
	hash := tokenHash(rawToken)
	ttlHours := s.cfg.Email.PasswordResetTTLHours
	if ttlHours < 1 {
		ttlHours = 1
	}
	expires := time.Now().UTC().Add(time.Duration(ttlHours) * time.Hour)

	rec := &models.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: expires,
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.tokenRepo.DeleteUnusedPasswordResetsForUser(tx, user.ID); err != nil {
			return err
		}
		return s.tokenRepo.CreatePasswordReset(tx, rec)
	})
	if err != nil {
		logger.Log.Warn().Err(err).Msg("password reset request persistence failed")
		return nil
	}
	s.enqueuePasswordResetEmail(ctx, user.Email, rawToken)
	return nil
}

// ConfirmPasswordReset consumes a reset token and rotates the password; revokes all refresh sessions.
func (s *authService) ConfirmPasswordReset(ctx context.Context, rawToken, newPassword string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" || s.tokenRepo == nil || s.db == nil {
		return ErrInvalidPasswordResetToken
	}
	if err := ValidatePasswordStrength(newPassword); err != nil {
		return err
	}
	hash := tokenHash(rawToken)

	var resetUID uuid.UUID
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rt, err := s.tokenRepo.FindActivePasswordResetByHash(tx, hash)
		if err != nil {
			if err == repositories.ErrTokenNotFound {
				return ErrInvalidPasswordResetToken
			}
			return err
		}
		now := time.Now().UTC()
		if now.After(rt.ExpiresAt) {
			return ErrInvalidPasswordResetToken
		}
		resetUID = rt.UserID
		passHash, err := authinfra.HashPassword(newPassword)
		if err != nil {
			return ErrPasswordHashing
		}
		if err := tx.Model(&models.User{}).Where("id = ?", rt.UserID).Updates(map[string]interface{}{
			"pass_hash":  passHash,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
		if err := s.revokeAllUserSessionsTx(tx, rt.UserID, "password_reset"); err != nil {
			return err
		}
		return s.tokenRepo.MarkPasswordResetUsed(tx, rt.ID, now)
	})
	if err != nil {
		return err
	}
	s.invalidateUserCache(ctx, resetUID)
	return nil
}

func (s *authService) issueEmailVerification(ctx context.Context, userID uuid.UUID, emailAddr string) {
	if s.tokenRepo == nil || s.db == nil || s.jobQueue == nil || s.cfg == nil {
		return
	}
	rawToken, err := newOpaqueToken()
	if err != nil {
		logger.Log.Warn().Err(err).Msg("verification token generation failed after registration")
		return
	}
	tokHash := tokenHash(rawToken)
	ttlHours := s.cfg.Email.VerificationTTLHours
	if ttlHours < 1 {
		ttlHours = 48
	}
	expires := time.Now().UTC().Add(time.Duration(ttlHours) * time.Hour)

	rec := &models.EmailVerificationToken{
		UserID:    userID,
		TokenHash: tokHash,
		ExpiresAt: expires,
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.tokenRepo.DeleteUnusedEmailVerificationsForUser(tx, userID); err != nil {
			return err
		}
		return s.tokenRepo.CreateEmailVerification(tx, rec)
	})
	if err != nil {
		logger.Log.Warn().Err(err).Msg("email verification issuance failed after registration")
		return
	}
	s.enqueueVerificationEmail(ctx, emailAddr, rawToken)
}

func (s *authService) enqueueVerificationEmail(ctx context.Context, to, rawToken string) {
	if s.jobQueue == nil || s.cfg == nil {
		return
	}
	payload, err := json.Marshal(struct {
		To       string `json:"to"`
		RawToken string `json:"raw_token"`
	}{To: to, RawToken: rawToken})
	if err != nil {
		logger.Log.Warn().Err(err).Msg("verification email job marshal failed")
		return
	}
	if err := s.jobQueue.Enqueue(ctx, queuejobs.EmailSendVerification, payload, queue.EnqueueOptions{}); err != nil {
		logger.Log.Warn().Err(err).Msg("verification email enqueue failed")
	}
}

func (s *authService) enqueuePasswordResetEmail(ctx context.Context, to, rawToken string) {
	if s.jobQueue == nil || s.cfg == nil {
		return
	}
	payload, err := json.Marshal(struct {
		To       string `json:"to"`
		RawToken string `json:"raw_token"`
	}{To: to, RawToken: rawToken})
	if err != nil {
		logger.Log.Warn().Err(err).Msg("password reset email job marshal failed")
		return
	}
	if err := s.jobQueue.Enqueue(ctx, queuejobs.EmailSendPasswordReset, payload, queue.EnqueueOptions{}); err != nil {
		logger.Log.Warn().Err(err).Msg("password reset email enqueue failed")
	}
}
