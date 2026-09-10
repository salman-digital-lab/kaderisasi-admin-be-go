-- name: CountCertificateTemplates :one
SELECT count(*) FROM certificate_templates WHERE (sqlc.narg('search')::text IS NULL OR name ILIKE '%'||sqlc.narg('search')::text||'%') AND (sqlc.narg('status')::text IS NULL OR lifecycle_status=sqlc.narg('status')::text);

-- name: ListCertificateTemplates :many
SELECT sqlc.embed(t),(SELECT count(*) FROM activities WHERE certificate_template_id=t.id) AS activity_usage_count,(SELECT count(*) FROM issued_certificates WHERE template_id=t.id) AS issued_certificate_count FROM certificate_templates t
WHERE (sqlc.narg('search')::text IS NULL OR t.name ILIKE '%'||sqlc.narg('search')::text||'%') AND (sqlc.narg('status')::text IS NULL OR t.lifecycle_status=sqlc.narg('status')::text)
ORDER BY t.created_at DESC LIMIT sqlc.narg('page_size')::bigint OFFSET @page_offset::bigint;

-- name: CertificateTemplateCounts :one
SELECT sqlc.embed(t),(SELECT count(*) FROM activities WHERE certificate_template_id=t.id) AS activity_usage_count,(SELECT count(*) FROM issued_certificates WHERE template_id=t.id) AS issued_certificate_count FROM certificate_templates t WHERE t.id=CAST(CAST(@identifier AS text) AS integer);

-- name: LockCertificateTemplate :one
SELECT * FROM certificate_templates WHERE id=CAST(CAST(@identifier AS text) AS integer) FOR UPDATE;

-- name: CreateCertificateTemplate :one
INSERT INTO certificate_templates(name,description,background_image,template_data,lifecycle_status,is_active,version,background_asset_version,published_at,archived_at,created_at,updated_at)
VALUES(@name::text,sqlc.narg('description')::text,NULL,@template_data::jsonb,@lifecycle_status::text,false,1,0,NULL,sqlc.narg('archived_at')::timestamptz,now(),now()) RETURNING *;

-- name: UpdateCertificateTemplate :one
UPDATE certificate_templates SET name = @name::text,description=sqlc.narg('description')::text,background_image=sqlc.narg('background_image')::text,template_data= @template_data::jsonb,lifecycle_status= @lifecycle_status::text,is_active=sqlc.narg('is_active')::boolean,
version=CAST(CAST(@version AS text) AS integer),background_asset_version=CAST(CAST(@background_asset_version AS text) AS integer),published_at=sqlc.narg('published_at')::timestamptz,archived_at=sqlc.narg('archived_at')::timestamptz,updated_at=now() WHERE id = @id::integer RETURNING *;

-- name: CompleteDuplicatedTemplate :one
UPDATE certificate_templates SET background_image=sqlc.narg('background_image')::text,template_data= @template_data::jsonb,updated_at=CASE WHEN ROW(background_image,template_data) IS DISTINCT FROM ROW(sqlc.narg('background_image')::text,@template_data::jsonb) THEN now() ELSE updated_at END WHERE id = @id::integer RETURNING *;

-- name: DeleteCertificateTemplate :exec
DELETE FROM certificate_templates WHERE id=$1;

-- name: ReplaceCertificateBackground :one
UPDATE certificate_templates SET background_image = @background_image::text,background_asset_version=CAST(CAST(@background_asset_version AS text) AS integer),version=CAST(CAST(@version AS text) AS integer),updated_at=now() WHERE id = @id::integer RETURNING *;
