-- name: CountAchievements :one
SELECT count(*) FROM achievements a LEFT JOIN public_users u ON u.id=a.user_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN admin_users admin ON admin.id=a.approver_id
WHERE (sqlc.narg('status')::text IS NULL OR a.status=CAST(CAST(sqlc.narg('status') AS text) AS integer))
AND (sqlc.narg('email')::text IS NULL OR u.email=sqlc.narg('email')::text)
AND (sqlc.narg('name')::text IS NULL OR p.name ILIKE '%'||sqlc.narg('name')::text||'%')
AND (sqlc.narg('kind')::text IS NULL OR a.type=CAST(CAST(sqlc.narg('kind') AS text) AS integer));

-- name: ListAchievements :many
SELECT sqlc.embed(a),(to_jsonb(u)-'password')::jsonb AS public_user,row_to_json(p) AS profile,(to_jsonb(admin)-'password')::jsonb AS approver
FROM achievements a LEFT JOIN public_users u ON u.id=a.user_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN admin_users admin ON admin.id=a.approver_id
WHERE (sqlc.narg('status')::text IS NULL OR a.status=CAST(CAST(sqlc.narg('status') AS text) AS integer))
AND (sqlc.narg('email')::text IS NULL OR u.email=sqlc.narg('email')::text)
AND (sqlc.narg('name')::text IS NULL OR p.name ILIKE '%'||sqlc.narg('name')::text||'%')
AND (sqlc.narg('kind')::text IS NULL OR a.type=CAST(CAST(sqlc.narg('kind') AS text) AS integer))
ORDER BY CASE WHEN @date_order::boolean AND @ascending::boolean THEN a.achievement_date END ASC,
CASE WHEN @date_order::boolean AND NOT @ascending::boolean THEN a.achievement_date END DESC,
CASE WHEN NOT @date_order::boolean AND @ascending::boolean THEN a.created_at END ASC,
CASE WHEN NOT @date_order::boolean AND NOT @ascending::boolean THEN a.created_at END DESC
LIMIT CAST(sqlc.narg('page_size')::text AS bigint) OFFSET CAST(@page_offset::text AS bigint);

-- name: AchievementByIdentifier :one
SELECT * FROM achievements WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: AchievementDetails :one
SELECT sqlc.embed(a),(to_jsonb(u)-'password')::jsonb AS public_user,row_to_json(p) AS profile,(to_jsonb(admin)-'password')::jsonb AS approver
FROM achievements a LEFT JOIN public_users u ON u.id=a.user_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN admin_users admin ON admin.id=a.approver_id
WHERE a.id = @id::integer;

-- name: AchievementExport :many
SELECT sqlc.embed(a),(to_jsonb(u)-'password')::jsonb AS public_user,row_to_json(p) AS profile,(to_jsonb(admin)-'password')::jsonb AS approver
FROM achievements a LEFT JOIN public_users u ON u.id=a.user_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN admin_users admin ON admin.id=a.approver_id
ORDER BY a.created_at DESC;

-- name: UpdateAchievement :one
UPDATE achievements SET name=sqlc.narg('name')::text,description=sqlc.narg('description')::text,type=CAST(CAST(sqlc.narg('type') AS text) AS integer),score=CAST(CAST(sqlc.narg('score') AS text) AS integer),proof=sqlc.narg('proof')::text,status=CAST(CAST(sqlc.narg('status') AS text) AS integer),remark=sqlc.narg('remark')::text,approver_id=CAST(CAST(sqlc.narg('approver_id') AS text) AS integer),approved_at=sqlc.narg('approved_at')::date,
updated_at=CASE WHEN @touch::boolean OR ROW(name,description,type,score,proof,status,remark,approver_id,approved_at) IS DISTINCT FROM ROW(sqlc.narg('name')::text,sqlc.narg('description')::text,CAST(CAST(sqlc.narg('type') AS text) AS integer),CAST(CAST(sqlc.narg('score') AS text) AS integer),sqlc.narg('proof')::text,CAST(CAST(sqlc.narg('status') AS text) AS integer),sqlc.narg('remark')::text,CAST(CAST(sqlc.narg('approver_id') AS text) AS integer),sqlc.narg('approved_at')::date) THEN now() ELSE updated_at END WHERE id = @id::integer RETURNING *;

-- name: CountMonthlyLeaderboard :one
SELECT count(*) FROM monthly_leaderboards b LEFT JOIN public_users u ON u.id=b.user_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN universities university ON university.id=p.university_id
WHERE (sqlc.narg('email')::text IS NULL OR u.email ILIKE '%'||sqlc.narg('email')::text||'%') AND (sqlc.narg('name')::text IS NULL OR p.name ILIKE '%'||sqlc.narg('name')::text||'%')
AND (NOT @filter_month::boolean OR b.month IS NOT DISTINCT FROM CAST(CAST(sqlc.narg('month') AS text) AS date))
AND (NOT @filter_year::boolean OR b.month BETWEEN CAST(CAST(sqlc.narg('start_date') AS text) AS date) AND CAST(CAST(sqlc.narg('end_date') AS text) AS date));

