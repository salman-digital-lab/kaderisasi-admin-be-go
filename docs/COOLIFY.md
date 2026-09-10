# Coolify deployment

Application: `kaderisasi-admin-be-prod` (`x8sssk488ssgg44000og04oo`).
CLI context: `salman`.
Domain: `https://api-admin-kaderisasi.salmanitb.com`.
Container port: `3334`. Health check: `GET /health`.

## Image

The repository Dockerfile builds the API and separate job executable on Linux.
It pins Go 1.26.8 and verifies the SHA-256 of the libvips 8.18.6 source archive.
The build runs dependency verification, `go vet`, and the full race-enabled unit
suite, including bidirectional Adonis cryptography fixtures. Node 24.14.1 and
the locked Adonis crypto libraries are used only in the build stage.

The runtime uses Debian trixie, libvips, CA certificates, and timezone data. It
runs as UID 10001 and contains `/app/admin-api` and `/app/admin-jobs`. Node and
the compiler are not required at runtime. The Docker context allowlist excludes
environment files, local artifacts, and binaries. Runtime credentials must be
provided by Coolify, never copied into the image or injected as build arguments.

## Updating the existing application

The installed CLI is 1.8.0. Its [command reference](https://raw.githubusercontent.com/coollabsio/coolify-cli/refs/heads/main/llms-full.txt)
documents application configuration, deployment, logs, rollback, and tasks. This
version does not expose a build-pack flag on `app update`; switching from Railpack
to Dockerfile requires the documented Coolify `PATCH /api/v1/applications/{uuid}`
field `build_pack: "dockerfile"`, or the equivalent application setting.

Use repository `salman-digital-lab/kaderisasi-admin-be-go`, branch `main`,
Dockerfile `/Dockerfile`, and start command `/app/admin-api`. Clear the old Node
install/build overrides and disable `inject_build_args_to_dockerfile`. Retain
the existing domain, network, port, database, storage, APP_KEY, Google client,
CORS origins, and timezone configuration. Use a 30-second stop grace period.

Before replacing the service, retain its configuration and previous image,
check Ace migration status and the existing Adonis RBAC preflight, and validate
the new image in an internal application with no public domain or enabled jobs.
The Go service must connect to the same database schema. No data copy, seeding,
or schema reset is part of a runtime switch.

```sh
coolify app start x8sssk488ssgg44000og04oo
coolify app deployments list x8sssk488ssgg44000og04oo
coolify app deployments logs x8sssk488ssgg44000og04oo
coolify app logs x8sssk488ssgg44000og04oo
```

This application has a custom internal container name. Coolify reports that
rolling updates are not supported with that setting: it removes the old
container before starting the replacement. Allow for a brief restart window.
Verify public HTTPS health, reference-data reads, rejected
unauthenticated requests, allowed-origin CORS, container health, and retained
environment values. Keep the prior image available until these checks pass.

## Scheduled jobs

The API does not start a scheduler. Coolify tasks invoke:

```sh
/app/admin-jobs close:registration
/app/admin-jobs clubs:close-registration
/app/admin-jobs clubs:update-visibility
```

The existing activity task's daily cadence is `0 0 * * *` in the server's UTC
timezone (07:00 Asia/Jakarta). The club tasks use the same cadence. The process
environment remains `TZ=Asia/Jakarta` for business date calculations. Disable
old Node job commands during the switch, then enable the Go commands after the
API passes its checks. Do not run both implementations' schedules concurrently.

## Returning to Adonis

Disable the Go tasks. Restore repository `salman-digital-lab/kaderisasi-admin-be`,
build pack `railpack`, start command `node build/bin/server.js`, and the saved
build settings. Retain all database, storage, signing-key, domain, and port
values. The last validated Adonis image is
`dd8d0ff409c34eaaebb8c2e3ec8a046efbf11356`.

```sh
coolify app rollback images x8sssk488ssgg44000og04oo
coolify app rollback run x8sssk488ssgg44000og04oo --commit dd8d0ff409c34eaaebb8c2e3ec8a046efbf11356
```

After Adonis is healthy, restore the original activity task command
`node build/ace close:registration` and its previous enabled state. Remove or
disable the added Go club tasks. Do not roll back migrations or reset tables.

## Deployment evidence

Production cutover completed September 10, 2026, at approximately **14:18 WIB**
(07:18 UTC). The live Go commit is
`1e738e9491b46d6849754f7fb4d29bfbe48bd330`; the changes since the complete rewrite
verification add deployment packaging and build-only test dependencies. No Go
business-handler or service source changed.

- Internal validation deployment: `yovthlwctemzyood4bldhfzw`, passed.
- Production deployment: `lkjbz3a4mrfrwst0xn10orfm`, finished successfully.
- Linux image: dependency verification, vet, all race-enabled unit packages,
  Adonis password/cookie/JWT interoperability, native image presets, and API/job
  compilation passed. Runtime libvips reports 8.18.6; the process runs as UID
  10001. Node is absent from the runtime image.
- Production container: healthy; image and executable confirmed directly.
  All 28 configured runtime variables match the original Adonis container.
  The existing `NODE_ENV=development` value was retained with the other settings;
  this deployment did not change cookie attributes or diagnostic-mode policy.
- Public `/health` and `/v2/countries` returned HTTP 200 and matched the Adonis
  baseline. An unauthenticated `/v2/profiles` request returned the same HTTP 401
  response. Credentialed CORS preflights passed for both production frontends.
- Health monitoring recorded 180 samples: 177 HTTP 200 and three HTTP 503 samples
  during the container replacement, at 07:17:57, 07:17:59, and 07:18:01 UTC.
  The service recovered immediately after that short restart window. These
  two-second samples do not establish an exact outage duration.
- All three Go tasks are enabled at the existing daily cadence, with a 300-second
  timeout. No job was manually triggered against production data during smoke
  testing. The API does not run a second scheduler.
- All 34 Ace migrations were already completed. No migration, seed, data copy,
  or production fixture mutation was performed. The retained Adonis image's
  direct Ace command discovery raised an existing `Invalid URL` metadata error.
  Its actual `RbacPreflight.run` implementation was executed after normal Adonis
  application boot with a read-only PostgreSQL session: the RBAC schema was
  present, an active Super Admin existed, and there were no blockers. Future
  Adonis command maintenance should account for that CLI issue. The Go runtime
  contains no Ace migration runner; use an Adonis checkout/image with database
  network access for subsequent schema changes.
- The temporary internal application and diagnostic task were deleted. The
  temporary SSH key and raw credential-bearing local snapshots were removed.
  The prior Adonis image remains available for rollback. The frontends and
  `web-be` remain on their existing healthy deployments.

The production commit was pinned during cutover. After recording these results,
the application follows `main` / `HEAD` again for future deployment requests.
The existing automatic-deployment setting is retained; webhook delivery from
the new repository is not established by these manual deployment checks.

Configuration rollback data, sanitized smoke results, and operational logs are
retained in the ignored, restricted `.artifacts/deployment/` directory. The earlier
[rewrite verification report](VERIFICATION.md) records full contract, real test
storage, shared-database, concurrency, and browser testing; production smoke
checks here were read-only and did not repeat fixture workflows on live data.
