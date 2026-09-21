-- name: ListCalendarEvents :many
SELECT e.*, a.name AS activity_name, a.slug AS activity_slug, a.is_published AS activity_published
FROM calendar_events e LEFT JOIN activities a ON a.id = e.activity_id
WHERE e.starts_at < sqlc.arg(range_end)::timestamptz AND e.ends_at > sqlc.arg(range_start)::timestamptz
ORDER BY e.starts_at, e.id;

-- name: GetCalendarEvent :one
SELECT e.*, a.name AS activity_name, a.slug AS activity_slug, a.is_published AS activity_published
FROM calendar_events e LEFT JOIN activities a ON a.id = e.activity_id WHERE e.id = $1;

-- name: CreateCalendarEvent :one
INSERT INTO calendar_events (title, description, location, starts_at, ends_at, all_day, activity_id)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id;

-- name: UpdateCalendarEvent :execrows
UPDATE calendar_events SET title=$2, description=$3, location=$4, starts_at=$5,
ends_at=$6, all_day=$7, activity_id=$8, updated_at=now() WHERE id=$1;

-- name: DeleteCalendarEvent :execrows
DELETE FROM calendar_events WHERE id=$1;
