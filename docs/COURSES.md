# Kelas

The Go API manages courses at `/v2/courses`. The AdonisJS public API serves the
eligible learner catalog, lessons, progress, and downloads at the same prefix
on port 3333. Admin routes require `courses.read` or `courses.manage`.
`super_admin`, `admin`, `operations_admin`, and the requestable `course_manager`
role receive both permissions. Course Manager also has `dashboard.read`.

## Storage and rollout

1. Reuse the existing `DRIVE_BUCKET` and storage credentials in both APIs.
   PDFs are stored at `courses/<uuid>.pdf` with a private object ACL and
   `Cache-Control: private, no-store`. No additional bucket or environment
   variable is needed. Verify that the bucket policy does not grant public
   object reads under `courses/`: a public bucket policy can override a private
   object ACL. The current service supports separate object ACLs, so existing
   public uploads retain their behavior. Verify anonymous PDF denial before
   publishing. Downloads pass through the APIs and need no browser CORS policy.
2. Run the additive migration from `kaderisasi-admin-be` against the explicitly
   selected environment: `node ace migration:status`, `node ace migration:run`,
   and `node ace rbac:preflight`. Follow the existing environment selection
   procedure; never reset tables or run demo seeds during deployment.
3. Deploy the Go admin API and AdonisJS public API, then the admin and public
   frontends. All applications keep their existing ports and framework versions.
4. Create a draft as Course Manager, add a valid video and PDF, then publish it.
   Verify access with one eligible and one ineligible test account, manual
   completion and undo, the admin progress report, and authenticated PDF download.
   Confirm an anonymous request for the stored object fails before adding real
   courses. Unpublish the smoke-test course afterward.

Missing bucket configuration returns `503` for document operations.
Other upload paths retain their existing public storage behavior.
Course PDFs are limited to 20 MiB, validated by filename, PDF signature, and EOF
marker. The admin multipart request limit is 21 MiB only on course upload routes.
Set any reverse-proxy body limit high enough for that request, and leave the
existing upload contracts unchanged.

To withdraw a course, save its status as `draft` or `archived`. Learner access
ends immediately while progress remains. Removed lessons and documents are soft
deleted. Their private objects and historical progress remain stored; there is
no automatic purge or retention scheduler in this version. Rollback should hide
the feature or restore prior applications while retaining the additive tables.

## API behavior

Admin course create/update accepts `title`, `summary`, `description`,
`minimum_level` (0, 3, 6, or 10), and `status` (`draft`, `published`, `archived`).
Creation always starts as a draft. Updates replace these editable fields.
Lesson create/update accepts `title`, `description`, and `youtube_url`, normalized
to an 11-character video ID. Publishing requires at least one current lesson,
with a valid video ID for every lesson. Changes to published content are live
on save; use draft status while staging edits.

| Owner | Method and path below `/v2/courses` | Purpose |
| --- | --- | --- |
| Admin | GET /, POST /, GET /:id, PUT /:id | Catalog and course editing |
| Admin | POST /:id/lessons, PUT or DELETE /:id/lessons/:lessonId | Lesson editing and removal |
| Admin | PUT /:id/lesson-order | Complete ordered `lesson_ids` array |
| Admin | GET /:id/learners | Searchable, paginated learner progress |
| Admin | POST /:id/lessons/:lessonId/documents | Multipart `file` PDF |
| Admin | DELETE /:id/lessons/:lessonId/documents/:documentId | Remove PDF |
| Both | GET /:id/lessons/:lessonId/documents/:documentId/download | Authenticated PDF attachment |
| Learner | GET /, GET /:id, GET /:id/lessons/:lessonId | Eligible content and personal progress |
| Learner | POST /:id/lessons/:lessonId/visit | Actual lesson visit |
| Learner | PUT /:id/lessons/:lessonId/completion | `{ "completed": true }` or `false` |

Lists accept `page`, `per_page`, and `search`; the admin catalog also accepts
`status`. Responses use the existing `{ message, data }` envelope, with
`{ data, meta }` inside `data` for lists. Learner responses and downloads use
`Cache-Control: private, no-store`. The public frontend streams downloads through
`/api/courses/:id/lessons/:lessonId/documents/:documentId/download` using its
HTTP-only session cookie, without exposing bearer tokens in links.

Every learner request checks the current database profile and publication state.
Guests receive `401`. Missing profiles, unpublished courses, ineligible levels,
removed lessons/documents, and parent-ID mismatches receive `404`. Reading or
prefetching content never records a visit; the visible lesson component calls
the visit action after mounting. Completion uses the authenticated user only.
Repeated completion updates retain the first completion timestamp. A normal
visit preserves completion. Resume follows the latest visited current lesson.
Progress denominators use current lessons, including additions and excluding
removals; membership levels are never changed by course progress.

New access tokens distinguish `kaderisasi-admin` and `kaderisasi-public` audiences.
Course endpoints require their respective audience, preventing overlapping IDs
in the admin and public user tables from granting access across APIs. The admin
API accepts existing unscoped 15-minute admin tokens for legacy routes while
rejecting legacy one-day learner tokens, including session migration. Existing
admin sessions refresh into scoped tokens. Learners with an older token sign in
again when first opening Kelas and return to the requested page. JWT signing keys,
refresh-cookie behavior, session response envelopes, and role checks remain in
place. The extra `aud` claim is an intentional security addition; the historical
Adonis reference is unchanged.

Rich text is sanitized with the public API's existing sanitizer. YouTube videos
use ordinary embeds: managers upload them to YouTube and paste their links.
Unlisted video links can still be shared outside Kaderisasi; see
[YouTube visibility guidance](https://support.google.com/youtube/answer/157177).

## Verification

From this repository (the browser suite builds the public frontend itself):

```sh
make check
make test-unit
make test-courses
make test-courses-browser
```

The course suite is native Go/public-Adonis coverage; it does not change the
historical compatibility baseline. `node scripts/coverage.mjs --native=courses
--check` checks all course routes against their separate applicability catalog,
including successful requests, authentication/permission denials, invalid input,
and missing resources. It uses the ownership-verified `cross` test
schema under the fixture lease and runs current Ace migrations. It uses the
configured test bucket, records unique object keys before upload, deletes only
those objects, and verifies their removal in `finally`. It never creates or
deletes a bucket. The suite temporarily borrows recognized workspace processes and
restores them afterward. Set `BORROW_WORKSPACE=` to require idle ports.

Evidence is written to `.artifacts/courses.json` (checks and cleanup) and
`.artifacts/courses-browser/` (browser checks, downloads, and desktop/mobile
screenshots). `make clean-fixtures` removes the recorded fixture schemas when
no other suite is using them. Also run the frontend and public-backend lint,
type checks/builds and the migration repository's required maintenance suite.
