# ADR-001: HTTP package layout under internal/transport/http

## Status

Accepted

## Context

The API had parallel HTTP layers: root `routes/`, `middleware/`, and `controllers/` plus thin delegates under `internal/transport/http/`. Unused `internal/domain` alias packages added noise without modeling behavior.

## Decision

Implement routes, middleware, and HTTP handlers only under `internal/transport/http/`. Delete the root HTTP packages after migration. Wire dependencies through `container/container.go` without separate factory packages.

## Consequences

- **Easier navigation:** One tree for HTTP concerns.
- **One-time churn:** Imports and generated Swagger definitions moved from `controllers.*` to `handlers.*`.
- **Documentation:** Architecture docs describe `internal/transport/http` as the HTTP layer.
