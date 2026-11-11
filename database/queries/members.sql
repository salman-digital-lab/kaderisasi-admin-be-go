-- name: PublicUserByID :one
SELECT * FROM public_users WHERE id=$1;

-- name: PublicUserByEmail :one
SELECT id FROM public_users WHERE email=$1;

-- name: OtherPublicUserByEmail :one
SELECT id FROM public_users WHERE email=$1 AND id<>$2;

-- name: PublicUserByMemberNumber :one
SELECT id FROM public_users WHERE member_id=$1;

-- name: CreateMemberUser :one
INSERT INTO public_users(email,password,account_status,created_at,updated_at)
VALUES ($1,$2,$3,now(),now()) RETURNING *;

-- name: SetMemberNumber :one
UPDATE public_users SET member_id=$2,updated_at=now() WHERE id=$1 RETURNING *;

-- name: ActivateMemberAccount :exec
UPDATE public_users SET email=$2,password=$3,account_status='active',updated_at=now() WHERE id=$1;

-- name: CreateMemberProfile :one
INSERT INTO profiles(user_id,name,gender,personal_id,whatsapp,instagram,tiktok,linkedin,line,birth_date,province_id,city_id,country,created_at,updated_at)
VALUES (@user_id,@name,sqlc.narg('gender'),sqlc.narg('personal_id'),sqlc.narg('whatsapp'),sqlc.narg('instagram'),sqlc.narg('tiktok'),sqlc.narg('linkedin'),sqlc.narg('line'),
CAST(CAST(sqlc.narg('birth_date') AS text) AS date),CAST(CAST(sqlc.narg('province_id') AS text) AS integer),CAST(CAST(sqlc.narg('city_id') AS text) AS integer),sqlc.narg('country'),now(),now())
RETURNING id,user_id,name,gender,personal_id,whatsapp,instagram,tiktok,linkedin,line,birth_date,province_id,city_id,country,created_at,updated_at;

-- name: UpdateMemberCredentials :one
UPDATE public_users SET email=COALESCE(sqlc.narg('email')::text,email),password=COALESCE(sqlc.narg('password')::text,password),updated_at=now()
WHERE id = @id::integer AND (email IS DISTINCT FROM COALESCE(sqlc.narg('email')::text,email) OR password IS DISTINCT FROM COALESCE(sqlc.narg('password')::text,password)) RETURNING *;

-- name: UpdateMemberPassword :exec
UPDATE public_users SET password=$2,updated_at=now() WHERE id=$1;

-- An admin email change replaces any pending self-service change.
-- name: CancelOpenEmailChanges :exec
UPDATE public_user_email_changes SET cancelled_at=now(), updated_at=now()
WHERE user_id=$1 AND used_at IS NULL AND cancelled_at IS NULL;

-- name: LockProfileByID :one
SELECT * FROM profiles WHERE id=$1 FOR UPDATE;

-- name: ProfileRefByUser :one
SELECT id,name FROM profiles WHERE user_id=$1 ORDER BY id LIMIT 1;

-- name: DeleteProfileReturning :one
DELETE FROM profiles WHERE id=CAST(CAST(@identifier AS text) AS integer) RETURNING *;

-- Member audit trail: insert-only, written in the same transaction as the change.
-- name: InsertMemberAuditLog :exec
INSERT INTO member_audit_logs(profile_id,user_id,member_name,action,changes,source,actor_admin_user_id,actor_name,request_id)
VALUES (sqlc.narg('profile_id'),sqlc.narg('user_id'),@member_name,@action,@changes,@source,sqlc.narg('actor_admin_user_id'),@actor_name,sqlc.narg('request_id'));

-- A member's trail is matched by profile and, for account changes, by user.
-- name: ListMemberAuditLogs :many
SELECT * FROM member_audit_logs
WHERE profile_id = @profile_id::integer OR user_id = sqlc.narg('user_id')::integer
ORDER BY id DESC LIMIT @row_limit OFFSET @row_offset;

-- name: CountMemberAuditLogs :one
SELECT count(*) FROM member_audit_logs
WHERE profile_id = @profile_id::integer OR user_id = sqlc.narg('user_id')::integer;
