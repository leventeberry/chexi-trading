package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/services"
)

// CreateOrgInvitationRequest is the body for POST /organizations/:id/invitations.
type CreateOrgInvitationRequest struct {
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role" binding:"required,oneof=admin member"`
}

// AcceptOrgInvitationRequest is the body for POST /organizations/invitations/accept.
type AcceptOrgInvitationRequest struct {
	Token string `json:"token" binding:"required"`
}

// CreateOrganizationInvitation godoc
// @Summary      Create organization invitation
// @Description  Owner or admin only. Sends email with single-use token.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                       true  "Organization ID"
// @Param        body  body      CreateOrgInvitationRequest  true  "Invite"
// @Success      201   {object}  services.OrganizationInvitationDTO
// @Router       /api/v1/organizations/{id}/invitations [post]
func CreateOrganizationInvitation(svc services.OrganizationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		orgID, ok := organizationPathUUID(c, svc)
		if !ok {
			return
		}
		var req CreateOrgInvitationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		dto, err := svc.CreateOrganizationInvitation(c.Request.Context(), orgID, actorID, &services.CreateOrganizationInvitationInput{
			Email: req.Email,
			Role:  req.Role,
		})
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusCreated, dto)
	}
}

// ListOrganizationInvitations godoc
// @Summary      List organization invitations
// @Description  Owner or admin only
// @Tags         organizations
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Organization ID"
// @Success      200  {array}   services.OrganizationInvitationDTO
// @Router       /api/v1/organizations/{id}/invitations [get]
func ListOrganizationInvitations(svc services.OrganizationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		orgID, ok := organizationPathUUID(c, svc)
		if !ok {
			return
		}
		list, err := svc.ListOrganizationInvitations(c.Request.Context(), orgID, actorID)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

// AcceptOrganizationInvitation godoc
// @Summary      Accept organization invitation
// @Description  Authenticated user; account email must match invite email.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      AcceptOrgInvitationRequest  true  "Token from email"
// @Success      204   "No Content"
// @Router       /api/v1/organizations/invitations/accept [post]
func AcceptOrganizationInvitation(svc services.OrganizationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		var req AcceptOrgInvitationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		if err := svc.AcceptOrganizationInvitation(c.Request.Context(), actorID, req.Token); err != nil {
			handleServiceError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// RemoveOrganizationMember godoc
// @Summary      Remove organization member
// @Description  Owner or admin only; cannot remove the last owner.
// @Tags         organizations
// @Security     BearerAuth
// @Param        id      path      string  true  "Organization ID"
// @Param        userId  path      string  true  "Member user ID"
// @Success      204     "No Content"
// @Router       /api/v1/organizations/{id}/members/{userId} [delete]
func RemoveOrganizationMember(svc services.OrganizationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		orgID, ok := organizationPathUUID(c, svc)
		if !ok {
			return
		}
		targetID, err := uuid.Parse(c.Param("userId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}
		if err := svc.RemoveOrganizationMember(c.Request.Context(), orgID, actorID, targetID); err != nil {
			handleServiceError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
