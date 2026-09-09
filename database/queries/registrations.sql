-- name: ProfileForRegistration :one
SELECT * FROM profiles WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: CountMemberRegistrations :one
SELECT count(*) FROM activity_registrations WHERE user_id IS NOT DISTINCT FROM sqlc.narg('user_id')::integer AND activity_id = @activity_id::integer;

-- name: CreateActivityRegistration :one
INSERT INTO activity_registrations(user_id,activity_id,status,questionnaire_answer,created_at,updated_at)
VALUES ($1,$2,'TERDAFTAR',$3,now(),now()) RETURNING *;

-- name: ActivityRegistrationByID :one
SELECT * FROM activity_registrations WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: RegistrationsByUser :many
SELECT sqlc.embed(ar),to_jsonb(a) AS activity FROM activity_registrations ar LEFT JOIN activities a ON a.id=ar.activity_id WHERE ar.user_id=CAST(CAST(@identifier AS text) AS integer);

-- name: RegistrationStatusCounts :many
SELECT status,count(*) AS count FROM activity_registrations WHERE activity_id=CAST(CAST(@identifier AS text) AS integer) GROUP BY status;

-- name: ActivityForRegistration :one
SELECT a.* FROM activities a JOIN activity_registrations ar ON a.id=ar.activity_id WHERE ar.id=CAST(CAST(@identifier AS text) AS integer);

-- name: RegistrationPublicUsersByEmail :many
SELECT id FROM public_users WHERE email=ANY($1::text[]);

-- name: RegistrationsByActivityUsers :many
SELECT id FROM activity_registrations WHERE activity_id=$1 AND user_id=ANY($2::integer[]);

-- name: UsersForRegistrationIDs :many
SELECT user_id FROM activity_registrations WHERE id=ANY(CAST(CAST(@ids AS text[]) AS integer[]));

-- name: ChangeRegistrationStatusByIDs :execrows
UPDATE activity_registrations SET status = @status::text WHERE id=ANY(CAST(CAST(@ids AS text[]) AS integer[]));

-- name: ChangeRegistrationStatusByUsers :execrows
UPDATE activity_registrations SET status=$1 WHERE activity_id=$2 AND user_id=ANY($3::integer[]);

-- name: ChangeRegistrationStatusBulk :execrows
UPDATE activity_registrations SET status = @new_status::text WHERE activity_id = CAST(CAST(@activity_id AS text) AS integer) AND (sqlc.narg('current_status')::text IS NULL OR status=sqlc.narg('current_status')::text);

-- name: SetRegistrationProfileLevel :exec
UPDATE profiles SET level=$1 WHERE user_id=ANY($2::integer[]);

-- name: RegistrationProfilesByUsers :many
SELECT * FROM profiles WHERE user_id=ANY($1::integer[]);

-- name: SetRegistrationProfileBadges :exec
UPDATE profiles SET badges=$2,updated_at=now() WHERE id=$1;

-- name: RegistrationHasCertificate :one
SELECT EXISTS(SELECT 1 FROM issued_certificates WHERE registration_id=$1);

-- name: DeleteActivityRegistration :execrows
DELETE FROM activity_registrations WHERE id=$1;

-- name: RegistrationTotal :one
SELECT count(*) FROM activity_registrations WHERE activity_id=CAST(CAST(@identifier AS text) AS integer);

-- name: RegistrationActivityByIdentifier :one
SELECT * FROM activities WHERE id=CAST(CAST(@identifier AS text) AS integer);
