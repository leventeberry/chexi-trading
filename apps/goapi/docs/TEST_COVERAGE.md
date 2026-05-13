# Test Coverage Report

This project uses a layered test strategy:

- Unit and package smoke tests for pure logic and wiring.
- Integration tests (tag `integration`) for API and repository behavior against real Postgres/Redis.
- Focus on high-value risk areas instead of chasing 100% statement coverage.

## What Is Covered

- **Auth and user flows (integration):**
  - Register, login, refresh, logout, email verification, password reset.
  - `/users/me` profile/settings contract and validation.
  - Authorization behavior for user/admin routes.
- **Repository integration (real DB):**
  - `userRepository`, `auth_token_repository`, `user_settings_repository`.
- **Cache package:**
  - No-op cache miss/no-op semantics.
  - Redis cache behavior with `miniredis` (miss/error, JSON round-trip, TTL, rate-limit counters).
- **Startup/health safety:**
  - `/health` response contract and degraded behavior.
  - Initializer production migration guardrails and missing-env failures.
- **Wiring smoke:**
  - Route registration and auth gates.
  - Container construction.
  - App/httpserver smoke construction paths.
  - Logger init/log-level parsing smoke checks.

## Intentionally Not Covered (For Now)

- **Generated/static code**:
  - `api/openapi`, model `TableName()` helpers, and other declarative/static surfaces.
- **Operational bootstrap internals that require process/network orchestration**:
  - Full `internal/app.Run()` lifecycle with real signals and live listeners.
  - Mid-flight dependency failures created by tearing down Docker services.
- **Low-value framework internals**:
  - Gin internals/middleware behavior already guaranteed by framework contracts.

## Coverage Gap Categories

- **Should test now (high value, low risk)**:
  - Recently added smoke/unit behavior in wiring packages and email sender selection.
- **Integration-covered enough**:
  - Repository CRUD semantics and core auth/api workflows are covered via integration harness.
- **Generated/static/no-test-needed**:
  - OpenAPI generated docs, simple model table name methods.
- **Wiring-only/no further tests needed (for now)**:
  - Thin constructors and route setup paths that are already smoke-tested.

## Run Coverage Locally

```bash
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

Run the full validation matrix:

```bash
go test -race ./...
make ci
make test-integration
go test -race -coverprofile=coverage.out ./...
```

## Recommended Future Test Areas

- Add focused service-level tests for selected `services` methods with fakes for repositories to raise confidence where integration coverage is broad but coarse.
- Add handler-level unit tests for a few critical error-mapping paths (not full endpoint duplication).
- Add narrow startup tests around `config.Load()` edge combinations in staging/production policy paths.
