# Go rewrite progress

Status: complete, September 10, 2026. All implementation and testing phases passed.
The reviewed results and operational limits are in [docs/VERIFICATION.md](docs/VERIFICATION.md).

## Agreed scope

Separate Go API and job binaries; Adonis retains migrations and seeders. Preserve
the existing API and shared schema. Use only isolated schemas and owned objects
in the configured test environment. No production deployment.

## Baseline

Adonis revision: `9e5a57182d123736e271adcb9417126a00981f2c`.
138 route declarations, 22 controllers, 16 services, 23 models, 34 migrations.
The source advanced independently to `d1cc1b146ea0c2b0458292f0a0f6b0ac718b1b35`
during implementation. Its added certificate lookup endpoint and array-query fix
are now included: 139 routes and 54 schemas (48 file exports and six inline).
The source has since advanced to `dd8d0ff409c34eaaebb8c2e3ec8a046efbf11356`
with club export and Jakarta deadline fixes. These are adopted without changing
the 139-route inventory. `docs/BASELINE.json` records the revisions.

## Phase ledger

- A — inventory and baseline checks: captured the original 138 routes and 16 roles;
  expanded to the adopted baseline above. All original eight checks passed.
- B — Go foundation, authentication, images, isolated fixture harness: core gates
  passed, including signed Google tokens with controlled key responses and live
  Adonis/Go password, access-token, and refresh-cookie transfer in both directions.
- C — all listed business modules are implemented: reference data, dashboard,
  administrators/access requests, members/profiles, activities/registrations,
  clubs/membership/roles, custom forms, counseling, achievements/leaderboards.
  Their Go HTTP/database workflow tests and all module differential comparisons pass.
- D — all certificate handlers and the three separate scheduled jobs are implemented.
  Template lifecycle, real asset duplication, copy-failure rollback, issuance races,
  immutable snapshots, revocation, preparation, and 1,000-recipient bulk issuance
  pass. Template changes pause bulk work without losing progress; resume passes.
  Job calendar-boundary, idempotence, cancellation, and failure tests pass.
- E — complete contracts, integration, browser verification, cleanup: passed.
  The final run passed all 17 stages: 1,593 race-enabled unit tests/subtests,
  1,634 race-enabled integration tests/subtests, 2,001 contract comparisons,
  12 bidirectional session/password checks, 53 shared-database checks, six job
  executions, 14 browser workflows, and all 12 existing-application checks.
  Route coverage is 139/139. No required tests failed or were skipped.

## Completion rule

All inventoried endpoints and jobs must be implemented and their required checks
must pass. Missing tests and externally blocked checks remain incomplete.

## Next action

No implementation or testing work remains in the agreed scope. A future
production deployment or traffic switch is a separate task. The workspace
launcher continues to default to Adonis; Go is selected with `--admin-go`.

Final aggregate: `.artifacts/verify/2026-09-10T04-21-36-636Z/results.json`,
Go revision `cd2e360bef31c2a6805ff8b100305fd194b54187`, source fingerprint
`3380de8d7ec5bd167a9d179aab374d81521165261ae168cd9e8dd02b9176a7e4`.
This includes the latest club API changes and real image-backed PDF downloads on
both frontends at desktop and mobile widths. Four downloaded PDFs were rendered
and visually reviewed. The three schemas for run `a62222104353e081` were removed
and checked absent; storage cleanup verified recorded objects absent. No object
journals remain. The original bucket CORS setting, workspace services, and the
temporary public preview were restored. Measured latency and process memory,
versions, commands, evidence, and resolved failures are in the final report.

The sections below preserve earlier checkpoints. Their pending items and active
fixture references are historical and have been resolved by the final run above.

## Current continuation

- Added image boundary comparisons: 38/38 pass in production HTTP mode, using
  the real test bucket and isolated schemas. Orientation, unsupported GIFs,
  truncated JPEGs, pixel caps, individual file limits and total multipart limits
  are covered on all five upload routes. Both runs' five objects were deleted
  and checked absent. Framework debug stacks are absent in production mode.
- Added 157 route-edge comparisons for missing path resources and invalid bodies;
  all pass. Fixed custom-form and club validation error envelopes, record validation
  rule naming, JSON values submitted as files, and combined image-field errors.
  Export missing-resource diagnostics retain the error identity and actual Go
  stack; the comparator validates stack shape and the export call site before
  normalizing source paths, line numbers and runtime frames.
