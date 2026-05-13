# API Testing Guide

Authentication and user CRUD endpoints are under **`/api/v1`**. `GET /` returns a welcome JSON payload; **`GET /health`** checks database and Redis.

**Login/register responses:** Optional fields are controlled by environment variables (`AUTH_RESPONSE_INCLUDE_API_KEY`, `AUTH_RESPONSE_INCLUDE_USER`). By default only `token.jwt_token` is returned. Set those vars to `true` if you expect `api_key` or `user` in the JSON (see README).

## Prerequisites

Before testing, ensure:
1. Database is running (PostgreSQL)
2. `.env` file is configured with database credentials
3. Server is running: `make run` or `go run main.go`

**First admin:** Self-service registration cannot create admins. For an empty database, optional startup bootstrap uses `BOOTSTRAP_ADMIN_ENABLED`, `BOOTSTRAP_ADMIN_EMAIL`, and `BOOTSTRAP_ADMIN_PASSWORD` (see README **First admin (bootstrap)**). Remove those variables after the first admin exists.

## Quick Test Commands

### Using cURL (Linux/Mac/Git Bash)

```bash
# 1. Welcome (optional)
curl http://localhost:8080/

# 2. Health check (DB + Redis)
curl http://localhost:8080/health

# 3. Register User
curl -X POST http://localhost:8080/api/v1/register \
  -H "Content-Type: application/json" \
  -d '{
    "first_name": "John",
    "last_name": "Doe",
    "email": "john.doe@test.com",
    "password": "Password123!",
    "phone_number": "+1234567890",
    "role": "user"
  }'

# 4. Register Admin
curl -X POST http://localhost:8080/api/v1/register \
  -H "Content-Type: application/json" \
  -d '{
    "first_name": "Admin",
    "last_name": "User",
    "email": "admin@test.com",
    "password": "Adminpass123!",
    "phone_number": "+1234567891",
    "role": "admin"
  }'

# 5. Login
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john.doe@test.com",
    "password": "Password123!"
  }'

# 5b. Verify email (token from verification email or logs in dev)
curl -X POST http://localhost:8080/api/v1/verify-email \
  -H "Content-Type: application/json" \
  -d '{"token":"PASTE_TOKEN_HERE"}'

# 5c. Resend verification (generic success body)
curl -X POST http://localhost:8080/api/v1/resend-verification \
  -H "Content-Type: application/json" \
  -d '{"email":"john.doe@test.com"}'

# 5d. Password reset request + confirm (generic request response)
curl -X POST http://localhost:8080/api/v1/password-reset/request \
  -H "Content-Type: application/json" \
  -d '{"email":"john.doe@test.com"}'

curl -X POST http://localhost:8080/api/v1/password-reset/confirm \
  -H "Content-Type: application/json" \
  -d '{"token":"PASTE_RESET_TOKEN_HERE","password":"NewPassword123!"}'

**Password reset vs JWTs:** `password-reset/confirm` revokes **refresh sessions** (you cannot rotate access tokens with the old refresh token after reset). **Access JWTs** that were already issued stay usable until they expire naturally—there is no server-side revocation for those stateless tokens. To revoke access immediately you would need an access-token **denylist**, **token versioning** enforced on each request, or a **shorter access-token TTL**.

# Save the JWT token from response, then:

# 6. Get All Users (replace TOKEN with actual token)
curl -X GET http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer TOKEN"

# 7. Get User by ID
curl -X GET http://localhost:8080/api/v1/users/1 \
  -H "Authorization: Bearer TOKEN"

# 8. Create User
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "first_name": "Jane",
    "last_name": "Smith",
    "email": "jane.smith@test.com",
    "password": "Password123!",
    "phone_number": "+1234567892",
    "role": "user"
  }'

# 9. Update User
curl -X PUT http://localhost:8080/api/v1/users/1 \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "first_name": "John Updated",
    "last_name": "Doe Updated"
  }'

# 10. Delete User (Admin only - replace ADMIN_TOKEN)
curl -X DELETE http://localhost:8080/api/v1/users/2 \
  -H "Authorization: Bearer ADMIN_TOKEN"
```

