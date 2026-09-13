# Short links

The admin API owns `/v2/short-links`; the sibling Rust service owns public
redirects at `https://s.salmanitb.com/{code}`. Both read the same `urls` table.
The production read-only preflight on 2026-09-13 found no existing table in the
configured production database, so the forward Ace migration creates it.

All six active admin roles have `short_links.read` and `short_links.manage`.
Unassigned and inactive accounts do not receive these permissions. Runtime
catalog updates require no role reseeding or changes to existing assignments.

| Method | Path | Input | Success |
| --- | --- | --- | --- |
| GET | `/v2/short-links` | `search`, `page`, `per_page` | 200, data contains `data`, `meta`, `base_url` |
| POST | `/v2/short-links` | `original_url`, optional `code` | 201, created link |
| PATCH | `/v2/short-links/{code}` | `original_url` | 200, updated link |
| DELETE | `/v2/short-links/{code}` | none | 200, null data |

Each link contains `code`, `short_url`, `original_url`, `created_at`, and
`visit_count`. Pagination metadata contains `total`, `current_page`, `per_page`,
and `last_page`. Defaults are page 1 and 12 rows; page size is bounded to 100.
Search is literal and case-insensitive across code and destination. Redirect
lookup remains case-sensitive. Lists sort by creation time descending, then code.

Configure `SHORT_URL_BASE_URL` as `https://s.salmanitb.com` in production (the
production default), or `http://localhost:4000` in development/tests. Only an
HTTP(S) origin is accepted. Returned `base_url` also supplies the creation form's
prefix when the list is empty.

Custom codes are 3–10 ASCII letters, numbers, `_`, or `-`; `health` is reserved.
Random generation uses cryptographic randomness with five collision attempts.
Duplicate custom codes return 409 `SHORT_CODE_TAKEN`; exhausted random retries
return 503. Invalid inputs return 422. Updates cannot rename codes. Missing rows
return 404. Destinations must be absolute HTTP(S), without credentials/control
characters/backslashes/spaces, and outside the shortener's own hostname.

Run `make check`, `make test-unit`, and `node scripts/short-links.mjs --browser`.
The last command requires the admin FE running on port 3005 against localhost
3334; it creates a uniquely owned schema through Ace and records cleanup. Rust
must be built from the sibling `url-shortener` checkout. No production database
or shared test-table cleanup occurs in these tests.

Deployment order: Ace migration, Rust redirect service, Go admin API, admin FE.
Retain prior application images. Never roll back or reset the shared table to
roll back an application image. See the Rust repository's `API.md` for runtime
configuration and redirect behavior.
