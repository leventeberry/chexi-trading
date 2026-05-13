package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"goapi/cache"
	"goapi/config"
	"goapi/internal/email"
	"goapi/internal/events"
	authinfra "goapi/internal/infra/auth"
	"goapi/internal/queue"
	"goapi/internal/rbac"
	"goapi/models"
	"goapi/repositories"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// authService implements AuthService interface
type authService struct {
	userRepo   repositories.UserRepository
	tokenRepo  repositories.AuthTokenRepository
	db         *gorm.DB
	jwt        *authinfra.Manager
	recorder   events.Recorder
	refreshTTL time.Duration
	mail       email.Sender
	cfg        *config.Config
	appCache   cache.Cache
	jobQueue   queue.Enqueuer
}

// NewAuthService creates a new instance of AuthService
// Factory function for creating auth service
func NewAuthService(db *gorm.DB, userRepo repositories.UserRepository, tokenRepo repositories.AuthTokenRepository, jwt *authinfra.Manager, recorder events.Recorder, refreshExpirationHours int, mail email.Sender, cfg *config.Config, appCache cache.Cache, jobQueue queue.Enqueuer) AuthService {
	if refreshExpirationHours < 1 {
		refreshExpirationHours = 24 * 30
	}
	return &authService{
		userRepo:   userRepo,
		tokenRepo:  tokenRepo,
		db:         db,
		jwt:        jwt,
		recorder:   recorder,
		refreshTTL: time.Duration(refreshExpirationHours) * time.Hour,
		mail:       mail,
		cfg:        cfg,
		appCache:   appCache,
		jobQueue:   jobQueue,
	}
}

func (s *authService) invalidateUserCache(ctx context.Context, id uuid.UUID) {
	if s.appCache == nil || id == uuid.Nil {
		return
	}
	u, err := s.userRepo.FindByID(id)
	if err != nil {
		return
	}
	_ = s.appCache.DeleteUser(ctx, id, u.Email)
}

// Login authenticates a user and returns tokens, or an MFA challenge when TOTP is enabled.
func (s *authService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	user, err := s.ValidateCredentials(email, password)
	if err != nil {
		events.RecordSafe(s.recorder, ctx, events.Event{
			OccurredAt: events.NowUTC(),
			EventType:  "auth.login.failure",
			Metadata:   events.MetadataJSON(map[string]string{"email": email}),
			RequestID:  events.RequestIDFromContext(ctx),
		})
		return nil, err
	}

	if user.EmailVerifiedAt == nil {
		events.RecordSafe(s.recorder, ctx, events.Event{
			OccurredAt: events.NowUTC(),
			EventType:  "auth.login.failure",
			Metadata:   events.MetadataJSON(map[string]string{"email": email, "reason": "email_unverified"}),
			RequestID:  events.RequestIDFromContext(ctx),
		})
		return nil, ErrEmailNotVerified
	}

	uid := user.ID
	if !user.TotpEnabled {
		authPair, err := s.issueAuthentication(ctx, user)
		if err != nil {
			return nil, err
		}
		events.RecordSafe(s.recorder, ctx, events.Event{
			OccurredAt:  events.NowUTC(),
			EventType:   "auth.login.success",
			ActorUserID: &uid,
			Metadata:    events.MetadataJSON(map[string]string{"email": user.Email}),
			RequestID:   events.RequestIDFromContext(ctx),
		})
		return &LoginResult{User: user, Auth: authPair}, nil
	}

	if !config.MFAEncryptionConfigured(s.cfg) {
		events.RecordSafe(s.recorder, ctx, events.Event{
			OccurredAt: events.NowUTC(),
			EventType:  "auth.login.failure",
			Metadata:   events.MetadataJSON(map[string]string{"email": email, "reason": "mfa_unconfigured"}),
			RequestID:  events.RequestIDFromContext(ctx),
		})
		return nil, ErrMFAConfigurationUnavailable
	}

	var factor models.UserTOTPFactor
	if err := s.db.WithContext(ctx).Where("user_id = ?", user.ID).First(&factor).Error; err != nil {
		events.RecordSafe(s.recorder, ctx, events.Event{
			OccurredAt: events.NowUTC(),
			EventType:  "auth.login.failure",
			Metadata:   events.MetadataJSON(map[string]string{"email": email, "reason": "mfa_factor_missing"}),
			RequestID:  events.RequestIDFromContext(ctx),
		})
		return nil, ErrMFAConfigurationUnavailable
	}
	if len(factor.EncryptedSecret) == 0 {
		events.RecordSafe(s.recorder, ctx, events.Event{
			OccurredAt: events.NowUTC(),
			EventType:  "auth.login.failure",
			Metadata:   events.MetadataJSON(map[string]string{"email": email, "reason": "mfa_secret_missing"}),
			RequestID:  events.RequestIDFromContext(ctx),
		})
		return nil, ErrMFAConfigurationUnavailable
	}

	jti := uuid.NewString()
	challengeTTL := config.MFAChallengeTTL(s.cfg)
	now := time.Now().UTC()
	chRow := models.MFAChallenge{
		ID:        uuid.New(),
		UserID:    user.ID,
		JTIHash:   tokenHash(jti),
		ExpiresAt: now.Add(challengeTTL),
		CreatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&chRow).Error; err != nil {
		return nil, ErrSessionCreation
	}
	challengeJWT, err := s.jwt.CreateMFAChallengeToken(user.ID, jti, challengeTTL)
	if err != nil {
		return nil, ErrTokenGeneration
	}

	events.RecordSafe(s.recorder, ctx, events.Event{
		OccurredAt:  events.NowUTC(),
		EventType:   "auth.login.mfa_challenge_issued",
		ActorUserID: &uid,
		Metadata:    events.MetadataJSON(map[string]string{"email": user.Email}),
		RequestID:   events.RequestIDFromContext(ctx),
	})

	return &LoginResult{User: user, MFARequired: true, MFAChallengeToken: challengeJWT}, nil
}

