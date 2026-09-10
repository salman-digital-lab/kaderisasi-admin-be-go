-- name: ClubRegistrationByIdentifier :one
SELECT * FROM club_registrations WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: ClubRegistrationUser :one
SELECT * FROM public_users WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: ClubRegistrationDuplicate :one
SELECT EXISTS(SELECT 1 FROM club_registrations WHERE club_id= @club_id::integer AND member_id= @member_id::integer);

-- name: CreateClubRegistration :one
INSERT INTO club_registrations(club_id,member_id,status,additional_data,created_at,updated_at)
VALUES(@club_id::integer,@member_id::integer,'PENDING',@additional_data::jsonb,now(),now()) RETURNING *;

-- name: UpdateClubRegistration :one
UPDATE club_registrations SET status= @status::text,additional_data= @additional_data::jsonb,
updated_at=CASE WHEN ROW(status,additional_data) IS DISTINCT FROM ROW(@status::text,@additional_data::jsonb) THEN now() ELSE updated_at END WHERE id= @id::integer RETURNING *;

-- name: LockClubRegistrations :many
SELECT * FROM club_registrations WHERE id=ANY(CAST(@identifiers::text[] AS integer[])) FOR UPDATE;

-- name: DeleteClubRegistration :exec
DELETE FROM club_registrations WHERE id=$1;

-- name: CountClubRegistrations :one
SELECT count(*) FROM club_registrations cr LEFT JOIN public_users u ON u.id=cr.member_id LEFT JOIN profiles p ON p.user_id=u.id
WHERE cr.club_id= @club_id::integer AND (sqlc.narg('status')::text IS NULL OR cr.status=sqlc.narg('status'))
AND (sqlc.narg('search')::text IS NULL OR u.email ILIKE '%'||sqlc.narg('search')::text||'%' OR p.name ILIKE '%'||sqlc.narg('search')::text||'%');

-- name: ListClubRegistrations :many
SELECT sqlc.embed(cr),(to_jsonb(u)-'password')::jsonb AS member,row_to_json(p) AS profile,row_to_json(c) AS club,
COALESCE((SELECT jsonb_agg(role ORDER BY role.sort_order ASC,role.is_primary DESC,CASE WHEN @member_order::boolean THEN role.created_at END ASC) FROM club_member_roles role WHERE role.club_registration_id=cr.id),'[]'::jsonb)::jsonb AS roles
FROM club_registrations cr LEFT JOIN public_users u ON u.id=cr.member_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN clubs c ON c.id=cr.club_id
WHERE cr.club_id= @club_id::integer AND (sqlc.narg('status')::text IS NULL OR cr.status=sqlc.narg('status'))
AND (sqlc.narg('search')::text IS NULL OR u.email ILIKE '%'||sqlc.narg('search')::text||'%' OR p.name ILIKE '%'||sqlc.narg('search')::text||'%')
ORDER BY CASE WHEN @ascending::boolean THEN cr.created_at END ASC NULLS LAST,CASE WHEN @ascending::boolean THEN cr.id END ASC,
CASE WHEN NOT @ascending::boolean THEN cr.created_at END DESC NULLS LAST,CASE WHEN NOT @ascending::boolean THEN cr.id END DESC
LIMIT CAST(sqlc.narg('page_size')::text AS bigint) OFFSET CAST(@page_offset::text AS bigint);

-- name: ClubRegistrationDetails :one
SELECT sqlc.embed(cr),(to_jsonb(u)-'password')::jsonb AS member,row_to_json(p) AS profile,row_to_json(c) AS club,
COALESCE((SELECT jsonb_agg(role ORDER BY role.sort_order ASC,role.is_primary DESC,CASE WHEN @member_order::boolean THEN role.created_at END ASC) FROM club_member_roles role WHERE role.club_registration_id=cr.id),'[]'::jsonb)::jsonb AS roles
FROM club_registrations cr LEFT JOIN public_users u ON u.id=cr.member_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN clubs c ON c.id=cr.club_id
WHERE cr.id= @id::integer;

-- name: ClubRegistrationExport :many
SELECT sqlc.embed(cr),(to_jsonb(u)-'password')::jsonb AS member,row_to_json(p) AS profile,province.name AS province,university.name AS university
FROM club_registrations cr LEFT JOIN public_users u ON u.id=cr.member_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN provinces province ON province.id=p.province_id LEFT JOIN universities university ON university.id=p.university_id
WHERE cr.club_id= @club_id::integer ORDER BY cr.created_at DESC;

-- name: ClubRoleByIdentifier :one
SELECT * FROM club_member_roles WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: ApprovedClubRoleRegistration :one
SELECT * FROM club_registrations WHERE id=CAST(CAST(@identifier AS text) AS integer) AND club_id= @club_id::integer AND status='APPROVED';

-- name: ClubRoleSuggestions :many
SELECT DISTINCT role_name FROM club_member_roles role JOIN club_registrations cr ON cr.id=role.club_registration_id WHERE cr.club_id= @club_id::integer ORDER BY role_name ASC;

-- name: ClubRoleList :many
SELECT sqlc.embed(role),sqlc.embed(cr),(to_jsonb(u)-'password')::jsonb AS member,row_to_json(p) AS profile FROM club_member_roles role JOIN club_registrations cr ON cr.id=role.club_registration_id LEFT JOIN public_users u ON u.id=cr.member_id LEFT JOIN profiles p ON p.user_id=u.id WHERE cr.club_id= @club_id::integer AND cr.status='APPROVED' ORDER BY role.sort_order ASC,role.is_primary DESC,role.created_at ASC;

-- name: ClearPrimaryClubRoles :exec
UPDATE club_member_roles SET is_primary=false WHERE club_registration_id= @registration_id::integer AND (sqlc.narg('except_id')::integer IS NULL OR id <> sqlc.narg('except_id'));

-- name: CreateClubRole :one
INSERT INTO club_member_roles(club_registration_id,role_name,start_date,end_date,is_primary,sort_order,created_at,updated_at)
VALUES(@registration_id::integer,@role_name::text,sqlc.narg('start_date')::date,sqlc.narg('end_date')::date,@is_primary::boolean,CAST(CAST(@sort_order AS text) AS integer),now(),now()) RETURNING *;

-- name: UpdateClubRole :one
UPDATE club_member_roles SET role_name= @role_name::text,start_date=sqlc.narg('start_date')::date,end_date=sqlc.narg('end_date')::date,is_primary= @is_primary::boolean,sort_order=CAST(CAST(@sort_order AS text) AS integer),
updated_at=CASE WHEN ROW(role_name,start_date,end_date,is_primary,sort_order) IS DISTINCT FROM ROW(@role_name::text,sqlc.narg('start_date')::date,sqlc.narg('end_date')::date,@is_primary::boolean,CAST(CAST(@sort_order AS text) AS integer)) THEN now() ELSE updated_at END WHERE id= @id::integer RETURNING *;

-- name: DeleteClubRole :exec
DELETE FROM club_member_roles WHERE id=$1;
