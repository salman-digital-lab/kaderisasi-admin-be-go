-- name: InsertIssuedCertificate :one
INSERT INTO issued_certificates (
 certificate_code, registration_id, activity_id, user_id, template_id,
 template_snapshot, participant_snapshot, activity_snapshot, snapshot_version,
 template_version, issued_by, issued_at, created_at, updated_at
) VALUES (
 sqlc.arg(code), sqlc.arg(registration_id), sqlc.arg(activity_id), sqlc.narg(user_id), sqlc.arg(template_id),
 sqlc.arg(template_snapshot), sqlc.arg(participant_snapshot), sqlc.arg(activity_snapshot), 1,
 sqlc.arg(template_version), sqlc.narg(issued_by), sqlc.arg(issued_at), sqlc.arg(issued_at), sqlc.arg(issued_at)
) ON CONFLICT (registration_id) DO NOTHING RETURNING id;

-- name: IssuedByRegistration :one
SELECT * FROM issued_certificates WHERE registration_id=$1;

-- name: IssuedByID :one
SELECT * FROM issued_certificates WHERE id=$1;

-- name: IssuedByCode :one
SELECT * FROM issued_certificates WHERE certificate_code=$1;

-- name: LockIssued :one
SELECT * FROM issued_certificates WHERE id=$1 FOR UPDATE;

-- name: RevokeIssued :one
UPDATE issued_certificates SET revoked_at=$2,revoked_reason=$3,revoked_by=$4,updated_at=$2 WHERE id=$1 RETURNING *;