- Certificate wide-identifier cases expanded issuance comparisons to 74/74.
  The full race run after those fixes and the image fixes passed 390 tests with
  no failed or skipped tests (`.artifacts/go-race-current.jsonl`).
- Reference data, dashboard and administrators now use explicit request/response
  types and generated sqlc queries. The access-change service also uses typed
  updates and preserves its advisory lock and last-active-Super-Admin rule.
  Reference 54/54 and administrator/access-request 34/34 comparisons pass.
  Their affected race-enabled workflow and concurrency tests pass (7.734 seconds).
- Other business modules still contain generic JSON query/DTO code; the requested
  typed design is not complete. Its completion review remains pending.
- LOG_LEVEL now controls structured logging. Omitted, null and false PATCH values
  have a typed representation that preserves all three states.
- Cleanup now retains Go test storage journals until HeadObject verifies absence.
  Schema creation records intended names before atomically creating the schema
  and its ownership marker. Cleanup archives schema-removal evidence and can
  restore only a previously recorded, approved temporary CORS change.
- Pagination boundaries initially exposed 67 differences, then passed 98/98 after
  fixes. Added pagination unit assertions and expanded real Vine fixtures from 318
  to 486, all passing. The all-group rerun includes ten further query cases for
  unattached forms and certificate recipients and is still running.
- Typed-design progress and remaining adapters are recorded in `docs/TYPE_DESIGN.md`.
- The 16-group run reached 966/967 passing comparisons. The one new failure was
  the missing SQL diagnostic prefix for negative limits on unattached forms;
  that fix passed the affected group at 108/108. Current group reports therefore
  contain 967 passing scenarios. No required browser check has been waived.
- Access-request creation, cancellation and review now live in a separate typed
  service using sqlc queries. Their 34 comparisons and race-enabled permission/
  transaction checks pass (6.195 seconds). Login and Google use explicit DTOs;
  the login/session group reran at 28/28. Member creation/account generation also
  use typed requests, responses and queries; all 24 member comparisons pass.
- The fresh complete race run (`.artifacts/go-race-typed.jsonl`) failed four
  storage-dependent tests because system DNS cannot resolve `nos.wjv-1.neo.id`.
  Required cleanup verification also failed; the ownership journals are retained.
  This is an external blocker, not a passing run. Public DNS resolves the host,
  but no system DNS configuration or bucket settings have been changed. DNS later
  recovered; all four retained journals were cleaned and each key checked absent.
- Profile reads now use explicit response types and generated filtered/relation
  queries. All 29 member/profile comparisons pass, including legacy JSON and
  nullable users. The latest native package build and smoke check pass.
  Profile updates, regional assignments and credential edits now also use explicit
  DTOs and generated queries; all 44 comparisons pass, including empty/null values,
  dates, arrays, legacy JSON merges and unchanged-value timestamp preservation.

## Verified foundation

- Go 1.26.8, sqlc 1.31.1, govips/libvips installed; dependencies pinned in go.mod/go.sum.
- Ace migrated 25 tables into each of three isolated schemas. Ownership is recorded
  in `.artifacts/schemas.json`; these schemas are still in use and must be cleaned
  at the end. Never drop shared public tables.
- Bidirectional Adonis/Go password, JWT, and signed-cookie tests passed.
- Real PostgreSQL session lifecycle, replay detection, concurrent refresh, logout,
  and inactive-account tests passed with the race detector.
- Real S3 upload/read/copy/delete and image preset tests passed; owned test files
  were removed by test cleanup.
- HTTP login, refresh, protected reads, stale-cookie isolation, and image upload
  with a verified database effect passed.
- 318 validation fixtures generated from the actual Vine validators pass in Go,
  including all six inline certificate schemas and the newly added lookup schema.
- Reference CRUD, dashboard/permission catalog, administrator lifecycle, and
  access-request approve/reject/cancel workflows pass against isolated PostgreSQL.
- Member creation, duplicate protection, account generation, password reset,
  profile filtering/merging, and regional assignments pass with the race detector.
- Activity slug collisions, date fields, configuration merging, image order,
  member/guest registration reads, status changes, level/badge updates, and Excel
  content checks pass with the race detector. Additional image boundary and failure scenarios remain to be reviewed.
- Club logo replacement, image upload/delete, YouTube deduplication, form open-state
  protection, concurrent attachment, member approval, primary roles, bulk-update
  rollback, and Excel answers pass. Recorded storage objects were cleaned.
- Counseling updates, achievement approval/rejection attribution, monthly/lifetime
  score effects, filtering, and achievement export values pass.
