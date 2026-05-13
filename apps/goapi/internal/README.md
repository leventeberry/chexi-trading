# Internal Package Layout

This directory defines the long-term architecture boundaries for the service.

- `app`: composition root and runtime bootstrap orchestration.
- `infra`: infrastructure adapters and implementation details.
  - `auth`: JWT creation/parsing and password hashing helpers.

Dependency direction contract:

1. transport packages call service/domain interfaces
2. service/domain packages call repositories; they may call **concrete** infrastructure helpers when injected at construction time (e.g. `internal/infra/auth.Manager` for JWT, shared password hashing utilities).
3. infrastructure packages do not import transport packages

This contract prevents circular dependencies and keeps HTTP concerns out of business logic.
