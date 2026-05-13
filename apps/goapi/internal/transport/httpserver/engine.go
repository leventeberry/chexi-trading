// Package httpserver builds the Gin engine with the same middleware stack used in production.
package httpserver

import (
	"github.com/gin-gonic/gin"
	"goapi/cache"
	"goapi/config"
	"goapi/container"
	httpmiddleware "goapi/internal/transport/http/middleware"
	httproutes "goapi/internal/transport/http/routes"
)

// NewEngine returns a Gin engine with rate limiting, request logging, recovery, and application routes.
// Mount Swagger and other non-route concerns in the caller (e.g. internal/app).
func NewEngine(cache cache.Cache, cfg *config.Config, c *container.Container) *gin.Engine {
	router := gin.New()
	router.Use(httpmiddleware.SecurityHeaders(cfg))
	router.Use(httpmiddleware.RequestID())
	router.Use(httpmiddleware.RateLimitMiddlewareWithCache(cache, cfg))
	router.Use(httpmiddleware.RequestLogger())
	router.Use(gin.Recovery())
	router.Use(httpmiddleware.HTTPEventAudit(c.Recorder, cfg))
	httproutes.SetupRoutes(router, c, cfg)
	return router
}