-- name: ListMonthlyLeaderboard :many
SELECT sqlc.embed(b),(to_jsonb(u)-'password')::jsonb AS public_user,row_to_json(p) AS profile,row_to_json(university) AS university
FROM monthly_leaderboards b LEFT JOIN public_users u ON u.id=b.user_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN universities university ON university.id=p.university_id
WHERE (sqlc.narg('email')::text IS NULL OR u.email ILIKE '%'||sqlc.narg('email')::text||'%') AND (sqlc.narg('name')::text IS NULL OR p.name ILIKE '%'||sqlc.narg('name')::text||'%')
AND (NOT @filter_month::boolean OR b.month IS NOT DISTINCT FROM CAST(CAST(sqlc.narg('month') AS text) AS date))
AND (NOT @filter_year::boolean OR b.month BETWEEN CAST(CAST(sqlc.narg('start_date') AS text) AS date) AND CAST(CAST(sqlc.narg('end_date') AS text) AS date))
ORDER BY b.score DESC LIMIT CAST(sqlc.narg('page_size')::text AS bigint) OFFSET CAST(@page_offset::text AS bigint);

-- name: FindMonthlyLeaderboard :one
SELECT * FROM monthly_leaderboards WHERE user_id IS NOT DISTINCT FROM sqlc.narg('user_id')::integer AND month=sqlc.narg('month')::date LIMIT 1;

-- name: CreateMonthlyLeaderboard :one
INSERT INTO monthly_leaderboards(user_id,month,score,score_academic,score_competition,score_organizational,created_at,updated_at) VALUES (sqlc.narg('user_id')::integer,sqlc.narg('month')::date,CAST(CAST(sqlc.narg('score') AS text) AS integer),CAST(CAST(sqlc.narg('score_academic') AS text) AS integer),CAST(CAST(sqlc.narg('score_competition') AS text) AS integer),CAST(CAST(sqlc.narg('score_organizational') AS text) AS integer),now(),now()) RETURNING *;

-- name: UpdateMonthlyLeaderboard :one
UPDATE monthly_leaderboards SET score=CAST(CAST(sqlc.narg('score') AS text) AS integer),score_academic=CAST(CAST(sqlc.narg('score_academic') AS text) AS integer),score_competition=CAST(CAST(sqlc.narg('score_competition') AS text) AS integer),score_organizational=CAST(CAST(sqlc.narg('score_organizational') AS text) AS integer),updated_at=CASE WHEN ROW(score,score_academic,score_competition,score_organizational) IS DISTINCT FROM ROW(CAST(CAST(sqlc.narg('score') AS text) AS integer),CAST(CAST(sqlc.narg('score_academic') AS text) AS integer),CAST(CAST(sqlc.narg('score_competition') AS text) AS integer),CAST(CAST(sqlc.narg('score_organizational') AS text) AS integer)) THEN now() ELSE updated_at END WHERE id = @id::integer RETURNING *;

-- name: CountLifetimeLeaderboard :one
SELECT count(*) FROM lifetime_leaderboards b LEFT JOIN public_users u ON u.id=b.user_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN universities university ON university.id=p.university_id
WHERE (sqlc.narg('email')::text IS NULL OR u.email ILIKE '%'||sqlc.narg('email')::text||'%') AND (sqlc.narg('name')::text IS NULL OR p.name ILIKE '%'||sqlc.narg('name')::text||'%');

-- name: ListLifetimeLeaderboard :many
SELECT sqlc.embed(b),(to_jsonb(u)-'password')::jsonb AS public_user,row_to_json(p) AS profile,row_to_json(university) AS university
FROM lifetime_leaderboards b LEFT JOIN public_users u ON u.id=b.user_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN universities university ON university.id=p.university_id
WHERE (sqlc.narg('email')::text IS NULL OR u.email ILIKE '%'||sqlc.narg('email')::text||'%') AND (sqlc.narg('name')::text IS NULL OR p.name ILIKE '%'||sqlc.narg('name')::text||'%')
ORDER BY b.score DESC LIMIT CAST(sqlc.narg('page_size')::text AS bigint) OFFSET CAST(@page_offset::text AS bigint);

-- name: FindLifetimeLeaderboard :one
SELECT * FROM lifetime_leaderboards WHERE user_id IS NOT DISTINCT FROM sqlc.narg('user_id')::integer LIMIT 1;

-- name: CreateLifetimeLeaderboard :one
INSERT INTO lifetime_leaderboards(user_id,score,score_academic,score_competition,score_organizational,created_at,updated_at) VALUES (sqlc.narg('user_id')::integer,CAST(CAST(sqlc.narg('score') AS text) AS integer),CAST(CAST(sqlc.narg('score_academic') AS text) AS integer),CAST(CAST(sqlc.narg('score_competition') AS text) AS integer),CAST(CAST(sqlc.narg('score_organizational') AS text) AS integer),now(),now()) RETURNING *;

-- name: UpdateLifetimeLeaderboard :one
UPDATE lifetime_leaderboards SET score=CAST(CAST(sqlc.narg('score') AS text) AS integer),score_academic=CAST(CAST(sqlc.narg('score_academic') AS text) AS integer),score_competition=CAST(CAST(sqlc.narg('score_competition') AS text) AS integer),score_organizational=CAST(CAST(sqlc.narg('score_organizational') AS text) AS integer),updated_at=CASE WHEN ROW(score,score_academic,score_competition,score_organizational) IS DISTINCT FROM ROW(CAST(CAST(sqlc.narg('score') AS text) AS integer),CAST(CAST(sqlc.narg('score_academic') AS text) AS integer),CAST(CAST(sqlc.narg('score_competition') AS text) AS integer),CAST(CAST(sqlc.narg('score_organizational') AS text) AS integer)) THEN now() ELSE updated_at END WHERE id = @id::integer RETURNING *;

