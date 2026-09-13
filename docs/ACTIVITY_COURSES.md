# Linked activity courses

Activity and club editors can save an ordered list of courses from the Kelas Online
tab. Activity setup also offers the same searchable, paginated chooser. Current
progress lives in a separate table below course configuration, rather than the
ordinary participant-management table. Registrant readers see progress; export
permission remains separate. No course permission, registration prerequisite,
publication requirement, or automatic participant-status change is introduced.

## API

- `GET /v2/activities/course-options?search=&page=1&per_page=50` requires
  `activities.manage`; returns a paginated list of ID, title, summary, minimum level, status, and lesson count.
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


## Club support and dedicated progress tables

The additive `1789283638947_create_create_club_courses_table` Ace migration adds
`club_courses` and `club_course_progress`. It does not change activity links or
learning records. Club progress includes every registration status, including
approved members. Null member accounts are unverifiable. Clubs use the same
current-lesson calculation as activities.

- `GET /v2/clubs/course-options` uses `clubs.manage` and the same paginated metadata
  catalogue as activities, including draft and archived classes.
- `GET /v2/clubs/:id/courses` allows `clubs.read` or `club_registrations.read`.
- `PUT /v2/clubs/:id/courses` uses `clubs.manage`, ordered `course_ids`, 422 validation,
  and a club-row lock for atomic replacements.
- `GET /v2/{activities|clubs}/:id/course-progress` requires the corresponding
  registration-read permission. It returns ordered `courses`, paginated `data`
  (registration ID, name, email, registration status, course progress), and `meta`.
  Search, registration status, `course_id`, and `course_completion` filters apply
  before pagination with identical count predicates and a repeatable-read snapshot.
- The same path with `/export` uses registration-export permission and exports
  all registrants, regardless of screen filters. Columns contain course IDs.

No endpoint adds a `courses.read` requirement. The existing club-manager role does
already have course access; activity-manager tests independently cover admins
without it. Ordinary club registration responses and exports remain compatible.

The chooser shows summary, course status, minimum level and current lesson count.
Selections persist across catalogue pages and searches; admins can reorder them
before saving. Cancel discards modal edits. Saving refreshes the progress table and
resets its filters. Progress columns have preferences separate from participant
profile fields. Failed progress loads show an explicit retry/reset state.

Deploy the club migration before the updated Go API (its dedicated progress queries
reference both views), then deploy the frontend. Leave both additive schemas in
place during an application rollback.
