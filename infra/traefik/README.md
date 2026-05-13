# Traefik (optional dev edge)

File-based routing sends **`Host: api.localhost`** on Traefik’s HTTP entrypoint to **`chexi-api:8080`** on the Compose network.

## Run with the stack

From the repo root (same pattern as the main Compose docs):

```sh
make docker-up-traefik
```

Or explicitly:

```sh
docker compose --env-file .env \
  -f infra/docker/docker-compose.yml \
  -f infra/docker/docker-compose.dev.yml \
  -f infra/docker/docker-compose.traefik.yml up -d
```

## Local DNS / hosts

Add:

```text
127.0.0.1 api.localhost
```

## Ports (defaults)

| Purpose        | URL / port |
|----------------|------------|
| API via Traefik | `http://api.localhost:9080` (`TRAEFIK_HTTP_PORT`) |
| Traefik dashboard (insecure, dev only) | `http://127.0.0.1:9081/dashboard/` (`TRAEFIK_DASHBOARD_PORT`) |
| API direct (Compose publish) | `http://127.0.0.1:8080` (`API_PUBLISH_PORT`) — unchanged |

OAuth and link generation should use the URL clients actually call (for example `APP_PUBLIC_URL` when everything goes through Traefik).

## Changing routes

Edit `infra/traefik/dynamic/chexi-api.yml`. Traefik watches that directory when using the dev overlay.

The admin UI (`apps/shadcn-admin`) is a separate Vite dev server unless you add another router and a container for static assets or a dev proxy.
