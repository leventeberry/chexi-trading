# Documentation index

Start with the [app README](../README.md) for installation, environment variables, Makefile commands, and Docker. For monorepo layout and Compose from the repo root, see the [repository README](../../README.md).

## Reading guide

| Document | Purpose |
|----------|---------|
| [SECURITY.md](SECURITY.md) | **Start here for production:** env checklist, secret rotation, bootstrap admin, MFA/webhook keys, JWT/webhook/rate-limit caveats, Docker/migrations, CI scanners, incident basics. |
| [DOCKER.md](DOCKER.md) | Baseline vs dev overlay, required env for hardened deploys, pinned images, upgrades. |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Layers, patterns, dependency flow, cache behavior. |
| [MIGRATIONS.md](MIGRATIONS.md) | AutoMigrate vs versioned SQL (`migrations/`) for production. |
| [TESTING.md](TESTING.md) | Manual API checks (curl/PowerShell); integration tests with `-tags=integration`. |
| [ADMIN_TOOLS.md](ADMIN_TOOLS.md) | Redis Commander and pgAdmin when using Docker Compose. |
| [MIGRATION_GO_MODULE.md](MIGRATION_GO_MODULE.md) | Optional rename of the Go module path (`goapi` → …): steps, risks, verification. |
| [adr/](adr/) | Architecture Decision Records (see [adr/README.md](adr/README.md)). |
| [CODE_REVIEW.md](CODE_REVIEW.md) | Non-exhaustive backlog of improvements from reviews—not a specification. |
| [RESTRUCTURE_ROADMAP.md](RESTRUCTURE_ROADMAP.md) | Historical migration phases and optional follow-ups. |

## API surface

Versioned JSON APIs live under **`/api/v1`**. `GET /` returns a welcome payload; **`GET /health`** is the health check. OpenAPI is generated under `api/openapi/` (`make swagger`).
