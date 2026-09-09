-- name: ListProvinces :many
SELECT * FROM provinces;

-- name: ProvinceByID :one
SELECT * FROM provinces WHERE id = $1;

-- name: CreateProvince :one
INSERT INTO provinces(name) VALUES ($1) RETURNING id, name;

-- name: UpdateProvince :one
UPDATE provinces SET name = $2 WHERE id = $1 RETURNING *;

-- name: DeleteProvince :execrows
DELETE FROM provinces WHERE id = $1;

-- name: ListCities :many
SELECT * FROM cities;

-- name: CitiesByProvince :many
SELECT * FROM cities WHERE province_id = $1;

-- name: CityByID :one
SELECT * FROM cities WHERE id = $1;

-- name: CreateCity :one
INSERT INTO cities(name, province_id) VALUES (@name, CAST(CAST(sqlc.narg('province_id') AS text) AS integer))
RETURNING id, name, province_id;

-- name: UpdateCity :one
UPDATE cities SET name = @name,
province_id = CASE WHEN @province_present::boolean THEN CAST(CAST(sqlc.narg('province_id') AS text) AS integer) ELSE province_id END
WHERE id = @id RETURNING *;

-- name: DeleteCity :execrows
DELETE FROM cities WHERE id = $1;

-- name: CountUniversities :one
SELECT count(*) FROM universities WHERE name ILIKE @search::text;

-- name: ListUniversities :many
SELECT sqlc.embed(u), p.id AS parent_id, p.name AS parent_name, p.is_active AS parent_active
FROM universities u LEFT JOIN provinces p ON p.id = u.province_id
WHERE u.name ILIKE @search::text ORDER BY u.name ASC LIMIT sqlc.narg('page_size')::bigint OFFSET @page_offset::bigint;

-- name: UniversityByID :one
SELECT sqlc.embed(u), p.id AS parent_id, p.name AS parent_name, p.is_active AS parent_active
FROM universities u LEFT JOIN provinces p ON p.id = u.province_id WHERE u.id = $1;

-- name: CreateUniversity :one
INSERT INTO universities(name, province_id) VALUES (@name, CAST(CAST(@province_id AS text) AS integer))
RETURNING id, name, province_id;

-- name: UpdateUniversity :one
UPDATE universities SET name = @name, province_id = CAST(CAST(@province_id AS text) AS integer) WHERE id = @id RETURNING *;

-- name: DeleteUniversity :execrows
DELETE FROM universities WHERE id = $1;

-- name: ListCountries :many
SELECT * FROM countries ORDER BY name ASC;

-- name: DashboardStats :one
SELECT (SELECT count(*) FROM profiles) AS "totalProfiles", (SELECT count(*) FROM activities) AS "totalActivities",
(SELECT count(*) FROM clubs) AS "totalClubs", (SELECT count(*) FROM ruang_curhats) AS "totalRuangCurhats";

-- name: CountProfilesByLevel :many
SELECT count(id)::text AS profile_amounts, level FROM profiles GROUP BY level;

-- name: CountProfilesByGender :many
SELECT count(id)::text AS profile_amounts, gender FROM profiles GROUP BY gender;
