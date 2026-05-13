package handlers

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/config"
	"goapi/services"
)

// OAuthCompleteInput exchanges a one-time oauth_code from the browser redirect for API tokens.
type OAuthCompleteInput struct {
	OAuthCode string `json:"oauth_code" binding:"required"`
}

// OAuthStart begins the OAuth browser redirect (login / signup).
func OAuthStart(cfg *config.Config, auth services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := strings.ToLower(strings.TrimSpace(c.Param("provider")))
		if !oauthValidProviderParam(provider) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid oauth provider"})
			return
		}
		if !oauthProviderEnabledInConfig(cfg, provider) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "OAuth provider is not enabled"})
			return
		}
		u, err := auth.OAuthAuthorizeURL(c.Request.Context(), provider, nil)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.Redirect(http.StatusFound, u)
	}
}

// OAuthCallback handles the IdP redirect; on success redirects to OAUTH_REDIRECT_BASE_URL with oauth_code or oauth_error.
func OAuthCallback(cfg *config.Config, auth services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := strings.ToLower(strings.TrimSpace(c.Param("provider")))
		if !oauthValidProviderParam(provider) {
			base := strings.TrimSpace(cfg.OAuth.RedirectBaseURL)
			if base != "" {
				c.Redirect(http.StatusFound, oauthAppendQueryParam(base, "oauth_error", "invalid_provider"))
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid oauth provider"})
			return
		}
		if qErr := strings.TrimSpace(c.Query("error")); qErr != "" {
			base := strings.TrimSpace(cfg.OAuth.RedirectBaseURL)
			if base != "" {
				c.Redirect(http.StatusFound, oauthAppendQueryParam(base, "oauth_error", "provider_denied"))
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "oauth denied"})
			return
		}
		code := strings.TrimSpace(c.Query("code"))
		state := strings.TrimSpace(c.Query("state"))
		res, err := auth.OAuthHandleCallback(c.Request.Context(), provider, code, state)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		if res != nil && res.RedirectURL != "" {
			c.Redirect(http.StatusFound, res.RedirectURL)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	}
}

// OAuthComplete exchanges oauth_code for session tokens or MFA challenge JSON.
func OAuthComplete(auth services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in OAuthCompleteInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx := services.WithAuthRequestMeta(c.Request.Context(), services.AuthRequestMeta{
			UserAgent: c.Request.UserAgent(),
			IPAddress: c.ClientIP(),
		})
		result, err := auth.OAuthCompleteExchange(ctx, in.OAuthCode)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		if result.MFARequired {
			c.JSON(http.StatusOK, gin.H{
				"mfa_required":        true,
				"mfa_challenge_token": result.MFAChallengeToken,
			})
			return
		}
		ReturnAuthSuccess(c, http.StatusOK, result.User, result.Auth)
	}
}

// OAuthLinkStart returns an authorization URL for authenticated account linking (SPA opens location).
func OAuthLinkStart(cfg *config.Config, auth services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := strings.ToLower(strings.TrimSpace(c.Param("provider")))
		if !oauthValidProviderParam(provider) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid oauth provider"})
			return
		}
		if !oauthProviderEnabledInConfig(cfg, provider) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "OAuth provider is not enabled"})
			return
		}
		actorID, _, ok := actorFromGin(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid Authorization header"})
			return
		}
		u, err := auth.OAuthAuthorizeURL(c.Request.Context(), provider, ptrUUID(actorID))
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"authorization_url": u})
	}
}

func ptrUUID(id uuid.UUID) *uuid.UUID {
	return &id
}

func oauthValidProviderParam(p string) bool {
	return p == services.OAuthProviderGoogle || p == services.OAuthProviderGitHub
}

func oauthProviderEnabledInConfig(cfg *config.Config, provider string) bool {
	if cfg == nil {
		return false
	}
	switch provider {
	case services.OAuthProviderGoogle:
		return config.OAuthGoogleConfigured(cfg)
	case services.OAuthProviderGitHub:
		return config.OAuthGitHubConfigured(cfg)
	default:
		return false
	}
}

func oauthAppendQueryParam(rawURL, key, val string) string {
	return oauthMergeQuery(rawURL, key, val)
}

func oauthMergeQuery(rawURL, key, val string) string {
	// Small helper duplicated from services to avoid exporting; keeps redirect building in handler only.
	u, err := parseURLSafe(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	q.Set(key, val)
	u.RawQuery = q.Encode()
	return u.String()
}

func parseURLSafe(raw string) (*url.URL, error) {
	return url.Parse(strings.TrimSpace(raw))
}
