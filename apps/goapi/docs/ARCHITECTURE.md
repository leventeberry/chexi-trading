# Architecture Documentation

## Overview

This codebase follows **enterprise-grade design principles** with a clean, modular architecture using multiple design patterns.

## Design Patterns Implemented

### 1. **Repository Pattern**

- **Location**: `repositories/`
- **Purpose**: Abstracts data access layer from business logic
- **Benefits**:
  - Easy to swap database implementations
  - Testable with mock repositories
  - Single Responsibility Principle

**Structure:**

```code
repositories/
├── interfaces.go      # Repository interfaces
├── userRepository.go  # User repository implementation
└── errors.go         # Repository-specific errors
```

### 2. **Service Layer Pattern**

- **Location**: `services/`
- **Purpose**: Contains business logic separate from HTTP handling
- **Benefits**:
  - Reusable business logic
  - Testable without HTTP layer
  - Separation of concerns

**Structure:**

```code
services/
├── interfaces.go    # Service interfaces
├── dto.go          # Data Transfer Objects
├── userService.go  # User business logic
├── authService.go  # Authentication business logic
└── errors.go       # Service-specific errors
```

### 3. **Dependency Injection Container**

- **Location**: `container/`
- **Purpose**: Manages all application dependencies
- **Benefits**:
  - Single source of truth for dependencies
  - Easy to test with mock containers
  - Dependencies wired explicitly (JWT/auth helpers injected via container)

**Structure:**

```code
container/
└── container.go  # DI container with all dependencies
```

### 4. **Cache Abstraction Layer**

- **Location**: `cache/`
- **Purpose**: Provides caching abstraction with Redis and no-op implementations
- **Benefits**:
  - Optional caching (graceful degradation)
  - Easy to swap implementations
  - Testable with mock cache
  - Supports both user caching and rate limiting

**Structure:**

```code
cache/
├── interfaces.go      # Cache interface definition
├── redis_cache.go     # Redis implementation
├── noop_cache.go      # No-op implementation (when Redis disabled)
├── constants.go       # Cache key patterns and TTL values
└── errors.go          # Cache-specific errors
```

**Cache Strategy:**

- **Cache-Aside Pattern**: Application manages cache, checks cache before database
- **Dual-Key Caching**: Stores user data by both ID and email for efficient lookups
- **Automatic Invalidation**: Cache invalidated on user updates/deletes
- **TTL-Based Expiration**: User cache expires after 15 minutes
- **Distributed Rate Limiting**: Redis enables shared rate limits across instances

### 5. **Background Job Queue**

- **Location**: `internal/queue/` with job handlers under `internal/queue/jobs/`
- **Purpose**: Async side effects (starting with transactional email) with retries, scheduling via Redis sorted sets, and dead-letter capture after max attempts.
- **Primary backend**: Redis (`RedisQueue`) stores job envelopes (`queue:v1:job:{id}`) and a due-time index (`queue:v1:due` ZSET, score = run-at UNIX ms).
- **Fallback**: When Redis is unavailable/disabled or async dispatch is turned off (`QUEUE_ASYNC_ENABLED=false`), `InlineQueue` runs registered handlers **on the enqueue call path** so jobs are never silently dropped (warning logs + `queue.inline_execute` events).
- **Worker**: `internal/queue/worker.go` polls Redis with configurable concurrency; graceful shutdown waits for in-flight handlers up to `QUEUE_SHUTDOWN_TIMEOUT_SEC` after HTTP server shutdown; Redis is closed only after the worker stops.
- **Bootstrap wiring**: `initializers.Bootstrap()` builds the registry (email jobs today), creates `queue.NewBundle(...)`, and exposes `QueueEnqueue` / `QueueWorker` on `AppDependencies`. `internal/app/run.go` starts the worker when non-nil. Optional `JOB_QUEUE_ENABLED` overrides `QUEUE_ENABLED`; `JOB_WORKER_ENABLED=false` keeps Redis enqueue but omits the worker.

## Architecture Layers

```code
┌─────────────────────────────────────┐
│         HTTP Layer                  │
│  internal/transport/http (routes,  │
│  handlers, middleware)            │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│         Service Layer                │
│  (Business Logic, Validation)       │
│  (Cache-Aside Pattern)              │
└──────────────┬──────────────────────┘
               │
               ├──────────────────────┐
               │                      │
               ▼                      ▼
┌─────────────────────────┐  ┌─────────────────────────┐
│      Cache Layer         │  │    Repository Layer      │
│  (Redis/No-Op Cache)     │  │  (Data Access, DB Ops)  │
└────────────┬────────────┘  └────────────┬────────────┘
             │                             │
             │                             ▼
             │              ┌─────────────────────────┐
             │              │      Database            │
             │              │  (PostgreSQL via GORM)   │
             │              └─────────────────────────┘
             │
             ▼
┌─────────────────────────┐
│      Redis Cache         │
│  (Optional, Distributed) │
└─────────────────────────┘
```

