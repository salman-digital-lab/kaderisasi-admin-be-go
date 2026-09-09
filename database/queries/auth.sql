-- name: FindAdminByID :one
SELECT * FROM admin_users WHERE id = $1;

-- name: FindAdminByEmail :one
SELECT * FROM admin_users WHERE normalized_email = $1;

-- name: AdminIdentityProviders :many
SELECT provider FROM admin_auth_identities WHERE admin_user_id = $1 ORDER BY id;

-- name: FindRefreshForUpdate :one
SELECT * FROM admin_refresh_tokens WHERE token_hash = $1 FOR UPDATE;

-- name: CreateRefresh :one
INSERT INTO admin_refresh_tokens
(family_id, admin_user_id, token_hash, parent_token_id, user_agent, ip_address, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now()) RETURNING *;

-- name: RotateRefresh :exec
UPDATE admin_refresh_tokens SET last_used_at = now(), revoked_at = now(),
revocation_reason = 'rotated', replaced_by_token_id = $2 WHERE id = $1;

-- name: RevokeFamily :exec
UPDATE admin_refresh_tokens SET revoked_at = now(), revocation_reason = 'refresh_token_reuse'
WHERE family_id = $1 AND revoked_at IS NULL;

-- name: RevokeRefresh :exec
UPDATE admin_refresh_tokens SET revoked_at = now(), revocation_reason = $2
WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: RevokeAdminSessions :exec
UPDATE admin_refresh_tokens SET revoked_at = now(), revocation_reason = $2
WHERE admin_user_id = $1 AND revoked_at IS NULL;

-- name: FindGoogleIdentity :one
SELECT * FROM admin_auth_identities WHERE provider = 'google' AND provider_subject = $1;

-- name: TouchIdentity :exec
UPDATE admin_auth_identities SET email = $2, last_used_at = now(), updated_at = now() WHERE id = $1;

-- name: CreateGoogleAdmin :one
INSERT INTO admin_users (email, normalized_email, display_name, is_active, created_at, updated_at)
VALUES ($1, $1, $2, true, now(), now()) RETURNING *;

-- name: CreateGoogleIdentity :exec
INSERT INTO admin_auth_identities (admin_user_id, provider, provider_subject, email, last_used_at, created_at, updated_at)
VALUES ($1, 'google', $2, $3, now(), now(), now());
