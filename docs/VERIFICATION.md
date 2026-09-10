# Rewrite verification report

Status: **in progress**. The current aggregate is still running. This report does
not authorize production cutover.

## Scope and revisions

The Go implementation covers 139 current API declarations and the three separate
job commands. Adonis owns all migrations, seeders, and RBAC preflight. Both APIs
retain port 3334; web-be uses 3333, the admin frontend 3005, and public frontend 3000.
No production deployment or traffic switch was performed.

- Go: `a9ba92f9a8628779d1a0fb877d75a2acf2a5a74c`.
- Adonis: `dd8d0ff409c34eaaebb8c2e3ec8a046efbf11356`.
- web-be: `d3663039e812075bba4595b1c432b684b996f6ae`.
- Admin frontend: `898b93cc67a06efe5cf065c25ff27b863a275361`.
- Public frontend: `23b5714e4af16ac919c880e681487b4a64269304`.

The adopted club export, Jakarta deadline, registration submission, and public
club/activity UI changes are included. The original source had 138 declarations;
its subsequently added JSON certificate lookup brings the current total to 139.

## Reproduction and evidence

From `kaderisasi-admin-be-go`, with dependencies installed as described in
[README.md](../README.md):

```sh
GO_REWRITE_DIRECT_DNS=1 node scripts/storage-cors.mjs apply --approved-test-bucket-change
GO_REWRITE_DIRECT_DNS=1 node scripts/verify.mjs --borrow-workspace
GO_REWRITE_DIRECT_DNS=1 node scripts/storage-cors.mjs restore
```

Temporary localhost GET/HEAD CORS was approved for this task. The harness records
the previous setting and restores it during cleanup. An outer shell trap also
restores it on exit. Working environment files are unchanged; child processes
receive configuration from the workspace's test environment documents.

Current aggregate evidence:
[2026-09-10T03-33-22-245Z](../.artifacts/verify/2026-09-10T03-33-22-245Z/results.json).
Source fingerprint:
`bee3dcd9ac20280e543d2d31e28485344d32d89918b65f6d97c2a068cc86aa3f`.
It covers Go, queries, the harness, dependency/configuration files, and source files
in all four existing applications. Finalization refuses changed sources, failed
checks, skipped required tests, incomplete route applicability, or pending reviews.

| Check | Current result |
|---|---|
| Formatting, vet, compilation, dependency integrity, sqlc drift, normalization | Passed |
| Race-enabled Go unit tests | 1,593 tests/subtests passed; zero failed or skipped |
| Native package and startup checks | Passed, macOS arm64 |
| Ace schema fixtures and isolated connection failure | Passed |
| Original Adonis/web-be club integration workflows | Passed, rollback effects checked |
| Race-enabled PostgreSQL/storage integration | 1,634 tests/subtests passed; zero failed or skipped |
| Full contract comparisons | All 18 groups, 2,001 comparisons passed; zero differences |
| Session/password transfer and shared database | 12 bidirectional transfer and 53 Go/web-be checks passed |
| Three jobs and repeat execution | All three jobs and six first/repeated executions passed |
| Desktop/mobile admin and public browsers | Pending current aggregate |
| Existing application checks | Pending current aggregate; affected rerun passed all 12 |
| Bounded latency/memory comparison | Pending current aggregate |
| Route coverage and final cleanup | Pending current aggregate |

## Dependencies and native runtime

Go 1.26.8, sqlc 1.31.1, Node 24.14.1, libvips 8.18.6, pkgconf 3.0.0,
Playwright 1.63.0. Direct application dependencies are pinned in `go.mod/go.sum`:
pgx/v5 5.11.0; JWT/v5 5.3.1; x/crypto 0.57.0; AWS SDK core 1.46.0,
credentials 1.20.3 and S3 1.112.0; govips/v2 2.18.0; Excelize 2.11.0;
Google API 0.297.0. Node harness dependencies are pinned in `package-lock.json`.

The [native manifest](../.artifacts/release/admin-go-darwin-arm64/manifest.json)
records dynamic libraries and successful executable startup validation. This is
a macOS arm64 package; Linux requires a native build and matching libvips runtime.

## Resolved failures and environment observations

The first aggregate, [02-57-58-858Z](../.artifacts/verify/2026-09-10T02-57-58-858Z/results.json),
was incomplete and is retained as failure evidence:

- The storage endpoint intermittently returned DNS NXDOMAIN. A bounded S3 retry
  now retains the SDK's three-attempt limit, persistent error identity, request
  body, and cancellation. The harness's optional direct DNS fallback changes
  neither the machine's resolver settings nor TLS validation; no IP is pinned.
- Certificate editor images loaded without CORS and could poison immutable browser
  cache entries used by PDF export. Consistent anonymous CORS image requests fixed
  real desktop and mobile downloads. Bucket CORS remains required for image export.
- Mobile browser tests used the previous editor restriction and table presentation.
  They now exercise the actual 390px editor and expandable participant cards.
- Two independently added public poster tests lacked the app's Next image
  configuration. Their renderer now uses the real allowlist. All 12 existing-app
  checks passed in the affected rerun.
- Public browser startup initially refused to stop another task's temporary club
  preview on port 3000. Its temporary stop/restart was described before continuation;
  restoration evidence will be recorded with the final browser result.

Earlier frontend traces also contain existing Google-font CSP and Ant Design
deprecation messages. These are distinct from the corrected certificate image
failure. No backend endpoint proxies to Adonis; PDF rendering stays in the frontend.

## Browser, cleanup, performance, and final review

Final current-source browser evidence, storage/schema/CORS/preview restoration,
measured performance, and completion review are pending the aggregate result.
Review found that the public certificate canvas still uses a CSS background URL,
the request pattern implicated in the admin export failure. An additional real
image-backed public download scenario is drafted and must be executed after the
current aggregate, with a compatibility fix if it reproduces the problem. The
current public scenario uses a template without a background image.
