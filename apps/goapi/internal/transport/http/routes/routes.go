package routes

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"goapi/config"
	"goapi/container"
	"goapi/internal/transport/http/handlers"
	httpmiddleware "goapi/internal/transport/http/middleware"
)

// SetupRoutes registers all application routes on the provided Gin engine.
func SetupRoutes(router *gin.Engine, c *container.Container, cfg *config.Config) {
	// @Summary      Welcome message
	// @Description  Returns API welcome message
	// @Tags         health
	// @Accept       json
	// @Produce      json
	// @Success      200  {object}  map[string]interface{}  "API is running"
	// @Router       / [get]
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Welcome!",
			"status":  http.StatusOK,
		})
	})

	// @Summary      Health check
	// @Description  Comprehensive health check verifying database and Redis connectivity
	// @Tags         health
	// @Accept       json
	// @Produce      json
	// @Success      200  {object}  map[string]interface{}  "All systems healthy"
	// @Failure      503  {object}  map[string]interface{}  "Service unavailable"
	// @Router       /health [get]
	router.GET("/health", healthCheckHandler(c))

	v1 := router.Group("/api/v1")
	{
		// @Summary      Login user
		// @Description  Authenticate a user with email and password, returns JWT token
		// @Tags         authentication
		// @Accept       json
		// @Produce      json
		// @Param        credentials  body      handlers.RequestUserInput  true  "Login credentials"
		// @Success      200          {object}  map[string]interface{}  "Login successful"
		// @Failure      400          {object}  map[string]string  "Invalid request"
		// @Failure      401          {object}  map[string]string  "Invalid credentials"
		// @Failure      500          {object}  map[string]string  "Server error"
		// @Router       /api/v1/login [post]
		v1.POST("/login", handlers.LoginUser(c.AuthService))
		v1.POST("/login/verify-mfa", handlers.VerifyMFALogin(c.AuthService))
		v1.POST("/refresh", handlers.RefreshToken(c.AuthService))
		v1.POST("/logout", handlers.Logout(c.AuthService))

		// @Summary      Register new user
		// @Description  Self-signup as user only; admin role returns 403
		// @Tags         authentication
		// @Accept       json
		// @Produce      json
		// @Param        user  body      handlers.SignupUserInput  true  "User registration data"
		// @Success      201   {object}  map[string]interface{}  "Registration successful"
		// @Failure      400   {object}  map[string]string  "Invalid request"
		// @Failure      403   {object}  map[string]string  "Cannot self-register as admin"
		// @Failure      409   {object}  map[string]string  "Email already registered"
		// @Failure      500   {object}  map[string]string  "Server error"
		// @Router       /api/v1/register [post]
		v1.POST("/register", handlers.SignupUser(c.AuthService))

		v1.GET("/verify-email", handlers.VerifyEmail(c.AuthService))
		v1.POST("/verify-email", handlers.VerifyEmail(c.AuthService))
		v1.POST("/resend-verification", handlers.ResendVerificationEmail(c.AuthService))
		v1.POST("/password-reset/request", handlers.RequestPasswordReset(c.AuthService))
		v1.POST("/password-reset/confirm", handlers.ConfirmPasswordReset(c.AuthService))

		oauth := v1.Group("/oauth")
		oauth.POST("/complete", handlers.OAuthComplete(c.AuthService))
		oauth.GET("/:provider/start", handlers.OAuthStart(cfg, c.AuthService))
		oauth.GET("/:provider/callback", handlers.OAuthCallback(cfg, c.AuthService))
		oauthLink := v1.Group("/oauth/:provider")
		oauthLink.Use(httpmiddleware.AuthMiddleware(c.JWT))
		oauthLink.POST("/link", handlers.OAuthLinkStart(cfg, c.AuthService))

		mfaTOTP := v1.Group("/mfa/totp")
		mfaTOTP.Use(httpmiddleware.AuthMiddleware(c.JWT))
		mfaTOTP.POST("/setup", handlers.SetupTOTP(c.AuthService, cfg))
		mfaTOTP.POST("/confirm", handlers.ConfirmTOTP(c.AuthService, cfg))
		mfaTOTP.POST("/disable", handlers.DisableTOTP(c.AuthService, cfg))

		eventGroup := v1.Group("/events")
		eventGroup.Use(httpmiddleware.AuthMiddleware(c.JWT))
		eventGroup.Use(httpmiddleware.RequireRole("admin"))
		eventGroup.Use(httpmiddleware.EventTelemetryRateLimit(cfg))
		eventGroup.GET("", handlers.ListEventLogs(c.DB))
		eventGroup.POST("", handlers.IngestClientEvent(c.Recorder))

		adminJobs := v1.Group("/admin/jobs")
		adminJobs.Use(httpmiddleware.AuthMiddleware(c.JWT))
		adminJobs.Use(httpmiddleware.RequireRole("admin"))
		adminJobsDeps := handlers.AdminJobsDeps{Cfg: cfg, RedisQueue: c.RedisQueue}
		adminJobs.GET("/health", handlers.AdminJobsHealth(adminJobsDeps))
		adminJobs.GET("/failed", handlers.AdminJobsFailed(adminJobsDeps))
		adminJobs.POST("/:id/retry", handlers.AdminJobsRetry(adminJobsDeps))

		SetupOrganizationRoutes(v1, c)
		SetupUserRoutes(v1, c)
	}
}

func healthCheckHandler(container *container.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		health, statusCode := container.HealthService.Check(ctx)
		health["timestamp"] = time.Now().Unix()
		c.JSON(statusCode, health)
	}
}
