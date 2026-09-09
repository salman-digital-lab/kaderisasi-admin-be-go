-- name: CloseActivityRegistration :many
UPDATE activities SET is_published=false
WHERE is_published=true AND registration_end < $1::date
RETURNING id,slug;

-- name: CloseClubRegistration :many
UPDATE clubs SET is_registration_open=false
WHERE is_registration_open=true AND registration_end_date IS NOT NULL AND registration_end_date < $1::date
RETURNING id,name,registration_end_date;

-- name: UpdateClubVisibility :many
UPDATE clubs SET is_show=false
WHERE is_show=true AND end_period IS NOT NULL AND end_period < $1::date
RETURNING id,name,end_period;
