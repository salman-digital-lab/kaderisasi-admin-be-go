# Linked activity courses

Activity editors can save an ordered list of courses in the registration step of
activity setup. Registrant readers see current progress for those courses; export
permission remains separate. No course permission, registration prerequisite,
publication requirement, or automatic participant-status change is introduced.

## API

- `GET /v2/activities/course-options?search=&page=1&per_page=50` requires
  `activities.manage`; returns a paginated list of ID, title, status, and lesson count.
- `GET /v2/activities/:id/courses` requires `activities.read` or
  `activity_registrations.read`; returns the ordered linked-course metadata.
- `PUT /v2/activities/:id/courses` requires `activities.manage`; accepts
  `{"course_ids":[2,1]}`. An empty array removes all links. Missing/null arrays,
  duplicates, nonpositive IDs, and nonexistent courses return 422. Missing
  activities return 404. An activity-row lock serializes atomic replacements.
- Registration lists include `course_progress`, an array of `course_id`, `status`,
  `completed_lessons`, and `total_lessons`. Unlinked activities return `[]`.
- Optional `course_completion=completed|incomplete|unverifiable` filters all linked
  courses; `course_id` narrows the check to one linked course. Invalid filters,
  unlinked course IDs, and filters on activities without courses return 422.
  Search, existing status filters, counts, sorting, and pagination still apply.

The `activity_course_progress` SQL view is the shared calculation for list,
filter, and export queries. Reads use a consistent transaction snapshot. Only
current, non-deleted lessons count. A course is completed only when it has at
least one lesson and all lessons are marked complete. A visit without completion
is in progress. Guests are unverifiable; a course without lessons has `empty`
status and is incomplete for linked users. Archiving does not remove links.
Adding/removing lessons or unmarking completion can change current completion.

Excel appends two columns per course (status and completed/total), in saved order,
with course IDs in headers. It continues exporting all registrants regardless of
screen filters. Guest counts are blank. Exports without links retain their layout.

## Verification

```sh
make check
make test-unit
make test-activity-courses
node scripts/contracts-all.mjs --groups=activities,registrations --borrow-workspace
```

The native suite exercises permissions without course access, validation,
concurrent replacements, current lesson changes, guests, filtering/pagination,
and Excel data. Its browser suite covers desktop/mobile setup, keyboard selection,
unsaved changes, save retry, column preferences, filtering, export, and load retry.
Evidence is under `.artifacts/activity-courses.json` and `.artifacts/browser/`.
Historical contract normalization excludes only an empty `course_progress` field
on successful registration-list responses; nonempty or invalid values remain
visible to comparisons. Native tests independently require the empty array.

Run migration repository lint, typecheck, build, and
`MIGRATION_TEST_ENV=../docs/.env.test.be npm test`. Run admin frontend lint, build,
and tests. Finish fixture-based verification with `make clean-fixtures`.

### Implementation verification (2026-09-13)

Go checks, unit tests, the affected activity/registration integration tests, and
the native linked-course suite passed (42 API checks and both desktop/mobile
browser cases). Migration lint/typecheck/build and all five maintenance tests
passed. Admin frontend lint/build and all 151 tests passed.

The historical activity comparison stops because its fixture creates a published
activity, which the existing Go draft-only creation rule rejects. The separate
registration comparison completed with 68 of 128 cases equivalent. Its remaining
differences concern activity registration defaults, education-history formatting,
and null education/work-history export handling; the corresponding existing
activity and profile/export implementation files were unchanged by this feature.
These historical suites are not green; their reports remain under
`.artifacts/contracts/`. New behavior is verified separately by the native suite.

## Rollout

1. Run the new Ace migration from `kaderisasi-admin-be` against the explicitly
   selected environment, following `../docs/ENVIRONMENTS.md` in the workspace.
   It adds `activity_courses` and the progress view without modifying existing data.
2. Deploy the Go API, then the admin frontend. Do not deploy this API before the
   migration: registration reads and exports use the new relations.
3. Check an activity with no links, save links on a test activity, verify guest
   and member progress, and download its export. Existing public registration
   and learning APIs require no code changes or historical backfill.

For application rollback, retain the additive schema and restore the previous
API/frontend versions. Do not roll back migrations or delete learning history.
