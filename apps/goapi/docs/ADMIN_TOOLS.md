# Admin Tools Guide

Redis Commander and pgAdmin run only with the **local dev overlay** (`docker-compose.dev.yml`), not the hardened baseline compose file alone.

## Redis Commander

Web-based Redis management interface for viewing and managing Redis cache data.

### Access

- **URL (canonical):** `http://redis.localhost` — add **`redis.localhost`** to **`/etc/hosts`** (see repository root [`infra/traefik/README.md`](../../../infra/traefik/README.md)). Traefik is started with **`make docker-up`** (dev overlay). If **`TRAEFIK_HTTP_PORT`** is not **80**, include the port (e.g. `http://redis.localhost:9080`).
- **Username / password**: `REDIS_COMMANDER_HTTP_USER` / `REDIS_COMMANDER_HTTP_PASSWORD` (see repository root `.env.example`; dev overlay supplies non-production defaults if unset)

### Features

- Browse Redis keys and values
- View cached user data (`user:id:*`, `user:email:*`)
- View rate limiting data (`ratelimit:*`)
- Edit/delete keys
- Monitor Redis operations
- Execute Redis commands

### Usage

1. Start the dev stack (from repository root):
   ```bash
   make docker-up
   ```
   Or explicitly:
   ```bash
   docker compose --env-file .env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml up -d
   ```

2. Access Redis Commander:
   ```bash
   make docker-open-redis-commander
   # Or open http://redis.localhost (default Traefik on host port 80)
   ```

3. Log in with the credentials from your **repository root** `.env` (or the dev-overlay defaults documented in `.env.example`).

### Common Tasks

**View cached users:**
- Filter keys by pattern: `user:*`
- Click on a key to view its JSON value
- Keys follow patterns:
  - `user:id:{id}` - User cached by ID
  - `user:email:{email}` - User cached by email
  - `ratelimit:{ip}` - Rate limiting counters

**Monitor cache activity:**
- Watch keys being created/updated in real-time
- View TTL (Time To Live) for each key
- See when cache entries expire

## pgAdmin

Web-based PostgreSQL administration and development platform.

### Access

- **URL (canonical):** `http://pgadmin.localhost` — add **`pgadmin.localhost`** to **`/etc/hosts`** (see [`infra/traefik/README.md`](../../../infra/traefik/README.md)). If **`TRAEFIK_HTTP_PORT`** is not **80**, include the port.
- **Email / password**: `PGADMIN_DEFAULT_EMAIL` / `PGADMIN_DEFAULT_PASSWORD` in the repository root `.env`

### Features

- Database browser and query tool
- SQL editor with syntax highlighting
- Table data viewer and editor
- Query history
- Database statistics and monitoring
- Export/import data

### Usage

1. Start the dev stack:
   ```bash
   make docker-up
   ```
   Or:
   ```bash
   docker compose --env-file .env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml up -d
   ```

2. Access pgAdmin:
   ```bash
   make docker-open-pgadmin
   ```

3. Log in with the email/password from the **repository root** `.env`.

### Setting Up Database Connection

After logging in, register a server:

1. Right-click "Servers" → "Register" → "Server"

2. **General Tab:**
   - Name: `GoAPI Database` (or any name)

3. **Connection Tab:**
   - Host name/address: `chexi-db` (Docker service name from inside the compose network)
   - Port: `5432`
   - Maintenance database: value of `POSTGRES_DB` (e.g. `goapi`)
   - Username: `POSTGRES_USER` (e.g. `goapi_dev` with dev defaults)
   - Password: `POSTGRES_PASSWORD` from the **repository root** `.env`
   - Check "Save password"

4. Click "Save"

### Common Tasks

**View tables:**
- Navigate: Servers → GoAPI Database → Databases → goapi → Schemas → public → Tables
- Right-click on `users` table → "View/Edit Data" → "All Rows"

**Run SQL queries:**
- Right-click on database → "Query Tool"
- Write SQL queries:
  ```sql
  SELECT * FROM users;
  SELECT * FROM users WHERE email = 'test@example.com';
  ```

**View table structure:**
- Right-click on table → "Properties"
- See columns, constraints, indexes

## Makefile Commands

Quick access commands:

```bash
make docker-open-redis-commander
make docker-open-pgadmin
make docker-logs-redis-commander
make docker-logs-pgadmin
```

## Security Note

⚠️ Admin UIs and their credentials are **local development only**. They are not started by the hardened baseline compose file.

- Do not expose these ports beyond `127.0.0.1` on shared or production hosts without strong auth and network controls.
- Prefer `make docker-up-baseline` (baseline only) for production-like deployments and supply secrets via a secure secret store — not committed `.env` files.

## Troubleshooting

### Redis Commander won't connect

- Ensure Redis container is running: `docker ps | grep chexi-redis`
- Check Redis is healthy: `docker compose -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml ps chexi-redis` (from repository root)
- Verify Redis Commander logs: `make docker-logs-redis-commander`

### pgAdmin can't connect to database

- Ensure database container is running: `docker ps | grep chexi-db`
- Check database is healthy: `docker compose -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml ps chexi-db` (from repository root)
- Verify connection details:
  - Host: `chexi-db` (not `localhost` when connecting from the pgAdmin container)
  - Port: `5432`
  - Username/password: match `POSTGRES_USER` / `POSTGRES_PASSWORD`

### Port conflicts

For **Traefik** (default HTTP entry for admin tools), set **`TRAEFIK_HTTP_PORT`** in the **repository root** `.env` if **port 80** is taken (for example **`9080`**).

For **Postgres** / **Redis** host publishes, use **`POSTGRES_PUBLISH_PORT`**, **`REDIS_PUBLISH_PORT`** as documented in the repository root `.env.example`.

For **direct** API/admin HTTP on loopback (**`8080`** / **`5174`**), use [`infra/docker/docker-compose.direct-http.yml`](../../../infra/docker/docker-compose.direct-http.yml) (see [`infra/docker/README.md`](../../../infra/docker/README.md)).
