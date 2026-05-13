# Database schema: AutoMigrate vs versioned SQL

The API supports two paths (see [`initializers/migrateDB`](../initializers/initializers.go)):

| Mode | When | What runs |
|------|------|-----------|
| **GORM AutoMigrate** (default locally) | `USE_VERSIONED_MIGRATIONS` is not `true` | GORM applies schema from registered models (see `migrateDB` list). New models must be added to this list or tables will be missing. |
| **Versioned SQL migrations** | `USE_VERSIONED_MIGRATIONS=true` | Files under [`migrations/`](../migrations/) via golang-migrate. |

**Production:** startup refuses AutoMigrate unless versioned migrations are enabled (`APP_ENV=production` guard).

**Avoiding drift**

- Prefer **one source of truth** per environment: either rely on versioned SQL everywhere in staging/production, or maintain AutoMigrate only for local/dev and ensure new models are mirrored in SQL migrations before release.
- When adding a `models.*` struct, update AutoMigrate in [`initializers/initializers.go`](../initializers/initializers.go) **and** add a matching migration when using versioned mode.

## Extensions (`pgcrypto`)

Versioned SQL uses `DEFAULT gen_random_uuid()` on UUID primary keys. That function requires the **`pgcrypto`** extension.

- **`000001_init_users.up.sql`** runs `CREATE EXTENSION IF NOT EXISTS pgcrypto` before creating tables that use `gen_random_uuid()`.
- **`000013_pgcrypto_ensure.up.sql`** repeats the same `CREATE EXTENSION IF NOT EXISTS pgcrypto` for idempotency and as an explicit guard when upgrading existing databases (cannot safely insert a migration numbered before `000001` once deployments exist).

**Managed PostgreSQL** (Amazon RDS, Google Cloud SQL, Azure Database for PostgreSQL, Supabase, Neon, etc.):

- The migration role may need permission to run `CREATE EXTENSION`. Many providers require a **superuser** or a cloud-specific role (e.g. RDS: migration user with `rds_superuser` / extension creation enabled), or pre-enabling **pgcrypto** in the provider console for the database once.
- If `CREATE EXTENSION` fails with permission denied, create **pgcrypto** using the provider’s documented procedure, then re-run migrations (`make migrate-sql-up` or application startup with `USE_VERSIONED_MIGRATIONS=true`).

The same [`migrations/`](../migrations/) directory is used by **`make migrate-sql-up`** (via `DATABASE_URL`) and by **`RunVersionedMigrations`** at startup when versioned mode is enabled.
