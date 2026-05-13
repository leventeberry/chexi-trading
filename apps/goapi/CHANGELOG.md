# Changelog

All notable changes to this project should be documented in this file.

The format is inspired by [Keep a Changelog](https://keepachangelog.com/), and versions are expected to be represented by Git tags.

## [v1.0.0] - 2026-05-10

**chexi-trading** baseline **v1.0.0** — production-oriented API baseline with documented security posture and supply-chain gates (forked from the prior GoAPI template release).

### Added

- **[docs/SECURITY.md](docs/SECURITY.md)** — single security and operations entrypoint (production env checklist, secret rotation, bootstrap admin removal, MFA/webhook encryption keys, JWT access-token limitation, Redis rate-limit failure modes, webhook DNS/SSRF caveat, Docker baseline vs dev, migrations/`pgcrypto`, CI scanner expectations, incident response basics).
- **`docs/TEST_COVERAGE.md`** — coverage strategy and guidance.
- **`docs/API_VERSIONING.md`** — API/versioning/release policy.
- Expanded repository tests (integration coverage for user, auth token, and user settings repositories; cache no-op/Redis behavior; health/startup and route/app/container/logger smoke coverage).

### Security

- HTTP security headers; access logs omit query strings; HTTP audit redaction for sensitive query parameters when enabled.
- Webhook outbound URL validation (HTTPS in staging/production; private/link-local blocking; DNS resolution; re-validation at delivery); webhook HTTP client rejects redirects.
- Organization tenant isolation and org API key binding; Redis-backed rate limiting with `fail_open` forbidden outside development/test.
- Versioned SQL migrations with `pgcrypto` / `gen_random_uuid()` guard migrations.
- Bumped `github.com/jackc/pgx/v5` to **v5.9.0** (addresses CVE-2026-33816 per Trivy / dependency scan).

### Tooling / CI

- `govulncheck`, `gosec`, race unit tests, Docker integration tests, **gitleaks** secret scanning (`.gitleaks.toml`, `.gitleaksignore`; `secret-scan` CI job), **Trivy** filesystem + container image scanning (`.trivyignore`; `container-scan` CI job); local parity via `Makefile`; docs in `docs/TESTING.md`.

### Known limitations (documented)

- Stateless JWT access tokens are not revoked immediately on password reset (valid until `exp`).
- Webhook URL controls reduce SSRF but cannot fully eliminate DNS rebinding between validation and delivery.
- Under Redis outages, `local_fallback` rate limits are enforced per process, not cluster-wide.

## [Unreleased]

### Fixed

- **CI integration job:** `scripts/test_integration_local.sh` now exports `APP_ENV`, `DB_SSLMODE`, and related compose variables before `docker compose up`, so GitHub Actions (and any minimal env) satisfies baseline `${DB_SSLMODE:?...}` interpolation when only starting `db` and `redis`.
