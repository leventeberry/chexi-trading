# Traefik (default dev HTTP ingress)

File-based routing under [`dynamic/`](dynamic/) on Traefik’s HTTP entrypoint (`:80` in the container, published to the host as **`TRAEFIK_HTTP_PORT`**, default **80**).

The **`chexi-traefik`** service is defined in [`infra/docker/docker-compose.dev.yml`](../docker/docker-compose.dev.yml) and starts with **`make docker-up`** (base + dev compose files).

| Host | Behavior |
|------|-----------|
| **`api.localhost`** | All paths → **`chexi-api:8080`** ([`dynamic/chexi-api.yml`](dynamic/chexi-api.yml)) |
| **`admin.localhost`** | **`PathPrefix(/api)`** → **`chexi-api:8080`**; everything else → **`chexi-web:80`** ([`dynamic/chexi-admin.yml`](dynamic/chexi-admin.yml)) |
| **`web.localhost`** | Same as **`admin.localhost`** (alias for the admin SPA + `/api`) ([`dynamic/chexi-admin.yml`](dynamic/chexi-admin.yml)) |
| **`pgadmin.localhost`** | → **`pgadmin:80`** ([`dynamic/chexi-devtools.yml`](dynamic/chexi-devtools.yml)) |
| **`redis.localhost`** | → **Redis Commander** `8081` ([`dynamic/chexi-devtools.yml`](dynamic/chexi-devtools.yml)) |

Use **`admin.localhost`** or **`web.localhost`** when you want the admin UI and **`GET /api/...`** on the **same origin** (empty `VITE_GOAPI_BASE_URL`) without relying on the in-container nginx `/api` proxy.

## Run with the stack

From the repo root:

```sh
make docker-up
```

Or explicitly:

```sh
docker compose --env-file .env \
  -f infra/docker/docker-compose.yml \
  -f infra/docker/docker-compose.dev.yml up -d
```

## Local DNS / hosts

Add:

```text
127.0.0.1 api.localhost admin.localhost web.localhost pgadmin.localhost redis.localhost
```

## Ports (defaults)

| Purpose | URL / port |
|---------|------------|
| **Admin UI** (Traefik → `chexi-web` + `/api` → API) | `http://admin.localhost` or `http://web.localhost` |
| **API** via Traefik | `http://api.localhost` |
| **pgAdmin** via Traefik | `http://pgadmin.localhost` |
| **Redis Commander** via Traefik | `http://redis.localhost` |
| Traefik dashboard (insecure, dev only) | `http://127.0.0.1:9081/dashboard/` (`TRAEFIK_DASHBOARD_PORT`) |
| **Direct HTTP** (optional compose file) | [`infra/docker/docker-compose.direct-http.yml`](../docker/docker-compose.direct-http.yml) — `http://127.0.0.1:8080` (API) and `http://127.0.0.1:5174` (admin nginx) |

If **port 80** is already in use on your machine, set **`TRAEFIK_HTTP_PORT`** in the repository root `.env` (for example **`9080`**) and use `http://admin.localhost:9080`, etc.

OAuth and link generation should use the URL clients actually call (for example **`APP_PUBLIC_URL=http://api.localhost`** when using Traefik on port 80).

## nginx vs Traefik

- **`chexi-web` image** still ships **nginx** to serve the Vite `dist/` and SPA `try_files`, and (if you use the optional **direct-http** overlay) to **reverse-proxy `/api`** to `chexi-api`.
- **Traefik** is the **primary** dev entry for HTTP: routing rules live in **`infra/traefik/dynamic/*.yml`**. Traefik watches that directory (`--providers.file.watch=true`).

## Changing routes

Edit YAML under [`dynamic/`](dynamic/). Traefik reloads when file watch is enabled (see `chexi-traefik` command in `docker-compose.dev.yml`).
