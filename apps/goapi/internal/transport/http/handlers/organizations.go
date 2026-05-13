package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/services"
)

// CreateOrganizationRequest is the JSON body for POST /organizations.
type CreateOrganizationRequest struct {
	Name string `json:"name" binding:"required,min=1,max=255"`
	Slug string `json:"slug" binding:"omitempty,max=128"`
}

// PatchOrganizationRequest is the JSON body for PATCH /organizations/:id.
type PatchOrganizationRequest struct {
	Name *string `json:"name" binding:"omitempty,min=1,max=255"`
	Slug *string `json:"slug" binding:"omitempty,max=128"`
}

// CreateOrganization godoc
// @Summary      Create organization
// @Description  Creates an organization; the creator becomes owner.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      CreateOrganizationRequest  true  "Organization"
// @Success      201   {object}  services.OrganizationDTO
// @Failure      400   {object}  map[string]string
// @Failure      401   {object}  map[string]string
// @Failure      409   {object}  map[string]string
// @Router       /api/v1/organizations [post]
func organizationPathUUID(c *gin.Context, svc services.OrganizationService) (uuid.UUID, bool) {
	id, err := svc.ResolveOrganizationRouteID(c.Request.Context(), c.Param("id"))
	if err != nil {
		handleServiceError(c, err)
		return uuid.Nil, false
	}
	return id, true
}

func CreateOrganization(svc services.OrganizationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		var req CreateOrganizationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		in := &services.CreateOrganizationInput{Name: req.Name}
		if req.Slug != "" {
			s := req.Slug
			in.Slug = &s
		}

		dto, err := svc.CreateOrganization(c.Request.Context(), actorID, in)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusCreated, dto)
	}
}

// ListOrganizations godoc
// @Summary      List my organizations
// @Description  Returns organizations the current user belongs to.
// @Tags         organizations
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   services.OrganizationDTO
// @Failure      401  {object}  map[string]string
// @Router       /api/v1/organizations [get]
func ListOrganizations(svc services.OrganizationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		list, err := svc.ListOrganizations(c.Request.Context(), actorID)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

// GetOrganization godoc
// @Summary      Get organization
// @Description  Returns organization details if the user is a member.
// @Tags         organizations
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Organization ID"
// @Success      200  {object}  services.OrganizationDTO
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /api/v1/organizations/{id} [get]
func GetOrganization(svc services.OrganizationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		id, ok := organizationPathUUID(c, svc)
		if !ok {
			return
		}

		dto, err := svc.GetOrganization(c.Request.Context(), id, actorID)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto)
	}
}

// PatchOrganization godoc
// @Summary      Update organization
// @Description  Owner or admin only.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                     true  "Organization ID"
// @Param        body  body      PatchOrganizationRequest  true  "Fields"
// @Success      200   {object}  services.OrganizationDTO
// @Failure      400   {object}  map[string]string
// @Failure      401   {object}  map[string]string
// @Failure      403   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Failure      409   {object}  map[string]string
// @Router       /api/v1/organizations/{id} [patch]
func PatchOrganization(svc services.OrganizationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		id, ok := organizationPathUUID(c, svc)
		if !ok {
			return
		}

		var req PatchOrganizationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		in := &services.UpdateOrganizationInput{Name: req.Name, Slug: req.Slug}
		dto, err := svc.UpdateOrganization(c.Request.Context(), id, actorID, in)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto)
	}
}

// ListOrganizationMembers godoc
// @Summary      List organization members
// @Description  Members of the organization (members-only).
// @Tags         organizations
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Organization ID"
// @Success      200  {array}   services.OrganizationMemberDTO
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /api/v1/organizations/{id}/members [get]
func ListOrganizationMembers(svc services.OrganizationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _, authed := actorFromGin(c)
		if !authed {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		id, ok := organizationPathUUID(c, svc)
		if !ok {
			return
		}

		list, err := svc.ListOrganizationMembers(c.Request.Context(), id, actorID)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
