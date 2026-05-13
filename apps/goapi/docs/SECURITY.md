# Security and operations (chexi-trading API)

This document is the **single entrypoint** for production security and day‑two operations for the chexi-trading API. It summarizes behavior documented in the [root README](../README.md), [DOCKER.md](DOCKER.md), [MIGRATIONS.md](MIGRATIONS.md), and [TESTING.md](TESTING.md).

**Scope:** Forks and deployments are responsible for secrets, TLS at the edge, IAM, backups, and org‑specific threat models. This file does not replace a full security program.

---

## Production environment checklist

Use this before promoting any build to staging or production.

- [ ] **`APP_ENV`** (or `GO_ENV` / `ENV`) is set to `staging` or `production` as appropriate; config validation is stricter in those modes.
- [ ] **PostgreSQL:** `DB_*` credentials are strong and unique per environment; **`DB_SSLMODE`** meets policy (`verify-full` in production is recommended in [README](../README.md)).
- [ ] **`JWT_SECRET`** is a long, random value (secrets manager / vault); not committed or logged.
- [ ] **Redis (if enabled):** `REDIS_TLS_ENABLED=true` for non‑local hosts in staging/production; `REDIS_TLS_INSECURE_SKIP_VERIFY=true` is **not** allowed there.
- [ ] **Versioned migrations:** `USE_VERSIONED_MIGRATIONS=true` in production (AutoMigrate is refused in production — see [MIGRATIONS.md](MIGRATIONS.md)).
- [ ] **Swagger:** Default is off in staging/production; enable only with intent (`SWAGGER_ENABLED`) and admin JWT if required.
- [ ] **TLS termination:** Reverse proxy sets **`X-Forwarded-Proto: https`** when appropriate so **HSTS** (`HSTS_ENABLED`) can apply when you turn it on.
- [ ] **Security headers:** `SECURITY_HEADERS_ENABLED` left at default (`true`) unless you have a documented exception (e.g. COOP for specific OAuth flows — see README).
- [ ] **Email:** `EMAIL_ENABLED` and provider keys (`RESEND_API_KEY`, etc.) aligned with real mail in prod; `APP_PUBLIC_URL` matches the public API base for links.
- [ ] **OAuth (if used):** `OAUTH_*` client secrets in a secrets manager; `OAUTH_REDIRECT_BASE_URL` matches registered redirect URIs.
- [ ] **MFA / webhooks (if used):** valid **`MFA_ENCRYPTION_KEY`** and **`WEBHOOK_ENCRYPTION_KEY`** (see below); endpoints return **503** if keys are missing or invalid.
- [ ] **Rate limiting:** `RATE_LIMIT_REDIS_FAILURE_MODE` is **`local_fallback`** or **`fail_closed`** in staging/production — **`fail_open` is forbidden** there (process will not start).
- [ ] **Bootstrap admin:** only for first deploy; removed from env after success (see [Bootstrap admin](#bootstrap-admin-one-time)).
- [ ] **Logging:** Access logs omit query strings; enable HTTP audit with awareness of redaction rules ([README](../README.md) — `AUDIT_HTTP_ENABLED`).
- [ ] **CI green:** At least one full green run on your default branch including unit, integration, and supply‑chain jobs ([CI and scanners](#ci-and-scanner-expectations)).

---

## Secret rotation checklist

Rotate credentials on a schedule and after any suspected compromise.

| Secret / material | Typical rotation trigger | Notes |
|-------------------|-------------------------|--------|
| **`JWT_SECRET`** | Compromise, annual policy, staff change | Invalidates **existing** JWTs after deploy; users re‑login; plan a maintenance window or short overlap if you dual‑sign (not built into template). |
| **`MFA_ENCRYPTION_KEY`** | Rare; treat like DB encryption key | Changing it **breaks** decryption of stored MFA secrets until users re‑enroll MFA; coordinate with product policy. |
| **`WEBHOOK_ENCRYPTION_KEY`** | Rare | Changing it invalidates stored webhook secrets / payloads until org webhooks are reconfigured or re‑saved. |
| **DB password** (`DB_PASS` / `POSTGRES_PASSWORD`) | Policy, leak, offboarding | Update DB role + app env together; verify connections. |
| **Redis password** | Policy, leak | Update Redis ACL/password + `REDIS_PASSWORD` on all API instances. |
| **OAuth client secrets** | Provider rotation, leak | Update provider console + deployment env. |
| **Email provider API keys** (`RESEND_API_KEY`, etc.) | Policy, leak | Rotate in provider; update env. |
| **Org API keys / webhook signing** (application data) | Per‑org policy | Use admin/org APIs to revoke and reissue; audit who had access. |

**After each rotation**

- [ ] Redeploy all API replicas with the new value.
- [ ] Smoke test: login, one protected route, health, optional webhook test URL save.
- [ ] Confirm old credentials are removed from env files, compose files, and CI variables.

---

## Bootstrap admin (one‑time)

Optional first admin when no JWT exists yet. Variables: `BOOTSTRAP_ADMIN_ENABLED`, `BOOTSTRAP_ADMIN_EMAIL`, `BOOTSTRAP_ADMIN_PASSWORD`.

**Do**

1. Enable **only** on the first deploy where no `admin` user exists.
2. Use a **strong** password (staging/production enforce strength + denylist).
3. After the admin exists, **remove all three variables** from environment, compose, and secrets stores so the password is not retained.

**Do not**

- Leave bootstrap enabled indefinitely.
- Commit bootstrap passwords to git.
- Rely on bootstrap to “reset” an existing admin password (if the email already exists, bootstrap **skips** — see [README](../README.md)).

---

## MFA and webhook encryption keys

Both use the **same encoding rules** (see [repository `.env.example`](../../../.env.example)):

- **32 bytes** of key material encoded as **Base64** or **64 hex characters**.
- If unset: features that need encryption may be unavailable or degraded.
- If set but **invalid**: config logs a warning; **MFA enrollment** and **organization webhook** endpoints respond with **503** until the key is fixed.

Store keys only in a secrets manager. Never log or paste them into tickets.

---

## JWT access token limitation

Access tokens are **stateless JWTs** until they expire (`exp` claim). **Password reset** revokes **refresh** sessions server‑side but does **not** invalidate already‑issued access tokens immediately.

**Mitigations to consider for sensitive deployments**

- Shorter access token TTL (`JWT_ACCESS_TOKEN_MINUTES`).
- Server‑side access token versioning or denylist (product‑specific; not part of this template’s default path).

---

## Redis rate limiter failure modes

When Redis backs distributed rate limiting, `RATE_LIMIT_REDIS_FAILURE_MODE` controls behavior if the Redis increment fails:

| Mode | Behavior |
|------|-----------|
| **`local_fallback`** | Default in staging/production when unset. Falls back to **per‑process** in‑memory limiter for that request — limits are **not cluster‑wide** during the outage. |
| **`fail_closed`** | Returns **503** with a distinct error from **429** (rate limit exceeded). |
| **`fail_open`** | Allows the request through the limiter step. **Allowed only in development/test**; **forbidden** in staging/production — the application **will not start** with `fail_open` there. |

Invalid values log a warning and fall back to the environment default.

---

## Webhooks: DNS, SSRF, and redirects

Outbound webhook URL validation mitigates **SSRF** by:

- Requiring **HTTPS** in staging/production (narrow HTTP+loopback exception in dev/test only).
- Blocking private / link‑local / metadata‑style targets and rejecting URLs with embedded credentials.
- Resolving DNS at validation time and **re‑validating at delivery time** to reduce (not eliminate) **DNS rebinding** risk between validation and the HTTP request.

The webhook HTTP client **rejects redirects** (`CheckRedirect`), so a “safe” first hop cannot redirect into an internal network.

**Residual risk:** DNS TTL can still change addresses; rebinding cannot be fully eliminated without **IP pinning** or a custom `Dial` (possible future hardening). Document this for security reviewers and restrict webhook targets to trusted endpoints where possible.

---

## Docker: production baseline vs local dev

| Stack | Files | When to use |
|-------|--------|-------------|
| **Hardened baseline** | `docker/docker-compose.yml` only (`make docker-up-baseline`) | Production‑like or minimal exposure: requires explicit **repository root** `.env` (**`APP_ENV`**, **`DB_SSLMODE`**, strong secrets). No published DB/Redis ports, no admin UIs by default. |
| **Baseline + dev overlay** | `docker-compose.yml` + `docker-compose.dev.yml` (`make docker-up`) | **Local workstations** only: loopback ports, optional pgAdmin/Redis Commander, documented dev defaults. |

Image pins and upgrade workflow: [DOCKER.md](DOCKER.md).

---

## Migrations and `pgcrypto`

For production, use **versioned SQL migrations** (`USE_VERSIONED_MIGRATIONS=true`). UUID defaults use **`gen_random_uuid()`**, which requires the **`pgcrypto`** extension.

- Initial migration creates `pgcrypto` before tables that need it; a later migration repeats the guard for upgrades.
- **Managed Postgres** may require superuser‑like rights or pre‑enabling `pgcrypto` in the provider console — see [MIGRATIONS.md](MIGRATIONS.md).

---

## CI and scanner expectations

The template’s CI and Makefile aim for reproducible gates (pinned tool versions in CI — see [TESTING.md](TESTING.md)):

- **`go test -race ./...`** — unit tests with race detector.
- **`govulncheck`** — reachable vulnerability scan (needs network for DB).
- **`gosec`** — static security rules (medium thresholds; generated OpenAPI excluded where configured).
- **`gitleaks`** — secret detection in git history (`make secret-scan`).
- **Trivy** — HIGH/CRITICAL vulns on filesystem + built API image (`make container-scan`).
- **`make test-integration`** — Dockerized Postgres/Redis + integration tests.

**Release readiness:** Tag or publish a release candidate only after a **full green** pipeline on the commit you intend to ship, including integration and scan jobs if you enable them in your fork.

---

## Incident response basics

Use this as a **starting runbook**; extend with your org’s paging, legal, and customer comms.

1. **Detect and triage** — Alerts, error rate, auth anomalies, webhook failures, or external report. Capture time range, affected services, and first indicators.
2. **Contain** — Block malicious IPs at edge/WAF if applicable; disable compromised OAuth apps; revoke leaked API keys; rotate **JWT_SECRET** if signing key is exposed (forces re‑auth).
3. **Preserve evidence** — Export relevant logs (no query strings in default access logs; audit tables if enabled). Snapshot DB if forensic retention is required.
4. **Eradicate and recover** — Patch dependencies (re‑run `govulncheck` / Trivy), deploy fixes, rotate all credentials that may have been exposed.
5. **Communicate** — Internal status; external notice per policy if user data or tokens were at risk.
6. **Postmortem** — Blameless review: timeline, root cause, what worked, action items (monitoring, rate limits, key management, tests).

**Contacts:** Define in your fork who owns security, infra on‑call, and DPO/legal.

---

## Related documentation

- [README.md](../README.md) — env vars, rate limits, headers, bootstrap, JWT caveat.
- [DOCKER.md](DOCKER.md) — compose profiles, pins, production defaults.
- [MIGRATIONS.md](MIGRATIONS.md) — AutoMigrate vs SQL, `pgcrypto`.
- [QUEUE.md](QUEUE.md) — background jobs and Redis.
- [TESTING.md](TESTING.md) — local CI parity, secret/container scans, E2E.
