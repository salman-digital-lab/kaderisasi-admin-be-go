-- name: FormHasResponses :one
SELECT EXISTS(SELECT 1 FROM custom_form_responses WHERE form_id=$1);

-- name: FormHasAttachments :one
SELECT EXISTS(SELECT 1 FROM custom_form_attachments a JOIN custom_form_sessions s ON s.id=a.session_id WHERE s.form_id=$1);

-- name: ListFormResponses :many
SELECT * FROM custom_form_responses WHERE form_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3;

-- name: CountFormResponses :one
SELECT count(*) FROM custom_form_responses WHERE form_id=$1;

-- name: FormResponseByID :one
SELECT * FROM custom_form_responses WHERE form_id=$1 AND id=sqlc.arg(response_id)::uuid;

-- name: FormAttachmentByID :one
SELECT a.* FROM custom_form_attachments a JOIN custom_form_sessions s ON s.id=a.session_id
WHERE a.id=sqlc.arg(attachment_id)::uuid AND s.form_id=$1 AND a.claimed_at IS NOT NULL;

-- name: ResponseAttachments :many
SELECT * FROM custom_form_attachments WHERE session_id=$1 AND claimed_at IS NOT NULL ORDER BY created_at,id;

-- name: ExpiredFormAttachments :many
SELECT a.* FROM custom_form_attachments a JOIN custom_form_sessions s ON s.id=a.session_id
WHERE a.claimed_at IS NULL AND s.expires_at < now() ORDER BY a.created_at LIMIT 100;

-- name: DeleteExpiredFormAttachment :exec
DELETE FROM custom_form_attachments WHERE id=$1 AND claimed_at IS NULL;

-- name: LockCleanupFormSession :one
SELECT id FROM custom_form_sessions WHERE id=$1 FOR UPDATE;

-- name: LockExpiredFormAttachment :one
SELECT a.* FROM custom_form_attachments a JOIN custom_form_sessions s ON s.id=a.session_id
WHERE a.id=$1 AND a.session_id=$2 AND a.storage_key=$3 AND a.claimed_at IS NULL
  AND s.expires_at < clock_timestamp()
FOR UPDATE OF a;
