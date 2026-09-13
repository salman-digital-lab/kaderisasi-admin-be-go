-- name: ActivityCourseOptions :many
SELECT c.id, c.title, c.status, c.summary, c.minimum_level,
 (SELECT count(*)::integer FROM course_lessons l WHERE l.course_id=c.id AND l.deleted_at IS NULL) AS lesson_count
FROM courses c WHERE c.title ILIKE '%' || @search::text || '%'
ORDER BY c.title, c.id LIMIT @page_limit OFFSET @page_offset;

-- name: CountActivityCourseOptions :one
SELECT count(*) FROM courses WHERE title ILIKE '%' || @search::text || '%';

-- name: LinkedActivityCourses :many
SELECT c.id, c.title, c.status, c.summary, c.minimum_level,
 (SELECT count(*)::integer FROM course_lessons l WHERE l.course_id=c.id AND l.deleted_at IS NULL) AS lesson_count
FROM activity_courses ac JOIN courses c ON c.id=ac.course_id
WHERE ac.activity_id=$1 ORDER BY ac.position, ac.course_id;

-- name: LockActivityCourses :one
SELECT id FROM activities WHERE id=$1 FOR UPDATE;

-- name: CountExistingActivityCourses :one
SELECT count(*) FROM courses WHERE id=ANY(@course_ids::integer[]);

-- name: ClearActivityCourses :exec
DELETE FROM activity_courses WHERE activity_id=$1;

-- name: InsertActivityCourses :exec
INSERT INTO activity_courses(activity_id,course_id,position)
SELECT @activity_id, id, ordinality::integer FROM unnest(@course_ids::integer[]) WITH ORDINALITY AS selected(id,ordinality);

-- name: RegistrationCourseProgress :many
SELECT registration_id, course_id, status::text, total_lessons::integer, completed_lessons::integer
FROM activity_course_progress WHERE activity_id = @activity_id::integer AND registration_id=ANY(@registration_ids::integer[])
ORDER BY registration_id, position, course_id;