## Dependency Flow

1. **main.go** → `internal/app` bootstraps Redis (if enabled) and builds `Container`
2. **Container** → Wires repositories and services (constructors in `container/container.go`)
3. **Routes** (`internal/transport/http/routes`) → Receives `Container`, registers handlers
4. **Handlers** (`internal/transport/http/handlers`) → Call services only (not database directly)
5. **Services** → Use cache (cache-aside pattern) and repositories for data access
6. **Cache** → Redis or No-Op implementation (based on configuration)
7. **Repositories** → Use GORM for database operations

### Cache Flow (Cache-Aside Pattern)

**GetUserByID Example:**

1. Service calls `cache.GetUserByID()`
2. If cache hit → return cached user
3. If cache miss → query database via repository
4. Store result in cache (both ID and email keys)
5. Return user to controller

**UpdateUser Example:**

1. Service updates user in database via repository
2. Service invalidates cache: `cache.DeleteUser(id, email)`
3. Service stores updated user in cache
4. Return updated user to the HTTP handler

## Key Principles Applied

### SOLID Principles

1. **Single Responsibility Principle (SRP)**
   - Handlers: HTTP requests/responses only
   - Services: Business logic only
   - Repositories: Data access only

2. **Open/Closed Principle (OCP)**
   - Interfaces allow extension without modification
   - New repositories/services can be added easily

3. **Liskov Substitution Principle (LSP)**
   - Any implementation of an interface can be substituted
   - Enables easy mocking for testing

4. **Interface Segregation Principle (ISP)**
   - Small, focused interfaces
   - Services don't depend on unused methods

5. **Dependency Inversion Principle (DIP)**
   - High-level modules depend on abstractions (interfaces)
   - Low-level modules implement interfaces

### Other Patterns

- **Dependency Injection**: All dependencies injected via constructor
- **Interface-Based Design**: Everything depends on interfaces
- **Composition root**: `container` constructs dependencies
- **Repository Pattern**: Data access abstraction
- **Service Layer**: Business logic separation

## Benefits of This Architecture

1. **Testability**: Easy to mock interfaces for unit testing
2. **Maintainability**: Clear separation of concerns
3. **Scalability**: Easy to add new features
4. **Flexibility**: Swap implementations without changing business logic
5. **No Global State**: All dependencies injected
6. **Type Safety**: Interfaces ensure contracts are met

## Example: Adding a New Feature

To add a new feature (e.g., "Products"):

1. **Create Model**: `models/product.go`
2. **Create Repository Interface**: `repositories/productRepository.go`
3. **Implement Repository**: `repositories/productRepository.go`
4. **Create Service Interface**: `services/productService.go`
5. **Implement Service**: `services/productService.go`
6. **Add to Container**: `container/container.go` (wire repository and service)
7. **Create Handler**: `internal/transport/http/handlers/product.go`
8. **Add Routes**: `internal/transport/http/routes/` (register handlers)

Each layer is independent and testable!

## Testing Strategy

With this architecture, you can:

1. **Unit Test Services**: Mock repositories
2. **Unit Test Handlers**: Mock services
3. **Integration Test Repositories**: Use test database
4. **Integration Test Services**: Use real repositories with test DB
5. **E2E Tests**: Test full stack

## Migration from Old Architecture

The old architecture had:

- HTTP layer directly using `*gorm.DB`
- Business logic in handlers
- Global `initializers.DB` variable
- No separation of concerns

The new architecture:

- ✅ Handlers use services
- ✅ Services contain business logic
- ✅ Repositories handle data access
- ✅ Dependency Injection via Container
- ✅ Explicit wiring in the container
- ✅ Interface-based design

## File Structure

```code
goapi/
├── api/openapi/        # Generated Swagger/OpenAPI docs
├── cache/              # Cache abstraction layer
├── container/          # Dependency injection container (wires repos + services)
├── internal/
│   ├── app/            # Application entry (server lifecycle)
│   ├── infra/auth/     # JWT parsing and signing helpers
│   └── transport/http/
│       ├── handlers/   # HTTP handlers (thin layer)
│       ├── middleware/ # Auth, logging, rate limiting
│       └── routes/     # Route registration
├── models/             # Data models
├── repositories/       # Data access layer
├── services/           # Business logic layer
├── initializers/       # App initialization (DB + Redis)
└── main.go             # Delegates to internal/app
```

