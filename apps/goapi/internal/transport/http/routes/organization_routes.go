package routes

import (
	"github.com/gin-gonic/gin"
	"goapi/container"
	"goapi/internal/transport/http/handlers"
	httpmiddleware "goapi/internal/transport/http/middleware"
)

// SetupOrganizationRoutes registers multi-tenant organization APIs (authenticated).
func SetupOrganizationRoutes(router *gin.RouterGroup, c *container.Container) {
	g := router.Group("/organizations")
	g.Use(httpmiddleware.OrganizationGroupAuthMiddleware(c.JWT, c.OrganizationAPIKeyService))
	{
		g.POST("/invitations/accept", handlers.AcceptOrganizationInvitation(c.OrganizationService))
		g.POST("", handlers.CreateOrganization(c.OrganizationService))
		g.GET("", handlers.ListOrganizations(c.OrganizationService))

		tenantOpts := httpmiddleware.OrganizationTenantOptions{
			ParamName:              "id",
			AllowGlobalAdminBypass: false,
		}
		withTenant := g.Group("/:id")
		withTenant.Use(httpmiddleware.OrganizationTenantMiddleware(c.OrganizationRepository, tenantOpts))
		{
			notes := withTenant.Group("")
			notes.GET("/notes", httpmiddleware.RequireOrgAccessRead(), handlers.ListOrganizationNotes(c.OrganizationNoteService))
			notes.POST("/notes", httpmiddleware.RequireOrgAccessWrite(), handlers.CreateOrganizationNote(c.OrganizationNoteService))

			apiKeys := withTenant.Group("")
			apiKeys.Use(httpmiddleware.RequireOrgAdmin())
			apiKeys.POST("/api-keys", handlers.CreateOrganizationAPIKey(c.OrganizationAPIKeyService))
			apiKeys.GET("/api-keys", handlers.ListOrganizationAPIKeys(c.OrganizationAPIKeyService))
			apiKeys.DELETE("/api-keys/:keyId", handlers.RevokeOrganizationAPIKey(c.OrganizationAPIKeyService))

			wh := withTenant.Group("")
			wh.Use(httpmiddleware.RequireOrgAdmin())
			wh.POST("/webhooks", handlers.CreateOrganizationWebhook(c.OrganizationWebhookService))
			wh.GET("/webhooks", handlers.ListOrganizationWebhooks(c.OrganizationWebhookService))
			wh.PATCH("/webhooks/:webhookId", handlers.PatchOrganizationWebhook(c.OrganizationWebhookService))
			wh.DELETE("/webhooks/:webhookId", handlers.DeleteOrganizationWebhook(c.OrganizationWebhookService))
			wh.GET("/webhooks/:webhookId/deliveries", handlers.ListOrganizationWebhookDeliveries(c.OrganizationWebhookService))

			withTenant.DELETE("/notes/:noteId", httpmiddleware.RequireOrgMember(), httpmiddleware.RequireOrgAdmin(), handlers.DeleteOrganizationNote(c.OrganizationNoteService))
		}

		g.GET("/:id", handlers.GetOrganization(c.OrganizationService))
		g.PATCH("/:id", handlers.PatchOrganization(c.OrganizationService))
		g.GET("/:id/members", handlers.ListOrganizationMembers(c.OrganizationService))
		g.DELETE("/:id/members/:userId", handlers.RemoveOrganizationMember(c.OrganizationService))
		g.POST("/:id/invitations", handlers.CreateOrganizationInvitation(c.OrganizationService))
		g.GET("/:id/invitations", handlers.ListOrganizationInvitations(c.OrganizationService))
	}
}
