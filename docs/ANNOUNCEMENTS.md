# Announcements and notification inboxes

Super Admin manages announcements at `/announcements` in the admin frontend.
Every authenticated admin and active public account can read its own inbox at
`/notifications` in its respective frontend. Announcements are plain text with an
optional HTTPS action link. Email, push, automatic events, and scheduling are not
part of this release.

## API and ownership

The Go API owns `/v2/announcements` (GET, POST), `/:id` (GET, PATCH, DELETE),
`/:id/preview`, `/:id/publish`, and `/:id/withdraw` (POST). All require
`announcements.manage`, granted only to Super Admin. `/v2/announcements/options`
provides bounded searches by `kind=member|admin|activity|club`, `search`, and
optional comma-separated `selected` IDs so saved selections retain their labels.

Both APIs expose `/v2/notifications` (GET), `/unread-count` (GET), `/:id` (GET),
`/:id/read` (PUT), and `/read-all` (PUT). They authenticate their own account type;
no recipient identifier is accepted from clients. The public frontend calls a
private Next.js proxy that reads the session cookie server-side and checks
same-origin PUT requests. All notification responses use `private, no-store`.

Create/update input:

```json
{
  "title": "Informasi kegiatan",
  "body": "Isi pengumuman dalam teks biasa.",
  "link_label": null,
  "link_url": null,
  "version": 1,
  "audience": {
    "all_members": false,
    "member_ids": [],
    "activity_ids": [12],
    "activity_statuses": [],
    "club_ids": [],
    "club_statuses": ["APPROVED"],
    "all_admins": false,
    "admin_ids": [],
    "role_codes": []
  }
}
```

Selections are unioned and deduplicated by account identity, including additional
admin roles. Activity status selections use the application's existing values
and custom statuses; an empty selection includes every status. Club status
selection defaults to APPROVED. Inactive admins and public records with an
account status other than active are excluded. Preview returns eligible,
excluded, members, and admins counts.

Publish and draft-delete require `{ "version": 1 }`. PATCH replaces the editable
content/audience and rejects stale versions. Publishing locks the draft, resolves
recipients again, inserts the recipient snapshot, and transitions state in one
transaction. Concurrent retries return the existing publication. Empty audiences
fail without changing state. Published content cannot be edited or deleted;
withdrawal removes it from all inbox queries but preserves management history.
No automatic expiry or historical backfill is performed.

Inbox list responses contain `items`, `next_cursor`, and `cutoff`. Pass the opaque
cursor to obtain the next page (20 items); `unread=true` filters unread records.
Read-all takes `{ "cutoff": "<value from list response>" }`. Publication and cutoff
issuance share advisory transaction lock 1790045423 so an in-flight publication
cannot commit behind an earlier cutoff. Timestamps preserve microseconds in
pagination cursors. Opening the bell does not mark messages read; opening a
message or an explicit read action does. Visible tabs refresh every 30 seconds
and on focus; requests are cancelled when the view unmounts.

## Rollout and verification

Apply the additive `1790045423426_create_announcements_table.ts` migration using
Ace in `kaderisasi-admin-be` with the intended environment, as described in
`../../docs/ENVIRONMENTS.md`. Deploy both backend APIs after that migration, then
both frontends. Go never runs migrations. Existing permissions are computed from
the runtime catalogs; no demo or RBAC seed is needed for this permission addition.

Publication logs include announcement ID, duration, and recipient count; they do
not include message bodies. Retain the additive tables if reverting application
binaries so published history is preserved.

Focused verification from this repository:

```sh
make check
make test-unit
make fixtures
node scripts/test-go.mjs ./internal/httpapi -run TestAnnouncementWorkflow -v
node scripts/browser.mjs announcements.spec.mjs --borrow-workspace
node scripts/browser.mjs announcements.spec.mjs --public --borrow-workspace
make clean-fixtures
```

The browser suites include member API isolation, shared delivery, read state,
cutoffs, withdrawal, error recovery, and desktop/mobile screenshots. Fixture
runners use owned schemas in the configured test database, and restore borrowed
workspace services. Both frontends also run their lint, typecheck/build, and test
scripts; the migration repository runs lint, typecheck, build and its migration
tests. The public backend runs lint, typecheck and build.

When the frontends are already running outside the workspace tmux launcher, add
`--reuse-admin-frontend` for admin tests or `--reuse-public-frontend` for public
tests. These options verify the frontend's connection to the isolated test API
without stopping the existing frontend process.

Verified on 2026-09-22: Go checks and race-enabled unit tests, the announcement
PostgreSQL integration suite, six desktop/mobile browser cases, six migration
tests, 169 admin frontend tests, 143 public frontend tests, and lint/typecheck/
build checks across the affected TypeScript repositories passed. Owned fixture
schemas were cleaned afterward. The public production build succeeded with a
sitemap fetch warning because the activity API was stopped. No shared or
production database migration was applied by this implementation task.
