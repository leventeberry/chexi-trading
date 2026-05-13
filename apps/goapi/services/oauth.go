package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"

	"goapi/config"
	"goapi/internal/events"
	authinfra "goapi/internal/infra/auth"
	oauthprofile "goapi/internal/oauth"
	"goapi/internal/rbac"
	"goapi/models"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	OAuthProviderGoogle = "google"
	OAuthProviderGitHub = "github"
)

func oauthProviderAllowed(provider string) bool {
	switch provider {
	case OAuthProviderGoogle, OAuthProviderGitHub:
		return true
	default:
		return false
	}
}

func oauthProviderConfigured(cfg *config.Config, provider string) bool {
	if cfg == nil {
		return false
	}
	switch provider {
	case OAuthProviderGoogle:
		return config.OAuthGoogleConfigured(cfg)
	case OAuthProviderGitHub:
		return config.OAuthGitHubConfigured(cfg)
	default:
		return false
	}
}

func (s *authService) oauth2ClientConfig(provider string) oauth2.Config {
	redirect := config.OAuthProviderCallbackURL(s.cfg, provider)
	switch provider {
	case OAuthProviderGoogle:
		return oauth2.Config{
			ClientID:     s.cfg.OAuth.GoogleClientID,
			ClientSecret: s.cfg.OAuth.GoogleClientSecret,
			RedirectURL:  redirect,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		}
	case OAuthProviderGitHub:
		return oauth2.Config{
			ClientID:     s.cfg.OAuth.GitHubClientID,
			ClientSecret: s.cfg.OAuth.GitHubClientSecret,
			RedirectURL:  redirect,
			Scopes:       []string{"read:user", "user:email"},
			Endpoint:     github.Endpoint,
		}
	default:
		return oauth2.Config{}
	}
}

func oauthGenerateVerifier() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func oauthChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func oauthAppendQuery(rawURL, key, val string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	q.Set(key, val)
	u.RawQuery = q.Encode()
	return u.String()
}

