.PHONY: help build run test test-race test-integration test-e2e-docker docker-e2e test-coverage clean docker-build docker-up docker-up-baseline docker-down docker-down-baseline docker-down-volumes docker-logs docker-restart swagger install deps db-migrate migrate-sql-up migrate-sql-down fmt-check vulncheck gosec-scan security-check secret-scan container-scan security-scan ci

# Variables
APP_NAME=goapi
# Monorepo: Docker Compose reads the repository-root `.env` (../../.env) when present.
REPO_ROOT := $(abspath $(CURDIR)/../..)
MONOREPO_MARKER := $(REPO_ROOT)/pnpm-workspace.yaml
DOCKER_ENV_FILE := $(shell if [ -f "$(MONOREPO_MARKER)" ] && [ -f "$(REPO_ROOT)/.env" ]; then printf '%s' "$(REPO_ROOT)/.env"; elif [ -f "$(CURDIR)/docker/.env" ]; then printf '%s' "$(CURDIR)/docker/.env"; else printf ''; fi)
DOCKER_ENV_FLAG := $(if $(DOCKER_ENV_FILE),--env-file $(DOCKER_ENV_FILE),)
# Pin versions to match .github/workflows/ci.yml (bump together).
GOVULNCHECK_VERSION?=v1.1.4
GOSEC_VERSION?=v2.22.0
GITLEAKS_VERSION?=v8.21.2
# Trivy container tag (aquasec/trivy — no leading v).
TRIVY_VERSION?=0.58.2
# Ephemeral tag for `make container-scan` image build.
CONTAINER_SCAN_IMAGE?=chexi-trading-api:scan
DOCKER_COMPOSE_BASE=docker/docker-compose.yml
DOCKER_COMPOSE_DEV=docker/docker-compose.dev.yml
# Local/dev stack: baseline + dev overlay (host ports, admin UIs, convenience defaults)
DOCKER_COMPOSE=docker compose -f $(DOCKER_COMPOSE_BASE) -f $(DOCKER_COMPOSE_DEV)
# Hardened baseline only (no published DB/Redis ports; set POSTGRES_* / DB_* / JWT_SECRET explicitly)
DOCKER_COMPOSE_STRICT=docker compose -f $(DOCKER_COMPOSE_BASE)
GO=go
SWAG=swag

