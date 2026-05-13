package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"goapi/config"
	"goapi/internal/events"
	authinfra "goapi/internal/infra/auth"
	"goapi/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func randomRecoveryMaterial() (plain, hash string, err error) {
	buf := make([]byte, 5)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	h := hex.EncodeToString(buf)
	plain = strings.ToUpper(h[:5] + "-" + h[5:])
	return plain, tokenHash(plain), nil
}

// SetupTOTP starts enrollment: stores an encrypted pending secret and returns the secret + otpauth URI once.
func (s *authService) SetupTOTP(ctx context.Context, userID uuid.UUID) (*TOTPSetupResult, error) {
	if !config.MFAEncryptionConfigured(s.cfg) {
		return nil, ErrMFAUnavailable
	}
	keyMaterial := s.cfg.MFA.EncryptionKey

	u, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	if u.TotpEnabled {
		return nil, ErrMFAAlreadyEnabled
	}

	otpKey, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.cfg.MFA.TOTPIssuer,
		AccountName: u.Email,
		Period:      30,
		SecretSize:  20,
		Algorithm:   otp.AlgorithmSHA1,
		Digits:      otp.DigitsSix,
	})
	if err != nil {
		return nil, ErrTokenGeneration
	}
	secret := otpKey.Secret()
	enc, err := authinfra.EncryptAESGCM([]byte(secret), keyMaterial)
	if err != nil {
		return nil, ErrTokenGeneration
	}

	var factor models.UserTOTPFactor
	err = s.db.WithContext(ctx).Where("user_id = ?", userID).First(&factor).Error
	now := time.Now().UTC()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		factor = models.UserTOTPFactor{
			ID:                     uuid.New(),
			UserID:                 userID,
			PendingEncryptedSecret: enc,
			CreatedAt:              now,
			UpdatedAt:              now,
		}
		if err := s.db.WithContext(ctx).Create(&factor).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		factor.PendingEncryptedSecret = enc
		factor.UpdatedAt = now
		if err := s.db.WithContext(ctx).Save(&factor).Error; err != nil {
			return nil, err
		}
	}

	return &TOTPSetupResult{Secret: secret, URI: otpKey.URL()}, nil
}

// ConfirmTOTP validates a code against the pending secret, enables MFA, and optionally issues recovery codes (hashed in DB).
func (s *authService) ConfirmTOTP(ctx context.Context, userID uuid.UUID, code string) (*TOTPConfirmResult, error) {
	if !config.MFAEncryptionConfigured(s.cfg) {
		return nil, ErrMFAUnavailable
	}
	u, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	if u.TotpEnabled {
		return nil, ErrMFAAlreadyEnabled
	}

	var factor models.UserTOTPFactor
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).First(&factor).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMFANoPendingEnrollment
		}
		return nil, err
	}
	if len(factor.PendingEncryptedSecret) == 0 {
		return nil, ErrMFANoPendingEnrollment
	}

	plain, err := authinfra.DecryptAESGCM(factor.PendingEncryptedSecret, s.cfg.MFA.EncryptionKey)
	if err != nil {
		return nil, ErrMFAInvalidCode
	}
	if !totp.Validate(strings.TrimSpace(code), string(plain)) {
		return nil, ErrMFAInvalidCode
	}

	confirmedEnc, err := authinfra.EncryptAESGCM(plain, s.cfg.MFA.EncryptionKey)
	if err != nil {
		return nil, ErrTokenGeneration
	}

	var result *TOTPConfirmResult
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.User{}).Where("id = ?", userID).Update("totp_enabled", true).Error; err != nil {
			return err
		}
		factor.EncryptedSecret = confirmedEnc
		factor.PendingEncryptedSecret = nil
		factor.UpdatedAt = time.Now().UTC()
		if err := tx.Save(&factor).Error; err != nil {
			return err
		}

		n := s.cfg.MFA.RecoveryCodeCount
		if n < 0 {
			n = 0
		}
		if n > 20 {
			n = 20
		}
		if n == 0 {
			result = &TOTPConfirmResult{}
			return nil
		}
		codes := make([]string, 0, n)
		for i := 0; i < n; i++ {
			plainCode, h, genErr := randomRecoveryMaterial()
			if genErr != nil {
				return genErr
			}
			row := models.UserMFARecoveryCode{
				ID:        uuid.New(),
				UserID:    userID,
				CodeHash:  h,
				CreatedAt: time.Now().UTC(),
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			codes = append(codes, plainCode)
		}
		result = &TOTPConfirmResult{RecoveryCodes: codes}
		return nil
	})
	if err != nil {
		return nil, err
	}

	uid := userID
	events.RecordSafe(s.recorder, ctx, events.Event{
		OccurredAt:  events.NowUTC(),
		EventType:   "auth.mfa.totp_enabled",
		ActorUserID: &uid,
		Metadata:    events.MetadataJSON(map[string]string{"email": u.Email}),
		RequestID:   events.RequestIDFromContext(ctx),
	})

	s.invalidateUserCache(ctx, userID)
	return result, nil
}

