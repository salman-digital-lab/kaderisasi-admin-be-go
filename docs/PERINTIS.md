# Pembinaan Beasiswa Perintis

The mentoring programme for Beasiswa Perintis scholars. Participants (mentee,
mentor, koordinator) use `kaderisasi-web-be-go`; the pengelola uses this API.
Both serve the `kaderisasi-perintis-fe` app. Product and data design live in
the workspace (`docs/perintis/PRD.md`, `DB_DESIGN.md`).

## Data

Ace migration `1793168600200_create_perintis_tables` in `kaderisasi-admin-be`
creates three tables and alters none:

| Table | Holds |
| --- | --- |
| `perintis_participants` | One row per person: roles, mentee fields, mentor → mentee and koordinator → mentor links, status |
| `perintis_journals` | One Catatan Perjalanan (lapbul) per mentee per month: answers, files, mentor response, wellbeing flag, follow-up |
| `perintis_session_reports` | One mentoring report per mentor per month: session, photos, case, koordinator review |

Profiles, activities, clubs and achievements are read from the existing
kaderisasi tables. The SQL is `database/queries/perintis.sql`, kept identical
in both Go repositories (each generates `internal/dbgen`).

## Code

`internal/perintis` holds the programme rules (deadlines in WIB, lapbul
windows, tingkat, form checks, wellbeing flag, curriculum, T1–T3) and the data
loading shared with `kaderisasi-web-be-go`. `rules.go`, `curriculum.go`,
`curriculum.json`, `views.go`, `data.go` and `logic.go` are identical in both
repositories apart from the module path; change them together. `staff.go` is
only here; `participant.go` only in the web API.

## Access

Role **Pengelola Perintis** (`perintis_manager`) is assigned by a Super Admin
and cannot be requested. Super Admin also holds every `perintis.*` permission,
and is the only role with `perintis.participants.add`: searching all kaderisasi
members (name, email, member ID; at least 3 characters, 20 results) and enrolling
one with roles. A mentee may be enrolled (or edited) without NIM,
kategori and KIP-K; migration `1793195667780_relax_perintis_mentee_data` dropped
the database check. The participant list flags them, and the web API locks the
lapbul until a kategori is set, because the kategori picks the form. To let Pengelola Perintis do it too, add that permission to the
role in `internal/auth/roles.json`.

| Permission | Routes |
| --- | --- |
| `perintis.recap.read` | `GET /v2/perintis/dashboard?month=` |
| `perintis.followups.manage` | `GET /v2/perintis/followups?source=&status=`, `PUT /v2/perintis/followups/{source}/{id}` |
| `perintis.participants.read` | `GET /v2/perintis/participants?role=&status=&q=`, `GET /v2/perintis/participants/{id}` |
| `perintis.participants.manage` | `PUT /v2/perintis/participants/{id}`, `PUT …/{id}/mentor`, `PUT …/{id}/status`, `POST /v2/perintis/journals/reopen`, `POST /v2/perintis/imports/preview`, `POST /v2/perintis/imports/apply` |
| `perintis.participants.add` | `GET /v2/perintis/members?q=`, `POST /v2/perintis/participants` |
| `perintis.content.read` | `GET /v2/perintis/journals/{id}`, `GET /v2/perintis/files?key=` |
| `perintis.recap.export` | `GET /v2/perintis/exports/{kind}?month=` |

## Contract

Responses are `{"message":"OK","data":…}` with the camelCase shapes of the
app's `src/api/types.ts`. Failures are `{"message":"<Indonesian text>"}` with
the HTTP status, shown to the user as is. Bodies are JSON (CSV imports up to
2 MB). Exports return `{filename, csv}`; cells that start like a spreadsheet
formula are prefixed with `'`. Uploaded files are private objects served only
through `/v2/perintis/files` after checking the key is recorded on a row.

`PERINTIS_CLOCK` (RFC 3339, optional) shifts the programme clock for local
demos and tests. Never set it in production.

## Verification

```sh
go test ./internal/perintis/ ./internal/auth/ ./internal/httpapi/ -run 'Perintis|Months|Tingkat|Flag|Missing|Clean|Curriculum|Roles'
set -a; . ../env/test/admin-be-test-env; set +a
go test -tags='integration perintis' -count=1 ./internal/perintis/
```

The integration test creates and drops its own `perintis_admin_<run>` schema
with an explicit search path; it never touches `public` tables.

## Production deployment (28 October 2026)

- Ace migration `1793168600200_create_perintis_tables` applied to production;
  no other migration was pending. No data was copied or seeded.
- Coolify: `ADMIN_CORS_ORIGINS` gained `https://perintis.salmanitb.com`
  (admin API); `GOOGLE_EXTRA_REDIRECT_URIS=https://perintis.salmanitb.com/masuk/google`
  added to `kaderisasi-web-be-prod`. `env/prod` mirrors both.
- Deployed `kaderisasi-admin-be-prod` at `ca0ae63` and `kaderisasi-web-be-prod`
  at `c6fff35` (manual deploys: pushes do not trigger Coolify yet).
- New application `kaderisasi-perintis-fe-prod` (`jvtumqbksug1yqs8qluoih46`):
  nixpacks static build of `kaderisasi-perintis-fe` `main`, `/dist` on nginx with
  SPA fallback, domain `https://perintis.salmanitb.com` (Cloudflare proxied). Build
  variables: `VITE_USE_MOCK_API=false`, `VITE_WEB_API`, `VITE_ADMIN_API`,
  `VITE_GOOGLE_CLIENT_ID` (the shared Kaderisasi client).
- Smoke checks: both `/health` 200; Perintis routes answer 401 without a session;
  CORS preflight from the new origin passes; the app loads in real-API mode and
  email sign-in reaches the web API; an unlisted Google callback is refused.
- Google sign-in on the new domain needs the OAuth client to list
  `https://perintis.salmanitb.com/masuk/google` (redirect URI) and
  `https://perintis.salmanitb.com` (JavaScript origin).
