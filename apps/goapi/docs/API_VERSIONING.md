# API Versioning and Release Policy

This document defines how this project versions:

1. HTTP API contracts (`/api/v1`)
2. Application releases (Git tags like `v0.1.0`)
3. Database schema changes (numbered SQL migrations)

These versioning systems solve different problems and must not be mixed.

## 1) Route/API contract versioning (`/api/v1`)

`/api/v1` is the **public API contract namespace**. It tells API clients what request/response behavior they can rely on.

- Example endpoints:
  - `POST /api/v1/login`
  - `GET /api/v1/users/me`
  - `POST /api/v1/password-reset/confirm`
- Contract changes in this namespace are governed by backward-compatibility rules below.

## 2) App release versioning (Git tags, e.g. `v0.1.0`)

Git tags represent a **build/release snapshot** of the application source code and deployment artifact.

- Example: `v0.1.0`, `v0.1.1`, `v0.2.0`
- Multiple releases can happen while staying on `/api/v1`.
- A new release tag does **not** imply a new API namespace.

Recommended semantic meaning:

- `vX.Y.Z`
  - `Z`: patch/fix release (no intended breaking behavior)
  - `Y`: feature/minor release (backward compatible)
  - `X`: major release (can include breaking changes, often aligned with API version transition plans)

## 3) DB migration versioning (numbered SQL migrations)

DB migration versions (e.g. `000001_*.up.sql`) track **schema evolution order**, not API or app release compatibility by themselves.

- Migrations are sequential and operational.
- They can be introduced in patch/minor releases.
- A migration number is not an API version.

## What counts as a breaking API change

A change is breaking if an existing client that follows the documented `/api/v1` contract may fail without changing client code.

Examples:

- Removing an endpoint or changing its path/method.
- Removing response fields that clients depend on.
- Changing field type/format (e.g. string -> object, UUID -> integer).
- Tightening validation in a way that previously valid requests now fail.
- Changing auth/authorization requirements for an existing endpoint.
- Changing status code semantics for successful/expected flows in incompatible ways.

## Allowed changes inside `/api/v1`

The following are allowed as backward-compatible evolution:

- Adding new endpoints under `/api/v1`.
- Adding optional request fields.
- Adding new response fields (without removing/changing existing fields).
- Internal performance, observability, and refactoring changes that preserve API behavior.
- Bug fixes that align behavior with existing documented contract.

When in doubt, prefer additive behavior and document changes in `CHANGELOG.md`.

## When to introduce `/api/v2`

Introduce `/api/v2` when a needed change is breaking and cannot be delivered safely as an additive `/api/v1` update.

Typical triggers:

- Required payload shape redesign.
- Endpoint semantics/status model redesign.
- Auth model changes that break old clients.
- Removal/replacement of widely used `/api/v1` fields or routes.

Recommended process:

1. Define `/api/v2` contract and migration guide.
2. Keep `/api/v1` and `/api/v2` running in parallel for a deprecation window.
3. Communicate cutoff timeline in `CHANGELOG.md` and release notes.

## Release checklist

Before creating a release tag:

- [ ] `go test -race ./...` passes
- [ ] `make ci` passes
- [ ] `make test-integration` passes
- [ ] migrations reviewed (order, rollback, data-safety implications)
- [ ] `CHANGELOG.md` updated
- [ ] Git tag created (for example `v0.1.0`)
