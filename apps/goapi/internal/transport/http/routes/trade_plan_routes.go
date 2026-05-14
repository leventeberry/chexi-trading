package routes

import (
	"github.com/gin-gonic/gin"
	"goapi/container"
	"goapi/internal/transport/http/handlers"
	httpmiddleware "goapi/internal/transport/http/middleware"
)

// SetupTradePlanRoutes registers advisory manual trade plan CRUD (create/list/get in V1).
func SetupTradePlanRoutes(v1 *gin.RouterGroup, c *container.Container) {
	g := v1.Group("/trade-plans")
	g.Use(httpmiddleware.AuthMiddleware(c.JWT))
	{
		g.POST("", handlers.CreateTradePlan(c.TradePlanService))
		g.GET("", handlers.ListTradePlans(c.TradePlanService))
		g.GET("/:id", handlers.GetTradePlan(c.TradePlanService))
	}
}
