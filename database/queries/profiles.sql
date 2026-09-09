-- name: CountProfilesFiltered :one
SELECT count(*) FROM profiles p LEFT JOIN public_users u ON u.id=p.user_id
WHERE ((@search::text <> '' AND p.name ILIKE '%' || @search::text || '%')
OR ((@search::text = '' OR u.email ILIKE '%' || @search::text || '%' OR u.member_id ILIKE '%' || @search::text || '%')
AND (@member_number::text = '' OR u.member_id = @member_number::text)
AND (@institution::text = '' OR EXISTS(SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(p.education_history)='array' THEN p.education_history ELSE '[]'::jsonb END) edu WHERE edu->>'institution' ILIKE '%' || @institution::text || '%'))))
AND (@badge::text = '' OR EXISTS(SELECT 1 FROM jsonb_array_elements_text(CASE WHEN jsonb_typeof(p.badges)='array' THEN p.badges WHEN jsonb_typeof(p.badges)='string' THEN jsonb_build_array(p.badges #>> '{}') ELSE '[]'::jsonb END) badge WHERE badge ILIKE '%' || @badge::text || '%'))
;

-- name: ListProfilesFiltered :many
SELECT sqlc.embed(p), (to_jsonb(u)-'password')::jsonb AS public_user FROM profiles p LEFT JOIN public_users u ON u.id=p.user_id
WHERE ((@search::text <> '' AND p.name ILIKE '%' || @search::text || '%')
OR ((@search::text = '' OR u.email ILIKE '%' || @search::text || '%' OR u.member_id ILIKE '%' || @search::text || '%')
AND (@member_number::text = '' OR u.member_id = @member_number::text)
AND (@institution::text = '' OR EXISTS(SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(p.education_history)='array' THEN p.education_history ELSE '[]'::jsonb END) edu WHERE edu->>'institution' ILIKE '%' || @institution::text || '%'))))
AND (@badge::text = '' OR EXISTS(SELECT 1 FROM jsonb_array_elements_text(CASE WHEN jsonb_typeof(p.badges)='array' THEN p.badges WHEN jsonb_typeof(p.badges)='string' THEN jsonb_build_array(p.badges #>> '{}') ELSE '[]'::jsonb END) badge WHERE badge ILIKE '%' || @badge::text || '%'))
ORDER BY p.name ASC LIMIT sqlc.narg('page_size')::bigint OFFSET @page_offset::bigint;

-- name: ProfileDetails :many
SELECT sqlc.embed(p), (to_jsonb(u)-'password')::jsonb AS public_user,
to_jsonb(province) AS province,to_jsonb(city) AS city,to_jsonb(university) AS university
FROM profiles p LEFT JOIN public_users u ON u.id=p.user_id
LEFT JOIN provinces province ON province.id=p.province_id
LEFT JOIN cities city ON city.id=p.city_id
LEFT JOIN universities university ON university.id=p.university_id
WHERE CASE WHEN @by_user::boolean THEN p.user_id = @id::integer ELSE p.id = @id::integer END;

-- name: ProfileByID :one
SELECT * FROM profiles WHERE id=$1;

-- name: DeleteProfile :execrows
DELETE FROM profiles WHERE id=$1;

-- name: UpdateMemberProfile :one
UPDATE profiles SET
name = COALESCE(sqlc.narg('name')::text,name),
gender = COALESCE(sqlc.narg('gender')::text,gender),
personal_id = COALESCE(sqlc.narg('personal_id')::text,personal_id),
whatsapp = COALESCE(sqlc.narg('whatsapp')::text,whatsapp),
line = COALESCE(sqlc.narg('line')::text,line),
instagram = COALESCE(sqlc.narg('instagram')::text,instagram),
tiktok = COALESCE(sqlc.narg('tiktok')::text,tiktok),
linkedin = COALESCE(sqlc.narg('linkedin')::text,linkedin),
province_id = COALESCE(CAST(CAST(sqlc.narg('province_id') AS text) AS integer),province_id),
city_id = COALESCE(CAST(CAST(sqlc.narg('city_id') AS text) AS integer),city_id),
level = COALESCE(CAST(CAST(sqlc.narg('level') AS text) AS integer),level),
birth_date = COALESCE(CAST(CAST(sqlc.narg('birth_date') AS text) AS date),birth_date),
origin_province_id = COALESCE(CAST(CAST(sqlc.narg('origin_province_id') AS text) AS integer),origin_province_id),
origin_city_id = COALESCE(CAST(CAST(sqlc.narg('origin_city_id') AS text) AS integer),origin_city_id),
country = COALESCE(sqlc.narg('country')::text,country),
badges = COALESCE(sqlc.narg('badges')::jsonb,badges),
education_history = COALESCE(sqlc.narg('education_history')::jsonb,education_history),
work_history = COALESCE(sqlc.narg('work_history')::jsonb,work_history),
extra_data = COALESCE(sqlc.narg('extra_data')::jsonb,extra_data),
updated_at=now()
WHERE id = @id::integer AND (sqlc.narg('birth_date')::text IS NOT NULL OR
ROW(name,gender,personal_id,whatsapp,line,instagram,tiktok,linkedin,province_id,city_id,level,birth_date,origin_province_id,origin_city_id,country,badges,education_history,work_history,extra_data) IS DISTINCT FROM
ROW(COALESCE(sqlc.narg('name')::text,name),COALESCE(sqlc.narg('gender')::text,gender),COALESCE(sqlc.narg('personal_id')::text,personal_id),COALESCE(sqlc.narg('whatsapp')::text,whatsapp),COALESCE(sqlc.narg('line')::text,line),COALESCE(sqlc.narg('instagram')::text,instagram),COALESCE(sqlc.narg('tiktok')::text,tiktok),COALESCE(sqlc.narg('linkedin')::text,linkedin),COALESCE(CAST(CAST(sqlc.narg('province_id') AS text) AS integer),province_id),COALESCE(CAST(CAST(sqlc.narg('city_id') AS text) AS integer),city_id),COALESCE(CAST(CAST(sqlc.narg('level') AS text) AS integer),level),COALESCE(CAST(CAST(sqlc.narg('birth_date') AS text) AS date),birth_date),COALESCE(CAST(CAST(sqlc.narg('origin_province_id') AS text) AS integer),origin_province_id),COALESCE(CAST(CAST(sqlc.narg('origin_city_id') AS text) AS integer),origin_city_id),COALESCE(sqlc.narg('country')::text,country),COALESCE(sqlc.narg('badges')::jsonb,badges),COALESCE(sqlc.narg('education_history')::jsonb,education_history),COALESCE(sqlc.narg('work_history')::jsonb,work_history),COALESCE(sqlc.narg('extra_data')::jsonb,extra_data))) RETURNING *;