- The combined `node scripts/test-go.mjs ./...` race-enabled run passed after Phase C.
- `go vet ./...` passed after template handlers were added.
- Direct sequential comparisons now run on port 3334. The harness suspends only
  the recognized tmux workspace API and restores it afterward, including recreating
  its pane if the original launcher exits on Ctrl-C. Working env files are unchanged.
- `.artifacts/contracts/`: reference 54/54, authorization 243/243, auth 28/28,
  administrators/access requests 34/34, members 24/24, activities 23/23,
  registrations 21/21, clubs/forms 46/46, and club membership/roles 33/33
  achievements/counseling/leaderboards 28/28, templates 38/38, issuance 50/50,
  and signed Google login 18/18 scenarios match Adonis, including
  database effects for mutations. All 139 routes have a successful differential case.
- `.artifacts/session-transfer.json`: 12 live password/session-transfer checks pass
  across both API implementations using the same owned cross-backend schema.
- Last-Super-Admin concurrent self-demotion passes with one success and one conflict;
  the database retains exactly one active Super Admin.
- Validation fixtures now compare full metadata and error order as well as values.
- A process lease prevents integration and comparison commands from clearing the
  same fixture schema concurrently. Resets are limited to recorded, ownership-marked
  schemas; there is no fallback to shared public tables.

## Compatibility decisions

Adonis allows overlapping parameter paths that net/http.ServeMux rejects.
The Go HTTP adapter therefore uses an ordered static-segment-first dispatcher
while preserving the inventory's methods, paths, and middleware requirements.

## Coverage limits

All 139 adopted routes have registered Go handlers. This is implementation coverage,
not proof of compatibility. The aggregate completion gate must reject missing
handlers, unexecuted required scenarios, test skips, and unfinished cleanup.

- Real storage contract runs now instrument the actual Adonis AWS SDK and Go
  adapter before every write or copy. Recorded objects are deleted and checked
  absent after each backend run, including failures. Club/form and activity
  comparisons also check WebP dimensions, content types, and MinIO public ACLs.

- Full race-enabled Go integration run passed again after all module contract fixes.
  HTTP workflows took 152.961 seconds; authentication, certificate, image/storage,
  jobs, form, export, and validation packages also passed. See
  `.artifacts/go-race-full.log`.
- Google differential checks use real cryptographic validators and a loopback
  key-response server. The Go transport fixture is compiled only with the
  integration build tag; the production executable has no key override option.

## Latest verification evidence

- All 13 differential groups reran successfully together: 640/640 scenarios.
  See `.artifacts/contracts-all.log` and generated `docs/COMPATIBILITY.md`.
- Admin browser login, access review, members, activity/club registration, custom-form
  attachment, and Excel downloads pass at desktop and mobile sizes. Certificate
  upload/publication/issuance work; real image-backed PDF download fails because the
  shared test bucket has no CORS policy. No browser bypass or storage mock is used.
- Public browser end-to-end passes on desktop and mobile against a production Next
  build and real Go/web-be APIs, including references, profile updates, registrations,
  owner PDF download, public verification, and revocation. Successful evidence:
  use per-run `report.json` files under `.artifacts/browser/` for preserved evidence.
- `.artifacts/shared-database.json`: 27 real cross-backend checks pass.
- `.artifacts/job-contracts.json`: all three job commands match Adonis and remain
  idempotent; six direct command scenarios pass.
- Admin FE lint and public FE lint/types/unit checks pass (`.artifacts/frontend-baseline`).
- `.artifacts/performance.json`: two rounds with reversed backend order, four concurrent
  requests, compiled applications, 220 measured requests plus 20 warmups per round.
  Go RSS: 37–38 MiB; Adonis: 189–190 MiB. Member medians: Go 30–31 ms, Adonis 57–61 ms;
  dashboard: Go 19–20 ms, Adonis 40–42 ms. Reference requests approximately 8–9 ms.
  Results are bounded observations on this host and remote test database.
- Workspace `--admin-go` option added; default remains Adonis. Shell syntax/help pass.
- Runnable Make targets and an aggregate verifier were added. The verifier refuses
  success while explicit completion reviews remain pending and cleans recorded
  fixture resources even when a required suite fails.

## Request parsing continuation

- Added production HTTP protocol comparisons. The first run exposed 37 differences
  across 44 cases; all 44 pass after moving parsing before named middleware and
  matching JSON/form/content-type behavior, query precedence, compression errors,
  size limits and bodyless commands.
