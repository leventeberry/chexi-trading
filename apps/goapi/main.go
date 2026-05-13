package main

import "goapi/internal/app"

// @title           chexi-trading API
// @version         1.0
// @description     HTTP API for the chexi-trading monorepo (Go/Gin): user management, JWT authentication, RBAC, and security/logging middleware. Coinbase ingestion and trading-specific routes will layer on this baseline.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	app.Run()
}
