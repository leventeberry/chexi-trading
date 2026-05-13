SHELL := /bin/bash

ROOT_DIR := $(CURDIR)
API_DIR := apps/goapi
ROOT_ENV_FILE := .env
DOCKER_COMPOSE_BASE := infra/docker/docker-compose.yml
DOCKER_COMPOSE_DEV := infra/docker/docker-compose.dev.yml
DOCKER_COMPOSE := docker compose -f $(DOCKER_COMPOSE_BASE) -f $(DOCKER_COMPOSE_DEV)
DOCKER_COMPOSE_STRICT := docker compose -f $(DOCKER_COMPOSE_BASE)
DOCKER_COMPOSE_LEGACY := COMPOSE_PROJECT_NAME=docker docker compose -f $(DOCKER_COMPOSE_BASE) -f $(DOCKER_COMPOSE_DEV)
DOCKER_COMPOSE_TRAEFIK := docker compose -f $(DOCKER_COMPOSE_BASE) -f $(DOCKER_COMPOSE_DEV) -f infra/docker/docker-compose.traefik.yml
DOCKER_ENV_FILE := $(if $(wildcard $(ROOT_ENV_FILE)),$(ROOT_ENV_FILE),)
DOCKER_ENV_FLAG := $(if $(DOCKER_ENV_FILE),--env-file $(DOCKER_ENV_FILE),)

.PHONY: help api-help api-dev dev dev-local-api web-dev run build install deps test test-race fmt-check \
	vulncheck gosec-scan security-check secret-scan container-scan security-scan ci \
	test-integration test-e2e-docker docker-e2e test-coverage clean swagger swag \
	docker-build docker-up docker-up-baseline docker-down docker-down-baseline \
	docker-down-volumes docker-logs docker-logs-api docker-logs-db docker-logs-redis \
	docker-up-traefik docker-restart docker-rebuild docker-ps docker-shell-api docker-shell-db \
	docker-shell-redis docker-logs-redis-commander docker-logs-pgadmin \
	docker-open-redis-commander docker-open-pgadmin db-migrate migrate-sql-up \
	migrate-sql-down db-seed dev-docker setup prod-build all docker-all

help: ## Show monorepo commands
	@echo "chexi-trading monorepo"
	@echo ""
	@echo "Root-first commands:"
	@echo "  make dev              Start full Docker Compose stack (API runs in the chexi-api container)"
	@echo "  make dev-local-api    Start Compose, stop container api, run Go API on host (uses root .env PORT)"
	@echo "  make api-dev          Run the Go API on host only (expects DB/Redis reachable; often after docker-up)"
	@echo "  make web-dev          Run the admin UI (apps/shadcn-admin, Vite)"
	@echo "  make docker-up        Start Docker Compose from the repo root"
	@echo "  make docker-up-traefik  Same as docker-up plus Traefik (api.localhost → API)"
	@echo "  make test             Run aggregate tests"
	@echo "  make ci               Run aggregate CI gates"
	@echo ""
	@echo "API compatibility commands:"
	@echo "  make run/build/test/test-integration (test-e2e-docker is optional/manual)"
	@echo ""
	@echo "Use 'make api-help' for all API-local targets."

api-help:
	@$(MAKE) -C $(API_DIR) help

api-dev:
	@echo "Starting API from $(API_DIR)"
	@set -a; \
	if [ -f "$(ROOT_ENV_FILE)" ]; then source "$(ROOT_ENV_FILE)"; else echo "No root .env found; relying on exported environment and API defaults"; fi; \
	set +a; \
	$(MAKE) -C $(API_DIR) run

run: api-dev

dev: docker-up ## Full local stack: DB, Redis, admin tools, and API in Docker

dev-local-api: docker-up ## Compose deps + API on host (stops the api service to avoid port 8080 clash)
	@echo "Stopping containerized chexi-api so host process can bind to PORT (see root .env)..."
	@$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) stop chexi-api
	@$(MAKE) api-dev

