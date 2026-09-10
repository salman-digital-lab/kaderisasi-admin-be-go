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
ORDER BY a.is_published DESC,a.created_at DESC LIMIT CAST(sqlc.narg('page_size')::text AS bigint) OFFSET CAST(@page_offset::text AS bigint);

-- name: ActivityDetails :one
SELECT sqlc.embed(a),to_jsonb(c) AS club FROM activities a LEFT JOIN clubs c ON c.id=a.club_id WHERE a.id=$1;

-- name: ActivitySlugExists :one
SELECT EXISTS(SELECT 1 FROM activities WHERE slug=$1);

-- name: CertificateTemplateByID :one
SELECT * FROM certificate_templates WHERE id=$1;

-- name: DeleteActivity :execrows
DELETE FROM activities WHERE id=$1;

-- name: CreateActivity :one
INSERT INTO activities(name,description,badge,activity_start,activity_end,registration_start,registration_end,selection_start,selection_end,minimum_level,activity_type,activity_category,is_published,is_registration_open,slug,club_id,certificate_template_id,additional_config,created_at,updated_at)
VALUES (sqlc.narg('name')::text,sqlc.narg('description')::text,sqlc.narg('badge')::text,CAST(CAST(sqlc.narg('activity_start') AS text) AS date),CAST(CAST(sqlc.narg('activity_end') AS text) AS date),CAST(CAST(sqlc.narg('registration_start') AS text) AS date),CAST(CAST(sqlc.narg('registration_end') AS text) AS date),CAST(CAST(sqlc.narg('selection_start') AS text) AS date),CAST(CAST(sqlc.narg('selection_end') AS text) AS date),COALESCE(CAST(CAST(sqlc.narg('minimum_level') AS text) AS integer),0),CAST(CAST(sqlc.narg('activity_type') AS text) AS integer),CAST(CAST(sqlc.narg('activity_category') AS text) AS integer),COALESCE(CAST(CAST(sqlc.narg('is_published') AS text) AS boolean),false),COALESCE(sqlc.narg('is_registration_open')::boolean,true),@slug::text,CAST(CAST(sqlc.narg('club_id') AS text) AS integer),CAST(CAST(sqlc.narg('certificate_template_id') AS text) AS integer),COALESCE(sqlc.narg('additional_config')::jsonb,'{"images":[],"mandatory_profile_data":[],"custom_selection_status":[],"additional_questionnaire":[]}'::jsonb),now(),now()) RETURNING *;

-- name: UpdateActivity :one
UPDATE activities SET
name = COALESCE(sqlc.narg('name')::text,name),
description = COALESCE(sqlc.narg('description')::text,description),
badge = COALESCE(sqlc.narg('badge')::text,badge),
activity_start = COALESCE(CAST(CAST(sqlc.narg('activity_start') AS text) AS date),activity_start),
activity_end = COALESCE(CAST(CAST(sqlc.narg('activity_end') AS text) AS date),activity_end),
registration_start = COALESCE(CAST(CAST(sqlc.narg('registration_start') AS text) AS date),registration_start),
registration_end = COALESCE(CAST(CAST(sqlc.narg('registration_end') AS text) AS date),registration_end),
selection_start = COALESCE(CAST(CAST(sqlc.narg('selection_start') AS text) AS date),selection_start),
selection_end = COALESCE(CAST(CAST(sqlc.narg('selection_end') AS text) AS date),selection_end),
minimum_level = COALESCE(CAST(CAST(sqlc.narg('minimum_level') AS text) AS integer),minimum_level),
activity_type = COALESCE(CAST(CAST(sqlc.narg('activity_type') AS text) AS integer),activity_type),
activity_category = COALESCE(CAST(CAST(sqlc.narg('activity_category') AS text) AS integer),activity_category),
is_published = COALESCE(CAST(CAST(sqlc.narg('is_published') AS text) AS boolean),is_published),
is_registration_open = COALESCE(sqlc.narg('is_registration_open')::boolean,is_registration_open),
club_id=CASE WHEN @club_present::boolean THEN CAST(CAST(sqlc.narg('club_id') AS text) AS integer) ELSE club_id END,
certificate_template_id=CASE WHEN @template_present::boolean THEN CAST(CAST(sqlc.narg('certificate_template_id') AS text) AS integer) ELSE certificate_template_id END,
additional_config = @additional_config::jsonb,updated_at=now() WHERE id = @id::integer RETURNING *;

-- name: CertificateTemplateByIdentifier :one
SELECT * FROM certificate_templates WHERE id=CAST(CAST(@identifier AS text) AS integer);
