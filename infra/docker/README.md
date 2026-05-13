# chexi-trading Docker (infra)

Compose stack for **chexi-trading** local development: services **`chexi-db`**, **`chexi-redis`**, **`chexi-api`**, plus dev-only **Redis Commander** and **pgAdmin** when using the dev overlay.

From the monorepo root, use:

```sh
make docker-up
make docker-down
```

For custom local values, copy `.env.example` to `.env` at the repository root. That file is shared with the Go API and the admin Vite app; do not keep separate `.env` copies under `apps/`.

## Traefik (optional)

To put the API behind a local reverse proxy (host **`api.localhost`**, default **`http://api.localhost:9080`**), use the third Compose file or `make docker-up-traefik`. See `infra/traefik/README.md`.

## Database passwords

`POSTGRES_PASSWORD` (container init) and `DB_PASS` / `DB_USER` (API) must match for a fresh stack. The dev overlay (`docker-compose.dev.yml`) falls back so setting only `DB_PASS` or only `POSTGRES_PASSWORD` still initializes Postgres consistently. If you change passwords after Postgres has already created its data volume, run `make docker-down-volumes` (destructive) or change the password inside Postgres manually.
