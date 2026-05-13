package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/internal/events"
	"goapi/services"
)

// CreateOrganizationWebhookRequest is JSON for POST .../webhooks.
type CreateOrganizationWebhookRequest struct {
	URL    string   `json:"url" binding:"required"`
	Events []string `json:"events" binding:"required,min=1,dive,required"`
}

// PatchOrganizationWebhookRequest is JSON for PATCH .../webhooks/:webhookId.
type PatchOrganizationWebhookRequest struct {
	URL          *string   `json:"url"`
	Events       *[]string `json:"events"`
	Enabled      *bool     `json:"enabled"`
	RotateSecret *bool     `json:"rotate_secret"`
}

// CreateOrganizationWebhook godoc
// @Summary      Create organization webhook
// @Description  Owner or admin only. Secret returned once.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                              true  "Organization ID or slug"
// @Param        body  body      CreateOrganizationWebhookRequest    true  "Webhook"
// @Success      201   {object}  services.OrganizationWebhookCreatedDTO
// @Router       /api/v1/organizations/{id}/webhooks [post]
func CreateOrganizationWebhook(svc services.OrganizationWebhookService) gin.HandlerFunc {
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
		var req CreateOrganizationWebhookRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		dto, err := svc.CreateOrganizationWebhook(c.Request.Context(), tenant.OrganizationID, actorID, &services.CreateOrganizationWebhookInput{
			URL:    req.URL,
			Events: req.Events,
		})
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusCreated, dto)
	}
}

// ListOrganizationWebhooks godoc
// @Summary      List organization webhooks
// @Description  Owner or admin only. Never includes secret.
// @Tags         organizations
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Organization ID or slug"
// @Success      200  {array}   services.OrganizationWebhookListDTO
// @Router       /api/v1/organizations/{id}/webhooks [get]
func ListOrganizationWebhooks(svc services.OrganizationWebhookService) gin.HandlerFunc {
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
		list, err := svc.ListOrganizationWebhooks(c.Request.Context(), tenant.OrganizationID, actorID)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

// PatchOrganizationWebhook godoc
// @Summary      Update organization webhook
// @Description  Owner or admin only. Optional rotate_secret returns new secret once.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id         path      string                             true  "Organization ID or slug"
// @Param        webhookId  path      string                             true  "Webhook ID"
// @Param        body       body      PatchOrganizationWebhookRequest    true  "Patch"
// @Success      200        {object}  services.OrganizationWebhookPatchDTO
// @Router       /api/v1/organizations/{id}/webhooks/{webhookId} [patch]
func PatchOrganizationWebhook(svc services.OrganizationWebhookService) gin.HandlerFunc {
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
		webhookID, err := uuid.Parse(c.Param("webhookId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
			return
		}
		var req PatchOrganizationWebhookRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		dto, err := svc.UpdateOrganizationWebhook(c.Request.Context(), tenant.OrganizationID, webhookID, actorID, &services.UpdateOrganizationWebhookInput{
			URL:          req.URL,
			Events:       req.Events,
			Enabled:      req.Enabled,
			RotateSecret: req.RotateSecret,
		})
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto)
	}
}

// DeleteOrganizationWebhook godoc
// @Summary      Delete organization webhook
// @Description  Owner or admin only.
// @Tags         organizations
// @Security     BearerAuth
// @Param        id         path      string  true  "Organization ID or slug"
// @Param        webhookId  path      string  true  "Webhook ID"
// @Success      204        "No Content"
// @Router       /api/v1/organizations/{id}/webhooks/{webhookId} [delete]
func DeleteOrganizationWebhook(svc services.OrganizationWebhookService) gin.HandlerFunc {
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
		webhookID, err := uuid.Parse(c.Param("webhookId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
			return
		}
		if err := svc.DeleteOrganizationWebhook(c.Request.Context(), tenant.OrganizationID, webhookID, actorID); err != nil {
			handleServiceError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ListOrganizationWebhookDeliveries godoc
// @Summary      List webhook delivery history
// @Description  Owner or admin only.
// @Tags         organizations
// @Produce      json
// @Security     BearerAuth
// @Param        id         path      string  true  "Organization ID or slug"
// @Param        webhookId  path      string  true  "Webhook ID"
// @Param        limit      query     int     false "Max rows (default 50, max 200)"
// @Success      200        {array}   services.OrganizationWebhookDeliveryDTO
// @Router       /api/v1/organizations/{id}/webhooks/{webhookId}/deliveries [get]
func ListOrganizationWebhookDeliveries(svc services.OrganizationWebhookService) gin.HandlerFunc {
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
		webhookID, err := uuid.Parse(c.Param("webhookId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
			return
		}
		limit := 50
		if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				limit = n
			}
		}
		list, err := svc.ListOrganizationWebhookDeliveries(c.Request.Context(), tenant.OrganizationID, webhookID, actorID, limit)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
