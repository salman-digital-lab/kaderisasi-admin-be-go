-- name: AnnouncementCreate :one
INSERT INTO announcements(title,body,link_label,link_url,audience,author_id) VALUES($1,$2,$3,$4,$5,$6) RETURNING *;
-- name: AnnouncementGet :one
SELECT * FROM announcements WHERE id=$1;
-- name: AnnouncementLock :one
SELECT * FROM announcements WHERE id=$1 FOR UPDATE;
-- name: AnnouncementList :many
SELECT * FROM announcements WHERE (sqlc.arg(before_id)::integer=0 OR id<sqlc.arg(before_id)) ORDER BY id DESC LIMIT 21;
-- name: AnnouncementUpdate :one
UPDATE announcements SET title=$2,body=$3,link_label=$4,link_url=$5,audience=$6,version=version+1,updated_at=clock_timestamp() WHERE id=$1 AND version=$7 AND state='draft' RETURNING *;
-- name: AnnouncementDelete :execrows
DELETE FROM announcements WHERE id=$1 AND state='draft' AND version=$2;
-- name: AnnouncementPublish :one
UPDATE announcements SET state='published',publisher_id=$2,published_at=clock_timestamp(),recipient_count=$3,version=version+1,updated_at=clock_timestamp() WHERE id=$1 RETURNING *;
-- name: AnnouncementWithdraw :one
UPDATE announcements SET state='withdrawn',withdrawn_at=clock_timestamp(),updated_at=clock_timestamp(),version=version+1 WHERE id=$1 AND state='published' RETURNING *;
-- name: AnnouncementRecipients :many
WITH members AS (
 SELECT u.id, u.account_status='active' AS eligible FROM public_users u
 WHERE sqlc.arg(all_members)::boolean OR u.id=ANY(sqlc.arg(member_ids)::integer[])
 OR EXISTS (SELECT 1 FROM activity_registrations r WHERE r.user_id=u.id AND r.activity_id=ANY(sqlc.arg(activity_ids)::integer[]) AND (cardinality(sqlc.arg(activity_statuses)::text[])=0 OR r.status=ANY(sqlc.arg(activity_statuses)::text[])))
 OR EXISTS (SELECT 1 FROM club_registrations r WHERE r.member_id=u.id AND r.club_id=ANY(sqlc.arg(club_ids)::integer[]) AND r.status=ANY(sqlc.arg(club_statuses)::text[]))
), admins AS (
 SELECT u.id, u.is_active AS eligible FROM admin_users u
 WHERE sqlc.arg(all_admins)::boolean OR u.id=ANY(sqlc.arg(admin_ids)::integer[])
 OR u.role_code=ANY(sqlc.arg(role_codes)::text[]) OR u.additional_role_codes && sqlc.arg(role_codes)::text[]
)
SELECT id,eligible,'member'::text AS kind FROM members UNION ALL SELECT id,eligible,'admin'::text AS kind FROM admins;
-- name: AnnouncementInsertMembers :exec
INSERT INTO announcement_recipients(announcement_id,public_user_id) SELECT $1,unnest(sqlc.arg(ids)::integer[]) ON CONFLICT DO NOTHING;
-- name: AnnouncementInsertAdmins :exec
INSERT INTO announcement_recipients(announcement_id,admin_user_id) SELECT $1,unnest(sqlc.arg(ids)::integer[]) ON CONFLICT DO NOTHING;
-- name: AdminNotificationList :many
SELECT r.id,a.title,a.body,a.link_label,a.link_url,a.published_at,r.read_at FROM announcement_recipients r JOIN announcements a ON a.id=r.announcement_id
WHERE r.admin_user_id=$1 AND a.state='published' AND (NOT sqlc.arg(unread)::boolean OR r.read_at IS NULL)
AND (sqlc.arg(before_id)::integer=0 OR (a.published_at,r.id)<(sqlc.arg(before_time)::timestamptz,sqlc.arg(before_id)::integer))
ORDER BY a.published_at DESC,r.id DESC LIMIT 21;
-- name: AdminNotificationGet :one
SELECT r.id,a.title,a.body,a.link_label,a.link_url,a.published_at,r.read_at FROM announcement_recipients r JOIN announcements a ON a.id=r.announcement_id WHERE r.id=$1 AND r.admin_user_id=$2 AND a.state='published';
-- name: AdminNotificationCount :one
SELECT count(*)::integer FROM announcement_recipients r JOIN announcements a ON a.id=r.announcement_id WHERE r.admin_user_id=$1 AND a.state='published' AND r.read_at IS NULL;
-- name: AdminNotificationRead :execrows
UPDATE announcement_recipients r SET read_at=COALESCE(read_at,clock_timestamp()) FROM announcements a WHERE a.id=r.announcement_id AND a.state='published' AND r.id=$1 AND r.admin_user_id=$2;
-- name: AdminNotificationReadAll :exec
UPDATE announcement_recipients r SET read_at=clock_timestamp() FROM announcements a WHERE a.id=r.announcement_id AND a.state='published' AND r.admin_user_id=$1 AND r.read_at IS NULL AND a.published_at<=sqlc.arg(cutoff)::timestamptz;
-- name: AnnouncementClock :one
WITH publication_lock AS MATERIALIZED (SELECT pg_advisory_xact_lock(1790045423))
SELECT clock_timestamp()::timestamptz FROM publication_lock;
-- name: AnnouncementOptions :many
SELECT * FROM (
SELECT id,(COALESCE(display_name,email)||' · '||email)::text AS label,'admin'::text AS kind FROM admin_users
UNION ALL SELECT u.id,(COALESCE(p.name,u.email,u.member_id,'Anggota')||' · '||COALESCE(u.member_id,u.email,'#'||u.id))::text,'member'::text FROM public_users u LEFT JOIN LATERAL (SELECT name FROM profiles WHERE user_id=u.id ORDER BY id LIMIT 1) p ON true
UNION ALL SELECT id,name::text,'activity'::text FROM activities
UNION ALL SELECT id,name::text,'club'::text FROM clubs
) choices WHERE kind=sqlc.arg(kind)::text AND (label ILIKE '%'||sqlc.arg(search)::text||'%' OR id=ANY(sqlc.arg(selected)::integer[]))
ORDER BY (id=ANY(sqlc.arg(selected)::integer[])) DESC,label,id LIMIT (50+cardinality(sqlc.arg(selected)::integer[]));