# Colors for output
GREEN=\033[0;32m
YELLOW=\033[1;33m
NC=\033[0m # No Color

help: ## Show this help message
	@echo "$(GREEN)Available commands:$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}'
	@echo ""

# Local Development
install: ## Install Go dependencies
	@echo "$(GREEN)Installing dependencies...$(NC)"
	$(GO) mod download
	$(GO) mod tidy

deps: install ## Alias for install

run: ## Run the application locally
	@echo "$(GREEN)Running application...$(NC)"
	$(GO) run main.go

build: ## Build the application binary
	@echo "$(GREEN)Building application...$(NC)"
	$(GO) build -o $(APP_NAME) main.go
	@echo "$(GREEN)Build complete: $(APP_NAME)$(NC)"

test: ## Run tests
	@echo "$(GREEN)Running tests...$(NC)"
	$(GO) test -v ./...

test-race: ## Run tests with race detector (matches CI unit test step)
	@echo "$(GREEN)Running tests with -race...$(NC)"
	$(GO) test -race ./...

fmt-check: ## Fail if tracked .go files need gofmt -s
	@echo "$(GREEN)Checking gofmt...$(NC)"
	@test -n "$$(git rev-parse --git-dir 2>/dev/null)" || (echo "$(YELLOW)fmt-check requires a git checkout$(NC)" && exit 1)
	@test -z "$$(gofmt -s -l $$(git ls-files '*.go'))" || (echo "$(YELLOW)run: gofmt -s -w on:$(NC)" && gofmt -s -l $$(git ls-files '*.go') && exit 1)

vulncheck: ## Run govulncheck (fails on reachable vulnerabilities; needs network for vuln DB)
	@echo "$(GREEN)Running govulncheck $(GOVULNCHECK_VERSION)...$(NC)"
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

# gosec only — medium severity/confidence and exclude generated Swagger JSON (same as CI).
gosec-scan:
	@echo "$(GREEN)Running gosec $(GOSEC_VERSION)...$(NC)"
	$(GO) run github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION) \
		-fmt text \
		-severity medium \
		-confidence medium \
		-exclude-dir=api/openapi \
		./...

security-check: ## go vet + gosec (pragmatic thresholds; see Makefile / CI comments)
	@echo "$(GREEN)Running go vet...$(NC)"
	$(GO) vet ./...
	@$(MAKE) gosec-scan

# Secret scan — matches CI `secret-scan` job (requires git checkout).
secret-scan: ## Run gitleaks (pinned $(GITLEAKS_VERSION); uses .gitleaks.toml / .gitleaksignore)
	@echo "$(GREEN)Running gitleaks $(GITLEAKS_VERSION)...$(NC)"
	@test -n "$$(git rev-parse --git-dir 2>/dev/null)" || (echo "$(YELLOW)secret-scan requires a git checkout$(NC)" && exit 1)
	$(GO) install github.com/zricethezav/gitleaks/v8@$(GITLEAKS_VERSION)
	@"$(shell $(GO) env GOPATH)/bin/gitleaks" detect --source . --config .gitleaks.toml --redact

# Container + dependency scan — Trivy filesystem (repo) then API image from docker/Dockerfile (requires Docker).
container-scan: ## Trivy fs + image scan ($(CONTAINER_SCAN_IMAGE); HIGH/CRITICAL; uses .trivyignore)
	@command -v docker >/dev/null 2>&1 || (echo "$(YELLOW)container-scan requires docker on PATH$(NC)" && exit 1)
	@echo "$(GREEN)Trivy filesystem scan (docker run aquasec/trivy:$(TRIVY_VERSION))...$(NC)"
	docker run --rm \
		-v "$(CURDIR):/repo:ro" \
		-w /repo \
		aquasec/trivy:$(TRIVY_VERSION) \
		fs --scanners vuln --severity HIGH,CRITICAL --exit-code 1 --ignorefile .trivyignore .
	@echo "$(GREEN)Building $(CONTAINER_SCAN_IMAGE) for image scan...$(NC)"
	docker build -f docker/Dockerfile -t $(CONTAINER_SCAN_IMAGE) .
	@echo "$(GREEN)Trivy image scan ($(CONTAINER_SCAN_IMAGE))...$(NC)"
	docker run --rm \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v "$(CURDIR)/.trivyignore:/tmp/trivyignore:ro" \
		aquasec/trivy:$(TRIVY_VERSION) \
		image --scanners vuln --severity HIGH,CRITICAL --exit-code 1 --ignorefile /tmp/trivyignore $(CONTAINER_SCAN_IMAGE)

# Aggregate supply-chain gates (does not include govulncheck/gosec — use `make ci` for those).
security-scan: ## Run secret-scan then container-scan
	@$(MAKE) secret-scan
	@$(MAKE) container-scan

# Full static gate suite matching CI `test` job (no Docker integration — use `make test-integration` separately).
ci: fmt-check
	@echo "$(GREEN)Running CI gate suite...$(NC)"
	$(GO) mod verify
	$(GO) vet ./...
	$(GO) build ./...
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...
	@$(MAKE) gosec-scan
	$(GO) test -race ./...

test-integration: ## Run local integration tests with Docker deps + clean DB
	@echo "$(GREEN)Running local integration tests...$(NC)"
	bash ./scripts/test_integration_local.sh

test-e2e-docker: ## HTTP E2E against running Docker API (needs curl + python3; see docs/TESTING.md)
	@echo "$(GREEN)Running Docker HTTP E2E (stack must already be up)...$(NC)"
	bash ./scripts/e2e_docker.sh

docker-e2e: ## Rebuild/start dev stack then run HTTP E2E (set EMAIL_ENABLED=false in root .env for token signup)
	@$(MAKE) docker-all
	@$(MAKE) test-e2e-docker

test-coverage: ## Run tests with coverage
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

clean: ## Clean build artifacts
	@echo "$(GREEN)Cleaning build artifacts...$(NC)"
	rm -f $(APP_NAME)
	rm -f $(APP_NAME).exe
	rm -f coverage.out coverage.html
	@echo "$(GREEN)Clean complete$(NC)"

# Swagger Documentation
swagger: ## Generate Swagger documentation
	@echo "$(GREEN)Generating Swagger documentation...$(NC)"
	@which $(SWAG) > /dev/null || ($(GO) install github.com/swaggo/swag/cmd/swag@latest && echo "$(GREEN)swag installed$(NC)")
	$(GO) run github.com/swaggo/swag/cmd/swag@latest init -g main.go -d .,./internal/transport/http/handlers,./internal/transport/http/routes -o api/openapi
	@echo "$(GREEN)Swagger docs generated in api/openapi/$(NC)"

swag: ## Install swag CLI tool
	@echo "$(GREEN)Installing swag CLI...$(NC)"
	$(GO) install github.com/swaggo/swag/cmd/swag@latest
	@echo "$(GREEN)swag installed$(NC)"

# Docker Commands
docker-build: ## Build Docker images
	@echo "$(GREEN)Building Docker images...$(NC)"
	$(DOCKER_COMPOSE) build
	@echo "$(GREEN)Docker build complete$(NC)"

docker-up: ## Start Docker stack (baseline + dev overlay: DB/Redis on localhost, optional admin UIs)
	@echo "$(GREEN)Starting Docker containers (dev overlay)...$(NC)"
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) up -d
	@echo "$(GREEN)Containers started$(NC)"
	@echo "$(GREEN)API: http://localhost:8080$(NC)"
	@echo "$(GREEN)Swagger: http://localhost:8080/swagger/index.html$(NC)"

docker-up-baseline: ## Hardened baseline only (no dev overlay); requires full env — see repository root .env.example
	@echo "$(GREEN)Starting hardened baseline stack...$(NC)"
	@set -e; \
	if [ -f "$(MONOREPO_MARKER)" ]; then \
	  ENV_FILE="$(REPO_ROOT)/.env"; \
	  MSG="Create $$ENV_FILE from repository .env.example (POSTGRES_* , DB_* , JWT_SECRET , APP_ENV , DB_SSLMODE)"; \
	else \
	  ENV_FILE="$(CURDIR)/docker/.env"; \
	  MSG="Create docker/.env with required secrets, or use the monorepo root .env"; \
	fi; \
	test -f "$$ENV_FILE" || (echo "$(YELLOW)$$MSG$(NC)" && exit 1); \
	$(DOCKER_COMPOSE_STRICT) --env-file "$$ENV_FILE" up -d
	@echo "$(GREEN)Baseline containers started (DB/Redis not published to host by default)$(NC)"

docker-down: ## Stop Docker containers (dev stack)
	@echo "$(GREEN)Stopping Docker containers...$(NC)"
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) down
	@echo "$(GREEN)Containers stopped$(NC)"

