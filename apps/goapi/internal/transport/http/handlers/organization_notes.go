package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/internal/events"
	"goapi/services"
)

// CreateOrganizationNoteRequest is JSON for POST .../notes.
type CreateOrganizationNoteRequest struct {
	Title string `json:"title" binding:"required,min=1,max=255"`
	Body  string `json:"body" binding:"required,min=1"`
}

// CreateOrganizationNote godoc
// @Summary      Create organization note
// @Description  Members only. Scoped to tenant from route.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                       true  "Organization ID or slug"
// @Param        body  body      CreateOrganizationNoteRequest  true  "Note"
// @Success      201   {object}  services.OrganizationNoteDTO
// @Router       /api/v1/organizations/{id}/notes [post]
func CreateOrganizationNote(svc services.OrganizationNoteService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenant, ok := events.OrganizationTenantFromContext(c.Request.Context())
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Tenant context missing"})
			return
		}
		var actorID uuid.UUID
		if uid, _, ok := actorFromGin(c); ok {
			actorID = uid
		} else if tenant.APIKeyAuth && tenant.APIKeyCreatedByUserID != uuid.Nil {
			actorID = tenant.APIKeyCreatedByUserID
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		var req CreateOrganizationNoteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		dto, err := svc.CreateOrganizationNote(c.Request.Context(), tenant.OrganizationID, actorID, &services.CreateOrganizationNoteInput{
			Title: req.Title,
			Body:  req.Body,
		})
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusCreated, dto)
	}
}

// ListOrganizationNotes godoc
// @Summary      List organization notes
// @Tags         organizations
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Organization ID or slug"
// @Success      200  {array}   services.OrganizationNoteDTO
// @Router       /api/v1/organizations/{id}/notes [get]
func ListOrganizationNotes(svc services.OrganizationNoteService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenant, ok := events.OrganizationTenantFromContext(c.Request.Context())
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Tenant context missing"})
			return
		}

		list, err := svc.ListOrganizationNotes(c.Request.Context(), tenant.OrganizationID)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

// DeleteOrganizationNote godoc
// @Summary      Delete organization note
// @Description  Owner or admin (tenant role); explicit global-admin bypass may apply if configured on route.
// @Tags         organizations
// @Security     BearerAuth
// @Param        id      path      string  true  "Organization ID or slug"
// @Param        noteId  path      string  true  "Note ID"
// @Success      204     "No Content"
// @Router       /api/v1/organizations/{id}/notes/{noteId} [delete]
func DeleteOrganizationNote(svc services.OrganizationNoteService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenant, ok := events.OrganizationTenantFromContext(c.Request.Context())
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Tenant context missing"})
			return
		}

		noteID, err := uuid.Parse(c.Param("noteId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid note ID"})
			return
		}

		if err := svc.DeleteOrganizationNote(c.Request.Context(), tenant.OrganizationID, noteID); err != nil {
			handleServiceError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
