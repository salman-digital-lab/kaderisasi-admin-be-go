# Tested checkpoint — rewrite incomplete

Date: 2026-09-10. Adonis baseline: `dd8d0ff409c34eaaebb8c2e3ec8a046efbf11356`.
Application and harness fingerprint: `dff126ddcee53d8ff2ac7496f7d7eab1b1267c18264bb31976e2a210309769d2`.
This records a tested checkpoint, not cutover approval or the final verification report.

| Check | Result | Evidence |
|---|---|---|
| `node scripts/contracts-all.mjs --timezone=UTC --borrow-workspace` | 17 groups, 1,173/1,173 comparisons pass | `.artifacts/contracts-current-harness-utc.log` |
| `node scripts/coverage.mjs --check` | 139 routes implemented and success-covered; no stale/missing reports | `.artifacts/coverage-typed-clubs-current.json` |
| `node scripts/test-go.mjs -json ./...` | 837 passing tests/subtests; no failures or skipped tests; real PostgreSQL/storage | `.artifacts/go-race-typed-clubs-current.jsonl` |
| `go test -race -count=1 ./...` | Unit suite passes | `.artifacts/unit-typed-clubs.log` |
| `node scripts/check.mjs` | Formatting, vet, builds, dependency/query drift, JavaScript syntax and normalizer integrity pass | `.artifacts/check-typed-clubs-current.log` |
| `node scripts/package.mjs` | Native macOS arm64 archive and executable startup checks pass | `.artifacts/package-typed-clubs.log` |
| `node scripts/browser.mjs --public --borrow-workspace` | Four desktop/mobile workflows pass; screenshots inspected | `.artifacts/browser/2026-09-10T01-04-47-319Z/` |
| `node scripts/cleanup.mjs` | Three recorded schemas removed and checked absent; no pending storage journals | `.artifacts/cleaned-schemas-ce49aafd3320fdb9.json` |

Go 1.26.8 and sqlc 1.31.1 are pinned; the native build uses libvips 8.18.6.
Go/module and Node lockfiles are committed. This package is not a Linux deployment build.
The full race run removed and verified absence of all 12 recorded storage objects;
contract groups also cleaned their own journals. Ports 3334 and 3333 were restored.

The latest club changes are included: Jakarta deadline/export dates, null export
values, immutable answers during review, owner cancellation, active-form checks,
and public approval visibility. Typed club CRUD/media services additionally passed
106 direct cases covering dates, no-op timestamps, duplicate media and identifiers.
Default Vine date-format compatibility now has 662 generated fixtures.

The source's activity-registration export has no ORDER BY. Its comparison verifies
visible numbering and complete row multisets; headers, columns and explicitly
ordered exports remain strict. Comparator integrity tests guard that scope.

A transient database ENETUNREACH stopped an earlier run. The connection recovered,
and the complete replacement run above passed. Failure logs remain preserved.

Remaining work includes typed adapters in other modules, comprehensive per-route
edge applicability, current admin frontend verification, final interoperability,
performance/aggregate reruns and the final report. Image-backed admin PDF testing
has a pending test-bucket CORS approval; no bucket policy was changed. Admin-fe has
independent local responsive UI changes, so its earlier checks are historical.
All twelve earlier original-application checks passed, with their existing opt-in
skips reported separately. Both newly added source club workflow tests were enabled
and passed previously; the final source-app verification must be refreshed.

See [PROGRESS.md](../PROGRESS.md) for history and the next implementation step.