### Using PowerShell (Windows)

```powershell
# 1. Welcome
Invoke-RestMethod -Uri "http://localhost:8080/" -Method GET

# 2. Health check
Invoke-RestMethod -Uri "http://localhost:8080/health" -Method GET

# 3. Register User
$registerData = @{
    first_name = "John"
    last_name = "Doe"
    email = "john.doe@test.com"
    password = "Password123!"
    phone_number = "+1234567890"
    role = "user"
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/api/v1/register" -Method POST -Body $registerData -ContentType "application/json"

# 4. Login
$loginData = @{
    email = "john.doe@test.com"
    password = "Password123!"
} | ConvertTo-Json

$response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/login" -Method POST -Body $loginData -ContentType "application/json"
$token = $response.token.jwt_token

# 5. Get All Users
$headers = @{
    "Authorization" = "Bearer $token"
}
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/users" -Method GET -Headers $headers
```

## Test Coverage

### ✅ Endpoints to Test

#### Public Endpoints
- [x] `GET /` - Welcome message
- [x] `GET /health` - Health check (DB + Redis)
- [x] `POST /api/v1/register` - User registration
- [x] `POST /api/v1/login` - User authentication

#### Protected Endpoints (Require JWT)
- [x] `GET /api/v1/users` - Get all users
- [x] `GET /api/v1/users/:id` - Get user by ID
- [x] `POST /api/v1/users` - Create user
- [x] `PUT /api/v1/users/:id` - Update user
- [x] `DELETE /api/v1/users/:id` - Delete user (Admin only)

### ✅ Test Scenarios

#### Authentication Tests
1. ✅ Register new user (user role)
2. ✅ Register new admin (admin role)
3. ✅ Login with valid credentials
4. ✅ Login with invalid credentials (should return 401)
5. ✅ Register with duplicate email (should return 409)
6. ✅ Register with invalid role (should return 400)
7. ✅ Register with invalid email format (should return 400)
8. ✅ Register with short password (should return 400)

#### Authorization Tests
1. ✅ Access protected endpoint without token (should return 401)
2. ✅ Access protected endpoint with invalid token (should return 401)
3. ✅ Access protected endpoint with valid token (should succeed)
4. ✅ Delete user as regular user (should return 403)
5. ✅ Delete user as admin (should succeed)

#### User Management Tests
1. ✅ Get all users (authenticated)
2. ✅ Get user by ID (authenticated)
3. ✅ Get non-existent user (should return 404)
4. ✅ Create user (authenticated)
5. ✅ Update user (partial update)
6. ✅ Update user with empty body (should return 400)
7. ✅ Delete user (admin only)

#### Error Handling Tests
1. ✅ Invalid request body
2. ✅ Missing required fields
3. ✅ Invalid data types
4. ✅ Resource not found (404)
5. ✅ Unauthorized access (401)
6. ✅ Forbidden access (403)
7. ✅ Conflict (409 - duplicate email)

### ✅ Middleware Tests

#### Rate Limiting
- Test: Make 70 requests in quick succession
- Expected: First 60 succeed, then 429 Too Many Requests

#### Request Logging
- Check server logs for request details
- Verify: Method, **path without query string**, status code, response time, IP, user agent (sensitive query data must not appear in access logs)
- **Automated coverage:** `internal/transport/http/middleware/logger_middleware_test.go` (path-only logging) and `request_path_test.go` / `http_audit_test.go` (redacted query metadata for HTTP audit events)

#### Authentication Middleware
- Test: Access protected route without token
- Expected: 401 Unauthorized

#### RBAC Middleware
- Test: Regular user tries to delete user
- Expected: 403 Forbidden

