-- name: TicketByID :one
SELECT * FROM tickets WHERE id=$1;

-- name: LockTicket :one
SELECT * FROM tickets WHERE id=$1 FOR UPDATE;

-- name: TicketDetails :one
SELECT sqlc.embed(t), requester.display_name AS requester_name,requester.email AS requester_email,reviewer.display_name AS reviewer_name
FROM tickets t JOIN admin_users requester ON requester.id=t.requester_admin_user_id
LEFT JOIN admin_users reviewer ON reviewer.id=t.resolved_by_admin_user_id WHERE t.id=$1;

-- name: OwnTickets :many
SELECT * FROM tickets WHERE requester_admin_user_id=$1 ORDER BY created_at DESC;

-- name: ReviewTickets :many
SELECT sqlc.embed(t), requester.display_name AS requester_name,requester.email AS requester_email,requester.id AS requester_id
FROM tickets t JOIN admin_users requester ON requester.id=t.requester_admin_user_id
WHERE (@status_filter::text = '' OR t.status = @status_filter::text) ORDER BY t.created_at DESC;

-- name: AcquireRequestLock :exec
SELECT pg_advisory_xact_lock(7412,$1);

-- name: OpenRoleRequest :one
SELECT id FROM tickets WHERE requester_admin_user_id=$1 AND requested_role_code=$2 AND status='open';

-- name: CreateTicket :one
INSERT INTO tickets(number,status,requester_admin_user_id,requested_role_code,reason,created_at,updated_at)
VALUES ($1,'open',$2,$3,$4,now(),now()) RETURNING id;

-- name: CancelTicket :exec
UPDATE tickets SET status='cancelled',cancelled_at=now(),updated_at=now() WHERE id=$1;

-- name: ResolveTicket :exec
UPDATE tickets SET status='resolved',resolution=$2,rejection_reason=$3,resolved_by_admin_user_id=$4,resolved_at=now(),updated_at=now() WHERE id=$1;
