# Kalender Kegiatan

Calendar events are independent of activities and immediately public. The admin
form and both calendar displays use dates only, with an inclusive final date.
New and edited events are always all-day. Timestamp fields remain an internal
storage/API convention; no time picker or time label is shown to users.
The admin page is `/calendar`; the public page is `/tentang/kalender-bmka` under Tentang.
Every active admin role receives `calendar.read`. Only `admin` and `super_admin`
receive `calendar.manage`, including when assigned as an additional role.

## API

- `GET /v2/calendar-events?start=<RFC3339>&end=<RFC3339>` is anonymous.
- `GET /v2/admin/calendar-events` accepts the same range and requires `calendar.read`.
- `GET /v2/admin/calendar-events/:id` requires `calendar.read`.
- `POST /v2/admin/calendar-events`, `PUT /v2/admin/calendar-events/:id`, and
  `DELETE /v2/admin/calendar-events/:id` require `calendar.manage`.

Ranges are required, exclusive at the end, and limited to 93 days. Queries return
events overlapping the range, ordered by start and ID, without truncating busy
days. Responses use `{ message, data }` and `Cache-Control: no-store`.

Create and replacement-update payloads contain `title` (1–255 trimmed characters),
nullable `description` (up to 10,000 characters), nullable `location` (up to 500),
`starts_at`, `ends_at` (RFC3339 timestamps), `all_day`, and nullable `activity_id`.
End must follow start. All-day boundaries must be midnight in Asia/Jakarta;
forms convert the inclusive final date to the following midnight. The public
response includes `activity: null` for draft or missing activities, never their
identifiers or metadata. Published activity references contain ID, name, slug,
and publication status. Admin responses include linked drafts for editing.

Deleting an activity sets the event reference to null. There is no automatic
creation, synchronization, recurrence, notification, or activity-data backfill.

## Rollout and verification

1. Apply `1790003759767_create_calendar_events_table.ts` with Ace from the migration
   repository against the explicitly selected deployment environment.
2. Deploy the Go API, then both frontends. Preserve the existing frontend CORS
   origins and API URLs. Sessions obtain updated permissions when refreshed.
3. Verify a saved event appears anonymously, and a non-editor cannot mutate it.

The migration is additive. When reverting application code, keep its table and
event data; do not roll back shared database migrations.

Checks use owned test schemas:

```sh
make check
make test-unit
make test-calendar
make clean-fixtures
```

Run the migration repository checks and both frontend checks as required by the
workspace instructions. The calendar date tests cover WIB midnight, exclusive
ends, multi-day overlaps, leap days, and month/year navigation.