## Automated Testing

### Option 1: Use the Test Scripts

**Bash (Linux/Mac/Git Bash):**
```bash
chmod +x scripts/test_api.sh
./scripts/test_api.sh
```

**PowerShell (Windows):**
```powershell
powershell -ExecutionPolicy Bypass -File scripts/test_api.ps1
```

### Option 2: Use Go Tests

```bash
# Unit tests (default; integration tests use build tag `integration`)
go test -v ./...

# Same as GitHub Actions unit step (race detector)
go test -race ./...

# Full-stack integration tests (manual env/deps)
go test -tags=integration -race -v ./test/integration/...
```

### Background job queue

- **Unit tests** live in `internal/queue/*_test.go` and use **`miniredis`** (same pattern as `cache/redis_cache_test.go`)—no Docker Redis required.
- **Fallback behavior**: With `REDIS_ENABLED=false` or `QUEUE_ASYNC_ENABLED=false`, jobs execute **inline** during `Enqueue`; integration tests use the same bootstrap path as production.
- **Useful env vars**: `QUEUE_ENABLED` (default on), `QUEUE_ASYNC_ENABLED`, `JOB_QUEUE_ENABLED` (optional override), `JOB_WORKER_ENABLED` (disable worker only), `JOB_MAX_ATTEMPTS`, `JOB_RETRY_DELAY_SECONDS`, `QUEUE_WORKERS`, `QUEUE_POLL_INTERVAL_MS`, `QUEUE_MAX_ATTEMPTS`, `QUEUE_*_BACKOFF_MS`, `QUEUE_SHUTDOWN_TIMEOUT_SEC`.

### CI parity and security checks (local)

Run the same gates as `.github/workflows/ci.yml` **without** Docker integration:

```bash
make ci
```

Individual steps:

| Command | Notes |
|--------|--------|
| `make fmt-check` | Ensures tracked `*.go` files match `gofmt -s` |
| `go mod verify` | Module checksums |
| `go vet ./...` | Standard static analysis |
| `make vulncheck` | **govulncheck** — fails on **reachable** vulnerabilities; requires **network** to fetch `vuln.go.dev` data |
| `make security-check` | `go vet` + **gosec** (medium severity/confidence; excludes generated `api/openapi`) |
| `make test-race` or `go test -race ./...` | Race detector |
| `make secret-scan` | **gitleaks** — leaked secrets in git history / working tree |
| `make container-scan` | **Trivy** — HIGH/CRITICAL vulns in repo (`fs`) + API Docker image (`image`) |
| `make security-scan` | Runs **secret-scan** then **container-scan** |

**Caveats:** `govulncheck` needs outbound HTTPS to the Go vulnerability database (may fail offline/air-gapped). If it reports **reachable** issues in the **standard library**, bump the **`go` directive** in `go.mod` to a patched toolchain version (per `govulncheck` output), then `go mod tidy`. **gosec** may flag intentional patterns; document any `#nosec` with justification (see `BuildRedisTLSConfig` for `G402`). Pin **govulncheck** / **gosec** versions in the Makefile and `.github/workflows/ci.yml` together when upgrading tools.

### Secret scanning (gitleaks)

Local:

```bash
make secret-scan
```

Uses pinned **`GITLEAKS_VERSION`** (Makefile + `.github/workflows/ci.yml`). Config: **`.gitleaks.toml`** (extends default rules). Output is redacted (`--redact`).

**False positives (do not hide real secrets):**

- Prefer fixing the source (remove the string, use a secret manager, or replace with a non-secret placeholder in docs).
- **Git history:** fixing the working tree does not remove findings in old commits; use **`.gitleaksignore`** with the exact **fingerprint** lines that `gitleaks` prints (format `commit:path:rule:line`), plus a short comment above each entry — see repository **`.gitleaksignore`** for examples.
- If a match is a **known safe** string (e.g. example JWT in a test fixture), add a **fingerprint** line to **`.gitleaksignore`** (from `gitleaks detect` / CI logs — use the reported fingerprint, not the secret value).
- Avoid broad allowlists in `.gitleaks.toml` that disable whole rule classes or entire paths without review.

