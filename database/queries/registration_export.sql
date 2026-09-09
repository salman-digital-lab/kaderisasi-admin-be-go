-- name: RegistrationExportRows :many
SELECT * FROM activity_registrations WHERE activity_id=$1;

-- name: RegistrationExportRelations :many
SELECT ar.id,u.email,to_jsonb(p) AS profile,jsonb_build_object('province',province.name,'city',city.name,'origin_province',origin_province.name,'origin_city',origin_city.name,'university',university.name) AS locations
FROM activity_registrations ar LEFT JOIN public_users u ON u.id=ar.user_id
LEFT JOIN LATERAL (SELECT * FROM profiles WHERE user_id=u.id LIMIT 1) p ON true
LEFT JOIN provinces province ON province.id=p.province_id
LEFT JOIN cities city ON city.id=p.city_id
LEFT JOIN provinces origin_province ON origin_province.id=p.origin_province_id
LEFT JOIN cities origin_city ON origin_city.id=p.origin_city_id
LEFT JOIN universities university ON university.id=p.university_id
WHERE ar.id=ANY($1::integer[]);

-- name: RegistrationExportForm :one
SELECT form_schema FROM custom_forms WHERE feature_type='activity_registration' AND feature_id=$1 AND is_active=true LIMIT 1;

-- name: RegistrationExportProvinces :many
SELECT id,name FROM provinces WHERE id=ANY(CAST(CAST(@ids AS text[]) AS integer[]));

-- name: RegistrationExportCities :many
SELECT id,name FROM cities WHERE id=ANY(CAST(CAST(@ids AS text[]) AS integer[]));

-- name: RegistrationExportUniversities :many
SELECT id,name FROM universities WHERE id=ANY(CAST(CAST(@ids AS text[]) AS integer[]));
