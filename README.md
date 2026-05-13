# chexi-trading

Monorepo for **chexi-trading**: crypto trading analysis with a Go API, data ingestion (e.g. Coinbase), and an admin UI. Docker Compose, env examples, and UI metadata use the `chexi-trading` name for the runtime stack.

## Layout

| Area | Path |
|------|------|
| Go API | [`apps/goapi`](apps/goapi) |
| Admin UI (Vite/React today) | [`apps/shadcn-admin`](apps/shadcn-admin) |
| Docker Compose (local) | [`infra/docker`](infra/docker) |

## Local development

1. Copy [`.env.example`](.env.example) to `.env` at the repo root and adjust secrets.
2. Start the stack:

   ```sh
   make docker-up
   ```

3. API health: `http://127.0.0.1:${API_PUBLISH_PORT:-8080}/health` (see `.env.example`).

- **`make dev`** — same as `make docker-up` (API runs in the `chexi-api` service container).
- **`make dev-local-api`** — Compose + run the Go binary on the host (stops the `chexi-api` container to avoid port clashes).
- **`make api-dev`** — run the API on the host (expects DB/Redis reachable, e.g. after `make docker-up`).

See [`apps/goapi/README.md`](apps/goapi/README.md) and [`infra/docker/README.md`](infra/docker/README.md) for details.

## Go workspace

Root [`go.work`](go.work) includes `./apps/goapi`. The Go module path remains `goapi` until a deliberate rename; see [`apps/goapi/docs/MIGRATION_GO_MODULE.md`](apps/goapi/docs/MIGRATION_GO_MODULE.md).