web-dev:
	@if [ -f "apps/shadcn-admin/package.json" ]; then \
		pnpm --filter chexi-trading-admin dev; \
	else \
		echo "apps/shadcn-admin is missing; add the admin app before running make web-dev."; \
	fi

install deps test-race fmt-check vulncheck gosec-scan security-check secret-scan \
container-scan security-scan test-coverage clean swagger swag db-migrate migrate-sql-up \
migrate-sql-down db-seed setup prod-build all:
	@$(MAKE) -C $(API_DIR) $@

build:
	@$(MAKE) -C $(API_DIR) build

test:
	@$(MAKE) -C $(API_DIR) test
	@if [ -f "apps/shadcn-admin/package.json" ]; then pnpm --filter chexi-trading-admin test; fi

ci:
	@$(MAKE) -C $(API_DIR) ci
	@if [ -f "apps/shadcn-admin/package.json" ]; then pnpm --filter chexi-trading-admin lint && pnpm --filter chexi-trading-admin build; fi

test-integration:
	@$(MAKE) -C $(API_DIR) test-integration

test-e2e-docker:
	@EMAIL_ENABLED=false EMAIL_DRIVER=mock EMAIL_PROVIDER=mock EMAIL_LOG_INCLUDE_BODY=false RESEND_API_KEY= $(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) up -d --force-recreate chexi-api
	@$(MAKE) -C $(API_DIR) test-e2e-docker

docker-build:
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) build

docker-up:
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) up -d

docker-up-traefik:
	$(DOCKER_COMPOSE_TRAEFIK) $(DOCKER_ENV_FLAG) up -d

docker-up-baseline:
	@test -n "$(DOCKER_ENV_FILE)" || (echo "Create .env from .env.example before running the hardened baseline stack" && exit 1)
	$(DOCKER_COMPOSE_STRICT) --env-file "$(DOCKER_ENV_FILE)" up -d

docker-down:
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) down

docker-down-baseline:
	@test -n "$(DOCKER_ENV_FILE)" || (echo "Create .env from .env.example before stopping the hardened baseline stack" && exit 1)
	$(DOCKER_COMPOSE_STRICT) --env-file "$(DOCKER_ENV_FILE)" down

docker-down-volumes:
	$(DOCKER_COMPOSE_LEGACY) $(DOCKER_ENV_FLAG) down -v --remove-orphans
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) down -v

docker-logs:
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) logs -f

docker-logs-api:
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) logs -f chexi-api

docker-logs-db:
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) logs -f chexi-db

docker-logs-redis:
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) logs -f chexi-redis

docker-restart:
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) restart

docker-rebuild:
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) up -d --build

docker-ps:
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) ps

docker-shell-api:
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) exec chexi-api sh

docker-shell-db:
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) exec chexi-db sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"'

docker-shell-redis:
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) exec chexi-redis redis-cli

docker-logs-redis-commander:
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) logs -f redis-commander

docker-logs-pgadmin:
	$(DOCKER_COMPOSE) $(DOCKER_ENV_FLAG) logs -f pgadmin

docker-open-redis-commander:
	@set -a; \
	if [ -f "$(ROOT_ENV_FILE)" ]; then . "$(ROOT_ENV_FILE)"; fi; \
	set +a; \
	RC_PORT="$${REDIS_COMMANDER_PORT:-8081}"; \
	RC_HOST="$${REDIS_COMMANDER_PUBLISH_HOST:-127.0.0.1}"; \
	echo "Open Redis Commander at http://$${RC_HOST}:$${RC_PORT}"

docker-open-pgadmin:
	@set -a; \
	if [ -f "$(ROOT_ENV_FILE)" ]; then . "$(ROOT_ENV_FILE)"; fi; \
	set +a; \
	PGA_PORT="$${PGADMIN_PUBLISH_PORT:-5050}"; \
	PGA_HOST="$${PGADMIN_PUBLISH_HOST:-127.0.0.1}"; \
	echo "Open pgAdmin at http://$${PGA_HOST}:$${PGA_PORT}"

dev-docker: docker-up docker-logs-api

docker-all: docker-down-volumes docker-build docker-up

docker-e2e: docker-all test-e2e-docker
