-- name: ActivityByID :one
SELECT * FROM activities WHERE id = $1;

-- name: LockActivity :one
SELECT * FROM activities WHERE id = $1 FOR UPDATE;

-- name: SetActivityConfig :exec
UPDATE activities SET additional_config = $2, updated_at = now() WHERE id = $1;


-- name: UpdateActivityConfig :one
UPDATE activities SET additional_config=$2,updated_at=now()
WHERE id=$1 AND additional_config IS DISTINCT FROM $2 RETURNING *;