- Added 60 fixtures from the installed qs parser and 29 Node JSON syntax diagnostics.
  All pass after correcting nested-form merging and diagnostic positions/messages.
- The full race run in `.artifacts/go-race-protocol.jsonl` passed 648 test cases
  and failed eight workflows because the test helper sent JSON null for no-body
  requests. No tests were skipped. After correcting the helper, all eight affected
  workflows passed in `.artifacts/go-race-empty-body-fix.jsonl` and
  `.artifacts/go-race-empty-body-remaining.jsonl`; recorded storage cleanup passed.
  Explicit JSON-null rejection remains covered by the production comparisons.

- Activity list/detail responses and filters now use typed responses/sqlc queries.
  Four new club-relation cases exposed and fixed JSONB relation decoding. The
  expanded 27 activity comparisons, 44 protocol cases and 38 real upload cases pass
  together (`.artifacts/contracts-parser-activities.log`), with cleanup verified.
- Image reorder/delete now have typed requests and a separate transaction service
  using generated queries. Their affected workflow is the next check. Activity
  creation/update and other remaining adapters are still tracked as incomplete.
- Formatting, vet, compilation, dependency integrity and query generation passed
  after the parser changes (`.artifacts/check-protocol.log`). Native packaging and
  startup validation passed again (`.artifacts/package-protocol.log`).

## Saved checkpoint and current activity work

- Initial application/harness checkpoint committed as `6011b6a`, including
  `go.mod`, `go.sum`, and `package-lock.json`. It explicitly remains incomplete.
- Activity create/update requests, response projections, template assignment and
  slug handling now use typed services and generated queries. All 34 activity
  comparisons pass, including zero-valued template IDs, default records, date
  updates and nullable club assignment (`.artifacts/activity-crud-typed.log`).
- Activity media upload/reorder/delete business logic now lives in the activity
  service. Its affected race tests and certificate workflows pass; see
  `.artifacts/go-race-activity-types.jsonl` and
  `.artifacts/go-race-activity-upload-service.jsonl`.
- Contract reports now record the source fingerprint and adopted Adonis revision.
  Coverage rejects missing groups and stale reports. A fresh complete contract
  run is next; prior group reports remain historical evidence only.
- The last HTTP image fixture now retains its object journal for the harness to
  verify and archive, matching the other Go storage fixtures.

## Current source verification

- Activity/service and evidence-gate changes committed as `7cd6588`.
- The fresh complete contract run passed reference, authorization, auth,
  administrators, members, activities, registrations, clubs, club membership,
  achievements, templates, certificates, Google and image groups. During the Go
  route-edge run the harness's PostgreSQL client emitted an idle `ETIMEDOUT` error,
  ending the runner before teardown. The suite did not complete.
- Verified and stopped only that run's orphan API executable, recreated the
  workspace Adonis pane, confirmed `/health`, and cleaned its empty recorded
  journal. `.artifacts/borrowed-3334.json` records successful restoration.
- Idle PostgreSQL errors are now handled so awaited work rejects and finally
  blocks run. Contract teardown attempts server stop, DB close and object cleanup
  independently; group completion also checks the fixture connection.
- A real failure test terminated only its own uniquely tagged PostgreSQL connection
  (PID, backend start, application name, user and database all matched). It verified
  error capture, rejected future queries and a healthy control connection. Evidence:
  `.artifacts/fixture-disconnect.json`. It is included in integration/aggregate targets.
- Rerun route-edges, query-edges and protocol, then check coverage for the current
  source. Registration DTO/query drafts are in `.artifacts/next-registration/`;
  they have not been applied or counted as implementation.

## Registration continuation

- All 17 contract groups finished against the prior Go source: 139 routes,
  1,042 scenarios, no stale/missing groups and no pending success/auth/permission
  coverage. Evidence: `.artifacts/coverage-current-source.json`. Adonis restoration
  was recorded. This is a historical checkpoint once registration sources change.
- Registration creation, detail, statistics, status changes and deletion now use
  typed requests/responses and generated queries in a dedicated activity service.
  IDs pass through PostgreSQL conversion without narrowing to zero. Preserved
  null-user duplicate rules, profile-level/badge effects, transaction rollback,
  first-activity semantics and timestamp behavior.
- The list now has typed filters/results, generated count/sort/page queries and
  explicit dynamic profile-column projection. It preserves raw-query date formats,
  profile fields overriding guest values and exposed database diagnostics.