// DisableTOTP turns off MFA after password confirmation; wipes factors and recovery code hashes.
func (s *authService) DisableTOTP(ctx context.Context, userID uuid.UUID, password string) error {
	if !config.MFAEncryptionConfigured(s.cfg) {
		return ErrMFAUnavailable
	}
	u, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}
	if !u.TotpEnabled {
		return ErrMFANotEnabled
	}
	if !authinfra.ComparePasswords(u.PassHash, password) {
		return ErrInvalidCurrentPassword
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.User{}).Where("id = ?", userID).Update("totp_enabled", false).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserMFARecoveryCode{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserTOTPFactor{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	uid := userID
	events.RecordSafe(s.recorder, ctx, events.Event{
		OccurredAt:  events.NowUTC(),
		EventType:   "auth.mfa.totp_disabled",
		ActorUserID: &uid,
		Metadata:    events.MetadataJSON(map[string]string{"email": u.Email}),
		RequestID:   events.RequestIDFromContext(ctx),
	})
	s.invalidateUserCache(ctx, userID)
	return nil
}

// VerifyMFALogin completes login after password step using a challenge token and TOTP code.
func (s *authService) VerifyMFALogin(ctx context.Context, challengeToken, code string) (*models.User, *Authentication, error) {
	if !config.MFAEncryptionConfigured(s.cfg) {
		return nil, nil, ErrMFAConfigurationUnavailable
	}
	uid, jti, err := s.jwt.ParseMFAChallengeToken(challengeToken)
	if err != nil {
		return nil, nil, ErrMFAInvalidChallenge
	}
	jtiH := tokenHash(jti)

	var user *models.User

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ch models.MFAChallenge
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("jti_hash = ?", jtiH).First(&ch).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMFAInvalidChallenge
			}
			return err
		}
		now := time.Now().UTC()
		if ch.UsedAt != nil || !now.Before(ch.ExpiresAt) {
			return ErrMFAInvalidChallenge
		}
		if ch.UserID != uid {
			return ErrMFAInvalidChallenge
		}

		u, err := s.userRepo.FindByID(uid)
		if err != nil {
			return ErrMFAInvalidChallenge
		}
		if !u.TotpEnabled {
			return ErrMFANotEnabled
		}

		var factor models.UserTOTPFactor
		if err := tx.Where("user_id = ?", uid).First(&factor).Error; err != nil {
			return ErrMFAInvalidChallenge
		}
		if len(factor.EncryptedSecret) == 0 {
			return ErrMFAInvalidChallenge
		}

		plain, err := authinfra.DecryptAESGCM(factor.EncryptedSecret, s.cfg.MFA.EncryptionKey)
		if err != nil {
			return ErrMFAInvalidCode
		}
		if !totp.Validate(strings.TrimSpace(code), string(plain)) {
			return ErrMFAInvalidCode
		}

		ch.UsedAt = &now
		if err := tx.Save(&ch).Error; err != nil {
			return err
		}
		user = u
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	authPair, err := s.issueAuthentication(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	uidp := user.ID
	events.RecordSafe(s.recorder, ctx, events.Event{
		OccurredAt:  events.NowUTC(),
		EventType:   "auth.login.success",
		ActorUserID: &uidp,
		Metadata:    events.MetadataJSON(map[string]string{"email": user.Email, "via": "mfa_totp"}),
		RequestID:   events.RequestIDFromContext(ctx),
	})
	s.invalidateUserCache(ctx, user.ID)
	return user, authPair, nil
}
