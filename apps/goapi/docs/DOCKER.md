# Docker: baseline vs dev overlay, pins, and upgrades

## Compose layouts

| Stack | Files | Purpose |
|-------|--------|---------|
| **Local development** | `docker-compose.yml` + `docker-compose.dev.yml` | Published DB/Redis on loopback, optional Redis Commander / pgAdmin, **dev defaults** for `APP_ENV`, `DB_SSLMODE`, and credentials (`make docker-up`, `make docker-all`). |
| **Hardened baseline only** | `docker-compose.yml` | No admin UIs, DB/Redis not published by default. **`APP_ENV` and `DB_SSLMODE` must be set** (no insecure compose defaults). Use `make docker-up-baseline` with the **repository root** `.env`. |

The dev overlay overrides `APP_ENV` / `DB_SSLMODE` with `${VAR:-…}` so local workflows work without listing every variable. The baseline uses `"${APP_ENV:?…}"` / `"${DB_SSLMODE:?…}"` so **baseline-only** deploys fail fast if those are missing from the environment.

## Required variables (baseline-only / production-like)

With **only** `docker/docker-compose.yml`, set at least in the **repository root** `.env` (or the shell):

- **`APP_ENV`** — e.g. `staging` or `production` (not defaulted by the baseline).
- **`DB_SSLMODE`** — e.g. `require`, `verify-ca`, or `verify-full` for TLS to Postgres; avoid `disable` outside local/dev.
- **`POSTGRES_*`**, **`DB_*`**, **`JWT_SECRET`** — non-empty secrets (see [repository `.env.example`](../../../.env.example)).

With **baseline + dev overlay**, you still override passwords/JWT in the **repository root** `.env` on shared machines; `APP_ENV` defaults to `development` and `DB_SSLMODE` to `disable` via the overlay.

## Pinned images

Versions are pinned in compose and the API [`Dockerfile`](../docker/Dockerfile) to avoid floating `:latest` drift. Current pins (bump intentionally when upgrading):

| Image | Pin location |
|-------|----------------|
| `postgres` | `docker/docker-compose.yml` → `postgres:16.13-alpine` |
| `redis` | `docker/docker-compose.yml` → `redis:7.4.9-alpine3.21` |
| `golang` (builder) | `docker/Dockerfile` → `golang:1.25.10-alpine` (align with `go.mod`) |
| `alpine` (runtime) | `docker/Dockerfile` → `alpine:3.21` |
| `dpage/pgadmin4` | `docker/docker-compose.dev.yml` → e.g. `9.14.0` |
| `rediscommander/redis-commander` | Digest-pinned in `docker-compose.dev.yml` (upstream rarely tags releases). |

## Updating container images

1. Check release notes / CVEs for Postgres, Redis, Alpine, pgAdmin, Go base images.
2. Edit pins in `docker/docker-compose.yml`, `docker/docker-compose.dev.yml`, and/or `docker/Dockerfile`.
3. Rebuild with fresh bases: `docker compose … build --pull` (or `make docker-build` after `docker compose pull` where applicable).
4. Run tests: `go test -race ./...`, `make ci`, `make test-integration`, bring up stack (`make docker-all`), `make test-e2e-docker` as needed. For supply-chain checks: `make container-scan` (Trivy against the repo + the image built from `docker/Dockerfile`).
5. For Redis Commander, if the digest is pinned and you need a newer UI build, resolve a new digest (`docker pull rediscommander/redis-commander:latest && docker inspect …`) and update the compose file.

## Supply-chain scanning (Trivy)

After changing **`docker/Dockerfile`** base images or Compose image pins, run **`make container-scan`** locally (see [docs/TESTING.md](TESTING.md#container--dependency-scanning-trivy)). Versions: **`TRIVY_VERSION`** in the [Makefile](../Makefile) must match **`.github/workflows/ci.yml`** when you bump the scanner.

Optional hardening: pin runtime images by **digest** (`image: repo/name@sha256:…`) for reproducible deploys; document each digest change in commit messages.
