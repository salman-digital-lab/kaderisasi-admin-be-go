# P3RI committee registration review

Members register for the P3RI 1448 H committee on p3ri.salmanitb.com
(`../p3ri-salmanitb`) through `kaderisasi-web-be-go`
(`../kaderisasi-web-be-go/docs/P3RI.md`). Admins review the registrations here
and in the "Pendaftar P3RI" page of `kaderisasi-admin-fe`.

This is a fast, temporary feature: one table and a hardcoded form.

## Data

`p3ri_registrations` (Ace migrations
`1793195171058_create_p3ri_registrations_table` and
`1793205707665_narrow_p3ri_registration_statuses`), one row per member:

- `status`: `terdaftar` or `diterima`. Applicants can edit only while
  `terdaftar` and before the deadline; moving back to `terdaftar` clears the
  accepted division and reopens editing.
- `choices`: 1–3 ordered division codes. `accepted_division` is set exactly when
  `diterima` and must be one of the choices (table CHECKs).
- `answers`: `{"common": {<question id>: …}, "divisions": {<code>: {<question id>: …}}}`.
- `files`: `[{field, key, name, mime, size}]`, private S3 objects under
  `p3ri/1448h/<userId>/`.
- `admin_note`, `status_updated_by`, `status_updated_at`: admin only; the
  applicant sees the status and accepted division, never the note.

## Form

`internal/p3ri/form_1448h.json` is a **byte-identical copy** of
`../kaderisasi-web-be-go/internal/p3ri/form_1448h.json`, which validates the
submissions. This copy supplies labels for `GET /v2/p3ri/form` and the export.
Change both copies together; `cmp` them before committing.

## Access

| Permission | Grants |
| --- | --- |
| `p3ri.registrations.read` | List, detail, form, files |
| `p3ri.registrations.manage` | Status changes |
| `p3ri.registrations.export` | xlsx export |

Role `p3ri` ("Panitia P3RI") has only these three permissions, no dashboard,
calendar or short links, so the admin frontend lands it on `/p3ri`. It is not
requestable; Super Admin assigns it in "Akun Admin". Super Admin holds the same
permissions. `TestP3riIsAssignedOnlyAndP3riOnly` and
`TestP3riRoutesAreImplementedAndGuarded` guard this.

## Routes (`internal/httpapi/p3ri.go`)

| Route | Notes |
| --- | --- |
| `GET /v2/p3ri/form` | The form JSON |
| `GET /v2/p3ri/registrations` | `status`, `division`, `choice_rank` (with `division`), `gender`, `search` (name, nickname, email, WhatsApp, institution), `page`, `per_page`; returns `{meta, data, counts}` with counts per status |
| `GET /v2/p3ri/registrations/export` | Same filters; xlsx with common answers and each choice's answers |
| `GET /v2/p3ri/registrations/:id` | Full registration |
| `PUT /v2/p3ri/registrations/:id/status` | `{status, accepted_division, admin_note}`; 422 for an unknown status or a division outside the choices |
| `GET /v2/p3ri/files?key=` | Streams a file referenced by a registration |

Responses are `{"message":"OK","data":…}`; failures are
`{"message":<Indonesian>}`. `p3ri_controller` reads its own JSON bodies
(`internal/httpapi/body.go`).

## Verification

```sh
make check
make test-unit
make fixtures && node scripts/test-go.mjs -run TestP3ri ./internal/httpapi/; make clean-fixtures
```
