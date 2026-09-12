-- name: CountClubsFiltered :one
SELECT count(*) FROM clubs WHERE name ILIKE '%' || @search::text || '%'
AND (sqlc.narg('club_type')::text IS NULL OR club_type=sqlc.narg('club_type'))
AND (sqlc.narg('is_show')::boolean IS NULL OR is_show=sqlc.narg('is_show'))
AND (sqlc.narg('is_registration_open')::boolean IS NULL OR is_registration_open=sqlc.narg('is_registration_open'));

-- name: ListClubsFiltered :many
SELECT id,name,club_type,description,short_description,logo,created_at,updated_at,start_period,end_period,is_show,is_registration_open,registration_end_date FROM clubs
WHERE name ILIKE '%' || @search::text || '%'
AND (sqlc.narg('club_type')::text IS NULL OR club_type=sqlc.narg('club_type'))
AND (sqlc.narg('is_show')::boolean IS NULL OR is_show=sqlc.narg('is_show'))
AND (sqlc.narg('is_registration_open')::boolean IS NULL OR is_registration_open=sqlc.narg('is_registration_open'))
ORDER BY is_show DESC,created_at DESC LIMIT CAST(sqlc.narg('page_size')::text AS bigint) OFFSET CAST(@page_offset::text AS bigint);

-- name: ClubByIdentifier :one
SELECT * FROM clubs WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: LockClubByIdentifier :one
SELECT * FROM clubs WHERE id=CAST(CAST(@identifier AS text) AS integer) FOR UPDATE;

-- name: DeleteClub :exec
DELETE FROM clubs WHERE id=$1;

-- name: LatestClubForm :one
SELECT * FROM custom_forms WHERE feature_type='club_registration' AND feature_id=$1 ORDER BY updated_at DESC,id DESC LIMIT 1;

-- name: ClubHasActiveForm :one
SELECT EXISTS(SELECT 1 FROM custom_forms WHERE feature_type='club_registration' AND feature_id=$1 AND is_active=true);

-- name: CreateClub :one
INSERT INTO clubs(name,club_type,description,short_description,media,start_period,end_period,is_show,is_registration_open,registration_end_date,created_at,updated_at)
VALUES (@name::text,@club_type::text,sqlc.narg('description')::text,sqlc.narg('short_description')::text,@media::jsonb,sqlc.narg('start_period')::date,sqlc.narg('end_period')::date,false,false,sqlc.narg('registration_end_date')::date,now(),now()) RETURNING *;

-- name: UpdateClub :one
UPDATE clubs SET name= @name::text,club_type= @club_type::text,description=sqlc.narg('description')::text,short_description=sqlc.narg('short_description')::text,media=sqlc.narg('media')::jsonb,start_period=sqlc.narg('start_period')::date,end_period=sqlc.narg('end_period')::date,is_show=sqlc.narg('is_show')::boolean,is_registration_open=sqlc.narg('is_registration_open')::boolean,registration_end_date=sqlc.narg('registration_end_date')::date,
updated_at=CASE WHEN ROW(name,club_type,description,short_description,media,start_period,end_period,is_show,is_registration_open,registration_end_date) IS DISTINCT FROM ROW(@name::text,@club_type::text,sqlc.narg('description')::text,sqlc.narg('short_description')::text,sqlc.narg('media')::jsonb,sqlc.narg('start_period')::date,sqlc.narg('end_period')::date,sqlc.narg('is_show')::boolean,sqlc.narg('is_registration_open')::boolean,sqlc.narg('registration_end_date')::date) THEN now() ELSE updated_at END
WHERE id= @id::integer RETURNING *;

-- name: UpdateClubRegistrationInfo :one
UPDATE clubs SET registration_info= @registration_info::jsonb,updated_at=CASE WHEN registration_info IS DISTINCT FROM @registration_info::jsonb THEN now() ELSE updated_at END WHERE id= @id::integer RETURNING *;

-- name: UpdateClubMedia :one
UPDATE clubs SET media= @media::jsonb,updated_at=CASE WHEN media IS DISTINCT FROM @media::jsonb THEN now() ELSE updated_at END WHERE id= @id::integer RETURNING *;

-- name: UpdateClubLogo :one
UPDATE clubs SET logo= @logo::text,updated_at=CASE WHEN logo IS DISTINCT FROM @logo::text THEN now() ELSE updated_at END WHERE id= @id::integer RETURNING *;