- Expanded registration comparisons passed 87/87, including direct assertions of
  profile upgrades, rollback and unchanged timestamps, all sort fields, malformed
  pagination/IDs/configuration and dynamic profile columns. Evidence:
  `.artifacts/registration-list-contract.log`.
- Excel export now uses typed registrant/profile/guest records and generated
  relationship/form/location queries. Its expanded comparison suite is running.
  Formatting, vet, builds, module verification, query drift and harness syntax
  pass in `.artifacts/check-registration-types.log`. Affected race tests and
  related groups must run after the final export fixes.

## Latest club changes and registration checkpoint

- Reviewed the latest admin-be (`dd8d0ff`), web-be (`d366303`) and public frontend
  (`9144a84`) commits. Go now preserves null club-export dates and formats valid
  dates in Asia/Jakarta. The club closing job uses the Jakarta calendar regardless
  of the host timezone. Embedded timezone data supports native packaging.
- Club and club-member comparisons pass at `TZ=UTC`: 46/46 and 33/33. They caught
  and fixed model timestamps requiring `+00:00`, while raw JavaScript Date fields
  retain `Z`. Three recorded storage objects per backend were removed and checked
  absent. Evidence: `.artifacts/contracts-new-club-utc.log`.
- Shared Go/web-be tests pass 53 checks, including immutable club answers,
  duplicate submission, owner-only cancellation, approval visibility, member
  roles, closed registration, and required active forms. The new source club
  workflow tests also pass against owned schemas in both Adonis applications,
  with no skipped tests and rolled-back fixture rows verified absent.
  Evidence: `.artifacts/shared-database-club-update.log` and
  `.artifacts/source-club-workflows.log`.
- All three job commands and repeated execution match Adonis (six comparisons).
  The club CLI comparison runs under UTC. Boundary tests exercise leap/year
  changes, null dates, and immutable zero/false answers.
- Expanded registration export comparisons pass 92/92, including real Excel
  headers/cells, guest and profile locations, education/history, active custom
  form precedence and legacy fallback. Participant ordering assertions were
  ported into real HTTP/database tests for four lists and pass in both directions
  with null dates and pagination. Further malformed stored JSON and wide-ID
  diagnostics remain to be reviewed; this is not full edge-case completion.
- All four public browser workflows pass on desktop/mobile against the production
  Next build and actual Go/web-be APIs. Added required checkbox and whitespace
  validation, zero-valued answers, cancellation/resubmission, and Go approval.
  An initial combined run caught inconsistent reference fixtures interacting with
  Next's cache; the fixture setup was fixed and all four tests rerun. Screenshots
  were inspected. Evidence: `.artifacts/browser/2026-09-09T23-35-48-618Z/`.
  The latest public club request-time rendering and success loading changes were
  committed before this run. Workspace backend ports are restored.
- Twelve original application checks passed on the new club baseline. Their
  pre-existing opt-in skips are reported separately; the new source club tests
  were explicitly enabled and executed. No frontend product files were changed.
- Full current-source race tests passed 661 test cases with no failed or
  skipped tests in `.artifacts/go-race-club-complete.jsonl`. All 12 recorded
  storage objects were removed and checked absent. Final aggregate contracts,
  architecture review and the image-backed admin PDF browser check remain incomplete.

## Typed club service continuation

- Saved the tested registration/latest-club checkpoint as `8493eeb`.
- Club create/update/list/detail, registration information and all media operations
  now use explicit DTOs, generated queries and a dedicated service. Preserved
  draft/closed defaults, omitted fields, nullable dates, duplicate checks, row
  locks, active-form requirements and storage cleanup. No-op updates retain
  their original timestamp. Form mutation and membership/role adapters remain.
- Expanded club comparisons to 106 scenarios and passed them under UTC, including
  wide/fractional/encoded identifiers, Number() conversion on update, null dates,
  inactive latest forms, duplicate-media precedence and every club type filter.
  Evidence: `.artifacts/club-typed-affected-contracts.log`.
- The added cases exposed default Vine date formats: the source accepts strict
  `YYYY-MM-DD` and `YYYY-MM-DD HH:mm:ss`, without ISO offsets/fractional seconds.
  Go now matches them. Regenerated all 662 real Vine fixtures; all pass.
- Corrected escaped route-parameter handling, first-page pagination diagnostics,
  and explicit UTC offsets on administrator/certificate model timestamps. Raw
  date projections and session/access-request dates keep their source formatting.
