# Admin backend in Go

Agent contribution rules: [AGENTS.md](AGENTS.md). `CLAUDE.md` imports the same rules.

The Kaderisasi admin API on port **3334**. It serves
`/v2` and `/health`; it never starts scheduled jobs. The sibling `kaderisasi-admin-be` repository owns
database migrations, seeders, and RBAC preflight.

Production runs Go through [Coolify](docs/COOLIFY.md). The former Adonis API
repository now contains only database maintenance code.

Internal staff talent reflection adds self-assessment, resumable drafts, current
results, and Super Admin result review. See [assessment workflow and rollout](docs/TALENT_ASSESSMENT.md).

Kelas adds course management, learner progress, and private PDF storage. See
[course configuration, rollout, and native verification](docs/COURSES.md).
Activities can also track linked courses in their registrant list and Excel
exports. See [linked activity courses](docs/ACTIVITY_COURSES.md).

## Local setup

Required: Go **1.26.10**, Node **24**, PostgreSQL client tools, sqlc **1.31.1**, and
libvips with JPEG, PNG, and WebP support. The verified macOS setup uses libvips
**8.18.6** and pkgconf **3.0.0**. govips requires libvips 8.14 or later.

```sh
brew install go vips pkgconf libpq
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
# Confirm these match the pinned Go/sqlc versions before generating code.
go version
"$(go env GOPATH)/bin/sqlc" version
pkg-config --modversion vips
go mod download
npm ci
make prepare-reference
npx playwright install chromium
make build
make package
```

`go.mod`, `go.sum`, and `package-lock.json` pin dependencies. Go's toolchain directive
selects 1.26.10. `CGO_ENABLED=1` and libvips development headers are required to build;
the runtime must also contain libvips and its shared-library dependencies. Docker
is not required. The Go binary is not a standalone static executable.

`make package` builds a native archive for the current OS and architecture under
`.artifacts/release/`, records the shared-library dependencies, and checks that both
executables reach startup validation. Install the runtime described in its
`RUNTIME.txt` on the destination. Build on Linux for a Linux destination; the
verified macOS archive is not a cross-platform distribution.

The fixture harness and the session-transfer checks load Node dependencies (`pg`,
`jsonwebtoken`, Adonis password hashing) from a detached checkout of the former Adonis
repository at the revision in `docs/BASELINE.json`. Run `make prepare-reference` to create it
under `.artifacts/legacy-admin-be` and install the crypto fixture dependencies in
`tests/interop`. Set `ADONIS_REFERENCE_DIR` to use another checkout at that revision. It is
a test dependency only; no HTTP contract comparison against Adonis remains. Ace migrations
run from the migration-only `kaderisasi-admin-be` repository.

## Running

From this directory:

```sh
node scripts/run.mjs --environment=test api
node scripts/run.mjs --environment=test close:registration
node scripts/run.mjs --environment=test clubs:close-registration
node scripts/run.mjs --environment=test clubs:update-visibility
```

The runner reads `../env/test/admin-be-test-env` and injects values into its child process.
It does not replace an application env file. `--environment=prod` selects the
production document. The deployed service receives its configuration from Coolify.
For a prebuilt binary, inject the existing environment variables and start
`bin/admin-api` or `bin/admin-jobs <job>` directly.

All existing variable names are retained. `DB_SCHEMA` is optional and defaults to
`public`; test schemas use an explicit search path without `public` fallback.
The club registration closing job always uses Asia/Jakarta. Set `TZ=Asia/Jakarta`
for the other two jobs to preserve their existing calendar behavior. `ADMIN_CORS_ORIGINS` is a comma
separated allowlist; local public frontend requests require
`http://localhost:3000` alongside `http://localhost:3005`. `APP_KEY` must be identical
to the migration repository and web-be for session transfer. Never print or commit env files.

Run the API directly with `node scripts/run.mjs --environment=test api`, or start
`bin/admin-api` with injected configuration. The migration repository is not started.

## Database ownership

Run migrations and RBAC preflight **from Adonis**, never from Go:

```sh
cd ../kaderisasi-admin-be
node ace migration:run
node ace rbac:preflight
```

