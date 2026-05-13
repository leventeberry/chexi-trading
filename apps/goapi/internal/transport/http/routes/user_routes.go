package routes

import (
	"github.com/gin-gonic/gin"
	"goapi/container"
	"goapi/internal/transport/http/handlers"
	httpmiddleware "goapi/internal/transport/http/middleware"
)

// SetupUserRoutes registers self-service and admin user routes.
func SetupUserRoutes(router *gin.RouterGroup, c *container.Container) {
	selfGroup := router.Group("/users")
	selfGroup.Use(httpmiddleware.AuthMiddleware(c.JWT))
	{
		selfGroup.GET("/me", handlers.GetMe(c.UserService))
		selfGroup.PATCH("/me", handlers.PatchMe(c.UserService))
	}

	adminUserGroup := router.Group("/admin/users")
	adminUserGroup.Use(httpmiddleware.AuthMiddleware(c.JWT))
	adminUserGroup.Use(httpmiddleware.RequireRole("admin"))
	{
		adminUserGroup.GET("", handlers.GetUsers(c.UserService))
		adminUserGroup.GET("/:id", handlers.GetUser(c.UserService))
		adminUserGroup.POST("", handlers.CreateUser(c.UserService))
		adminUserGroup.PUT("/:id", handlers.UpdateUser(c.UserService))
		adminUserGroup.DELETE("/:id", handlers.DeleteUser(c.UserService))
	}
}
