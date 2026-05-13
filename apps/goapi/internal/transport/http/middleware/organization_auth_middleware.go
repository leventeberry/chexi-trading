package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/internal/events"
	authinfra "goapi/internal/infra/auth"
	"goapi/services"
)

// OrganizationGroupAuthMiddleware authenticates organization routes via JWT Bearer token
// or organization API key (Bearer orgk_... or X-API-Key). When Authorization Bearer is present,
// it takes precedence over X-API-Key; invalid JWT does not fall back to X-API-Key.
func OrganizationGroupAuthMiddleware(manager *authinfra.Manager, apiKeys services.OrganizationAPIKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		xKey := strings.TrimSpace(c.GetHeader("X-API-Key"))

		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if tokenString == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid Authorization header"})
				return
			}
			if strings.HasPrefix(tokenString, "orgk_") {
				if err := authenticateOrgAPIKey(c, apiKeys, tokenString); err != nil {
					return
				}
				c.Next()
				return
			}
			claims, err := manager.ParseToken(tokenString)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
				return
			}
			c.Set("apiKey", claims.ApiKey)
			c.Set("userID", claims.Subject)
			c.Set("role", claims.Role)
			if claims.ExpiresAt != nil {
				c.Set("expiresAt", claims.ExpiresAt.Time)
			}
			if uid, err := uuid.Parse(claims.Subject); err == nil {
				ctx := events.WithActorUserID(c.Request.Context(), uid)
				c.Request = c.Request.WithContext(ctx)
			}
			c.Next()
			return
		}

		if xKey != "" {
			if err := authenticateOrgAPIKey(c, apiKeys, xKey); err != nil {
				return
			}
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid Authorization header"})
	}
}

func authenticateOrgAPIKey(c *gin.Context, apiKeys services.OrganizationAPIKeyService, raw string) error {
	if apiKeys == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
		return errors.New("organization api key service not configured")
	}
	principal, err := apiKeys.AuthenticateOrganizationAPIKey(c.Request.Context(), raw)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
		return err
	}
	ctx := events.WithAPIKeyTenantPrincipal(c.Request.Context(), events.APIKeyTenantPrincipal{
		OrganizationID:  principal.OrganizationID,
		KeyID:           principal.KeyID,
		Scopes:          principal.Scopes,
		CreatedByUserID: principal.CreatedByUserID,
	})
	c.Request = c.Request.WithContext(ctx)
	return nil
}