**Upgrades:** bump `GITLEAKS_VERSION` in the Makefile and `ci.yml` together; run `make secret-scan` and re-check any new findings.

### Container & dependency scanning (Trivy)

Local (requires **Docker**; pulls `aquasec/trivy` once per version):

```bash
make container-scan
```

This runs, in order:

1. **`trivy fs`** on the repository — **vulnerability** scanner on the working tree (e.g. `go.mod` / lockfiles).
2. **`docker build -f docker/Dockerfile -t goapi:scan .`**
3. **`trivy image`** on **`goapi:scan`** — OS/app packages in the image (HIGH/CRITICAL only to reduce noise).

Suppressions: **`.trivyignore`** — one **CVE / vuln ID** per line; add a short comment with **rationale** and **review-by date**. Do not add catch-all or empty broad suppressions. Document the same decision in the PR or [docs/DOCKER.md](DOCKER.md) when bumping base images.

**Upgrades:** bump **`TRIVY_VERSION`** (image tag, no `v` prefix) in the Makefile and `ci.yml` together; re-run `make container-scan` and fix or document new findings.

Preferred local command (auto-starts Docker db/redis, resets DB, applies migrations, then runs tests):

```bash
make test-integration
```

The script starts dependencies with **both** `docker/docker-compose.yml` and `docker/docker-compose.dev.yml`, using Compose project **`chexi_trading_integration`** (override with `INTEGRATION_COMPOSE_PROJECT`) and host ports **`15432` (Postgres) / `16379` (Redis)** by default so it does not collide with a developer stack on `5432`/`6379`. Override ports with `INTEGRATION_POSTGRES_PUBLISH_PORT` / `INTEGRATION_REDIS_PUBLISH_PORT`.

Local integration runs with `APP_ENV=development` and keeps Redis TLS disabled by default (`REDIS_TLS_ENABLED=false`), preserving local Docker behavior.

The wrapper script **`scripts/test_integration_local.sh`** exports **`EMAIL_ENABLED=true`** so verification-token tests stay deterministic even if your shell still has `EMAIL_ENABLED=false` from Docker E2E workflows.

### Docker E2E (HTTP)

Black-box checks against the **real API URL** published by the dev Docker stack (`127.0.0.1:8080` by default). No host Postgres access, no mocks, no Go test binary — only `curl` and `python3` (stdlib JSON).

| Command | What it does |
|--------|----------------|
| `make test-e2e-docker` | Runs [`scripts/e2e_docker.sh`](../scripts/e2e_docker.sh). **Requires the stack to already be up** (`make docker-up` or `make docker-all`). |
| `make docker-e2e` | `make docker-all` then `make test-e2e-docker`. |

**Not the same as `make test-integration`:** integration tests use a **separate** Compose project (`chexi_trading_integration`), reset the DB via `psql` inside the container, and run **`go test -tags=integration`**. Docker E2E hits whatever database your **main** dev stack is using.

**Prerequisites**

1. **`curl`** and **`python3`** on the PATH.
2. **`EMAIL_ENABLED=false`** in `docker/.env` (or equivalent) so registration returns `token.jwt_token` / `token.refresh_token` immediately. With `EMAIL_ENABLED=true`, the API returns a verification message instead of tokens; the script exits with a hint.
3. Optional: **`E2E_BASE_URL`** if the API is not at `http://127.0.0.1:8080` (must match `API_PUBLISH_HOST` / `API_PUBLISH_PORT` in Compose).

**Covered flow (happy paths + one auth denial):** `GET /health` (healthy) → register → login → refresh → `GET /api/v1/users/me` → create organization → create org API key (validates `key_prefix`, does not print secret) → list org API keys → `GET /api/v1/users/me` without `Authorization` (expect 401).

