package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/internal/events"
	"goapi/services"
)

// CreateOrganizationAPIKeyRequest is JSON for POST .../api-keys.
type CreateOrganizationAPIKeyRequest struct {
	Name      string     `json:"name" binding:"required,min=1,max=255"`
	Scopes    []string   `json:"scopes" binding:"required,min=1,dive,required"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// CreateOrganizationAPIKey godoc
// @Summary      Create organization API key
// @Description  Owner or admin only. Full secret returned once.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                             true  "Organization ID or slug"
// @Param        body  body      CreateOrganizationAPIKeyRequest    true  "API key"
// @Success      201   {object}  services.OrganizationAPIKeyCreatedDTO
// @Router       /api/v1/organizations/{id}/api-keys [post]
func CreateOrganizationAPIKey(svc services.OrganizationAPIKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenant, ok := events.OrganizationTenantFromContext(c.Request.Context())
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Tenant context missing"})
			return
		}
		actorID, _, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		var req CreateOrganizationAPIKeyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		dto, err := svc.CreateOrganizationAPIKey(c.Request.Context(), tenant.OrganizationID, actorID, &services.CreateOrganizationAPIKeyInput{
			Name:      req.Name,
			Scopes:    req.Scopes,
			ExpiresAt: req.ExpiresAt,
		})
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusCreated, dto)
	}
}

// ListOrganizationAPIKeys godoc
// @Summary      List organization API keys
// @Description  Owner or admin only. Never includes secret or hash.
// @Tags         organizations
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Organization ID or slug"
// @Success      200  {array}   services.OrganizationAPIKeyListDTO
// @Router       /api/v1/organizations/{id}/api-keys [get]
func ListOrganizationAPIKeys(svc services.OrganizationAPIKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenant, ok := events.OrganizationTenantFromContext(c.Request.Context())
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Tenant context missing"})
			return
		}
		actorID, _, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		list, err := svc.ListOrganizationAPIKeys(c.Request.Context(), tenant.OrganizationID, actorID)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

// RevokeOrganizationAPIKey godoc
// @Summary      Revoke organization API key
// @Description  Owner or admin only.
// @Tags         organizations
// @Security     BearerAuth
// @Param        id     path      string  true  "Organization ID or slug"
// @Param        keyId  path      string  true  "API key ID"
// @Success      204    "No Content"
// @Router       /api/v1/organizations/{id}/api-keys/{keyId} [delete]
func RevokeOrganizationAPIKey(svc services.OrganizationAPIKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenant, ok := events.OrganizationTenantFromContext(c.Request.Context())
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Tenant context missing"})
			return
		}
		actorID, _, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		keyID, err := uuid.Parse(c.Param("keyId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key ID"})
			return
		}
		if err := svc.RevokeOrganizationAPIKey(c.Request.Context(), tenant.OrganizationID, keyID, actorID); err != nil {
			handleServiceError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