// OAuthAuthorizeURL builds the IdP authorize URL and persists PKCE + state.
func (s *authService) OAuthAuthorizeURL(ctx context.Context, provider string, linkUserID *uuid.UUID) (string, error) {
	if !oauthProviderAllowed(provider) {
		return "", ErrOAuthInvalidProvider
	}
	if !oauthProviderConfigured(s.cfg, provider) {
		return "", ErrOAuthProviderDisabled
	}
	if linkUserID != nil && *linkUserID != uuid.Nil {
		if _, err := s.userRepo.FindByID(*linkUserID); err != nil {
			return "", err
		}
	}

	rawState, err := newOpaqueToken()
	if err != nil {
		return "", ErrTokenGeneration
	}
	verifier, err := oauthGenerateVerifier()
	if err != nil {
		return "", ErrTokenGeneration
	}

	stateHash := tokenHash(rawState)
	now := time.Now().UTC()
	expires := now.Add(config.OAuthStateTTL(s.cfg))

	row := models.OAuthAuthorizationState{
		ID:           uuid.New(),
		StateHash:    stateHash,
		Provider:     provider,
		CodeVerifier: verifier,
		ExpiresAt:    expires,
		CreatedAt:    now,
	}
	if linkUserID != nil && *linkUserID != uuid.Nil {
		u := *linkUserID
		row.LinkUserID = &u
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return "", err
	}

	cfg := s.oauth2ClientConfig(provider)
	challenge := oauthChallengeS256(verifier)
	authURL := cfg.AuthCodeURL(rawState,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	return authURL, nil
}

// OAuthCallbackResult is returned after processing IdP redirect (success URL or error URL).
type OAuthCallbackResult struct {
	RedirectURL string
}

// OAuthHandleCallback exchanges the authorization code, resolves or creates the user, and returns a frontend redirect with oauth_code (or oauth_error).
func (s *authService) OAuthHandleCallback(ctx context.Context, provider, authCode, rawState string) (*OAuthCallbackResult, error) {
	base := strings.TrimSpace(s.cfg.OAuth.RedirectBaseURL)
	if base == "" {
		return nil, ErrOAuthUnavailable
	}
	if !oauthProviderAllowed(provider) {
		return &OAuthCallbackResult{RedirectURL: oauthAppendQuery(base, "oauth_error", "invalid_provider")}, nil
	}
	if !oauthProviderConfigured(s.cfg, provider) {
		return &OAuthCallbackResult{RedirectURL: oauthAppendQuery(base, "oauth_error", "provider_disabled")}, nil
	}
	if strings.TrimSpace(authCode) == "" || strings.TrimSpace(rawState) == "" {
		return &OAuthCallbackResult{RedirectURL: oauthAppendQuery(base, "oauth_error", "missing_code")}, nil
	}

	stateHash := tokenHash(rawState)
	var st models.OAuthAuthorizationState
	err := s.db.WithContext(ctx).Where("state_hash = ?", stateHash).First(&st).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &OAuthCallbackResult{RedirectURL: oauthAppendQuery(base, "oauth_error", "invalid_state")}, nil
		}
		return nil, err
	}
	now := time.Now().UTC()
	if st.UsedAt != nil || !now.Before(st.ExpiresAt) || st.Provider != provider {
		return &OAuthCallbackResult{RedirectURL: oauthAppendQuery(base, "oauth_error", "invalid_state")}, nil
	}

	cfg := s.oauth2ClientConfig(provider)
	tok, err := cfg.Exchange(ctx, authCode, oauth2.SetAuthURLParam("code_verifier", st.CodeVerifier))
	if err != nil {
		return &OAuthCallbackResult{RedirectURL: oauthAppendQuery(base, "oauth_error", "token_exchange_failed")}, nil
	}
	if !tok.Valid() || tok.AccessToken == "" {
		return &OAuthCallbackResult{RedirectURL: oauthAppendQuery(base, "oauth_error", "token_exchange_failed")}, nil
	}

	var providerUID, email string
	var emailVerified bool
	switch provider {
	case OAuthProviderGoogle:
		gp, err := oauthprofile.FetchGoogleUserinfo(ctx, tok.AccessToken)
		if err != nil {
			return &OAuthCallbackResult{RedirectURL: oauthAppendQuery(base, "oauth_error", "profile_failed")}, nil
		}
		providerUID = gp.ProviderUserID
		email = gp.Email
		emailVerified = gp.EmailVerified
	case OAuthProviderGitHub:
		gp, err := oauthprofile.FetchGitHubProfile(ctx, tok.AccessToken)
		if err != nil {
			return &OAuthCallbackResult{RedirectURL: oauthAppendQuery(base, "oauth_error", "profile_failed")}, nil
		}
		providerUID = gp.ProviderUserID
		email = gp.Email
		emailVerified = gp.EmailVerified
	default:
		return &OAuthCallbackResult{RedirectURL: oauthAppendQuery(base, "oauth_error", "invalid_provider")}, nil
	}

	if !emailVerified || email == "" {
		return &OAuthCallbackResult{RedirectURL: oauthAppendQuery(base, "oauth_error", "email_not_verified")}, nil
	}

	// Mark state used only after successful profile fetch (prevents replay).
	st.UsedAt = &now
	if err := s.db.WithContext(ctx).Save(&st).Error; err != nil {
		return nil, err
	}

	var user *models.User
	var linkIntent *uuid.UUID = st.LinkUserID

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existingAcct models.UserOAuthAccount
		findAcct := tx.Where("provider = ? AND provider_user_id = ?", provider, providerUID).First(&existingAcct)
		if findAcct.Error == nil {
			var u models.User
			if err := tx.First(&u, "id = ?", existingAcct.UserID).Error; err != nil {
				return err
			}
			if linkIntent != nil && *linkIntent != u.ID {
				return ErrOAuthIdentityAlreadyLinked
			}
			user = &u
			return nil
		}
		if !errors.Is(findAcct.Error, gorm.ErrRecordNotFound) {
			return findAcct.Error
		}

		if linkIntent != nil {
			var lu models.User
			if err := tx.First(&lu, "id = ?", *linkIntent).Error; err != nil {
				return ErrUserNotFound
			}
			if normalizeOAuthEmail(lu.Email) != email {
				return ErrOAuthLinkEmailMismatch
			}
			acct := models.UserOAuthAccount{
				ID:             uuid.New(),
				UserID:         lu.ID,
				Provider:       provider,
				ProviderUserID: providerUID,
				Email:          email,
				EmailVerified:  true,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if err := tx.Create(&acct).Error; err != nil {
				return err
			}
			user = &lu
			return nil
		}

		var conflict models.User
		qerr := tx.Where("LOWER(email) = LOWER(?)", email).First(&conflict).Error
		if qerr == nil {
			return ErrOAuthEmailConflict
		}
		if !errors.Is(qerr, gorm.ErrRecordNotFound) {
			return qerr
		}

		ph, err := authinfra.HashPassword(newRandomOAuthPassword())
		if err != nil {
			return ErrPasswordHashing
		}
		u := models.User{
			ID:              uuid.New(),
			FirstName:       oauthDefaultName(provider),
			LastName:        "User",
			Email:           email,
			PassHash:        ph,
			Role:            rbac.RoleUser.String(),
			EmailVerifiedAt: ptrTime(now),
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := tx.Create(&u).Error; err != nil {
			return err
		}
		acct := models.UserOAuthAccount{
			ID:             uuid.New(),
			UserID:         u.ID,
			Provider:       provider,
			ProviderUserID: providerUID,
			Email:          email,
			EmailVerified:  true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := tx.Create(&acct).Error; err != nil {
			return err
		}
		user = &u
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrOAuthEmailConflict):
			return &OAuthCallbackResult{RedirectURL: oauthAppendQuery(base, "oauth_error", "email_conflict")}, nil
		case errors.Is(err, ErrOAuthLinkEmailMismatch):
			return &OAuthCallbackResult{RedirectURL: oauthAppendQuery(base, "oauth_error", "link_email_mismatch")}, nil
		case errors.Is(err, ErrOAuthIdentityAlreadyLinked):
			return &OAuthCallbackResult{RedirectURL: oauthAppendQuery(base, "oauth_error", "identity_linked")}, nil
		default:
			return nil, err
		}
	}

	rawExchange, err := newOpaqueToken()
	if err != nil {
		return nil, ErrTokenGeneration
	}
	exHash := tokenHash(rawExchange)
	exExpires := now.Add(config.OAuthExchangeTTL(s.cfg))

	var mfaTok *string
	if user.TotpEnabled {
		if !config.MFAEncryptionConfigured(s.cfg) {
			return &OAuthCallbackResult{RedirectURL: oauthAppendQuery(base, "oauth_error", "mfa_unavailable")}, nil
		}
		var factor models.UserTOTPFactor
		if err := s.db.WithContext(ctx).Where("user_id = ?", user.ID).First(&factor).Error; err != nil || len(factor.EncryptedSecret) == 0 {
			return &OAuthCallbackResult{RedirectURL: oauthAppendQuery(base, "oauth_error", "mfa_unavailable")}, nil
		}
		jti := uuid.NewString()
		chTTL := config.MFAChallengeTTL(s.cfg)
		chRow := models.MFAChallenge{
			ID:        uuid.New(),
			UserID:    user.ID,
			JTIHash:   tokenHash(jti),
			ExpiresAt: now.Add(chTTL),
			CreatedAt: now,
		}
		if err := s.db.WithContext(ctx).Create(&chRow).Error; err != nil {
			return nil, ErrSessionCreation
		}
		chJWT, err := s.jwt.CreateMFAChallengeToken(user.ID, jti, chTTL)
		if err != nil {
			return nil, ErrTokenGeneration
		}
		mfaTok = &chJWT
	}

	exRow := models.OAuthExchangeCode{
		ID:                uuid.New(),
		CodeHash:          exHash,
		UserID:            user.ID,
		ExpiresAt:         exExpires,
		CreatedAt:         now,
		MFAChallengeToken: mfaTok,
	}
	if err := s.db.WithContext(ctx).Create(&exRow).Error; err != nil {
		return nil, err
	}

	uid := user.ID
	events.RecordSafe(s.recorder, ctx, events.Event{
		OccurredAt:  events.NowUTC(),
		EventType:   "auth.oauth.callback_success",
		ActorUserID: &uid,
		Metadata:    events.MetadataJSON(map[string]string{"provider": provider}),
		RequestID:   events.RequestIDFromContext(ctx),
	})

	redirect := oauthAppendQuery(base, "oauth_code", rawExchange)
	return &OAuthCallbackResult{RedirectURL: redirect}, nil
}

func normalizeOAuthEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func newRandomOAuthPassword() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func oauthDefaultName(provider string) string {
	switch provider {
	case OAuthProviderGoogle:
		return "Google"
	case OAuthProviderGitHub:
		return "GitHub"
	default:
		return "OAuth"
	}
}

// OAuthCompleteExchange consumes oauth_code from the frontend redirect and returns tokens or MFA challenge (same as password login).
func (s *authService) OAuthCompleteExchange(ctx context.Context, rawCode string) (*LoginResult, error) {
	if strings.TrimSpace(rawCode) == "" {
		return nil, ErrOAuthExchangeInvalid
	}
	h := tokenHash(strings.TrimSpace(rawCode))
	now := time.Now().UTC()

	var result *LoginResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row models.OAuthExchangeCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("code_hash = ?", h).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrOAuthExchangeInvalid
			}
			return err
		}
		if row.UsedAt != nil || !now.Before(row.ExpiresAt) {
			return ErrOAuthExchangeInvalid
		}

		user, err := s.userRepo.FindByID(row.UserID)
		if err != nil {
			return ErrOAuthExchangeInvalid
		}

		row.UsedAt = &now
		if err := tx.Save(&row).Error; err != nil {
			return err
		}

		if row.MFAChallengeToken != nil && strings.TrimSpace(*row.MFAChallengeToken) != "" {
			result = &LoginResult{
				User:              user,
				MFARequired:       true,
				MFAChallengeToken: strings.TrimSpace(*row.MFAChallengeToken),
			}
			return nil
		}

		authPair, err := s.issueAuthentication(ctx, user)
		if err != nil {
			return err
		}
		uid := user.ID
		events.RecordSafe(s.recorder, ctx, events.Event{
			OccurredAt:  events.NowUTC(),
			EventType:   "auth.login.success",
			ActorUserID: &uid,
			Metadata:    events.MetadataJSON(map[string]string{"email": user.Email, "via": "oauth"}),
			RequestID:   events.RequestIDFromContext(ctx),
		})
		s.invalidateUserCache(ctx, user.ID)
		result = &LoginResult{User: user, Auth: authPair}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
