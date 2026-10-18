# Certificate template lock

An activity's certificate template is chosen and then locked on the activity
detail page ("Sertifikat kegiatan" card). Issuance requires the lock.

## Rules

- Lock state lives in `activities.additional_config.certificate_template_lock`
  as `{ "locked_at": "<timestamp>" }`. No schema change. The public activity API
  returns `additional_config`, so the lock stores no admin data. The template
  itself stays in `activities.certificate_template_id`.
- The typed activity-update input never accepts this key, and other config
  writers (images, certificate settings) keep unknown keys, so saving an
  activity cannot drop the lock.
- An activity with an active (not withdrawn) issued certificate counts as
  locked even without the key, so activities published before this feature are
  protected without a backfill.
- Locking requires an assigned template that is published and ready.
  Locking again is a no-op that keeps the original `locked_at`.
- While locked, `PUT /v2/activities/:id` rejects a different
  `certificate_template_id` with `409 CERTIFICATE_TEMPLATE_LOCKED`. Resending
  the same template is allowed.
- Unlocking is refused with `409 CERTIFICATE_TEMPLATE_LOCK_IN_USE` while the
  activity has an active (not withdrawn) issued certificate.
- `prepare-issuance`, `issue-single`, `issue-bulk`, and approval requests
  return `422 CERTIFICATE_TEMPLATE_NOT_LOCKED` until the template is locked.
  Previews (`generate-single`, `generate`) do not require the lock.
- Lock, unlock, assignment, and issuance all take the activity row lock first,
  so an unlock cannot race a publication.

## Endpoints

| Method | Path | Permission |
| --- | --- | --- |
| GET | `/v2/certificates/activities/:activityId/template-lock` | `certificate.read` |
| PUT | `/v2/certificates/activities/:activityId/template-lock` | `certificate.template.manage` |
| DELETE | `/v2/certificates/activities/:activityId/template-lock` | `certificate.template.manage` |

All three return `{ template, locked, locked_at, published_count }`.

## Rollout

No migration. Deploy the Go admin API, then admin-fe. Activities with a
template but no published certificate start unlocked; an admin must lock them
before issuing.

Verification: `TestCertificateTemplateLock` in
`internal/httpapi/certificates_integration_test.go`.
