-- name: ScoringRubric :one
SELECT * FROM activity_scoring_rubrics WHERE activity_id=$1;

-- name: SaveScoringRubric :exec
INSERT INTO activity_scoring_rubrics(activity_id,definition,revision,updated_by,updated_at)
VALUES ($1,$2,1,$3,now()) ON CONFLICT(activity_id) DO UPDATE SET definition=EXCLUDED.definition,revision=activity_scoring_rubrics.revision+1,updated_by=EXCLUDED.updated_by,updated_at=now();

-- name: LockScoringRubric :exec
UPDATE activity_scoring_rubrics SET locked_at=COALESCE(locked_at,now()) WHERE activity_id=$1;

-- name: ScoringRegistration :one
SELECT id,scoring_data FROM activity_registrations WHERE id=$1 AND activity_id=$2 FOR UPDATE;

-- name: SaveScoringData :exec
UPDATE activity_registrations SET scoring_data=$2,updated_at=now() WHERE id=$1;

-- name: AddScoringPublication :exec
INSERT INTO activity_scoring_publications(activity_id,registration_id,revision,action,snapshot,actor_id,created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7);

-- name: HasScoringPublications :one
SELECT EXISTS(SELECT 1 FROM activity_scoring_publications WHERE activity_id=$1);

-- name: RegistrationHasScoringPublications :one
SELECT EXISTS(SELECT 1 FROM activity_scoring_publications WHERE registration_id=$1);

-- name: CountScoringRegistrations :one
SELECT count(*) FROM activity_registrations r WHERE r.activity_id=$1
AND COALESCE((SELECT NULLIF(p.name,'') FROM profiles p WHERE p.user_id=r.user_id ORDER BY p.id LIMIT 1),NULLIF(r.guest_data->>'name',''),'Peserta') ILIKE '%' || sqlc.arg('search')::text || '%'
AND (sqlc.arg('state')::text='' OR COALESCE(r.scoring_data->>'state','unscored')=sqlc.arg('state')::text);

-- name: ListScoringRegistrations :many
SELECT r.id,COALESCE((SELECT NULLIF(p.name,'') FROM profiles p WHERE p.user_id=r.user_id ORDER BY p.id LIMIT 1),NULLIF(r.guest_data->>'name',''),'Peserta')::text AS name,r.scoring_data,
count(*) OVER() AS total
FROM activity_registrations r WHERE r.activity_id=$1
AND COALESCE((SELECT NULLIF(p.name,'') FROM profiles p WHERE p.user_id=r.user_id ORDER BY p.id LIMIT 1),NULLIF(r.guest_data->>'name',''),'Peserta') ILIKE '%' || sqlc.arg('search')::text || '%'
AND (sqlc.arg('state')::text='' OR COALESCE(r.scoring_data->>'state','unscored')=sqlc.arg('state')::text)
ORDER BY r.id LIMIT @page_size::integer OFFSET @page_offset::integer;