// Register creates a new user account and returns a JWT token
func (s *authService) Register(ctx context.Context, input *RegisterInput) (*models.User, *Authentication, error) {
	// Create user directly here to avoid circular dependency
	// In a more advanced setup, we'd use a service orchestrator or composition

	trimmedRole := strings.TrimSpace(input.Role)
	if trimmedRole != "" && !IsValidRole(trimmedRole) {
		return nil, nil, ErrInvalidRole
	}
	if rbac.IsAdminRole(trimmedRole) {
		return nil, nil, ErrForbiddenAdminRegistration
	}

	// Self-service registration is always a normal user; admins are assigned by existing admins.
	role := rbac.RoleUser.String()

	// Check if email exists
	exists, err := s.userRepo.ExistsByEmail(input.Email)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to check email existence during registration: %w", err)
	}
	if exists {
		return nil, nil, ErrEmailExists
	}

	// Hash password
	hash, err := authinfra.HashPassword(input.Password)
	if err != nil {
		return nil, nil, ErrPasswordHashing
	}

	// Create user
	user := &models.User{
		FirstName: input.FirstName,
		LastName:  input.LastName,
		Email:     input.Email,
		PassHash:  hash,
		PhoneNum:  input.PhoneNum,
		Role:      role,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, nil, fmt.Errorf("failed to create user during registration: %w", err)
	}

	if s.cfg != nil && !s.cfg.Email.Enabled {
		now := time.Now().UTC()
		if err := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", user.ID).Update("email_verified_at", now).Error; err != nil {
			return nil, nil, fmt.Errorf("failed to mark email verified when outbound email is disabled: %w", err)
		}
		user.EmailVerifiedAt = &now
	} else {
		s.issueEmailVerification(ctx, user.ID, user.Email)
	}

	regID := user.ID
	events.RecordSafe(s.recorder, ctx, events.Event{
		OccurredAt:  events.NowUTC(),
		EventType:   "auth.register",
		ActorUserID: &regID,
		Metadata: events.MetadataJSON(map[string]string{
			"email": user.Email,
			"role":  user.Role,
		}),
		RequestID: events.RequestIDFromContext(ctx),
	})

	if user.EmailVerifiedAt == nil {
		return user, nil, nil
	}

	authPair, err := s.issueAuthentication(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	return user, authPair, nil
}

func (s *authService) issueAuthentication(ctx context.Context, user *models.User) (*Authentication, error) {
	token, err := s.jwt.CreateToken(user.ID, user.Role)
	if err != nil {
		return nil, ErrTokenGeneration
	}
	refreshToken, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return &Authentication{ApiKey: token.ApiKey, JWTToken: token.JWTToken, RefreshToken: refreshToken}, nil
}

