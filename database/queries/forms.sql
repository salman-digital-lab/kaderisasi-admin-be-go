-- name: CountFormsFiltered :one
SELECT count(*) FROM custom_forms WHERE (NOT @unattached::boolean OR feature_id IS NULL)
AND (sqlc.narg('search')::text IS NULL OR form_name ILIKE '%' || sqlc.narg('search')::text || '%')
AND (sqlc.narg('feature_type')::text IS NULL OR feature_type=sqlc.narg('feature_type'))
AND (sqlc.narg('feature_id')::text IS NULL OR feature_id=CAST(CAST(sqlc.narg('feature_id') AS text) AS integer))
AND (sqlc.narg('is_active')::boolean IS NULL OR is_active=sqlc.narg('is_active'));

-- name: ListFormsFiltered :many
SELECT * FROM custom_forms WHERE (NOT @unattached::boolean OR feature_id IS NULL)
AND (sqlc.narg('search')::text IS NULL OR form_name ILIKE '%' || sqlc.narg('search')::text || '%')
AND (sqlc.narg('feature_type')::text IS NULL OR feature_type=sqlc.narg('feature_type'))
AND (sqlc.narg('feature_id')::text IS NULL OR feature_id=CAST(CAST(sqlc.narg('feature_id') AS text) AS integer))
AND (sqlc.narg('is_active')::boolean IS NULL OR is_active=sqlc.narg('is_active'))
ORDER BY created_at DESC LIMIT CAST(sqlc.narg('page_size')::text AS bigint) OFFSET CAST(@page_offset::text AS bigint);

-- name: FormByIdentifier :one
SELECT * FROM custom_forms WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: LockFormByIdentifier :one
SELECT * FROM custom_forms WHERE id=CAST(CAST(@identifier AS text) AS integer) FOR UPDATE;

-- name: ActiveFormByFeature :one
SELECT * FROM custom_forms WHERE feature_type = @feature_type::text AND feature_id=CAST(CAST(@identifier AS text) AS integer) AND is_active=true ORDER BY updated_at DESC,id DESC LIMIT 1;

-- name: FormAvailableActivities :many
SELECT id,name FROM activities WHERE id NOT IN (SELECT feature_id FROM custom_forms WHERE feature_type='activity_registration' AND feature_id IS NOT NULL AND (sqlc.narg('current_form_id')::text IS NULL OR id<>CAST(CAST(sqlc.narg('current_form_id') AS text) AS integer))) ORDER BY name ASC;

-- name: FormAvailableClubs :many
SELECT id,name FROM clubs WHERE id NOT IN (SELECT feature_id FROM custom_forms WHERE feature_type='club_registration' AND feature_id IS NOT NULL AND (sqlc.narg('current_form_id')::text IS NULL OR id<>CAST(CAST(sqlc.narg('current_form_id') AS text) AS integer))) ORDER BY name ASC;

-- name: LockFormClubs :many
SELECT * FROM clubs WHERE id=ANY(CAST(@identifiers::text[] AS integer[])) ORDER BY id ASC FOR UPDATE;

-- name: OtherClubFormExists :one
SELECT EXISTS(SELECT 1 FROM custom_forms WHERE feature_type='club_registration' AND feature_id=CAST(CAST(@identifier AS text) AS integer) AND id <> @form_id::integer);

-- name: CreateForm :one
INSERT INTO custom_forms(form_name,form_description,post_submission_info,feature_type,feature_id,form_schema,is_active,created_at,updated_at)
VALUES (@form_name::text,sqlc.narg('form_description')::text,sqlc.narg('post_submission_info')::text,sqlc.narg('feature_type')::text,CAST(CAST(sqlc.narg('feature_id') AS text) AS integer),COALESCE(sqlc.narg('form_schema')::jsonb,'{}'::jsonb),COALESCE(sqlc.narg('is_active')::boolean,true),now(),now()) RETURNING *;

-- name: UpdateForm :one
UPDATE custom_forms SET form_name = @form_name::text,form_description=sqlc.narg('form_description')::text,post_submission_info=sqlc.narg('post_submission_info')::text,feature_type=sqlc.narg('feature_type')::text,feature_id=CAST(CAST(sqlc.narg('feature_id') AS text) AS integer),form_schema=sqlc.narg('form_schema')::jsonb,is_active=sqlc.narg('is_active')::boolean,
updated_at=CASE WHEN ROW(form_name,form_description,post_submission_info,feature_type,feature_id,form_schema,is_active) IS DISTINCT FROM ROW(@form_name::text,sqlc.narg('form_description')::text,sqlc.narg('post_submission_info')::text,sqlc.narg('feature_type')::text,CAST(CAST(sqlc.narg('feature_id') AS text) AS integer),sqlc.narg('form_schema')::jsonb,sqlc.narg('is_active')::boolean) THEN now() ELSE updated_at END WHERE id = @id::integer RETURNING *;

-- name: DeleteForm :exec
DELETE FROM custom_forms WHERE id=$1;

-- name: ToggleFormActive :one
UPDATE custom_forms SET is_active = @is_active::boolean,updated_at=now() WHERE id = @id::integer RETURNING *;

-- name: AttachFormToActivity :one
UPDATE custom_forms SET feature_type='activity_registration',feature_id=CAST(CAST(@identifier AS text) AS integer),updated_at=CASE WHEN feature_type IS DISTINCT FROM 'activity_registration' OR feature_id IS DISTINCT FROM CAST(CAST(@identifier AS text) AS integer) THEN now() ELSE updated_at END WHERE id = @id::integer RETURNING *;

-- name: DetachFormFromActivity :one
UPDATE custom_forms SET feature_id=null,updated_at=now() WHERE id=$1 RETURNING *;
