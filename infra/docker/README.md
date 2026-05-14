# chexi-trading Docker (infra)

Compose stack for **chexi-trading** local development: services **`chexi-db`**, **`chexi-redis`**, **`chexi-api`**, **`chexi-web`** (admin UI), **`chexi-traefik`**, plus dev-only **Redis Commander** and **pgAdmin** when using the dev overlay.

From the monorepo root, use:

```sh
make docker-up
make docker-down
```

For custom local values, copy `.env.example` to `.env` at the repository root. That file is shared with the Go API and the admin Vite app; do not keep separate `.env` copies under `apps/`.

## HTTP (canonical): Traefik + `*.localhost`

Traefik publishes **`TRAEFIK_HTTP_PORT`** on the host (default **80**). Use:

- `http://api.localhost`
- `http://admin.localhost` or `http://web.localhost` (admin SPA; same-origin **`/api`**)
- `http://pgadmin.localhost`
- `http://redis.localhost`

Add the hostnames to **`/etc/hosts`** (see [`infra/traefik/README.md`](../traefik/README.md)).

**Optional:** publish API and admin on loopback ports (**`8080`** / **`5174`**) for scripts, CI, or debugging:

```sh
docker compose --env-file .env \
  -f infra/docker/docker-compose.yml \
  -f infra/docker/docker-compose.dev.yml \
  -f infra/docker/docker-compose.direct-http.yml up -d
```

See [`docker-compose.direct-http.yml`](docker-compose.direct-http.yml).

**`make dev-local-api`:** the **`chexi-api`** container is stopped and the API runs on the host; Traefik can no longer reach the API at **`chexi-api:8080`**. Use **`http://127.0.0.1:${PORT:-8080}`** for the API (and optionally the **direct-http** overlay if you need published ports).

## Database passwords

`POSTGRES_PASSWORD` (container init) and `DB_PASS` / `DB_USER` (API) must match for a fresh stack. The dev overlay (`docker-compose.dev.yml`) falls back so setting only `DB_PASS` or only `POSTGRES_PASSWORD` still initializes Postgres consistently. If you change passwords after Postgres has already created its data volume, run `make docker-down-volumes` (destructive) or change the password inside Postgres manually.
