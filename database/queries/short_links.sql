-- name: ListShortLinks :many
SELECT * FROM urls
WHERE strpos(lower(id), lower(sqlc.arg(search)::text)) > 0
   OR strpos(lower(original_url), lower(sqlc.arg(search)::text)) > 0
ORDER BY created_at DESC, id ASC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountShortLinks :one
SELECT count(*) FROM urls
WHERE strpos(lower(id), lower(sqlc.arg(search)::text)) > 0
   OR strpos(lower(original_url), lower(sqlc.arg(search)::text)) > 0;

-- name: CreateShortLink :one
INSERT INTO urls (id, original_url) VALUES ($1, $2)
ON CONFLICT (id) DO NOTHING RETURNING *;

-- name: UpdateShortLink :one
UPDATE urls SET original_url = $2 WHERE id = $1 RETURNING *;

-- name: DeleteShortLink :execrows
DELETE FROM urls WHERE id = $1;
