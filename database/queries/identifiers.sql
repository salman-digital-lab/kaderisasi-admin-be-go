-- name: ProvinceByIdentifier :one
SELECT * FROM provinces WHERE id = CAST(CAST(@identifier AS text) AS integer);

-- name: CityByIdentifier :one
SELECT * FROM cities WHERE id = CAST(CAST(@identifier AS text) AS integer);

-- name: CitiesByProvinceIdentifier :many
SELECT * FROM cities WHERE province_id = CAST(CAST(@identifier AS text) AS integer);

-- name: UniversityByIdentifier :one
SELECT sqlc.embed(u), p.id AS parent_id, p.name AS parent_name, p.is_active AS parent_active
FROM universities u LEFT JOIN provinces p ON p.id = u.province_id WHERE u.id = CAST(CAST(@identifier AS text) AS integer);

-- name: ProfileByIdentifier :one
SELECT * FROM profiles WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: PublicUserByIdentifier :one
SELECT * FROM public_users WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: ActivityByIdentifier :one
SELECT * FROM activities WHERE id = CAST(CAST(@identifier AS text) AS integer);

-- name: LockActivityByIdentifier :one
SELECT * FROM activities WHERE id = CAST(CAST(@identifier AS text) AS integer) FOR UPDATE;

-- name: ActivityDetailsByIdentifier :one
SELECT sqlc.embed(a),to_jsonb(c) AS club FROM activities a LEFT JOIN clubs c ON c.id=a.club_id WHERE a.id=CAST(CAST(@identifier AS text) AS integer);

-- name: DeleteActivityByIdentifier :execrows
DELETE FROM activities WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: DeleteProfileByIdentifier :execrows
DELETE FROM profiles WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: FindAdminByIdentifier :one
SELECT * FROM admin_users WHERE id = CAST(CAST(@identifier AS text) AS integer);

-- name: LockAdminByIdentifier :one
SELECT * FROM admin_users WHERE id=CAST(CAST(@identifier AS text) AS integer) FOR UPDATE;

-- name: TicketByIdentifier :one
SELECT * FROM tickets WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: LockTicketByIdentifier :one
SELECT * FROM tickets WHERE id=CAST(CAST(@identifier AS text) AS integer) FOR UPDATE;

-- name: TicketDetailsByIdentifier :one
SELECT sqlc.embed(t), requester.display_name AS requester_name,requester.email AS requester_email,reviewer.display_name AS reviewer_name
FROM tickets t JOIN admin_users requester ON requester.id=t.requester_admin_user_id
LEFT JOIN admin_users reviewer ON reviewer.id=t.resolved_by_admin_user_id WHERE t.id=CAST(CAST(@identifier AS text) AS integer);

-- name: DeleteProvinceByIdentifier :execrows
DELETE FROM provinces WHERE id = CAST(CAST(@identifier AS text) AS integer);

-- name: DeleteCityByIdentifier :execrows
DELETE FROM cities WHERE id = CAST(CAST(@identifier AS text) AS integer);

-- name: DeleteUniversityByIdentifier :execrows
DELETE FROM universities WHERE id = CAST(CAST(@identifier AS text) AS integer);

-- name: ProfileDetailsByIdentifier :many
SELECT sqlc.embed(p), (to_jsonb(u)-'password')::jsonb AS public_user,
to_jsonb(province) AS province,to_jsonb(city) AS city,to_jsonb(university) AS university
FROM profiles p LEFT JOIN public_users u ON u.id=p.user_id
LEFT JOIN provinces province ON province.id=p.province_id
LEFT JOIN cities city ON city.id=p.city_id
LEFT JOIN universities university ON university.id=p.university_id
WHERE CASE WHEN @by_user::boolean THEN p.user_id = CAST(CAST(@id AS text) AS integer) ELSE p.id = CAST(CAST(@id AS text) AS integer) END;
