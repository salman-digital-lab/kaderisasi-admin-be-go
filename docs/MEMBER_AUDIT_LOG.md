# Member audit log

Every admin write to member data leaves one row in `member_audit_logs`
(Ace migration `1794372885000_create_member_audit_logs_table`). The row is
inserted in the **same transaction** as the change, so a change cannot land
without its entry, and a failed change leaves none. Changes that members make to
their own profile on the public website (`kaderisasi-web-be-go`) are **not**
recorded.

## What is recorded

| Action | Written by | `changes` |
| --- | --- | --- |
| `created` | `POST /v2/members` | Starting values (non-empty fields), `email`, `member_id`, `account_status` |
| `profile_updated` | `PUT /v2/profiles/:id`, `PUT /v2/perintis/participants/:id/profile` | Fields whose value changed |
| `regional_assignment_updated` | `PUT /v2/profiles/:id/regional-assignment` | `extra_data.alumni_regional_assignment` |
| `credentials_updated` | `PUT /v2/profiles/auth/:id`, or a `password` sent to `PUT /v2/profiles/:id` | `email`, `password` |
| `account_generated` | `POST /v2/members/:id/generate-account` | `email`, `account_status`, `password` |
| `deleted` | `DELETE /v2/profiles/:id` | The removed profile's non-empty fields |

`changes` is a JSON object keyed by field: `{"name":{"before":"A","after":"B"}}`.
Keys of `extra_data` are reported one by one as `extra_data.<key>`. A password
is only ever `{"password":{"changed":true}}`; no password or hash is stored.
A save that changes nothing writes no row. JSON values are compared by meaning,
so key order and spacing are not changes.

Each row also keeps the admin (`actor_admin_user_id`, `actor_name`), the member
(`profile_id`, `user_id`, `member_name`), `source` (`admin`, or `perintis` for
the Perintis participant profile screen), `request_id` and `created_at`. The ids
carry no foreign keys and the names are snapshots, so the trail survives a
deleted member or admin account. Entries hold earlier values of personal data
(NIK, WhatsApp, address); there is no retention limit.

The table is insert-only: the API has no update or delete route for it and the
service only inserts (`internal/member/audit.go`).

## Reading

`GET /v2/profiles/:id/audit-logs?page=&per_page=` (default 20, max 100), newest
first, permission **`members.audit.read`** (Super Admin and Asisten Manager
Program only, narrower than `members.read`). `:id` is the profile id; entries
recorded against the member's account (`user_id`) are included. The trail of a
deleted profile stays readable by its former id.

```json
{"message":"OK","data":{"meta":{"total":1,"per_page":20,"current_page":1},
 "data":[{"id":1,"action":"profile_updated","changes":{"name":{"before":"A","after":"B"}},
          "source":"admin","actor_admin_user_id":3,"actor_name":"Nama Admin",
          "created_at":"2026-11-11T10:00:00.000+07:00"}]}}
```

Failures: 404 for a non-numeric id, 422 for an invalid page.

## Code

- `internal/member/audit.go`: `Actor`, the field diff, `record`, `AuditLogs`.
- `internal/member/service.go`, `update.go`: each write takes an `Actor` and
  records inside its transaction. `saveProfile` locks the row first so the
  recorded "before" values are the ones replaced.
- `internal/httpapi/members.go`: `memberActor` and the read route.

## Verification

- `go test ./internal/member/ ./internal/auth/ ./internal/httpapi/`
  (`TestProfileDiff…`, `TestMemberAuditTrailIsLimitedToProgramAdmins`,
  `TestMemberAuditRouteIsReadOnlyAndGuarded`).
- `make fixtures && node scripts/test-go.mjs ./internal/httpapi -run TestMemberAuditTrail`
  covers every action, the no-secret check, pagination, permissions and the
  trail of a deleted member in an owned schema.

## Rollout

Ace migration first, then the Go admin API, then the admin frontend. Before the
migration the write routes would fail (the insert is part of each write), so
never deploy the API ahead of the migration.