**Note:** `make docker-all` runs `docker-down`, rebuild, and `docker-up`; it does **not** remove named volumes. Use `make docker-down-volumes` when you need a clean database.

**Webhooks** are not part of this script (org webhook CRUD typically needs `WEBHOOK_ENCRYPTION_KEY`).

**Troubleshooting**

- **Register succeeds but “no tokens”** → set `EMAIL_ENABLED=false`, recreate/restart the API container, retry.
- **Connection refused** → start the stack; wait for `GET /health` to return 200.
- **401 after login** → unexpected MFA or wrong password (E2E uses a fixed strong password).

### Option 3: Use Swagger UI

1. Start the server: `make run`
2. Open browser: `http://localhost:8080/swagger/index.html`
3. Use the interactive Swagger UI to test endpoints
4. Click "Authorize" and enter your JWT token
5. Test each endpoint interactively

## Expected Test Results

When all tests pass, you should see:

```
✓ Health check endpoint
✓ Register user endpoint
✓ Register admin user endpoint
✓ Login endpoint
✓ Login with invalid credentials
✓ Get all users endpoint
✓ Get all users without authentication
✓ Get user by ID endpoint
✓ Get non-existent user
✓ Create user endpoint
✓ Update user endpoint
✓ Update user with invalid data
✓ Delete user as admin
✓ Delete user as regular user
✓ Register duplicate email
✓ Register with invalid role
✓ Register with invalid email format
✓ Register with short password

Test Summary
Passed: 18
Failed: 0
Total: 18
```

## Troubleshooting

### Server won't start
- Check database connection in `.env`
- Ensure PostgreSQL is running
- Check port 8080 is not in use

### Tests fail with 401
- Verify JWT token is valid
- Check token hasn't expired
- Ensure token is in Authorization header: `Bearer <token>`

### Tests fail with 500
- Check database connection
- Review server logs
- Verify all environment variables are set

### Rate limiting issues
- Wait 1 minute between test runs
- Or restart server to reset rate limiter

## Performance Testing

### Load Testing with Apache Bench

```bash
# Test health endpoint with 1000 requests, 10 concurrent
ab -n 1000 -c 10 http://localhost:8080/

# Test authenticated endpoint (requires token)
ab -n 100 -c 5 -H "Authorization: Bearer TOKEN" http://localhost:8080/api/v1/users
```

### Load Testing with wrk

```bash
# Install: brew install wrk (Mac) or download from GitHub
wrk -t4 -c100 -d30s http://localhost:8080/
```

## Integration Testing

For full integration testing with a test database:

1. Create a separate test database
2. Update `.env.test` with test database credentials
3. Run migrations on test database
4. Execute test suite
5. Clean up test data

## Continuous Integration

The workflow **`.github/workflows/ci.yml`** runs these jobs:

1. **`test`** — `gofmt` (tracked files), `go mod verify`, `go vet ./...`, `go build ./...`, **govulncheck**, **gosec** (medium severity/confidence; excludes `api/openapi`), `go test -race ./...`
2. **`integration`** — **`make test-integration`** (requires Docker; starts Compose deps and runs tagged integration tests)
3. **`secret-scan`** — **gitleaks** (`fetch-depth: 0` checkout; `.gitleaks.toml`; redacted output)
4. **`container-scan`** — **Trivy** `fs` (repo) + **`docker build`** + Trivy `image` (`goapi:scan`; HIGH/CRITICAL)

Local equivalents:

```bash
make ci              # mirrors the `test` job
make test-integration # mirrors the `integration` job (needs Docker)
make secret-scan      # mirrors `secret-scan`
make container-scan   # mirrors `container-scan` (needs Docker)
```

For coverage HTML locally, use `make test-coverage` (not required by CI).