- The activity registration Excel source query has no ORDER BY. Different update
  plans changed heap order between isolated schemas. Only this unordered export's
  row order is now normalized after checking its visible 1..N numbering. Complete
  row contents, duplicate counts, headers, column order and all explicitly ordered
  exports remain strict. Normalizer integrity tests pass and run with `check`.
- A UTC run stopped when the shared PostgreSQL connection reported ENETUNREACH.
  The harness restored port 3334. A later read-only probe succeeded; the complete
  contract run was restarted in `.artifacts/contracts-current-harness-utc.log`.
- Source evidence version 2 hashes application code, queries, test definitions,
  fixtures, generation settings and dependency locks. It excludes environment
  files. Contracts verify the hash and Adonis revision again after teardown;
  Go integration tests verify the hash after cleanup. Earlier reports must be
  rerun under this evidence version before counting toward current completion.
- Local checks, race-enabled unit tests and native package/startup checks pass:
  `.artifacts/check-typed-clubs.log`, `.artifacts/unit-typed-clubs.log`, and
  `.artifacts/package-typed-clubs.log`. Current database/storage/race/browser
  checks still need to finish. The current owned schemas remain available;
  there are no retained failed storage journals.
- Typed form read/query drafts are in `.artifacts/next-club/`; they have not been
  applied or counted as completed functionality. After this club checkpoint's
  checks, continue form and club registration/role DTO/query ports.

- The complete contract run recovered and passed all 17 groups: 1,173 scenarios
  across 139 routes, under UTC, with no stale/missing reports under source-evidence
  version 2. Coverage succeeds in `.artifacts/coverage-typed-clubs-current.json`;
  detailed execution is `.artifacts/contracts-current-harness-utc.log`. Port 3334
  was restored and recorded. Full race integration and public browser reruns are
  the next checks for this checkpoint.

- Current full race integration passed 837 test cases with no failures or skipped
  tests. All 12 tracked storage objects were removed and verified absent. Evidence:
  `.artifacts/go-race-typed-clubs-current.jsonl`.
- Current public browser verification passed all four desktop/mobile workflows.
  Screenshots were inspected. Evidence:
  `.artifacts/browser/2026-09-10T01-04-47-319Z/` and
  `.artifacts/browser-public-typed-clubs.log`. Backend ports were restored.
- The public frontend, web-be and Adonis source remain clean at the adopted
  revisions. Admin-fe now contains substantial independent local responsive UI
  changes; these were not modified by this task. Previous admin frontend checks
  are historical and its current browser/build checks remain pending.
- Removed all three schemas belonging to fixture run `ce49aafd3320fdb9` and verified
  their absence. No pending storage journals remain. Cleanup evidence:
  `.artifacts/cleanup-typed-clubs.log` and
  `.artifacts/cleaned-schemas-ce49aafd3320fdb9.json`. The next continuation must run
  `node scripts/ensure-fixtures.mjs` before database or browser tests.

## Typed forms continuation

- Custom form reads, creation, updates, activity/club attachment, deletion and
  activation now use explicit DTOs and generated queries. Safe-integer behavior,
  null attachments, source lock ordering and unchanged timestamps are preserved.
- 104 additional form comparisons pass under UTC against the current Adonis
  baseline (`.artifacts/contracts-forms-boundaries.log`). The separate forms
  group is included in aggregate contract verification.
- New race-enabled database attachment/concurrent opposite-move tests pass;
  all three selected Go packages passed (`.artifacts/integration-typed-forms.log`).
  The harness still failed teardown because storage DNS is unavailable.
- `nos.wjv-1.neo.id` currently fails system lookup with ENOTFOUND. The attempted
  club upload journal remains recorded for cleanup/absence verification. No
  storage check or cleanup has been waived. Full comparison and cleanup need
  rerunning after this dependency recovers.

- Storage DNS recovery is available through `GO_REWRITE_DIRECT_DNS=1`, scoped to
  child/test processes with real resolver queries and unchanged TLS verification.
  The retained storage journal was cleaned and its key verified absent.
- Clubs, registrations, roles and forms now have typed request/response/service
  boundaries and generated queries, including role listing without one query per
  role. 430/430 affected UTC comparisons pass: club membership 112, forms 104,
  clubs 106 and query edges 108 (`.artifacts/contracts-club-form-typed.log`).
- `check` passes. The affected race-enabled form/club lifecycle, attachment
  concurrency, role review and real-storage tests pass with cleanup
  (`.artifacts/integration-club-form-typed.jsonl`). The Go run's three created
  storage keys were deleted and verified absent. Counseling/achievements,
  certificates, wider remaining identifier review and final verification remain.

