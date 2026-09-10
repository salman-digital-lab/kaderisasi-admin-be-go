# Go admin backend rules

This repository is the active Kaderisasi admin API and business-job implementation.
When working in the full workspace, also read `../AGENTS.md`. These repository
rules apply when this repository is opened independently.

## Ownership and entrypoints

- Implement all admin API features and fixes here, including auth, RBAC, members,
  activities, clubs, forms, counseling, achievements, certificates, files, and exports.
- `cmd/api` starts the HTTP API on port **3334**, with `/v2` routes and `/health`.
- `cmd/jobs` invokes one business job: `close:registration`,
  `clubs:close-registration`, or `clubs:update-visibility`. The API does not start
  jobs automatically. Keep scheduling external to the API.
- `../kaderisasi-admin-be` is database maintenance only: Ace migrations, seeders,
  and RBAC preflight. Do not restore its old API or proxy business endpoints to it.
- `../kaderisasi-web-be` remains an AdonisJS public API on port **3333** and shares
  the PostgreSQL schema. Keep its credentials and shared records compatible.

## Implementation

- Read `README.md`, `docs/TYPE_DESIGN.md`, and the relevant feature package before
  editing. Follow the toolchain and dependency versions pinned in `go.mod`,
  `go.sum`, `sqlc.yaml`, and the lockfiles.
- Use standard `net/http`. Keep HTTP parsing, middleware, handlers, and explicit
  request/response types in `internal/httpapi`; put workflows in the appropriate
  `internal/auth`, `access`, `member`, `activity`, `club`, `form`, `certificate`,
  `achievement`, or `counseling` package.
- Edit SQL in `database/queries` and run `sqlc generate` to update `internal/dbgen`.
  Do not hand-edit generated queries. `internal/database` owns connection helpers.
- Create and execute schema migrations only through Ace in the sibling migration
  repository. `database/schema.sql` is a sqlc snapshot, not executable migrations.
  Review shared web-backend models and queries when changing the schema.
- Preserve API methods, paths, statuses, error envelopes, JSON names, pagination,
  date formats, and missing/null behavior. Reuse `internal/domain`,
  `internal/validation`, and `internal/jscompat` for existing compatibility rules.
- Preserve password, JWT, signed refresh-cookie, rotation/reuse, and inactive-user
  behavior. Runtime roles and permissions live in `internal/auth/roles.json` and
  `internal/auth/permissions.json`; enforcement helpers live in `internal/auth/roles.go`.
  Keep the migration repository's preflight role-code list consistent with role changes.
- Pass contexts into database/storage operations. Preserve transactions, row and
  advisory locks, last-active-Super-Admin protection, and certificate duplicate
  prevention, immutable snapshots, and template-version checks.
- Use `internal/storage` for S3-compatible storage, `internal/media` for govips
  image processing, and `internal/export` for Excelize exports. Keep libvips in
  build/runtime packaging. Certificate PDF rendering remains in the frontend.

## Running and checking

```sh
make build
node scripts/run.mjs --environment=test api
node scripts/run.mjs --environment=test clubs:close-registration
make check
make test-unit
```

The runner injects `../docs/.env.test.be` into child processes; it does not replace
working env files. Use the configured test environment for fixtures. Production
credentials and resources are not test fixtures.

For integration and differential tests, follow `README.md`, install the sibling
applications' required dependencies, and prepare the historical API reference:

```sh
make prepare-reference
make test-integration
make test-contract
make test-shared
make test-jobs
make test-browser
make verify
make clean-fixtures
```

- Select affected suites for code changes; full-system verification uses `make verify`.
  Documentation-only changes need path/command checks rather than runtime suites.
- The pinned Adonis checkout in `.artifacts/legacy-admin-be` is a historical test
  reference, never the target for new features. Its revision is recorded in
  `docs/BASELINE.json`; do not silently change the baseline or regenerate its
  inventory to hide differences. Current migrations still run from the sibling
  maintenance repository.
- Use the existing fixture harness, ownership-marked schemas, explicit search
  paths without `public` fallback, and recorded storage keys. Always clean up
  owned resources, including after failures. Never reset shared tables.
- Keep changes compatible with both frontends and `web-be`. Extend meaningful
  HTTP, authorization, validation, database-effect, and browser tests as appropriate.
  Do not claim a suite passed if skipped or blocked; record the exact limitation.
- Use `docs/COOLIFY.md` for deployment and rollback. Historical rollback images
  are recovery options, not the default implementation for development.
