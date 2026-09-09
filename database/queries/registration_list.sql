-- name: CountRegistrationsFiltered :one
SELECT count(*) FROM activity_registrations ar LEFT JOIN public_users u ON u.id=ar.user_id LEFT JOIN profiles p ON p.user_id=ar.user_id
WHERE ar.activity_id = @activity_id::integer
AND (sqlc.narg('search')::text IS NULL OR p.name ILIKE '%'||sqlc.narg('search')||'%' OR u.email ILIKE '%'||sqlc.narg('search')||'%' OR ar.guest_data->>'name' ILIKE '%'||sqlc.narg('search')||'%' OR ar.guest_data->>'email' ILIKE '%'||sqlc.narg('search')||'%')
AND (sqlc.narg('status')::text IS NULL OR ar.status ILIKE '%'||sqlc.narg('status')||'%')
AND (sqlc.narg('university_id')::text IS NULL OR p.university_id=CAST(CAST(sqlc.narg('university_id') AS text) AS integer))
AND (sqlc.narg('province_id')::text IS NULL OR p.province_id=CAST(CAST(sqlc.narg('province_id') AS text) AS integer))
AND (sqlc.narg('intake_year')::text IS NULL OR p.intake_year=CAST(CAST(sqlc.narg('intake_year') AS text) AS integer));

-- name: ListRegistrationsFiltered :many
SELECT ar.id,u.id AS user_id,COALESCE(u.email,ar.guest_data->>'email') AS email,to_jsonb(COALESCE(p.name,ar.guest_data->>'name')) AS name_json,p.level,p.university_id,p.province_id,p.intake_year,p.major,COALESCE(p.gender,ar.guest_data->>'gender') AS gender,COALESCE(p.whatsapp,ar.guest_data->>'whatsapp') AS whatsapp,p.instagram,p.line,p.personal_id,p.education_history,ar.guest_data,ar.status,ar.created_at,to_jsonb(p) AS profile
FROM activity_registrations ar LEFT JOIN public_users u ON u.id=ar.user_id LEFT JOIN profiles p ON p.user_id=ar.user_id
WHERE ar.activity_id = @activity_id::integer
AND (sqlc.narg('search')::text IS NULL OR p.name ILIKE '%'||sqlc.narg('search')||'%' OR u.email ILIKE '%'||sqlc.narg('search')||'%' OR ar.guest_data->>'name' ILIKE '%'||sqlc.narg('search')||'%' OR ar.guest_data->>'email' ILIKE '%'||sqlc.narg('search')||'%')
AND (sqlc.narg('status')::text IS NULL OR ar.status ILIKE '%'||sqlc.narg('status')||'%')
AND (sqlc.narg('university_id')::text IS NULL OR p.university_id=CAST(CAST(sqlc.narg('university_id') AS text) AS integer))
AND (sqlc.narg('province_id')::text IS NULL OR p.province_id=CAST(CAST(sqlc.narg('province_id') AS text) AS integer))
AND (sqlc.narg('intake_year')::text IS NULL OR p.intake_year=CAST(CAST(sqlc.narg('intake_year') AS text) AS integer))
ORDER BY CASE WHEN @sort_by::text='created_at' AND @ascending::boolean=true THEN ar.created_at END ASC NULLS LAST,
CASE WHEN @sort_by::text='name' AND @ascending::boolean=true THEN p.name END ASC NULLS LAST,
CASE WHEN @sort_by::text='email' AND @ascending::boolean=true THEN u.email END ASC NULLS LAST,
CASE WHEN @sort_by::text='status' AND @ascending::boolean=true THEN ar.status END ASC NULLS LAST,
CASE WHEN @sort_by::text='level' AND @ascending::boolean=true THEN p.level END ASC NULLS LAST,
CASE WHEN @sort_by::text='university_id' AND @ascending::boolean=true THEN p.university_id END ASC NULLS LAST,
CASE WHEN @sort_by::text='province_id' AND @ascending::boolean=true THEN p.province_id END ASC NULLS LAST,
CASE WHEN @sort_by::text='intake_year' AND @ascending::boolean=true THEN p.intake_year END ASC NULLS LAST,
CASE WHEN @sort_by::text='major' AND @ascending::boolean=true THEN p.major END ASC NULLS LAST,
CASE WHEN @sort_by::text='whatsapp' AND @ascending::boolean=true THEN p.whatsapp END ASC NULLS LAST,
CASE WHEN @ascending::boolean=true THEN ar.id END ASC,
CASE WHEN @sort_by::text='created_at' AND @ascending::boolean=false THEN ar.created_at END DESC NULLS LAST,
CASE WHEN @sort_by::text='name' AND @ascending::boolean=false THEN p.name END DESC NULLS LAST,
CASE WHEN @sort_by::text='email' AND @ascending::boolean=false THEN u.email END DESC NULLS LAST,
CASE WHEN @sort_by::text='status' AND @ascending::boolean=false THEN ar.status END DESC NULLS LAST,
CASE WHEN @sort_by::text='level' AND @ascending::boolean=false THEN p.level END DESC NULLS LAST,
CASE WHEN @sort_by::text='university_id' AND @ascending::boolean=false THEN p.university_id END DESC NULLS LAST,
CASE WHEN @sort_by::text='province_id' AND @ascending::boolean=false THEN p.province_id END DESC NULLS LAST,
CASE WHEN @sort_by::text='intake_year' AND @ascending::boolean=false THEN p.intake_year END DESC NULLS LAST,
CASE WHEN @sort_by::text='major' AND @ascending::boolean=false THEN p.major END DESC NULLS LAST,
CASE WHEN @sort_by::text='whatsapp' AND @ascending::boolean=false THEN p.whatsapp END DESC NULLS LAST,
CASE WHEN @ascending::boolean=false THEN ar.id END DESC
LIMIT sqlc.narg('page_size')::bigint OFFSET @page_offset::bigint;
