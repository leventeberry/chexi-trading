package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/internal/events"
	authinfra "goapi/internal/infra/auth"
)

// AuthMiddleware validates the JWT token from the Authorization header.
func AuthMiddleware(manager *authinfra.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid Authorization header"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
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
	}
}

// RequireRole returns middleware that checks if the authenticated user has one of the required roles.
// Must be used after AuthMiddleware.
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User role not found in context"})
			return
		}

		roleStr, ok := role.(string)
		if !ok || roleStr == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Invalid role format in token"})
			return
		}

		hasRole := false
		for _, allowedRole := range allowedRoles {
			if roleStr == allowedRole {
				hasRole = true
				break
			}
		}

		if !hasRole {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			return
		}

		c.Next()
	}
}
