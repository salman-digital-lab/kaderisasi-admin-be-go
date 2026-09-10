# Admin backend in Go

Parallel implementation of the Kaderisasi admin API on port **3334**. It serves
`/v2` and `/health`; it never starts scheduled jobs. Adonis remains the only owner
of database migrations, seeders, and RBAC preflight.

**Verification is still in progress.** See [PROGRESS.md](PROGRESS.md),
[the route matrix](docs/COMPATIBILITY.md), and
[remaining completion reviews](docs/completion-review.json). A passing unit suite
or handler inventory does not mean the rewrite is ready for cutover.

## Local setup

Required: Go **1.26.8**, Node **24**, PostgreSQL client tools, sqlc **1.31.1**, and
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
npx playwright install chromium
make build
make package
```

`go.mod`, `go.sum`, and `package-lock.json` pin dependencies. Go's toolchain directive
selects 1.26.8. `CGO_ENABLED=1` and libvips development headers are required to build;
the runtime must also contain libvips and its shared-library dependencies. Docker
is not required. The Go binary is not a standalone static executable.

`make package` builds a native archive for the current OS and architecture under
`.artifacts/release/`, records the shared-library dependencies, and checks that both
executables reach startup validation. Install the runtime described in its
`RUNTIME.txt` on the destination. Build on Linux for a Linux destination; the
verified macOS archive is not a cross-platform distribution.

The test harness also uses the existing Adonis installation for Ace, Vine,
password/cookie fixtures, PostgreSQL, Sharp, and ExcelJS. Run `npm ci` in the
existing applications if their dependencies are absent. Frontend source remains
in its original application.

## Running

From this directory:

```sh
node scripts/run.mjs --environment=test api
node scripts/run.mjs --environment=test close:registration
node scripts/run.mjs --environment=test clubs:close-registration
node scripts/run.mjs --environment=test clubs:update-visibility
```

The runner reads `../docs/.env.test.be` and injects values into its child process.
It does not replace an application env file. `--environment=prod` selects the
production document; production execution is outside the current rewrite task.
For a prebuilt binary, inject the existing environment variables and start
`bin/admin-api` or `bin/admin-jobs <job>` directly.

All existing variable names are retained. `DB_SCHEMA` is optional and defaults to
`public`; test schemas use an explicit search path without `public` fallback.
The club registration closing job always uses Asia/Jakarta. Set `TZ=Asia/Jakarta`
for the other two jobs to preserve their existing calendar behavior. `ADMIN_CORS_ORIGINS` is a comma
separated allowlist; local public frontend requests require
`http://localhost:3000` alongside `http://localhost:3005`. `APP_KEY` must be identical
to Adonis for session transfer. Never print or commit env files.

The workspace launcher supports an opt-in selection:

```sh
../start-all.sh test --admin-go
# Return to the default implementation:
../start-all.sh test
```

The launcher preserves all four ports. Its existing environment-copy behavior
continues for the three original applications; Go receives injected configuration.
Switching implementation restarts the workspace tmux stack.

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

```sh
make check
make test-unit
make test-integration
make test-contract
make test-shared
make test-jobs
make test-browser
make benchmark
make verify
```

Integration targets create or reuse three recorded schemas in the configured
shared **test** database. They run Ace migrations and deterministic synthetic
fixtures. Ownership comments and explicit search paths protect shared tables.
Only recorded UUID storage objects may be deleted. A process lease prevents
concurrent fixture resets. Never run these suites concurrently. Contract reports record
the compiled source fingerprint and adopted Adonis revision; stale reports cannot
pass the coverage gate. For an affected subset, run
`node scripts/contracts-all.mjs --groups=members,activities --borrow-workspace`.

By default, API/browser targets temporarily suspend recognized workspace tmux
services and restore them afterward. Set `BORROW_WORKSPACE=` to require free ports.
No unrelated process is stopped. Adonis and Go contract runs use port 3334
sequentially; shared-data and public-browser checks use web-be on 3333.

`make verify` retains logs and results under `.artifacts/verify/`, runs all required
suites, then cleans recorded objects and schemas, including on failure. It exits
unsuccessfully while required completion reviews remain pending. After all checks pass
and the final report is reviewed, `node scripts/finalize-verification.mjs` completes
that review without rerunning tests. It refuses failed checks, unresolved reviews,
or a changed source fingerprint. Individual suites
keep schemas for debugging; finish with `make clean-fixtures`.

Playwright uses the real admin frontend, a production build of the public frontend,
and both real APIs. Desktop and mobile evidence is retained under
`.artifacts/browser/`. The existing certificate editor requires a tablet-width
screen; the mobile test verifies that guidance, edits at 1024px, then resumes the
remaining workflow at 390px. Public certificate rendering remains in the frontend.

The shared test bucket currently has no CORS policy. Real image-backed PDF download
is blocked until the reviewed rule in `tests/storage-cors.json` is approved.
The harness does not bypass browser CORS or replace storage with a mock.
`scripts/storage-cors.mjs` records and verifies an explicitly approved temporary
change and refuses to overwrite another actor's later configuration.

## Cutover and return to Adonis

Do not switch production traffic until the aggregate report passes and all review
criteria are resolved. Before a future cutover, retain the Adonis revision, apply
its migrations and RBAC preflight, install native runtime libraries on the target,
and inject the same database, storage, `APP_KEY`, Google client, timezone, and CORS
configuration. Stop the old API before starting Go on 3334. Schedule each Go job
separately and remove its Adonis schedule to avoid running both copies.

Check `/health`, login/refresh, permissions, a member read, and a certificate read.
To return to Adonis, stop Go and its job schedule, start Adonis on 3334, and restore
its job schedule. The shared schema is unchanged; do not roll back migrations or
reset tables. Session/password transfer is tested in both directions.

### macOS storage DNS recovery

If macOS `getaddrinfo` returns `ENOTFOUND` for the test storage host while direct
DNS queries resolve it, prefix a harness command with `GO_REWRITE_DIRECT_DNS=1`.
For example, `GO_REWRITE_DIRECT_DNS=1 node scripts/verify.mjs --borrow-workspace`.
Node falls back to real DNS queries only for the configured test storage host,
and Go child processes use the pure Go resolver. IP addresses are never pinned,
TLS verification and the real S3 operations remain enabled, and the machine's
network settings are unchanged. Contract/Go test evidence records this opt-in.
