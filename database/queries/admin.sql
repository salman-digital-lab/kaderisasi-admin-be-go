-- name: CountAdmins :one
SELECT count(*) FROM admin_users WHERE (email ILIKE @search::text OR display_name ILIKE @search::text)
AND (sqlc.narg('role_filter')::text IS NULL OR COALESCE(role_code,'unassigned')=sqlc.narg('role_filter')::text)
AND (sqlc.narg('active_filter')::boolean IS NULL OR is_active=sqlc.narg('active_filter')::boolean);

-- name: ListAdmins :many
SELECT * FROM admin_users WHERE (email ILIKE @search::text OR display_name ILIKE @search::text)
AND (sqlc.narg('role_filter')::text IS NULL OR COALESCE(role_code,'unassigned')=sqlc.narg('role_filter')::text)
AND (sqlc.narg('active_filter')::boolean IS NULL OR is_active=sqlc.narg('active_filter')::boolean)
ORDER BY created_at DESC LIMIT CAST(sqlc.narg('page_size')::text AS bigint) OFFSET CAST(@page_offset::text AS bigint);

-- name: AdminIdentities :many
SELECT provider,email,last_used_at,created_at FROM admin_auth_identities WHERE admin_user_id=$1 ORDER BY id;

-- name: CreateAdmin :one
INSERT INTO admin_users(email,normalized_email,password,display_name,is_active,role_code,created_at,updated_at)
VALUES (@email,@email,@password,@display_name,true,sqlc.narg('role_code'),now(),now()) RETURNING *;

-- name: SetAdminPassword :exec
UPDATE admin_users SET password=$2,updated_at=now() WHERE id=$1;

-- name: SetAdminDisplayName :one
UPDATE admin_users SET display_name=$2,updated_at=now() WHERE id=$1 RETURNING *;

-- name: LockAdmin :one
SELECT * FROM admin_users WHERE id=$1 FOR UPDATE;

-- name: CountActiveSuperAdmins :one
SELECT count(*) FROM admin_users WHERE is_active=true AND role_code='super_admin';

-- name: UpdateAdminAccess :exec
UPDATE admin_users SET
role_code=CASE WHEN @role_present::boolean THEN sqlc.narg('role_code')::text ELSE role_code END,
is_active=CASE WHEN @active_present::boolean THEN @is_active::boolean ELSE is_active END,
updated_at=now() WHERE id = @id;

-- name: AcquireRBACLock :exec
SELECT pg_advisory_xact_lock(7411,1);