docker-down-volumes: ## Stop Docker containers and remove volumes
	@echo "$(YELLOW)Stopping containers and removing volumes...$(NC)"
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) down -v
	@echo "$(GREEN)Containers stopped and volumes removed$(NC)"

docker-down-baseline: ## Stop hardened baseline stack (same env file as docker-up-baseline)
	@set -e; \
	if [ -f "$(MONOREPO_MARKER)" ]; then ENV_FILE="$(REPO_ROOT)/.env"; else ENV_FILE="$(CURDIR)/docker/.env"; fi; \
	$(DOCKER_COMPOSE_STRICT) --env-file "$$ENV_FILE" down

docker-logs: ## View Docker container logs
	@echo "$(GREEN)Viewing Docker logs...$(NC)"
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) logs -f

docker-logs-api: ## View API container logs only
	@echo "$(GREEN)Viewing API logs...$(NC)"
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) logs -f chexi-api

docker-logs-db: ## View database container logs only
	@echo "$(GREEN)Viewing database logs...$(NC)"
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) logs -f chexi-db

docker-logs-redis: ## View Redis container logs only
	@echo "$(GREEN)Viewing Redis logs...$(NC)"
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) logs -f chexi-redis

docker-restart: ## Restart Docker containers
	@echo "$(GREEN)Restarting Docker containers...$(NC)"
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) restart
	@echo "$(GREEN)Containers restarted$(NC)"

docker-rebuild: ## Rebuild and restart Docker containers
	@echo "$(GREEN)Rebuilding and restarting Docker containers...$(NC)"
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) up -d --build
	@echo "$(GREEN)Containers rebuilt and restarted$(NC)"

