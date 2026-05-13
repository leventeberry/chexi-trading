# Safe migration: Go module path (`goapi` → new path)

The API module is currently **`module goapi`** ([`go.mod`](../go.mod)) with imports like `goapi/internal/...`. Renaming the module **does not change business logic** but touches **every import**, CI paths, and any external consumers that `go get` this module.

## When to do it

- After local Docker and monorepo paths are stable.
- When you have a target module path (examples: `github.com/yourorg/chexi-trading/apps/api`, `chexi.trading/api`).

## Preconditions

- Full tree builds: `go test ./...` and `go build ./...` from `apps/goapi`.
- Decide the **exact** new module path; it must be valid for `go.mod` and match where the repo will live long-term.

## Recommended steps

1. **Choose the new path** and ensure `go.mod` `module` line uses it once.

2. **Mechanical rewrite** (pick one):
   - `gofmt` / editor: find-replace import prefix `"goapi/` → `"<newmodule>/` across `apps/goapi/**/*.go`.
   - Or `go run golang.org/x/tools/cmd/goimports@latest` after rewrites (won’t rename alone).
   - Or `sed`/ripgrep-assisted batch replace; re-run `go mod tidy`.

3. **Update `go.work`** at the repo root: the `use` path stays `./apps/goapi`; only the module name inside that folder’s `go.mod` changes.

4. **Verify** from repo root and from `apps/goapi`:
   ```bash
   cd apps/goapi && go test ./... && go build -o /tmp/chexi-api-test ./...
   cd ../.. && go work sync
   ```

5. **Docker / binary name (optional):** the container binary is still built as `goapi` in [`docker/Dockerfile`](../docker/Dockerfile) (`-o goapi`). Renaming the output binary to e.g. `chexi-api` is independent of the module path; update `Dockerfile` `CMD`, `Makefile` `APP_NAME`, and root `.gitignore` / `.dockerignore` entries if you change it.

6. **CI:** update any workflow that assumes the old module string (grep for `goapi` in `.github`).

7. **Tags / semver:** if you publish this as a library, treat the rename as a **major** API surface change for importers; for an app-only repo, tags are less critical.

## Rollback

- Revert the single commit that performs the rename, or restore `go.mod` and import paths from VCS.

## Do not rename blindly

- `goapi` also appears as the **compiled binary filename** and in **Postgres/dev defaults** in the repository root `.env.example`; those are **not** the Go module and can stay for backward compatibility until you intentionally rebrand volumes and local DB names.