`database/schema.sql` is a generated snapshot used by sqlc, not an executable
migration source. No Go migration runner or general demo seeder exists.

## Verification

`DELETE /v2/activities/:id` and `DELETE /v2/clubs/:id` require Super Admin
(`super_admin`) or Asisten Manager Program (`admin`) and a JSON body containing
`confirmation` equal to the current saved name. The transaction deletes registrations,
detaches and deactivates registration forms, and preserves member accounts. Club
activities remain with their club association cleared. Activities with issued
certificates or approval history return 409 without deleting data. Existing media
objects remain in storage. No schema migration is needed. Run
`make test-feature-deletion` for native authorization, data-effect, and desktop/mobile
browser checks; the full verification runner includes this suite.

```sh
make check
make test-unit
make test-integration
make test-shared
make test-browser
make benchmark
make verify
```

Integration targets create or reuse three recorded schemas in the configured
shared **test** database. They run Ace migrations and deterministic synthetic
fixtures. Ownership comments and explicit search paths protect shared tables.
Only recorded UUID storage objects may be deleted. A process lease prevents
concurrent fixture resets. Never run these suites concurrently.
The Go integration runner allows 30 minutes per package for the 1,000-recipient
workflow on remote PostgreSQL. Request cancellation and workflow assertions remain
active; a slow database does not turn a failed assertion into a passing test.

By default, API/browser targets temporarily suspend recognized workspace tmux
services and restore them afterward. Set `BORROW_WORKSPACE=` to require free ports.
No unrelated process is stopped. Shared-data and public-browser checks use web-be on 3333.

`make verify` retains logs and results under `.artifacts/verify/`, runs the required
suites, then cleans recorded objects and schemas, including on failure. Individual suites
keep schemas for debugging; finish with `make clean-fixtures`.

Playwright uses the real admin frontend, a production build of the public frontend,
and both real APIs. Desktop and mobile evidence is retained under
`.artifacts/browser/`. Certificate editing and the complete remaining workflow run
at both 1440px desktop and 390px mobile widths. Public certificate rendering
remains in the frontend.

Image-backed PDF download requires storage GET/HEAD CORS access from the frontend
origin. The user approved temporarily applying `tests/storage-cors.json` to the
shared test bucket and restoring its prior setting after these tests:

```sh
GO_REWRITE_DIRECT_DNS=1 node scripts/storage-cors.mjs apply --approved-test-bucket-change
GO_REWRITE_DIRECT_DNS=1 make verify
# Aggregate cleanup restores CORS; this also handles an interrupted individual suite:
GO_REWRITE_DIRECT_DNS=1 node scripts/storage-cors.mjs restore
```

The harness records and verifies the temporary configuration, refuses to overwrite
another actor's later change, and uses real storage without bypassing browser CORS.
Certificate image requests on both frontends use the same anonymous CORS mode as the exporter,
preventing immutable cached responses without CORS headers from breaking PDF output.

## Deployment and rollback

Production runs Go through [Coolify](docs/COOLIFY.md). Inject the same database,
storage, `APP_KEY`, Google client, timezone, and CORS configuration; schedule each
Go job separately. Check `/health`, login/refresh, permissions, a member read, and a
certificate read after a deploy. To roll back, redeploy the previous Go image in
Coolify. The shared schema is unchanged; do not roll back migrations or reset tables.

### macOS storage DNS recovery

If macOS `getaddrinfo` returns `ENOTFOUND` for the test storage host while direct
DNS queries resolve it, prefix a harness command with `GO_REWRITE_DIRECT_DNS=1`.
For example, `GO_REWRITE_DIRECT_DNS=1 node scripts/verify.mjs --borrow-workspace`.
Node falls back to real DNS queries only for the configured test storage host,
and Go child processes use the pure Go resolver. IP addresses are never pinned,
TLS verification and the real S3 operations remain enabled, and the machine's
network settings are unchanged. Contract/Go test evidence records this opt-in.
The Go S3 adapter also retries DNS-not-found failures within the SDK's normal
three-attempt limit; persistent failures remain errors and cancellation is retained.
