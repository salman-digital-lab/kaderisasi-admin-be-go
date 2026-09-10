-- name: CounselingByIdentifier :one
SELECT * FROM ruang_curhats WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: CountCounseling :one
SELECT count(*) FROM ruang_curhats rc LEFT JOIN public_users u ON u.id=rc.user_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN admin_users a ON a.id=rc.counselor_id
WHERE (sqlc.narg('status')::text IS NULL OR rc.status=CAST(CAST(sqlc.narg('status') AS text) AS integer))
AND (sqlc.narg('name')::text IS NULL OR p.name ILIKE '%'||sqlc.narg('name')::text||'%')
AND (sqlc.narg('gender')::text IS NULL OR p.gender=sqlc.narg('gender')::text)
AND (sqlc.narg('admin_name')::text IS NULL OR a.display_name ILIKE '%'||sqlc.narg('admin_name')::text||'%');

-- name: ListCounseling :many
SELECT sqlc.embed(rc),(to_jsonb(u)-'password')::jsonb AS public_user,row_to_json(p) AS profile,(to_jsonb(a)-'password')::jsonb AS admin_user
FROM ruang_curhats rc LEFT JOIN public_users u ON u.id=rc.user_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN admin_users a ON a.id=rc.counselor_id
WHERE (sqlc.narg('status')::text IS NULL OR rc.status=CAST(CAST(sqlc.narg('status') AS text) AS integer))
AND (sqlc.narg('name')::text IS NULL OR p.name ILIKE '%'||sqlc.narg('name')::text||'%')
AND (sqlc.narg('gender')::text IS NULL OR p.gender=sqlc.narg('gender')::text)
AND (sqlc.narg('admin_name')::text IS NULL OR a.display_name ILIKE '%'||sqlc.narg('admin_name')::text||'%')
ORDER BY rc.created_at DESC LIMIT sqlc.narg('page_size')::bigint OFFSET @page_offset::bigint;

-- name: CounselingDetails :one
SELECT sqlc.embed(rc),(to_jsonb(u)-'password')::jsonb AS public_user,row_to_json(p) AS profile,(to_jsonb(a)-'password')::jsonb AS admin_user
FROM ruang_curhats rc LEFT JOIN public_users u ON u.id=rc.user_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN admin_users a ON a.id=rc.counselor_id WHERE rc.id = @id::integer;

-- name: UpdateCounseling :one
UPDATE ruang_curhats SET counselor_id=CAST(CAST(sqlc.narg('counselor_id') AS text) AS integer),status=CAST(CAST(sqlc.narg('status') AS text) AS integer),additional_notes=sqlc.narg('additional_notes')::text,
updated_at=CASE WHEN ROW(counselor_id,status,additional_notes) IS DISTINCT FROM ROW(CAST(CAST(sqlc.narg('counselor_id') AS text) AS integer),CAST(CAST(sqlc.narg('status') AS text) AS integer),sqlc.narg('additional_notes')::text) THEN now() ELSE updated_at END WHERE id = @id::integer RETURNING *;