## Typed counseling and achievements continuation

- Counseling, achievements and both leaderboards now use explicit DTOs, services
  and generated queries. Nullable score categories, repeated approvals, rejection
  attribution, date filters, unchanged timestamps and integer-overflow rollback
  retain the Adonis behavior. Excel exports retain nullable values and ordering.
- All 113 affected comparisons pass under UTC:
  `.artifacts/contracts-achievements-typed.log`. Local checks pass.
- The first full race run failed one club logo upload with an uninformative 500;
  872 tests/subtests passed. This failed run is retained in
  `.artifacts/integration-before-certificates.jsonl`. The cause was not captured.
- HTTP test fixtures now log request errors to the test output. The diagnostic
  club lifecycle rerun passed, followed by the entire race suite with no failed
  or skipped tests in `.artifacts/integration-counseling-achievement-typed.jsonl`.
  All 12 tracked objects were deleted and verified absent. The earlier upload
  failure has not reproduced; no speculative storage change was made.
- Certificate template/service drafts remain under `.artifacts/next-review/`;
  they are not yet part of the tested application. Continue the certificate port,
  remaining identifier/export edge review, and final aggregate verification.

## Typed certificate continuation

- Template lifecycle, asset upload/copy and cleanup now use explicit DTOs and
  generated queries. Recipient lists, preparation, issuance, compact results and
  certificate reads also use generated queries. The API keeps template/registration
  version checks, registration→activity→template lock order, immutable snapshots,
  duplicate prevention and bulk pause/resume behavior.
- Template comparisons pass 107/107 under UTC, including malformed/wide identifiers,
  null/omitted fields, full element options, copy failure and version overflow.
  Evidence: `.artifacts/contracts-templates-typed-fixed.log`.
- Certificate comparisons pass 154/154 under UTC, including wide/very large IDs,
  expected-version context, guest university resolution, raw legacy template
  assignments, snapshot fallback and omitted list fields. Evidence:
  `.artifacts/contracts-certificates-typed-boundaries-fixed.log`.
- Local checks and affected race tests pass, including 1,000 real database
  issuances, concurrency, template-change pause/resume, immutable snapshots,
  revocation and real storage cleanup. Evidence:
  `.artifacts/check-certificates-typed.log` and
  `.artifacts/integration-certificates-typed.jsonl`.
- A boundary fixture initially attempted SQL NULL in a non-null snapshot column;
  it was corrected to JSON null without changing the schema constraint. The
  original failed harness run remains recorded. Scientific-notation pagination
  links now encode the plus sign, matching Adonis; the regression test passes.

## Identifier, stored-export, and final review continuation

- Expanded raw path comparisons to 337; all passed before the final additional
  module cases. PostgreSQL receives original path values for source actions that
  do not call Number; image storage keys use the resolved activity ID.
- Export comparisons include malformed questionnaire/configuration/form documents,
  null answers, history entries, and text `0`/`false`. One SQL NULL versus absent
  property mismatch was corrected; affected rerun is in progress.
- Replaced narrowing pagination bounds with typed text parameters cast by
  PostgreSQL; added wide, hex, and Unicode whitespace comparisons. Profile SQL
  diagnostics now include relation and JSON-expression filters.
- Reviewed all 139 routes in `tests/route-applicability.json`; the coverage gate now
  requires passing invalid-input and missing-resource evidence wherever applicable.
- Moved the generic JSON query helper behind the integration build tag. Production
  business DTO/query review is documented in `docs/TYPE_DESIGN.md`.
- Generated and passed 1,418 Vine validation fixtures, including Number coercion
  of boolean/array inputs and hexadecimal/Unicode numeric strings.
- Adopted independently committed admin frontend revision
  `37914ab019d03be4b959aaf8e1c5b281852eb78a` for final responsive browser checks.
  Adonis and web backend club revisions remain unchanged from the adopted baseline.

- All 189 expanded query comparisons and 444 expanded route-edge comparisons now
  pass. This includes every numeric path family, nonexistent referenced resources,
  SQL diagnostics, actual runtime stacks, and wide pagination bounds. Each route
  run removed and verified its recorded storage object; shared bucket settings
  remain unchanged. Registration/achievement/protocol reruns are finishing before
  the aggregate verification starts.

- Final affected checks pass: route edges 444/444, registrations 128/128,
  achievements 116/116, protocol 48/48. Formatting, vet (including integration),
  builds, module integrity, sqlc drift, and normalizer integrity all pass in
  `.artifacts/check-complete-boundaries.log`. The final aggregate run is next.

