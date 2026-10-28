-- P3RI committee registration review. The form is hardcoded in
-- internal/p3ri/form_1448h.json; answers are JSON keyed by question id
-- ({"common": {...}, "divisions": {<code>: {...}}}).

-- name: ListP3riRegistrations :many
SELECT sqlc.embed(r), u.email
FROM p3ri_registrations r JOIN public_users u ON u.id = r.user_id
WHERE (sqlc.narg('status')::text IS NULL OR r.status = sqlc.narg('status')::text)
AND (sqlc.narg('division')::text IS NULL OR (
  CASE WHEN sqlc.narg('choice_rank')::integer IS NULL
    THEN sqlc.narg('division')::text = ANY (r.choices)
    ELSE r.choices[sqlc.narg('choice_rank')::integer] = sqlc.narg('division')::text END))
AND (sqlc.narg('gender')::text IS NULL OR r.answers->'common'->>'jenis_kelamin' = sqlc.narg('gender')::text)
AND (sqlc.narg('search')::text IS NULL OR concat_ws(' ', u.email,
  r.answers->'common'->>'nama_lengkap', r.answers->'common'->>'nama_panggilan',
  r.answers->'common'->>'whatsapp', r.answers->'common'->>'instansi') ILIKE '%' || sqlc.narg('search')::text || '%')
ORDER BY r.submitted_at DESC, r.id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountP3riRegistrations :one
SELECT count(*)
FROM p3ri_registrations r JOIN public_users u ON u.id = r.user_id
WHERE (sqlc.narg('status')::text IS NULL OR r.status = sqlc.narg('status')::text)
AND (sqlc.narg('division')::text IS NULL OR (
  CASE WHEN sqlc.narg('choice_rank')::integer IS NULL
    THEN sqlc.narg('division')::text = ANY (r.choices)
    ELSE r.choices[sqlc.narg('choice_rank')::integer] = sqlc.narg('division')::text END))
AND (sqlc.narg('gender')::text IS NULL OR r.answers->'common'->>'jenis_kelamin' = sqlc.narg('gender')::text)
AND (sqlc.narg('search')::text IS NULL OR concat_ws(' ', u.email,
  r.answers->'common'->>'nama_lengkap', r.answers->'common'->>'nama_panggilan',
  r.answers->'common'->>'whatsapp', r.answers->'common'->>'instansi') ILIKE '%' || sqlc.narg('search')::text || '%');

-- name: P3riStatusCounts :many
SELECT status, count(*) AS total FROM p3ri_registrations GROUP BY status;

-- name: P3riRegistration :one
SELECT sqlc.embed(r), u.email, a.display_name AS status_updated_by_name
FROM p3ri_registrations r
JOIN public_users u ON u.id = r.user_id
LEFT JOIN admin_users a ON a.id = r.status_updated_by
WHERE r.id = $1;

-- name: UpdateP3riRegistrationStatus :one
UPDATE p3ri_registrations SET
  status = sqlc.arg('status')::text,
  accepted_division = sqlc.narg('accepted_division')::text,
  admin_note = sqlc.narg('admin_note')::text,
  status_updated_by = sqlc.arg('status_updated_by')::integer,
  status_updated_at = now(),
  updated_at = now()
WHERE id = sqlc.arg('id')::integer
RETURNING *;

-- name: P3riRegistrationFiles :many
SELECT files FROM p3ri_registrations
WHERE files @> jsonb_build_array(jsonb_build_object('key', sqlc.arg('key')::text));