docker-ps: ## Show running Docker containers
	@echo "$(GREEN)Running containers:$(NC)"
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) ps

docker-shell-api: ## Open shell in API container
	@echo "$(GREEN)Opening shell in API container...$(NC)"
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) exec chexi-api sh

docker-shell-db: ## Open PostgreSQL shell in database container
	@echo "$(GREEN)Opening PostgreSQL shell...$(NC)"
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) exec chexi-db sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"'

docker-shell-redis: ## Open Redis CLI in Redis container
	@echo "$(GREEN)Opening Redis CLI...$(NC)"
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) exec chexi-redis redis-cli

docker-logs-redis-commander: ## View Redis Commander logs
	@echo "$(GREEN)Viewing Redis Commander logs...$(NC)"
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) logs -f redis-commander

docker-logs-pgadmin: ## View pgAdmin logs
	@echo "$(GREEN)Viewing pgAdmin logs...$(NC)"
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) logs -f pgadmin

docker-open-redis-commander: ## Open Redis Commander in browser
	@echo "$(GREEN)Opening Redis Commander at http://localhost:8081$(NC)"
	@echo "$(YELLOW)Credentials: REDIS_COMMANDER_HTTP_USER / REDIS_COMMANDER_HTTP_PASSWORD in repository root .env$(NC)"
	@$(if $(shell which start 2>/dev/null),start http://localhost:8081,echo "Please open http://localhost:8081 in your browser")

docker-open-pgadmin: ## Open pgAdmin in browser
	@echo "$(GREEN)Opening pgAdmin at http://localhost:5050$(NC)"
	@echo "$(YELLOW)Credentials: PGADMIN_DEFAULT_EMAIL / PGADMIN_DEFAULT_PASSWORD in repository root .env$(NC)"
	@$(if $(shell which start 2>/dev/null),start http://localhost:5050,echo "Please open http://localhost:5050 in your browser")

# Database Commands
db-migrate: ## Show migration notes (AutoMigrate on startup, or versioned SQL via migrate-sql-up)
	@echo "$(YELLOW)Default: GORM AutoMigrate runs when the API starts.$(NC)"
	@echo "$(YELLOW)Production: set USE_VERSIONED_MIGRATIONS=true and use migrate-sql-up with DATABASE_URL — see docs/MIGRATIONS.md$(NC)"

migrate-sql-up: ## Apply versioned SQL migrations (requires DATABASE_URL, installs migrate CLI via go run)
	@echo "$(GREEN)Applying SQL migrations from ./migrations ...$(NC)"
	@test -n "$$DATABASE_URL" || (echo "$(YELLOW)Set DATABASE_URL e.g. postgres://USER:PASS@HOST:5432/DBNAME?sslmode=verify-full (prod) or sslmode=disable (local dev)$(NC)" && exit 1)
	$(GO) run github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path migrations -database "$$DATABASE_URL" up

migrate-sql-down: ## Roll back one versioned SQL migration
	@test -n "$$DATABASE_URL" || (echo "$(YELLOW)Set DATABASE_URL$(NC)" && exit 1)
	$(GO) run github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path migrations -database "$$DATABASE_URL" down 1

db-seed: ## Seed database with sample data (if implemented)
	@echo "$(GREEN)Seeding database...$(NC)"
	@echo "$(YELLOW)Not implemented yet$(NC)"

# Development Workflow
dev: install run ## Install dependencies and run locally

dev-docker: docker-up docker-logs-api ## Start Docker and follow API logs

# Full Setup
setup: install swagger ## Full setup: install deps and generate Swagger docs
	@echo "$(GREEN)Setup complete!$(NC)"
	@echo "$(GREEN)Run 'make run' to start the application$(NC)"
	@echo "$(GREEN)Or 'make docker-up' to start with Docker$(NC)"

# Production Build
prod-build: clean build ## Production build: clean and build
	@echo "$(GREEN)Production build complete: $(APP_NAME)$(NC)"

# All-in-one commands
all: clean install swagger build ## Clean, install, generate docs, and build
	@echo "$(GREEN)All tasks complete!$(NC)"

docker-all: docker-down docker-build docker-up ## Full Docker rebuild: down, build, up (dev overlay)
	@echo "$(GREEN)Docker stack ready!$(NC)"

