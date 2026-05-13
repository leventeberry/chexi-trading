#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# Dedicated project name so Postgres data volume is initialized with dev-overlay credentials
# (chexi_dev) and never clashes with a developer's default `docker compose` stack / legacy admin volume.
INTEGRATION_COMPOSE_PROJECT="${INTEGRATION_COMPOSE_PROJECT:-chexi_trading_integration}"
# Dev overlay publishes Postgres/Redis on localhost (same images/settings as Makefile docker-up)
COMPOSE=(
  docker compose
  -p "${INTEGRATION_COMPOSE_PROJECT}"
  -f "${ROOT_DIR}/docker/docker-compose.yml"
  -f "${ROOT_DIR}/docker/docker-compose.dev.yml"
)

# Non-default host ports so integration can run alongside `make docker-up` (5432/6379).
POSTGRES_PUBLISH_PORT="${INTEGRATION_POSTGRES_PUBLISH_PORT:-15432}"
REDIS_PUBLISH_PORT="${INTEGRATION_REDIS_PUBLISH_PORT:-16379}"
export POSTGRES_PUBLISH_PORT REDIS_PUBLISH_PORT

DB_USER="${INTEGRATION_DB_USER:-chexi_dev}"
DB_PASS="${INTEGRATION_DB_PASS:-chexi_dev_password_change_me}"
DB_HOST="${INTEGRATION_DB_HOST:-localhost}"
DB_PORT="${INTEGRATION_DB_PORT:-${POSTGRES_PUBLISH_PORT}}"
DB_NAME="${INTEGRATION_DB_NAME:-chexi_trading_integration}"
JWT_SECRET="${INTEGRATION_JWT_SECRET:-local-integration-only-jwt-secret-key-32chars}"
REDIS_HOST="${INTEGRATION_REDIS_HOST:-localhost}"
REDIS_PORT="${INTEGRATION_REDIS_PORT:-${REDIS_PUBLISH_PORT}}"
REDIS_PASSWORD="${INTEGRATION_REDIS_PASSWORD:-}"

# `docker compose` parses the merged project (including `chexi-api`) even for `up chexi-db chexi-redis`.
# Baseline compose requires DB_SSLMODE via ${DB_SSLMODE:?...}; unset vars also trigger
# noisy warnings for api.* placeholders. Export before any compose command.
export APP_ENV="${INTEGRATION_APP_ENV:-development}"
export DB_SSLMODE="${INTEGRATION_DB_SSLMODE:-disable}"
export DB_USER DB_PASS DB_NAME JWT_SECRET
export POSTGRES_PUBLISH_PORT REDIS_PUBLISH_PORT

echo "==> Starting Postgres/Redis dependencies"
"${COMPOSE[@]}" up -d chexi-db chexi-redis

echo "==> Waiting for Postgres readiness"
for _ in $(seq 1 60); do
  if "${COMPOSE[@]}" exec -T chexi-db pg_isready -U "${DB_USER}" -d postgres >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
"${COMPOSE[@]}" exec -T chexi-db pg_isready -U "${DB_USER}" -d postgres >/dev/null

echo "==> Waiting for Redis readiness"
for _ in $(seq 1 60); do
  if "${COMPOSE[@]}" exec -T chexi-redis redis-cli ping >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
"${COMPOSE[@]}" exec -T chexi-redis redis-cli ping >/dev/null

echo "==> Resetting integration database (${DB_NAME})"
"${COMPOSE[@]}" exec -T chexi-db psql -U "${DB_USER}" -d postgres -v ON_ERROR_STOP=1 \
  -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${DB_NAME}' AND pid <> pg_backend_pid();" \
  -c "DROP DATABASE IF EXISTS \"${DB_NAME}\";" \
  -c "CREATE DATABASE \"${DB_NAME}\";"

echo "==> Running integration tests (bootstrap applies versioned migrations)"
(
  cd "${ROOT_DIR}" && \
  DB_USER="${DB_USER}" \
  DB_PASS="${DB_PASS}" \
  DB_HOST="${DB_HOST}" \
  DB_PORT="${DB_PORT}" \
  DB_NAME="${DB_NAME}" \
  DB_SSLMODE=disable \
  JWT_SECRET="${JWT_SECRET}" \
  EMAIL_ENABLED=true \
  REDIS_ENABLED=true \
  REDIS_HOST="${REDIS_HOST}" \
  REDIS_PORT="${REDIS_PORT}" \
  REDIS_PASSWORD="${REDIS_PASSWORD}" \
  QUEUE_ASYNC_ENABLED=false \
  AUTH_RESPONSE_INCLUDE_API_KEY=true \
  AUTH_RESPONSE_INCLUDE_USER=true \
  RATE_LIMIT_REQUESTS_PER_MINUTE=100000 \
  RATE_LIMIT_BURST_SIZE=100000 \
  APP_ENV=development \
  USE_VERSIONED_MIGRATIONS=true \
  MIGRATIONS_DIR="${ROOT_DIR}/migrations" \
  go test -tags=integration -race -count=1 -v ./test/integration/...
)
