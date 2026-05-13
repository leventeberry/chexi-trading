package httpserver

import (
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/gin-gonic/gin"
	"goapi/config"
	authinfra "goapi/internal/infra/auth"
	"goapi/internal/rbac"
	httpmiddleware "goapi/internal/transport/http/middleware"
)

// MountSwaggerRoutes mounts Swagger endpoints based on environment and config.
func MountSwaggerRoutes(router *gin.Engine, cfg *config.Config, jwt *authinfra.Manager) {
	if router == nil || cfg == nil || !cfg.Swagger.Enabled {
		return
	}

	if config.IsStagingOrProductionEnvironment(cfg.Environment) {
		if jwt == nil {
			return
		}
		group := router.Group("/swagger")
		group.Use(httpmiddleware.AuthMiddleware(jwt))
		group.Use(httpmiddleware.RequireRole(rbac.RoleAdmin.String()))
		group.GET("/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
		return
	}

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