## Aggregate verification in progress

Run: `.artifacts/verify/2026-09-10T02-57-58-858Z/`, source commit `3d13dd0`.
Checks, 1,589 race unit tests/subtests, native packaging, fixture disconnect, and
both original club integration workflows pass. Full integration: 1,629 passed,
zero skipped, one failure in `TestCertificateTemplateLifecycleAndAssets`.
The captured failure is S3 PutObject DNS NXDOMAIN for the configured test endpoint;
no upload reached storage. This is an environment connection failure, not an
unexplained HTTP 500. The AWS standard retryer explicitly declines NXDOMAIN.
A bounded DNS retry and unit scenarios are drafted in ignored
`.artifacts/next-review/storage_retry*.go`; do not apply while verification runs.

The user approved temporary localhost CORS. Put succeeded; the immediate provider
read lacked rules, and later reads confirmed the exact approved configuration.
`storage-cors.mjs` now requires two matching reads and supports verifying a
recorded interrupted application. CORS is applied and verified; the running shell
restores it on verification exit. Review `.artifacts/cors-final.log` and the lease
status afterward. No further approval is needed for this agreed apply/test/restore.

## Browser and storage corrections (2026-09-10)

The first aggregate finished incomplete. All 18 contract groups (2,001 comparisons),
12 session-transfer checks, 53 shared-database checks, and three jobs passed. The
one integration failure was a real S3 DNS lookup failure. Admin browser failures
exposed a CORS/cache bug and outdated mobile assumptions; public browser startup
was refused because another task owns port 3000. Two independently added public
poster unit tests lacked the Next image configuration in their test renderer.

Applied a bounded S3 DNS retry, preserving the SDK's three-attempt limit and
cancellation. Its race unit tests verify full-body delivery after a transient
failure, persistent error identity, and zero requests after cancellation.
Admin frontend commit `898b93c` makes certificate editor image requests use CORS;
the mobile test now uses the current editor controls and expands participant card
details for revocation. Desktop and mobile image-backed PDFs downloaded successfully
and were rendered with Poppler: one landscape A4 page with the correct participant,
activity, background, Indonesian date, and certificate code, without clipping.
Evidence: `.artifacts/browser/2026-09-10T03-26-48-232Z/` (9/10, then corrected mobile
locator), `.artifacts/browser/2026-09-10T03-30-49-904Z/` (mobile certificate 1/1).

Public frontend commit `23b5714` supplies the real configured image allowlist to
its poster tests. It follows the independent club/activity UI commit `fccee17`.
All 12 existing-application checks now pass in `.artifacts/baseline-frontend-fixes.log`.
Formatting, vet, compilation, generated query drift, dependencies, and normalization
integrity pass. Source evidence v3 now fingerprints the four existing applications
as well as Go and the harness. A fresh aggregate with these changes is next.

## Public image-backed download correction

Aggregate `.artifacts/verify/2026-09-10T03-33-22-245Z/` passed all 17 automated
stages: 1,593 unit and 1,634 integration tests/subtests, 2,001 contract comparisons,
12 transfer checks, 53 shared-database checks, three jobs, 10 admin and four public
browser workflows, existing checks, performance and cleanup. No tests were skipped.
Its final review stays pending because an additional public image case was needed.
Its 48 evidence files were archived under that run's `evidence/` directory. CORS,
three schemas, recorded objects, and the temporary preview were restored/cleaned.

The strengthened public workflow uploads a real background through Go and checks
its immutable snapshot and browser CORS response. Before the fix, it reproduced a
PDF download timeout with a CORS error and EncodingError on the cached image:
`.artifacts/browser/2026-09-10T04-15-34-282Z/`. Public frontend commit `17a51e9`
replaces the CSS image request with an anonymous-CORS image while preserving its
cover geometry. All four public desktop/mobile workflows then passed, including
image-backed PDFs: `.artifacts/browser/2026-09-10T04-17-17-350Z/`. The downloaded
landscape A4 PDF and both viewport screenshots were inspected. Both recorded
backgrounds were deleted and verified absent; the preview was restarted.
All 71 public unit tests, lint and types pass.

The last race HTTP package took 599.403 seconds, close to Go's default 600-second
limit on the slower shared database. The integration runner now has a bounded
30-minute package limit; assertions, cancellation checks and the 1,000-recipient
workload are unchanged. The current fixture run is `a62222104353e081`; approved
localhost CORS is applied. A final aggregate must run on the updated source before
reporting completion.