## Cache Layer Details

### Cache Interface

The `cache.Cache` interface provides a unified API for:

- User caching operations (GetUserByID, SetUserByID, GetUserByEmail, SetUserByEmail)
- Cache invalidation (DeleteUser, DeleteUserByID, DeleteUserByEmail)
- Rate limiting operations (IncrementRateLimit, GetRateLimit, ResetRateLimit)
- General cache operations (Get, Set, Delete, Exists)

### Cache Implementations

**Redis Cache (`redis_cache.go`):**

- Wraps `github.com/redis/go-redis/v9` client
- Serializes user objects as JSON
- Uses key patterns: `user:id:{id}`, `user:email:{email}`, `ratelimit:{key}`
- TTL-based expiration (15 minutes for users, 1 minute for rate limits)

**No-Op Cache (`noop_cache.go`):**

- Used when Redis is disabled or unavailable
- All operations are no-ops (do nothing)
- Returns cache miss for all get operations
- Ensures application works without Redis

### Cache Configuration

**TTL Values** (defined in `cache/constants.go`):

- `UserCacheTTL`: 15 minutes - Balances freshness with efficiency
- `RateLimitWindow`: 1 minute - Matches rate limiter configuration

**Key Patterns:**

- User by ID: `user:id:{id}`
- User by Email: `user:email:{email}`
- Rate Limit: `ratelimit:{ip}`

### Cache Invalidation Strategy

1. **On User Update**:
   - Delete all cached entries for the user (ID and email keys)
   - Store updated user in cache

2. **On User Delete**:
   - Delete all cached entries for the user

3. **On Email Change**:
   - Delete old email cache key
   - Delete ID cache key
   - Store updated user with new keys

4. **TTL Expiration**:
   - Cache entries automatically expire after TTL
   - Next request will refresh from database

### Rate Limiting with Redis

When Redis is enabled:

- Rate limiting uses Redis INCR with expiration
- Distributed across all API instances
- Sliding window approach (1-minute window)
- Automatically falls back to in-memory if Redis unavailable

## Best Practices Followed

1. ✅ **Minimal globals**: `config.AppConfig` is set during `config.Load()` for legacy helpers (e.g. rate-limit bootstrap); JWT signing/verification uses an injected `internal/infra/auth.Manager` from the container.
2. ✅ **Interface-based design** for testability
3. ✅ **Dependency injection** throughout
4. ✅ **Composition root** in `container` for wiring
5. ✅ **Repository pattern** for data access
6. ✅ **Service layer** for business logic
7. ✅ **Cache abstraction** for optional caching
8. ✅ **Cache-aside pattern** for cache management
9. ✅ **Graceful degradation** (works without Redis)
10. ✅ **Error handling** with typed errors
11. ✅ **Separation of concerns** at every level

## RBAC roles

- Canonical role strings and helpers: [`internal/rbac/roles.go`](../internal/rbac/roles.go) (`RoleUser`, `RoleAdmin`, `IsValidRole`, `IsAdminRole`).
- **Register** (`authService.Register`): always persists role `user`; requests that try to self-register as `admin` return `ErrForbiddenAdminRegistration` (HTTP 403).
- **Create / update user** (`userService`): setting the persisted role to `admin` requires the caller’s JWT role to be `admin` (`ErrInsufficientPrivileges` / HTTP 403 otherwise).

## Login and register response shape

Login and register return JSON under control of config (see `config.Config.AuthResponse`), loaded like other booleans (`REDIS_ENABLED` style: value must be `"true"`):

- `AUTH_RESPONSE_INCLUDE_API_KEY` — include `api_key` in the `token` object (default off).
- `AUTH_RESPONSE_INCLUDE_USER` — include `user` with `id` and `email` (default off).

`jwt_token` is always included under `token`. Disabling `api_key` in the response does not remove it from JWT claims inside `internal/infra/auth` token generation.

## Future optional layout (not required now)

If the codebase grows, consider incremental changes rather than a big-bang reorganization:

- **`cmd/goapi/main.go`** — Idiomatic when adding a second binary; move root `main.go` and update the Docker `go build` path and Makefile targets.
- **Nest shared infra** — Move `cache/`, `config/`, and `logger/` under something like `internal/infra/` to reduce top-level packages (purely organizational; update imports once).
- **Vertical slices** (`internal/user`, `internal/auth`) — Worth it when domains have stable boundaries and separate ownership; avoid duplicating DB/cache wiring until needed.

Until then, the current layered layout (`internal/transport/http`, `services`, `repositories`) remains appropriate for a single bounded context.
