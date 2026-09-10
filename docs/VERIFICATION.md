# Rewrite verification report

Status: **passed**, September 10, 2026. All 17 automated stages passed on the same
source fingerprint; final review covered the PDFs, screenshots, dependency locks,
typed design, route applicability, cleanup, and observed performance. No required
test failed or was skipped. Production deployment and traffic switching remain
outside this task.

## Scope and revisions

The Go implementation covers 139 current API declarations and the three separate
job commands. Adonis owns all migrations, seeders, and RBAC preflight. Both APIs
retain port 3334; web-be uses 3333, the admin frontend 3005, and public frontend 3000.
No production deployment or traffic switch was performed.

- Go tested source: `cd2e360bef31c2a6805ff8b100305fd194b54187`.
- Adonis: `dd8d0ff409c34eaaebb8c2e3ec8a046efbf11356`.
- web-be: `d3663039e812075bba4595b1c432b684b996f6ae`.
- Admin frontend: `898b93cc67a06efe5cf065c25ff27b863a275361`.
- Public frontend: `17a51e97cdfd909923e71d2bb839896ce7aa8e81`.

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
# After reviewing the evidence and completing docs/completion-review.json:
node scripts/finalize-verification.mjs
```

Temporary localhost GET/HEAD CORS was approved for this task. The harness recorded
the previous setting and restored it during cleanup. The outer coordinator also
restored it in its exit handler. Working environment files were unchanged; child
processes received configuration from the workspace's test environment documents.
The test-host DNS recovery option was enabled. The coordinator temporarily paused
and restored the identified preview on port 3000; use a free port 3000 when
reproducing the run. The committed harness only borrows recognized workspace
services, never arbitrary processes.

Final aggregate evidence:
[2026-09-10T04-21-36-636Z](../.artifacts/verify/2026-09-10T04-21-36-636Z/results.json).
Automated execution ran from 04:21:36 to 04:48:22 UTC (11:21 to 11:48 Jakarta).
Source fingerprint:
`3380de8d7ec5bd167a9d179aab374d81521165261ae168cd9e8dd02b9176a7e4`.
It covers Go, queries, the harness, dependency/configuration files, and source files
in all four existing applications. Finalization refuses changed sources, failed
checks, skipped required tests, incomplete route applicability, or pending reviews.
The aggregate initially returned exit 1 solely for the pending manual report
review: every executed stage returned 0. The finalizer completed that review
against the unchanged source. Documentation-only completion changes follow the
tested commit. Per-stage commands, exit codes, timings, and logs are retained in
the aggregate directory; the checksummed [evidence manifest](../.artifacts/verify/2026-09-10T04-21-36-636Z/evidence/manifest.json)
archives the supporting reports.

| Check | Final result |
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
| Desktop/mobile admin and public browsers | 10 admin and four public cases passed; zero failed, skipped, or flaky |
| Existing application checks | All 12 passed; public production build also passed in the browser stage |
| Bounded latency/memory comparison | Passed; observations below |
| Route coverage | 139 implemented, 139 success-covered; no stale, missing, or pending groups |
| Cleanup | Recorded storage objects and all three schemas removed and checked absent; CORS and local services restored |

Go counts include subtests; the integration run also reruns unit tests, so the two
counts must not be added as distinct scenarios. The unit suite includes 1,418
cross-language Vine validation fixtures. Integration covers real PostgreSQL and
S3 operations, refresh replay/rotation and deactivation, last-Super-Admin races,
registration/form conflicts, certificate uniqueness and immutable snapshots,
1,000-recipient pause/resume, asset-copy failures, image boundaries and orientation,
Excel content, and job calendar/cancellation/failure behavior. Google validation
uses signed fixtures and controlled key responses with actual cryptographic
verification; the production binary contains no authentication bypass.

The existing-application stage ran Adonis and web-be lint, type checks, and unit
suites; admin frontend lint, tests, and build; and public frontend lint, type
checks, and tests. The original opt-in club PostgreSQL workflows were also run
separately in both backends, with zero residual fixture rows after rollback.
Runnable `check`, `test-unit`, `test-integration`, `test-contract`, `test-browser`,
and `verify` targets are supplied in the Makefile, with additional shared-data,
job, benchmark, package, and cleanup targets.

## Dependencies and native runtime

Go 1.26.8, sqlc 1.31.1, Node 24.14.1, libvips 8.18.6, pkgconf 3.0.0,
Playwright 1.63.0 and Chromium 153.0.8010.12 (revision 1243).
Direct application dependencies are pinned in `go.mod/go.sum`:
pgx/v5 5.11.0; JWT/v5 5.3.1; x/crypto 0.57.0; AWS SDK core 1.46.0,
credentials 1.20.3 and S3 1.112.0; govips/v2 2.18.0; Excelize 2.11.0;
Google API 0.297.0. Node harness dependencies are pinned in `package-lock.json`.
The downloaded PDFs were produced by admin jsPDF 4.1.0 and public jsPDF 4.2.1.

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
  real desktop and mobile downloads. A strengthened public test also reproduced
  the same CSS-background cache failure. The public canvas received the matching
  anonymous-CORS image fix; all four public cases then passed with real uploaded
  background images. Bucket CORS remains required for image export.
- Mobile browser tests used the previous editor restriction and table presentation.
  They now exercise the actual 390px editor and expandable participant cards.
- Two independently added public poster tests lacked the app's Next image
  configuration. Their renderer now uses the real allowlist. All 12 existing-app
  checks passed in both the affected rerun and final aggregate.
- Public browser startup initially refused to stop another task's temporary club
  preview on port 3000. Its temporary stop/restart was described before continuation;
  the recorded preview and port listener were restored after the final browser run.

The public image failure trace is retained in
[04-15-34-282Z](../.artifacts/browser/2026-09-10T04-15-34-282Z/html/index.html),
and its passing affected rerun in
[04-17-17-350Z](../.artifacts/browser/2026-09-10T04-17-17-350Z/html/index.html).
Slow remote database runs had approached Go's default ten-minute package timeout;
the integration harness now has a bounded 30-minute package budget with every
assertion and cancellation check retained. The final integration stage took
451 seconds. No external blocker remains for the agreed checks.

Earlier frontend traces also contain existing Google-font CSP and Ant Design
deprecation messages. These are distinct from the corrected certificate image
failure. No backend endpoint proxies to Adonis; PDF rendering stays in the frontend.

The original baseline's eight checks passed. The resolved failures above are
retained separately from the final passing evidence. Existing Google-font CSP,
Ant Design deprecation, and build warnings do not represent waived test failures.
The typed-design review documents two deliberately invalid legacy SQL operations
and the source's unspecified activity-export ordering; compatibility tests retain
their behavior rather than inventing a schema or ordering change.

## Browser and PDF review

Both frontends used real APIs, isolated PostgreSQL fixtures, and real test storage
at 1440px desktop and 390px mobile widths. No network mock replaced a required
database, storage, or browser integration. The public frontend used its production
build; the unrelated temporary preview was stopped during these tests.

- [Admin report, 10 passing cases](../.artifacts/browser/2026-09-10T04-44-35-925Z/html/index.html):
  login, refresh, logout, restricted navigation, access requests and review,
  member/profile/account updates, activity and club registrations, custom forms,
  Excel downloads, template upload/publication, issuance, PDF download and revocation.
- [Public report, four passing cases](../.artifacts/browser/2026-09-10T04-46-43-538Z/html/index.html):
  login, reference data and profiles, activity registration, image-backed certificate
  download/verification/revocation, and club answer validation, submission and Go review.
- All four downloaded PDFs were inspected with `pdfinfo`, rendered with
  `pdftoppm -scale-to 1000 -png -singlefile`, and visually reviewed alongside the
  desktop/mobile screenshots. Each is one landscape A4 page. Admin PDFs preserve
  participant, activity, Indonesian date, code, and background; public PDFs
  preserve the uploaded background and immutable participant name. The public
  template is intentionally a minimal synthetic fixture.

Screenshots, actual PDF and Excel downloads, JSON results, and HTML reports live
in those dated directories. Failure traces and screenshots from earlier runs are
retained. The final run had zero browser errors reported by Playwright's result
summary, zero unexpected outcomes, zero skipped cases, and zero flaky cases.

## Cleanup and restoration

Fixture run `a62222104353e081` used the baseline, candidate, and cross schemas
under its unique `go_rewrite_` prefix. Ace migrated the schemas; Go introduced no
migration runner. Cleanup checked ownership, dropped only those three schemas,
and queried PostgreSQL to confirm all were absent. Real storage cleanup used
recorded keys and HeadObject absence checks; no `storage-*.json` journals remain.
The dated evidence archive includes the schema and storage cleanup reports.

The original test-bucket CORS configuration was empty and was restored and
verified at 04:48:23 UTC. Approved temporary rules allowed only GET/HEAD from
localhost:3005 and localhost:3000. Ports 3334, 3333, and 3005 were returned to
their recorded workspace services. The original port-3000 preview script and
image adapter were hash-checked and restarted; its new process and descendant
port listener were verified. CORS, preview, and borrowed-service records are
included in the evidence archive. Shared public tables were not reset.

## Measured latency and process memory

Both backends used compiled code, the same host, remote PostgreSQL fixture, and
port 3334 sequentially with `NODE_ENV=test`. Two rounds reversed backend order.
Each backend received 20 warm-up and 220 measured requests per round, at four
concurrent requests. Values below are the range across the two rounds in
milliseconds, not confidence intervals. [Raw observations](../.artifacts/verify/2026-09-10T04-21-36-636Z/evidence/performance.json)
also include request counts, elapsed time, minimum/maximum, and throughput.

| Endpoint | Adonis p50 | Go p50 | Adonis p95 | Go p95 |
|---|---:|---:|---:|---:|
| `/health` | 0.59–1.03 | 0.47–0.67 | 1.43–2.65 | 2.46–2.64 |
| `/v2/countries` | 18.29–28.71 | 36.89–53.18 | 53.33–55.82 | 104.36–150.57 |
| `/v2/profiles?per_page=20` | 115.07–312.96 | 41.71–102.07 | 163.23–429.36 | 139.86–145.13 |
| `/v2/dashboard/stats` | 43.55–113.23 | 60.69–98.67 | 65.66–249.92 | 178.16–308.20 |

Post-workload API RSS was **188.56–190.02 MiB for Adonis** and
**37.75–38.41 MiB for Go**, including each process runtime but excluding database
memory. Go used less memory and served member reads faster in this sample;
reference reads were slower, and dashboard results varied by round. Remote
database and network variation is substantial. These bounded observations do
not establish production capacity or an overall latency improvement.

## Delivery and cutover

The application, typed query sources, dependency locks, fixture/comparison/browser
harness, [compatibility matrix](COMPATIBILITY.md), [typed-design review](TYPE_DESIGN.md),
[completion review](completion-review.json), and [progress ledger](../PROGRESS.md)
are delivered. The [setup and cutover guide](../README.md) includes API/job commands,
Ace migrations and RBAC preflight, native packaging, the opt-in launcher, and
returning to Adonis. Adonis remains the default. This report completes the
implementation/testing scope and performs no production deployment.
