-- name: CountActivitiesFiltered :one
SELECT count(*) FROM activities a WHERE a.name ILIKE '%' || @search::text || '%'
AND (sqlc.narg('category')::text IS NULL OR a.activity_category=CAST(CAST(sqlc.narg('category') AS text) AS integer))
AND (sqlc.narg('minimum_level')::text IS NULL OR a.minimum_level=CAST(CAST(sqlc.narg('minimum_level') AS text) AS integer))
AND (sqlc.narg('activity_type')::text IS NULL OR a.activity_type=CAST(CAST(sqlc.narg('activity_type') AS text) AS integer))
AND (sqlc.narg('is_published')::text IS NULL OR a.is_published=CAST(CAST(sqlc.narg('is_published') AS text) AS boolean))
AND (sqlc.narg('club_id')::text IS NULL OR a.club_id=CAST(CAST(sqlc.narg('club_id') AS text) AS integer));

-- name: ListActivitiesFiltered :many
SELECT a.id,a.name,a.activity_start,a.activity_end,a.registration_start,a.registration_end,a.selection_start,a.selection_end,a.activity_type,a.activity_category,a.club_id,a.is_published,a.is_registration_open,to_jsonb(c) AS club
FROM activities a LEFT JOIN clubs c ON c.id=a.club_id
WHERE a.name ILIKE '%' || @search::text || '%'
AND (sqlc.narg('category')::text IS NULL OR a.activity_category=CAST(CAST(sqlc.narg('category') AS text) AS integer))
AND (sqlc.narg('minimum_level')::text IS NULL OR a.minimum_level=CAST(CAST(sqlc.narg('minimum_level') AS text) AS integer))
AND (sqlc.narg('activity_type')::text IS NULL OR a.activity_type=CAST(CAST(sqlc.narg('activity_type') AS text) AS integer))
AND (sqlc.narg('is_published')::text IS NULL OR a.is_published=CAST(CAST(sqlc.narg('is_published') AS text) AS boolean))
AND (sqlc.narg('club_id')::text IS NULL OR a.club_id=CAST(CAST(sqlc.narg('club_id') AS text) AS integer))
ORDER BY a.is_published DESC,a.created_at DESC LIMIT sqlc.narg('page_size')::bigint OFFSET @page_offset::bigint;

-- name: ActivityDetails :one
SELECT sqlc.embed(a),to_jsonb(c) AS club FROM activities a LEFT JOIN clubs c ON c.id=a.club_id WHERE a.id=$1;

-- name: ActivitySlugExists :one
SELECT EXISTS(SELECT 1 FROM activities WHERE slug=$1);

-- name: CertificateTemplateByID :one
SELECT * FROM certificate_templates WHERE id=$1;

-- name: DeleteActivity :execrows
DELETE FROM activities WHERE id=$1;
