-- name: CertificateSigners :many
SELECT id, display_name, role_code, is_active FROM admin_users
WHERE is_active = true AND display_name IS NOT NULL ORDER BY display_name, id;

-- name: InsertCertificateApproval :one
INSERT INTO certificate_approvals (registration_id, activity_id, signer_id, requested_by, signer_name, signer_title, snapshot, content_hash, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now(),now()) RETURNING *;

-- name: PendingCertificateApproval :one
SELECT * FROM certificate_approvals WHERE registration_id=$1 AND status='pending';

-- name: CertificateApprovalByID :one
SELECT * FROM certificate_approvals WHERE id=$1;

-- name: LockCertificateApproval :one
SELECT * FROM certificate_approvals WHERE id=$1 FOR UPDATE;

-- name: ListCertificateApprovals :many
SELECT id, registration_id, activity_id, signer_id, requested_by, signer_name, signer_title, content_hash, status, decided_at, reason, certificate_id, created_at,
       snapshot->'participant'->>'name' AS participant_name,
       snapshot->'activity'->>'name' AS activity_name
FROM certificate_approvals
WHERE (signer_id=sqlc.arg(actor_id) OR requested_by=sqlc.arg(actor_id))
  AND (sqlc.narg(activity_id)::integer IS NULL OR activity_id=sqlc.narg(activity_id))
  AND (sqlc.narg(status)::text IS NULL OR status=sqlc.narg(status))
ORDER BY id DESC LIMIT sqlc.narg(page_size)::text::bigint OFFSET sqlc.arg(page_offset)::text::bigint;

-- name: CountCertificateApprovals :one
SELECT count(*) FROM certificate_approvals
WHERE (signer_id=sqlc.arg(actor_id) OR requested_by=sqlc.arg(actor_id))
  AND (sqlc.narg(activity_id)::integer IS NULL OR activity_id=sqlc.narg(activity_id))
  AND (sqlc.narg(status)::text IS NULL OR status=sqlc.narg(status));

-- name: DecideCertificateApproval :one
UPDATE certificate_approvals SET status=$2, decided_by=$3, decided_at=$4, reason=$5, certificate_id=$6, updated_at=$4
WHERE id=$1 AND status='pending' RETURNING *;

-- name: SetCertificateApprovalSnapshot :exec
UPDATE issued_certificates SET approval_snapshot=$2 WHERE id=$1 AND approval_snapshot IS NULL;
