# Admin access and activity setup

The catalog contains six roles: Super Admin, Admin Operasional, Panitia Kegiatan,
Pengelola Komunitas, Petugas Anggota, and Konselor. The five roles other than
Super Admin can be requested. A successful review replaces the applicant's
current role. Retired role names remain readable on completed requests and do
not authorize accounts.

`activities.manage` allows activity preparation and content edits, including
live activities. `activities.publish` controls publication and unpublication;
`activities.registration.manage` controls opening and closing registration.
Only Admin Operasional and Super Admin receive the latter two permissions.
Panitia Kegiatan receives neither. Certificate mutation permissions belong to
the operational roles, rather than Panitia.

The admin frontend starts role applications at `/my-requests/new`, using task
choices and short capability summaries. `/my-requests` shows current access and
pending requests. Request details retain the submitted reason, review result,
rejection reason, and timestamps. Unsaved changes and failed submissions retain
the user's input.

Activities start at `/activity/new`, with a name sufficient to save a private,
closed draft. Saved activities continue at `/activity/:id/setup`. Four steps
cover basics, description and poster, registration, and review. **Description
and at least one uploaded poster are mandatory before publication.** Activity
type, category, and minimum level must also be valid. Saved date pairs must be
complete and ordered. A published activity cannot have its description cleared
or lose its last poster.

`GET /v2/activities/:id/readiness` returns content readiness, actionable issues
with step indices, and the actor's allowed actions separately. Registration
requires a published activity, an active valid attached form, and a current
registration date window in Asia/Jakarta. Opening remains manual; the existing
closing job is unchanged. Publishing information before preparing registration
is supported. Unpublishing also closes registration.

Panitia's final step provides an internal activity link, a copyable help
message, and the existing WhatsApp support group. It does not create an approval
ticket or send a message. Form operations preserve a valid active form while
registration is open, while allowing valid live schema edits.

The public API hides draft details and forms. Both authenticated and guest
registration check publication, opening, date window, active form, and answers
inside a transaction that locks the activity, matching the admin workflow's
locking boundary.

## Data and release

The approved direct SQL is [sql/simplify-admin-roles.sql](sql/simplify-admin-roles.sql).
It changed two Asmen accounts to Admin Operasional, two Kapro accounts to Panitia
Kegiatan, and one Leaderboard account to Petugas Anggota. One newly pending Kapro
request was explicitly approved for retargeting to Panitia, preserving its
pending status and reason. The transaction committed on September 10, 2026,
at 15:56 UTC. It verified preserved account identities, credentials, active
states, and completed request history. No new Ace migration was created or run.
The SQL intentionally refuses to run again against different preconditions.
Restricted aggregate evidence is in `.artifacts/role-consolidation/result.json`.

The API, public API, and frontend changes need a coordinated application release;
the SQL does not deploy source code. Historical comparison fixtures retain the
old catalog, while native tests assert the revised product behavior.

## Verification

Go checks and race-enabled unit tests cover the role catalog and readiness.
`TestActivitySetupPermissionsAndHandoff` exercises real PostgreSQL rollback of
forbidden mixed updates, mandatory description and poster, separate publication
and opening, live edits, and form protections. Public API registration tests use
the owned cross schema and roll back their fixtures. Frontend type checking,
lint, build, and unit tests accompany the in-memory browser preview at
`kaderisasi-admin-fe/tests/browser/workflows.html`. Mobile (390 × 844) and desktop
(1440 × 900) checks include the established club page for visual consistency.
The browser preview also exercises failed access submissions with preserved
inputs, registration-form load failures with retry, and activation of an attached
form while activity registration stays closed. The role-request character
counter has its own line on mobile.