// ValidateCredentials validates user email and password
func (s *authService) ValidateCredentials(email, password string) (*models.User, error) {
	// Find user by email
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Verify password
	if !authinfra.ComparePasswords(user.PassHash, password) {
		return nil, ErrInvalidCredentials
	}

	return user, nil
}

// RefreshToken rotates a valid refresh token and returns a new access + refresh token pair.
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*Authentication, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, ErrInvalidRefreshToken
	}
	if s.db == nil {
		return nil, ErrSessionCreation
	}

	var auth *Authentication
	var reuseDetected bool
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		hash := tokenHash(refreshToken)

		var current models.Session
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ?", hash).
			First(&current).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return ErrInvalidRefreshToken
			}
			return err
		}

		if current.RevokedAt != nil {
			if current.RevokeReason == "rotated" {
				// Revoke all sessions for this user, then commit by returning nil.
				// Returning an error would roll back the bulk revoke (GORM transaction semantics).
				if revokeErr := s.revokeAllUserSessionsTx(tx, current.UserID, "reuse_detected"); revokeErr != nil {
					return revokeErr
				}
				reuseDetected = true
				return nil
			}
			return ErrRevokedRefreshToken
		}
		if now.After(current.ExpiresAt) {
			return ErrExpiredRefreshToken
		}

		user, err := s.userRepo.FindByID(current.UserID)
		if err != nil {
			return ErrInvalidRefreshToken
		}
		if user.EmailVerifiedAt == nil {
			return ErrEmailNotVerified
		}
		accessToken, err := s.jwt.CreateToken(user.ID, user.Role)
		if err != nil {
			return ErrTokenGeneration
		}
		meta := authRequestMetaFromContext(ctx)
		nextSession, nextRefreshToken, err := newSessionRecord(user.ID, s.refreshTTL, meta)
		if err != nil {
			return ErrSessionCreation
		}
		if err := tx.Create(nextSession).Error; err != nil {
			return ErrSessionCreation
		}

		current.RevokedAt = &now
		current.RevokeReason = "rotated"
		current.ReplacedBySession = &nextSession.ID
		if err := tx.Save(&current).Error; err != nil {
			return err
		}

		auth = &Authentication{
			ApiKey:       accessToken.ApiKey,
			JWTToken:     accessToken.JWTToken,
			RefreshToken: nextRefreshToken,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if reuseDetected {
		return nil, ErrRefreshTokenReuseDetected
	}
	return auth, nil
}

// Logout revokes all active refresh-token sessions for the user identified by the provided refresh token.
// The token may be the current active refresh token or a previously rotated token; any known hash
// for that user ends all sessions so no rotated successor remains valid.
func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return ErrInvalidRefreshToken
	}
	if s.db == nil {
		return ErrSessionCreation
	}
	hash := tokenHash(refreshToken)

	var sess models.Session
	if err := s.db.WithContext(ctx).Where("token_hash = ?", hash).First(&sess).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrInvalidRefreshToken
		}
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.revokeAllUserSessionsTx(tx, sess.UserID, "logout_all")
	})
}

func (s *authService) createSession(ctx context.Context, userID uuid.UUID) (string, error) {
	if s.db == nil {
		return "", ErrSessionCreation
	}
	meta := authRequestMetaFromContext(ctx)
	record, plaintext, err := newSessionRecord(userID, s.refreshTTL, meta)
	if err != nil {
		return "", ErrSessionCreation
	}
	if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
		return "", ErrSessionCreation
	}
	return plaintext, nil
}

func newSessionRecord(userID uuid.UUID, ttl time.Duration, meta AuthRequestMeta) (*models.Session, string, error) {
	rawToken, err := newOpaqueToken()
	if err != nil {
		return nil, "", err
	}
	now := time.Now().UTC()
	return &models.Session{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: tokenHash(rawToken),
		IssuedAt:  now,
		ExpiresAt: now.Add(ttl),
		UserAgent: meta.UserAgent,
		IPAddress: meta.IPAddress,
	}, rawToken, nil
}

func newOpaqueToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *authService) revokeAllUserSessionsTx(tx *gorm.DB, userID uuid.UUID, reason string) error {
	now := time.Now().UTC()
	return tx.Model(&models.Session{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]interface{}{
			"revoked_at":    &now,
			"revoke_reason": reason,
			"updated_at":    now,
		}).Error
}
