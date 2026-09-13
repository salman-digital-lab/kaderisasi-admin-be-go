-- name: LinkedClubCourses :many
SELECT c.id, c.title, c.status, c.summary, c.minimum_level,
 (SELECT count(*)::integer FROM course_lessons l WHERE l.course_id=c.id AND l.deleted_at IS NULL) AS lesson_count
FROM club_courses ac JOIN courses c ON c.id=ac.course_id
WHERE ac.club_id=$1 ORDER BY ac.position, ac.course_id;

-- name: LockClubCourses :one
SELECT id FROM clubs WHERE id=$1 FOR UPDATE;

-- name: CountExistingClubCourses :one
SELECT count(*) FROM courses WHERE id=ANY(@course_ids::integer[]);

-- name: ClearClubCourses :exec
DELETE FROM club_courses WHERE club_id=$1;

-- name: InsertClubCourses :exec
INSERT INTO club_courses(club_id,course_id,position)
SELECT @club_id, id, ordinality::integer FROM unnest(@course_ids::integer[]) WITH ORDINALITY AS selected(id,ordinality);


-- name: ListLinkedCoursePeople :many
WITH people AS (
 SELECT ar.id, ar.activity_id AS owner_id, 'activity'::text AS kind, ar.user_id,
   COALESCE(p.name, ar.guest_data->>'name', '')::text AS name,
   COALESCE(u.email, ar.guest_data->>'email', '')::text AS email,
   COALESCE(ar.status, '')::text AS registration_status
 FROM activity_registrations ar LEFT JOIN public_users u ON u.id=ar.user_id
 LEFT JOIN LATERAL (SELECT name FROM profiles WHERE user_id=ar.user_id ORDER BY id LIMIT 1) p ON true
 UNION ALL
 SELECT cr.id, cr.club_id, 'club'::text, cr.member_id,
   COALESCE(p.name, '')::text, COALESCE(u.email, '')::text, COALESCE(cr.status, '')::text
 FROM club_registrations cr LEFT JOIN public_users u ON u.id=cr.member_id
 LEFT JOIN LATERAL (SELECT name FROM profiles WHERE user_id=cr.member_id ORDER BY id LIMIT 1) p ON true
), progress AS (
 SELECT 'activity'::text AS kind, activity_id AS owner_id, registration_id, course_id, status FROM activity_course_progress
 UNION ALL SELECT 'club'::text, club_id, registration_id, course_id, status FROM club_course_progress
)
SELECT id, name, email, registration_status FROM people
WHERE people.kind = @kind::text AND people.owner_id = @owner_id::integer
 AND (people.name ILIKE '%' || @search::text || '%' OR people.email ILIKE '%' || @search::text || '%')
 AND (@registration_status::text = '' OR people.registration_status = @registration_status)
 AND (@completion::text = '' OR people.id IN (
 SELECT registration_id FROM progress
 WHERE kind = @kind AND owner_id = @owner_id AND (@course_id::integer = 0 OR course_id = @course_id)
 GROUP BY registration_id
 HAVING (@completion = 'completed' AND bool_and(status = 'completed'))
 OR (@completion = 'incomplete' AND bool_and(status <> 'unverifiable') AND bool_or(status <> 'completed'))
 OR (@completion = 'unverifiable' AND bool_or(status = 'unverifiable'))
 ))
ORDER BY name, id LIMIT @page_limit::integer OFFSET @page_offset::integer;

-- name: CountLinkedCoursePeople :one
WITH people AS (
 SELECT ar.id, ar.activity_id AS owner_id, 'activity'::text AS kind, ar.user_id,
   COALESCE(p.name, ar.guest_data->>'name', '')::text AS name,
   COALESCE(u.email, ar.guest_data->>'email', '')::text AS email,
   COALESCE(ar.status, '')::text AS registration_status
 FROM activity_registrations ar LEFT JOIN public_users u ON u.id=ar.user_id
 LEFT JOIN LATERAL (SELECT name FROM profiles WHERE user_id=ar.user_id ORDER BY id LIMIT 1) p ON true
 UNION ALL
 SELECT cr.id, cr.club_id, 'club'::text, cr.member_id,
   COALESCE(p.name, '')::text, COALESCE(u.email, '')::text, COALESCE(cr.status, '')::text
 FROM club_registrations cr LEFT JOIN public_users u ON u.id=cr.member_id
 LEFT JOIN LATERAL (SELECT name FROM profiles WHERE user_id=cr.member_id ORDER BY id LIMIT 1) p ON true
), progress AS (
 SELECT 'activity'::text AS kind, activity_id AS owner_id, registration_id, course_id, status FROM activity_course_progress
 UNION ALL SELECT 'club'::text, club_id, registration_id, course_id, status FROM club_course_progress
)
SELECT count(*) FROM people
WHERE people.kind = @kind::text AND people.owner_id = @owner_id::integer
 AND (people.name ILIKE '%' || @search::text || '%' OR people.email ILIKE '%' || @search::text || '%')
 AND (@registration_status::text = '' OR people.registration_status = @registration_status)
 AND (@completion::text = '' OR people.id IN (
 SELECT registration_id FROM progress
 WHERE kind = @kind AND owner_id = @owner_id AND (@course_id::integer = 0 OR course_id = @course_id)
 GROUP BY registration_id
 HAVING (@completion = 'completed' AND bool_and(status = 'completed'))
 OR (@completion = 'incomplete' AND bool_and(status <> 'unverifiable') AND bool_or(status <> 'completed'))
 OR (@completion = 'unverifiable' AND bool_or(status = 'unverifiable'))
 ))
;

-- name: LinkedCoursePeopleProgress :many
SELECT registration_id, course_id, status::text, total_lessons::integer, completed_lessons::integer
FROM (
 SELECT 'activity'::text AS kind, activity_id AS owner_id, registration_id, course_id, position, status, total_lessons, completed_lessons FROM activity_course_progress
 UNION ALL SELECT 'club'::text, club_id, registration_id, course_id, position, status, total_lessons, completed_lessons FROM club_course_progress
) p WHERE kind = @kind::text AND owner_id = @owner_id::integer AND registration_id=ANY(@registration_ids::integer[])
ORDER BY registration_id, position, course_id;
