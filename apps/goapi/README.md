# chexi-trading API

HTTP API for the **chexi-trading** monorepo (Go/Gin): user management, JWT authentication, role-based access control (RBAC), and security/logging middleware. Trading analytics and Coinbase ingestion will build on this service.

## Table of Contents

- [Documentation](#documentation)
- [Features](#features)
- [Tech Stack](#tech-stack)
- [Project Structure](#project-structure)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Makefile Commands](#makefile-commands)
- [Docker Setup](#docker-setup)
- [API Documentation](#api-documentation)
- [API Endpoints](#api-endpoints)
  - [Public Endpoints](#public-endpoints)
  - [Protected Endpoints](#protected-endpoints-require-authentication)
- [Authentication](#authentication)
- [User Model](#user-model)
- [Middleware](#middleware)
- [Caching](#caching)
- [Database](#database)
- [Error Responses](#error-responses)
- [Security Features](#security-features)
- [Development](#development)
- [License](#license)
- [Contributing](#contributing)

## Documentation

Extended docs live in **[docs/](docs/)**. See **[docs/README.md](docs/README.md)** for an index (architecture, testing, ADRs, backlog, and migration history).

**Security and operations (production):** **[docs/SECURITY.md](docs/SECURITY.md)** — env checklist, secret rotation, bootstrap admin, MFA/webhook keys, JWT and webhook caveats, rate limits, Docker/migrations, CI scanners, and incident basics.

## Features

- 🔐 **JWT Authentication** - Secure token-based authentication with 60-day expiration
- 👥 **User Management** - Full CRUD operations for user accounts
- 🛡️ **Role-Based Access Control** - Support for `user` and `admin` roles
- 🔒 **Password Security** - Bcrypt password hashing with secure defaults
- ⚡ **Rate Limiting** - IP-based rate limiting (60 requests/minute with burst of 10), supports Redis for distributed rate limiting
- 🚀 **Redis Caching** - Optional Redis integration for user caching and distributed rate limiting
- 📝 **Request Logging** - Comprehensive HTTP request logging with status codes
- 🗄️ **Database Migrations** - Automatic database schema migration using GORM
- 🏥 **Health Check** - Root endpoint for API status verification
- 📚 **Swagger/OpenAPI Documentation** - Interactive API documentation with Swagger UI

## Tech Stack

- **Go 1.25.10** - Programming language
- **Gin** - HTTP web framework
- **GORM** - ORM library for database operations
- **PostgreSQL** - Database (via GORM PostgreSQL driver)
- **Redis** - Caching and distributed rate limiting (optional)
- **JWT (golang-jwt/jwt/v5)** - JSON Web Token implementation
- **Bcrypt (golang.org/x/crypto)** - Password hashing
- **godotenv** - Environment variable management
- **Swagger/OpenAPI (swaggo)** - API documentation and interactive UI

## Project Structure

```
goapi/
├── api/openapi/              # Swagger/OpenAPI documentation (generated)
├── cache/                    # Cache abstraction (Redis / no-op)
├── container/                # Dependency wiring (repos + services)
├── internal/
│   ├── app/                  # Server bootstrap and graceful shutdown
│   ├── infra/auth/           # JWT helpers
│   └── transport/http/
│       ├── handlers/         # HTTP handlers (auth, users)
│       ├── middleware/       # Auth, logging, rate limiting
│       └── routes/           # Route registration
├── models/                   # GORM models
├── repositories/             # Data access
├── services/                 # Business logic
├── initializers/             # DB, Redis, config bootstrap
├── test/integration/       # Integration tests (needs DB)
├── docker/
│   ├── Dockerfile              # API image build (non-root runtime user)
│   ├── docker-compose.yml      # Hardened baseline (no weak defaults; DB/Redis internal)
│   └── docker-compose.dev.yml  # Local overlay: localhost ports + admin UIs + dev defaults
├── main.go
├── Makefile
├── go.mod
└── README.md
```

## Prerequisites

### For Local Development:
- Go 1.25.10 or higher (match `go` directive in `go.mod`)
- PostgreSQL database server (14+)
- Redis server (optional, for caching and distributed rate limiting)
- Git (for cloning the repository)

### For Docker:
- Docker and Docker Compose installed
- Git (for cloning the repository)

## Installation

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd goapi
   ```

2. **Install dependencies**
   
   Using Make (recommended):
   ```bash
   make install
   ```
   
   Or manually:
   ```bash
   go mod download
   go mod tidy
   ```

3. **Set up environment variables**
   
   For **local `make run`**, create a `.env` in the repo root (copy from **[`.env.example`](.env.example)**). For **Docker**, copy **[`docker/.env.example`](docker/.env.example)** → `docker/.env` (see [Docker Setup](#docker-setup)).
   
   Minimal local example (same variables appear with comments in `.env.example`):
   ```env
   # Database Configuration
   DB_USER=your_db_user
   DB_PASS=your_db_password
   DB_HOST=localhost
   DB_PORT=5432
   DB_NAME=your_database_name
   # DB_SSLMODE defaults to disable in development/test when unset.
   # Required in staging/production:
   # - staging: require | verify-ca | verify-full
   # - production: verify-full
   DB_SSLMODE=disable

   # JWT Secret (use a strong random string)
   JWT_SECRET=your_super_secret_jwt_key_here

   # Server Port (optional, defaults to 8080)
   PORT=8080

   # HTTP server timeouts (optional; defaults shown)
   # HTTP_READ_HEADER_TIMEOUT_SEC=5
   # HTTP_READ_TIMEOUT_SEC=30
   # HTTP_WRITE_TIMEOUT_SEC=30
   # HTTP_IDLE_TIMEOUT_SEC=120

   # OpenAPI host shown in Swagger UI (optional)
   # SWAGGER_HOST=localhost:8080

   # Runtime environment (optional; APP_ENV/GO_ENV/ENV)
   # APP_ENV=development

   # Swagger exposure control (optional)
   # - development/test/local defaults to enabled when unset
   # - staging/production defaults to disabled when unset
   # SWAGGER_ENABLED=true

   # Use golang-migrate SQL files instead of GORM AutoMigrate (production-style)
   # USE_VERSIONED_MIGRATIONS=false
   # MIGRATIONS_DIR=migrations

   # Redis Configuration (optional)
   # Set REDIS_ENABLED=true to enable Redis caching
   # If Redis is disabled, the application will use a no-op cache
   REDIS_ENABLED=false
   REDIS_HOST=localhost
   REDIS_PORT=6379
   REDIS_PASSWORD=
   REDIS_TLS_ENABLED=false
   REDIS_TLS_SERVER_NAME=
   REDIS_TLS_CA_CERT=
   REDIS_TLS_INSECURE_SKIP_VERIFY=false

   # Background job queue (optional; Redis-backed async worker when REDIS_ENABLED=true)
   # QUEUE_ENABLED=true                  # master toggle (default on)
   # QUEUE_ASYNC_ENABLED=true            # set false to force inline execution even with Redis
   # JOB_QUEUE_ENABLED=                # if set (true/false), overrides QUEUE_ENABLED (inline fallback when false)
   # JOB_WORKER_ENABLED=true           # if false, jobs enqueue to Redis but in-process worker does not run
   # JOB_MAX_ATTEMPTS=                 # if set, overrides QUEUE_MAX_ATTEMPTS default for new jobs
   # JOB_RETRY_DELAY_SECONDS=          # if set (>0), fixed delay between retries instead of exponential backoff
   # QUEUE_WORKERS=2
   # QUEUE_POLL_INTERVAL_MS=500
   # QUEUE_MAX_ATTEMPTS=5
   # QUEUE_INITIAL_BACKOFF_MS=1000
   # QUEUE_MAX_BACKOFF_MS=300000
   # QUEUE_SHUTDOWN_TIMEOUT_SEC=30
   # QUEUE_DEAD_LETTER_MAX_CAP=10000

   # Login/register JSON (optional; unset = false, same pattern as REDIS_ENABLED)
   # When both unset, response is { "token": { "jwt_token": "..." } } only.
   # AUTH_RESPONSE_INCLUDE_API_KEY=true   # add api_key next to jwt_token in token object
   # AUTH_RESPONSE_INCLUDE_USER=true      # add user { "id", "email" }

   # Email verification / password reset (optional)
   # Defaults: enabled in development with log sink (full body may contain tokens); disabled in staging/production unless EMAIL_ENABLED=true.
   # EMAIL_ENABLED=true
   # EMAIL_PROVIDER=log
   # EMAIL_FROM=noreply@localhost
   # APP_PUBLIC_URL=http://localhost:8080   # used for verification/reset links in outbound mail
   # EMAIL_VERIFICATION_TTL_HOURS=48
   # EMAIL_PASSWORD_RESET_TTL_HOURS=1
   # ORG_INVITATION_TTL_HOURS=168
   # EMAIL_RESEND_MIN_INTERVAL_SECONDS=60   # minimum gap between issuing verification or reset tokens per user
   # RESEND_API_KEY=
   # EMAIL_REDIRECT_ALL_TO=                 # dev/test only: send all transactional mail to one inbox
   ```

4. **Set up the database**
   
   The application will automatically create the necessary tables using GORM AutoMigrate. Ensure your PostgreSQL database exists and is accessible with the credentials provided in `.env`.

5. **Set up Redis (optional)**
   
   If you want to use Redis for caching and distributed rate limiting:
   - Install Redis locally or use Docker (pin matches compose baseline): `docker run -d -p 6379:6379 redis:7.4.9-alpine3.21`
   - Set `REDIS_ENABLED=true` in your `.env` file
   - Configure `REDIS_HOST` and `REDIS_PORT` if different from defaults
   - For staging/production with a non-local Redis host, set `REDIS_TLS_ENABLED=true` (and optionally `REDIS_TLS_SERVER_NAME` / `REDIS_TLS_CA_CERT`)
   - `REDIS_TLS_INSECURE_SKIP_VERIFY=true` is rejected in staging/production
   - If Redis is not available, the application will gracefully degrade to in-memory caching

6. **Run the application**
   
   Using Make (recommended):
   ```bash
   make run
   ```
   
   Or manually:
   ```bash
   go run main.go
   ```

   The server will start on `http://localhost:8080` (or the port specified in `PORT` environment variable).

7. **Generate Swagger documentation** (if you modify API endpoints)
   
   Using Make (recommended):
   ```bash
   make swagger
   ```
   
   Or manually:
   ```bash
   # Install swag CLI tool
   go install github.com/swaggo/swag/cmd/swag@latest
   
   # Generate Swagger docs from annotations
   make swagger
   ```

## Makefile Commands

This project includes a Makefile with convenient commands for common tasks. Run `make help` to see all available commands.

### Quick Start Commands

```bash
# Show all available commands
make help

# Full setup (install deps + generate Swagger docs)
make setup

# Run locally
make run

# Start with Docker
make docker-up

# View Docker logs
make docker-logs-api
```

### Common Commands

**Local Development:**
- `make install` or `make deps` - Install Go dependencies
- `make run` - Run the application locally
- `make build` - Build the application binary
- `make test` - Run tests
- `make test-race` - Run tests with the race detector (same as CI unit tests)
- `make ci` - Run CI parity gates locally (`gofmt`, `go mod verify`, `vet`, `build`, `govulncheck`, `gosec`, race tests; see [docs/TESTING.md](docs/TESTING.md))
- `make vulncheck` - Run **govulncheck** (needs network for vuln DB)
- `make security-check` - Run **go vet** + **gosec**
- `make secret-scan` - Run **gitleaks** on the repo (needs a git checkout; pin matches CI — see [docs/TESTING.md](docs/TESTING.md#secret-scanning-gitleaks))
- `make container-scan` - Run **Trivy** filesystem + Docker image scans (`docker/Dockerfile` tag `goapi:scan`; requires Docker — see [docs/TESTING.md](docs/TESTING.md#container--dependency-scanning-trivy))
- `make security-scan` - Run `secret-scan` then `container-scan` (supply-chain gates only; does not replace `make ci`)
- `make test-integration` - Run local integration tests (auto-starts Docker Postgres/Redis, resets `goapi_integration`, applies SQL migrations, then runs `go test -tags=integration -race -v ./test/integration/...`)
- `make test-e2e-docker` - HTTP end-to-end checks against the **already running** dev Docker API (`curl` + `python3`; see [docs/TESTING.md](docs/TESTING.md#docker-e2e-http))
- `make docker-e2e` - `make docker-all` then `make test-e2e-docker` (for signup tokens, set `EMAIL_ENABLED=false` in `docker/.env`)
- `make test-coverage` - Run tests with coverage report
- `make clean` - Clean build artifacts (binary, coverage files)

**Docker:**
- `make docker-build` - Build Docker images
- `make docker-up` - Start Docker containers in detached mode
- `make docker-down` - Stop Docker containers
- `make docker-down-volumes` - Stop containers and remove volumes (clears database)
- `make docker-logs` - View all container logs (follow mode)
- `make docker-logs-api` - View API container logs only
- `make docker-logs-db` - View database container logs only
- `make docker-logs-redis` - View Redis container logs only
- `make docker-restart` - Restart Docker containers
- `make docker-rebuild` - Rebuild and restart containers
- `make docker-ps` - Show running Docker containers
- `make docker-shell-api` - Open shell in API container
- `make docker-shell-db` - Open PostgreSQL shell in database container
- `make docker-shell-redis` - Open Redis CLI in Redis container
- `make docker-open-redis-commander` - Open Redis Commander web UI in browser
- `make docker-open-pgadmin` - Open pgAdmin web UI in browser

**Documentation:**
- `make swagger` - Generate Swagger documentation (auto-installs swag if needed)
- `make swag` - Install swag CLI tool

**Database:**
- `make db-migrate` - Show migration notes (AutoMigrate vs versioned SQL)
- `make migrate-sql-up` / `make migrate-sql-down` - Apply or roll back SQL in `migrations/` (needs `DATABASE_URL`)
- `make db-seed` - Seed database with sample data (placeholder)

**All-in-one:**
- `make dev` - Install deps and run locally
- `make dev-docker` - Start Docker and follow API logs
- `make setup` - Full setup: install deps and generate Swagger docs
- `make all` - Clean, install, generate docs, and build
- `make prod-build` - Production build: clean and build
- `make docker-all` - Full Docker rebuild: `docker-down`, build images, `docker-up` (does **not** remove named volumes; use `make docker-down-volumes` for a clean database)

## Docker Setup

### Baseline vs local dev overlay

- **`docker/docker-compose.yml` (baseline)** — Hardened defaults: **no** weak credential fallbacks, **no** published Postgres/Redis ports, **no** admin web UIs. Requires explicit `POSTGRES_*`, `DB_*`, `JWT_SECRET`, **`APP_ENV`**, and **`DB_SSLMODE`** (e.g. via `--env-file docker/.env`). The baseline does **not** default `APP_ENV` to `development` or `DB_SSLMODE` to `disable` (avoids accidental insecure deploys). API binds to **127.0.0.1** by default (`API_PUBLISH_HOST` / `API_PUBLISH_PORT`). Adds container hardening (`no-new-privileges`, `cap_drop: ALL`, read-only API root + `/tmp` tmpfs). Images use **pinned tags** (Postgres, Redis; see [docs/DOCKER.md](docs/DOCKER.md)).
- **`docker/docker-compose.dev.yml` (overlay)** — Local ergonomics: publishes **127.0.0.1** ports for Postgres/Redis, optional **Redis Commander** and **pgAdmin**, and convenience defaults (`chexi_dev` / documented passwords) **only when this file is combined with the baseline**. The overlay restores **`APP_ENV=${APP_ENV:-development}`** and **`DB_SSLMODE=${DB_SSLMODE:-disable}`** for local use.

**Local development (recommended):** `make docker-up` uses **both** compose files. Optionally copy `docker/.env.example` → `docker/.env` to override passwords, JWT, and ports.

**Production-like / minimal exposure:** `make docker-up-baseline` uses **only** the baseline file and requires a populated `docker/.env` with **`APP_ENV`**, **`DB_SSLMODE`**, and other required variables (no dev defaults). See [docs/DOCKER.md](docs/DOCKER.md).

### Quick Start with Docker Compose

#### Using Make (Recommended)

1. **Clone the repository** (if you haven't already)
   ```bash
   git clone <repository-url>
   cd goapi
   ```

2. **Optional: configure `docker/.env`**
   
   For custom secrets or ports, copy the example file:
   ```bash
   cp docker/.env.example docker/.env
   ```
   If you omit `docker/.env`, the dev overlay still supplies **local-only** defaults (change them for shared machines).

3. **Build and start services** (baseline + dev overlay)
   ```bash
   make docker-up
   ```
   
   Or for a complete rebuild:
   ```bash
   make docker-all
   ```

4. **View logs**
   ```bash
   make docker-logs-api
   ```

5. **Access the API and tools**
   - API: `http://127.0.0.1:8080` (or `localhost`)
   - Swagger UI: `http://127.0.0.1:8080/swagger/index.html`
   - PostgreSQL (host): `127.0.0.1:5432` — user/password/db match `POSTGRES_*` in `docker/.env` or dev defaults (`goapi_dev` / see `docker/.env.example`)
   - **Redis Commander**: `http://127.0.0.1:8081` — credentials from `REDIS_COMMANDER_HTTP_*` in `docker/.env`
   - **pgAdmin**: `http://127.0.0.1:5050` — `PGADMIN_DEFAULT_EMAIL` / `PGADMIN_DEFAULT_PASSWORD`

   **Security:** Dev-overlay defaults are for **local workstations** only. Use `make docker-up-baseline` and strong secrets for tighter deployments.

   **Optional — HTTP E2E:** Set `EMAIL_ENABLED=false` in `docker/.env` so `POST /api/v1/register` returns tokens without email verification. Requires **`curl`** and **`python3`**. Then run `make docker-e2e` (rebuild + up + E2E) or bring the stack up and run `make test-e2e-docker`. Details: [docs/TESTING.md — Docker E2E (HTTP)](docs/TESTING.md#docker-e2e-http).

6. **Stop services**
   ```bash
   make docker-down
   ```

7. **Stop and remove volumes** (clears database data)
   ```bash
   make docker-down-volumes
   ```

**PostgreSQL container unhealthy or `Permission denied` in `docker compose logs chexi-db`:** The named volume may have been created with incompatible permissions (often after changing Postgres layout or upgrading images). Reset data and recreate containers:

```bash
make docker-down-volumes
make docker-up
```

**Switched from old `admin`/`admin` Postgres volume:** run `make docker-down-volumes` once so the data volume is recreated with the new dev user (`goapi_dev` by default).

#### Using Docker Compose Directly

Run from the repository root so paths resolve correctly.

**Local stack (ports + admin UIs):**
```bash
docker compose -f docker/docker-compose.yml -f docker/docker-compose.dev.yml up -d
docker compose -f docker/docker-compose.yml -f docker/docker-compose.dev.yml logs -f chexi-api
docker compose -f docker/docker-compose.yml -f docker/docker-compose.dev.yml down
docker compose -f docker/docker-compose.yml -f docker/docker-compose.dev.yml down -v
```

**Hardened baseline only** (set `POSTGRES_*`, `DB_*`, `JWT_SECRET`, **`APP_ENV`**, **`DB_SSLMODE`** in `docker/.env` first):
```bash
docker compose -f docker/docker-compose.yml --env-file docker/.env up -d
```

Optional overrides: `docker compose ... --env-file docker/.env ...`

### Docker Compose Services

**Baseline (`docker-compose.yml`):**
- **`api`**: Go API (published on loopback; see `API_PUBLISH_HOST` / `API_PUBLISH_PORT`)
- **`db`**: PostgreSQL 16 (pinned image tag, e.g. `postgres:16.13-alpine`; not published without dev overlay)
- **`redis`**: Redis 7 (pinned tag; not published without dev overlay)

**With dev overlay (`docker-compose.dev.yml`):**
- **`redis-commander`**: Redis web UI (loopback)
- **`pgadmin`**: PostgreSQL web UI (loopback)

### Local database credentials (dev overlay)

With the dev overlay, Postgres defaults to user **`goapi_dev`**, database **`goapi`**, and the password in `docker/.env.example` / overlay default — **override in `docker/.env`**. The API container’s `DB_USER` / `DB_PASS` / `DB_NAME` must match the Postgres service.

### Docker Image Details

The Dockerfile uses a multi-stage build ([`docker/Dockerfile`](docker/Dockerfile)):
- **Builder stage**: `golang:1.25.10-alpine` (matches `go.mod`) with `GOTOOLCHAIN=auto` for toolchain alignment during `go mod download` / build
- **Final stage**: `alpine:3.21` for a minimal runtime image; runs as **non-root** (`appuser`)

Image pins and upgrade workflow: [docs/DOCKER.md](docs/DOCKER.md).

The image includes:
- Compiled Go binary
- Swagger documentation (if generated)
- CA certificates for HTTPS requests

### Building Docker Image Manually

If you want to build just the API Docker image:

```bash
# Build the image (Dockerfile lives under docker/)
docker build -f docker/Dockerfile -t goapi:latest .

# Run the container (use real secrets; match your Postgres user/password)
docker run -p 127.0.0.1:8080:8080 \
  -e DB_USER=goapi_dev \
  -e DB_PASS=your_db_password \
  -e DB_HOST=host.docker.internal \
  -e DB_PORT=5432 \
  -e DB_NAME=goapi \
  -e DB_SSLMODE=disable \
  -e JWT_SECRET=your_long_random_jwt_secret \
  goapi:latest
```

Or using Make:
```bash
make docker-build
```

## API Documentation

### Swagger UI

The API includes interactive Swagger/OpenAPI documentation accessible at:

**http://localhost:8080/swagger/index.html**

Swagger access is environment-aware:
- Development/test/local: enabled by default (unless `SWAGGER_ENABLED=false`).
- Staging/production: disabled by default (set `SWAGGER_ENABLED=true` to enable).
- When enabled in staging/production, access requires an admin JWT (`Authorization: Bearer <token>`).

The Swagger UI provides:
- Interactive API testing interface
- Complete endpoint documentation
- Request/response examples
- Authentication testing with JWT tokens
- Schema definitions for all models

### Generating Swagger Documentation

After modifying API endpoints or adding new ones, regenerate the Swagger documentation:

```bash
make swagger
```

This command scans your code for Swagger annotations (comments starting with `@Summary`, `@Description`, `@Tags`, etc.) and generates the documentation files in the `api/openapi/` directory.

## First admin (bootstrap)

Greenfield deployments need **one** initial admin before any JWT exists. The API supports an optional **one-time bootstrap** at process startup (after DB migrations):

| Variable | Meaning |
|----------|---------|
| `BOOTSTRAP_ADMIN_ENABLED` | Must be `true` to run bootstrap (default: unset / false). |
| `BOOTSTRAP_ADMIN_EMAIL` | Email for the first admin (required when enabled). |
| `BOOTSTRAP_ADMIN_PASSWORD` | Plaintext only for this bootstrapping step; hashed with the same bcrypt path as normal registration. **Never committed or logged.** |

**Behavior (idempotent):**

- If bootstrap is **disabled**, nothing runs.
- If **any** user with role `admin` already exists, bootstrap **skips** (no new user).
- If `BOOTSTRAP_ADMIN_EMAIL` **already exists** (any role), bootstrap **skips** and does **not** change the password (no takeover).
- In **staging** and **production**, weak or common bootstrap passwords are **rejected** (password strength rules + denylist). Development/test only enforce a minimum length when enabled.

**Operations:**

1. Set the three variables only for the **first** deploy where no admin exists.
2. After the admin account exists, **remove** `BOOTSTRAP_ADMIN_*` from environment / secrets managers so the password is not retained in config.
3. Prefer creating further users (including admins) via authenticated admin APIs (`/api/v1/admin/users` patterns in your deployment).

Public **`POST /api/v1/register`** still **cannot** create an admin (`403`); bootstrap is separate from self-registration.

## API Endpoints

### Public Endpoints

#### Root and health

- **GET** `/`
  - Lightweight welcome JSON (process is up).

- **GET** `/health`
  - Readiness-style check: pings PostgreSQL; reports cache as `disabled`, `healthy`, or `unhealthy` when Redis is enabled (may still return HTTP 200 if only cache fails—see handler implementation).

#### Authentication

JSON APIs are versioned under **`/api/v1`**. Passwords must meet strength rules (length, upper, lower, number, special character)—see `services/validation.go`.

- **POST** `/api/v1/register`
  - Register a new user
  - **Request Body:**
    ```json
    {
      "first_name": "John",
      "last_name": "Doe",
      "email": "john.doe@example.com",
      "password": "Password123!",
      "phone_number": "+1234567890",
      "role": "user"
    }
    ```
  - **Roles:** Self-registration always creates a **`user`**. Sending `"role": "admin"` returns **403** (`Cannot self-register as admin`). For the **first** admin in an empty system, use optional startup bootstrap (`BOOTSTRAP_ADMIN_*`, see **First admin (bootstrap)** above); afterward create admins with admin APIs only.
  - **Response (201):**
    ```json
    {
      "token": {
        "api_key": "uuid-string",
        "jwt_token": "jwt-token-string"
      },
      "user": {
        "id": 1,
        "email": "john.doe@example.com"
      }
    }
    ```

- **POST** `/api/v1/login`
  - Authenticate and receive JWT token
  - **Request Body:**
    ```json
    {
      "email": "john.doe@example.com",
      "password": "Password123!"
    }
    ```
  - **Response (200):**
    ```json
    {
      "token": {
        "api_key": "uuid-string",
        "jwt_token": "jwt-token-string"
      },
      "user": {
        "id": 1,
        "email": "john.doe@example.com"
      }
    }
    ```

- **GET** `/api/v1/verify-email?token=<token>`
  - Optional query-string verification (same token as registration email).

- **POST** `/api/v1/verify-email`
  - Body: `{ "token": "<token>" }` — consumes a single-use verification token and sets `email_verified_at`.

- **POST** `/api/v1/resend-verification`
  - Body: `{ "email": "user@example.com" }` — always returns **200** with a generic message (no email enumeration). If the account exists and still needs verification (and throttle allows), a new token may be issued.

- **POST** `/api/v1/password-reset/request`
  - Body: `{ "email": "user@example.com" }` — always returns **200** with the same generic message whether or not the email exists.

- **POST** `/api/v1/password-reset/confirm`
  - Body: `{ "token": "<token>", "password": "NewPassword123!" }` — single-use reset; **revokes existing refresh sessions** (server-side session rows); passwords must meet strength rules.
  - **JWT access-token caveat:** Already-issued **stateless JWT access tokens** remain **valid until their normal expiration** (`exp` claim). Password reset does not invalidate those bearer tokens. **Immediate** access-token revocation would require extra machinery—for example an **access-token denylist**, **token versioning** checked on each request, or a **shorter access-token TTL** so stale credentials expire sooner.

### Protected Endpoints (Require Authentication)

All user endpoints require a valid JWT token in the `Authorization` header:
```
Authorization: Bearer <jwt_token>
```

#### User Management

- **GET** `/api/v1/users`
  - Get all users
  - **Headers:** `Authorization: Bearer <token>`
  - **Response (200):** Array of user objects (`id`, `email`, …). With `?page=&page_size=`, returns `{ "data": [...], "total", "page", "page_size", "total_pages" }`.

- **GET** `/api/v1/users/:id`
  - Get a specific user by ID
  - **Headers:** `Authorization: Bearer <token>`
  - **Response (200):** User object
  - **Response (404):** `{"error": "User not found"}`

- **POST** `/api/v1/users`
  - Create a new user (authenticated users only)
  - **Headers:** `Authorization: Bearer <token>`
  - **Request Body:**
    ```json
    {
      "first_name": "Jane",
      "last_name": "Smith",
      "email": "jane.smith@example.com",
      "password": "Password123!",
      "phone_number": "+1234567891",
      "role": "user"
    }
    ```
  - **Note:** `phone_number` and `role` are optional. Default role is `user`. Only a JWT with role **`admin`** may set **`role`: `admin`**; otherwise the API returns **403**.
  - **Response (201):** Created user object

- **PUT** `/api/v1/users/:id`
  - Update a user (partial updates supported)
  - **Headers:** `Authorization: Bearer <token>`
  - **Request Body:** (all fields optional, but at least one required)
    ```json
    {
      "first_name": "Jane",
      "last_name": "Smith",
      "email": "jane.smith@example.com",
      "password": "Newpassword123!",
      "phone_number": "+1234567891",
      "role": "admin"
    }
    ```
  - **Note:** Setting `role` to `admin` requires an **admin** JWT; non-admins receive **403**.
  - **Response (200):** Updated user object

- **DELETE** `/api/v1/users/:id`
  - Delete a user (Admin only)
  - **Headers:** `Authorization: Bearer <token>`
  - **Response (200):** `{"message": "User deleted successfully"}`
  - **Response (403):** `{"error": "Insufficient permissions"}` (if not admin)

## Authentication

The API uses JWT (JSON Web Tokens) for authentication.

When outbound email is **enabled**, `POST /api/v1/register` creates the account and queues a verification email but does **not** return access tokens. The client should complete `POST /api/v1/verify-email`, then `POST /api/v1/login`. Password login requires a verified email. When `EMAIL_ENABLED=false` (or equivalent), new accounts are verified at signup and registration returns tokens as before.

Access tokens are valid for 60 days. Each includes:
- User ID (subject)
- API Key (UUID)
- Issued and expiration timestamps

### Using Authentication

Include the JWT token in the `Authorization` header for protected endpoints:
```bash
curl -H "Authorization: Bearer <your_jwt_token>" http://localhost:8080/api/v1/users
```

## User Model

```go
type User struct {
    ID        int       `json:"id"`
    FirstName string    `json:"first_name"`
    LastName  string    `json:"last_name"`
    Email     string    `json:"email"`        // Unique
    PassHash  string    `json:"-"`            // Never returned in JSON
    PhoneNum  string    `json:"phone_number"`
    Role      string    `json:"role"`         // "user" or "admin"
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

## Middleware

### HTTP security headers

Registered **first** in the Gin stack so error and recovery responses still include baseline headers. Controlled by config (`internal/transport/http/middleware/security_headers.go`).

When **`SECURITY_HEADERS_ENABLED`** is true (default), responses include:

| Header | Value |
|--------|--------|
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `Referrer-Policy` | `no-referrer` |
| `Permissions-Policy` | Conservative disables (e.g. camera, microphone, geolocation, payment) — adjust if you add browser features that need them |
| `Cross-Origin-Opener-Policy` | `same-origin` — appropriate for a JSON API; rare OAuth/popup flows that open **this origin** in a popup may need a relaxed COOP at the edge or on specific routes |

**HSTS** (`Strict-Transport-Security`) is emitted only when **`HSTS_ENABLED=true`** and the request is treated as HTTPS: TLS directly (`http.Request.TLS`) **or** **`X-Forwarded-Proto: https`** (case-insensitive). Terminating TLS on a reverse proxy **must** set `X-Forwarded-Proto` correctly or HSTS will not apply when expected. The header uses **`HSTS_MAX_AGE_SECONDS`** (default one year) and optional **`HSTS_INCLUDE_SUBDOMAINS=true`** for `includeSubDomains`. Defaults keep **`HSTS_ENABLED=false`** so local `http://` dev does not send HSTS unless you explicitly enable it and mark the request as HTTPS.

Handlers run after the middleware and may override individual headers (e.g. `X-Frame-Options`).

### Rate Limiting
- **Default:** 60 requests per minute per IP
- **Burst:** 10 requests
- **Response (429):** `{"error": "Rate limit exceeded. Please try again later."}` when the limit is exceeded and the backing store succeeded
- **Redis Support:** When Redis is enabled, rate limiting is distributed across all API instances. The process also keeps an in-memory limiter with the same RPM/burst so `local_fallback` can enforce limits per instance if Redis errors during a request.
- **Redis increment failures (`RATE_LIMIT_REDIS_FAILURE_MODE`):**
  - **`local_fallback`** (default in staging/production): use the in-memory limiter for that request. Limits are per process, not cluster-wide, during the outage.
  - **`fail_closed`**: respond with **503** and `{"error": "Rate limiter unavailable."}` (distinct from 429).
  - **`fail_open`**: allow the request through the rate limiter (development/test default when unset; **forbidden** in staging/production — the app will not start with `fail_open` there).
  Invalid values log a warning and fall back to the environment default.

### Request Logging
Logs all HTTP requests with:
- HTTP method
- Request path (**URL path only; the query string is omitted** from access logs so OAuth `code`/`state`, tokens, API keys, and similar parameters are never written to standard request logs)
- Status code
- Response time
- Client IP
- User agent

When **HTTP audit events** are enabled (`AUDIT_HTTP_ENABLED=true`), persisted `http.request` records include a **redacted** query string: known sensitive parameter names (`token`, `code`, `state`, `oauth_code`, `refresh_token`, `reset_token`, `verification_token`, `api_key`, `key`, `secret`, and related keys) have values replaced with `[redacted]`; non-sensitive parameters remain for debugging. **Malformed** query strings are not stored verbatim (`?[invalid_query]`).

Log levels:
- **INFO:** Status codes < 400
- **WARN:** Status codes 400-499
- **ERROR:** Status codes ≥ 500

### Authentication Middleware
- Validates JWT tokens from `Authorization` header
- Extracts user ID and API key from token claims
- Stores user information in request context

### Role-Based Access Control
- Role constants live in `internal/rbac` (`user`, `admin`); services validate via `IsValidRole`.
- `RequireRole("admin")` middleware restricts endpoints to admin users (e.g. user deletion, event log API).
- Self-service **register** cannot create admins; assigning **admin** on create/update requires an existing admin’s JWT.

## Caching

The application supports optional Redis caching for improved performance:

### Cache Features
- **User Caching**: Caches user lookups by ID and email (15-minute TTL)
- **Cache-Aside Pattern**: Checks cache first, falls back to database on miss
- **Automatic Invalidation**: Cache is invalidated on user updates and deletes
- **Distributed Rate Limiting**: Redis enables shared rate limits across multiple API instances
- **Graceful Degradation**: If Redis is unavailable, uses no-op cache (app continues to work)

### Cache Configuration
- **User Cache TTL**: 15 minutes (configurable in `cache/constants.go`)
- **Rate Limit Window**: 1 minute (configurable in `cache/constants.go`)
- **Key Patterns**:
  - User by ID: `user:id:{id}`
  - User by Email: `user:email:{email}`
  - Rate Limit: `ratelimit:{ip}`

### Cache Invalidation Strategy
- **On User Update**: All cached entries for the user are invalidated
- **On User Delete**: All cached entries for the user are removed
- **On User Create**: New user is stored in cache
- **Email Changes**: Old email cache key is deleted when email is updated

### Enabling Redis
Set `REDIS_ENABLED=true` in your `.env` file. The application will automatically:
- Connect to Redis on startup
- Use Redis for caching and rate limiting
- Fall back to in-memory if Redis connection fails
- Apply `RATE_LIMIT_REDIS_FAILURE_MODE` when the distributed rate-limit store errors mid-request (see **Rate Limiting** above)

### Redis TLS policy
- Development/test default to `REDIS_TLS_ENABLED=false`
- Staging/production require `REDIS_TLS_ENABLED=true` when Redis is enabled and `REDIS_HOST` is non-local
- `REDIS_TLS_INSECURE_SKIP_VERIFY=true` is allowed only outside staging/production

## Database

The application uses PostgreSQL with GORM for database operations. By default the schema is created via **GORM AutoMigrate** on startup. For production-style **versioned SQL migrations**, see [docs/MIGRATIONS.md](docs/MIGRATIONS.md) (`migrations/`, `USE_VERSIONED_MIGRATIONS`, `make migrate-sql-up`). Background Redis email/job behavior is described in [docs/QUEUE.md](docs/QUEUE.md).

### Manual Schema Setup

If you prefer to set up the database manually, refer to `schema.sql` for the table structure. The schema includes:
- Automatic `updated_at` timestamp updates via PostgreSQL trigger
- Index on email column for faster lookups
- Proper PostgreSQL data types (SERIAL for auto-increment IDs)

## Error Responses

The API returns standard HTTP status codes:

- **200 OK** - Successful request
- **201 Created** - Resource created successfully
- **400 Bad Request** - Invalid request data
- **401 Unauthorized** - Missing or invalid authentication
- **403 Forbidden** - Insufficient permissions
- **404 Not Found** - Resource not found
- **409 Conflict** - Resource already exists (e.g., duplicate email)
- **429 Too Many Requests** - Rate limit exceeded
- **500 Internal Server Error** - Server error

Error responses follow this format:
```json
{
  "error": "Error message description"
}
```

## Security Features

For deployment checklists and operational hardening, see **[docs/SECURITY.md](docs/SECURITY.md)**.

- **Password Hashing:** Bcrypt with default cost (10 rounds)
- **JWT Tokens:** HMAC-SHA256 signed tokens
- **Rate Limiting:** Prevents abuse and DoS attacks
- **Input Validation:** Request body validation using Gin's binding
- **SQL Injection Protection:** GORM parameterized queries
- **Password Requirements:** Strength rules (length ≥ 8, uppercase, lowercase, number, special character)—see `services/validation.go`

## Development

### Running in Development Mode

Using Make:
```bash
# Set GIN_MODE to development for verbose logging
export GIN_MODE=debug
make run
```

Or manually:
```bash
export GIN_MODE=debug
go run main.go
```

### Building for Production

Using Make:
```bash
make prod-build
# Binary will be created as 'goapi'
./goapi
```

Or manually:
```bash
go build -o goapi main.go
./goapi
```

### Testing

Run all tests:
```bash
make test
```

Run tests with coverage:
```bash
make test-coverage
# Opens coverage.html in your default browser
```

Run integration tests locally (self-contained):
```bash
make test-integration
```

`make test-integration` uses a **dedicated Compose project** (`goapi_integration` by default) plus **non-default host ports** (`15432` / `16379`) so it can run beside `make docker-up`. It aligns with the dev-overlay credentials (`goapi_dev` / `goapi_dev_password_change_me`, `goapi_integration`, `DB_SSLMODE=disable`). Override with `INTEGRATION_COMPOSE_PROJECT`, `INTEGRATION_POSTGRES_PUBLISH_PORT`, `INTEGRATION_REDIS_PUBLISH_PORT`, `INTEGRATION_DB_*`, etc.

### Generating Swagger Documentation

When you add or modify API endpoints, update the Swagger annotations in your controller functions and regenerate the docs:

```bash
# Install swag CLI (if not already installed)
go install github.com/swaggo/swag/cmd/swag@latest

# Generate Swagger documentation
make swagger
```

The Swagger annotations use the following format:
```go
// @Summary      Brief summary of the endpoint
// @Description  Detailed description
// @Tags         tag-name
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Parameter description"
// @Success      200  {object}  models.User
// @Failure      400  {object}  map[string]string
// @Router       /users/{id} [get]
```

## License

See [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
