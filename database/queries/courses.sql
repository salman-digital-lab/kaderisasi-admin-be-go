-- name: ListCourses :many
SELECT c.*, (SELECT count(*) FROM course_lessons l WHERE l.course_id=c.id AND l.deleted_at IS NULL)::integer AS lesson_count
FROM courses c
WHERE (@search::text='' OR c.title ILIKE '%' || @search || '%')
AND (@status::text='' OR c.status = @status)
ORDER BY c.id DESC LIMIT @page_limit OFFSET @page_offset;

-- name: CountCourses :one
SELECT count(*) FROM courses c WHERE (@search::text='' OR c.title ILIKE '%' || @search || '%') AND (@status::text='' OR c.status = @status);

-- name: CourseByID :one
SELECT * FROM courses WHERE id=$1;

-- name: LockCourse :one
SELECT * FROM courses WHERE id=$1 FOR UPDATE;

-- name: CreateCourse :one
INSERT INTO courses(title,summary,description,minimum_level) VALUES($1,$2,$3,$4) RETURNING *;

-- name: UpdateCourse :one
UPDATE courses SET title=$2,summary=$3,description=$4,minimum_level=$5,status=$6,updated_at=now() WHERE id=$1 RETURNING *;

-- name: CourseLessons :many
SELECT * FROM course_lessons WHERE course_id=$1 AND deleted_at IS NULL ORDER BY position,id;

-- name: CourseLessonByID :one
SELECT * FROM course_lessons WHERE course_id=$1 AND id=$2 AND deleted_at IS NULL;

-- name: CreateCourseLesson :one
INSERT INTO course_lessons(course_id,title,description,youtube_video_id,position)
VALUES(@course_id,@title,@description,@youtube_video_id,(SELECT COALESCE(max(position),0)+1 FROM course_lessons WHERE course_id = @course_id)) RETURNING *;

-- name: UpdateCourseLesson :one
UPDATE course_lessons SET title=$3,description=$4,youtube_video_id=$5,updated_at=now()
WHERE course_id=$1 AND id=$2 AND deleted_at IS NULL RETURNING *;

-- name: RemoveCourseLesson :exec
UPDATE course_lessons SET deleted_at=now(),updated_at=now() WHERE course_id=$1 AND id=$2 AND deleted_at IS NULL;

-- name: ReorderCourseLesson :exec
UPDATE course_lessons SET position=$3,updated_at=now() WHERE course_id=$1 AND id=$2 AND deleted_at IS NULL;

-- name: CourseDocuments :many
SELECT d.* FROM course_documents d JOIN course_lessons l ON l.id=d.lesson_id
WHERE l.course_id=$1 AND l.deleted_at IS NULL AND d.deleted_at IS NULL ORDER BY d.id;

-- name: CourseDocumentByID :one
SELECT d.* FROM course_documents d JOIN course_lessons l ON l.id=d.lesson_id
WHERE l.course_id=$1 AND l.id=$2 AND d.id=$3 AND l.deleted_at IS NULL AND d.deleted_at IS NULL;

-- name: CreateCourseDocument :one
INSERT INTO course_documents(lesson_id,storage_key,filename,size_bytes) VALUES($1,$2,$3,$4) RETURNING *;

-- name: RemoveCourseDocument :exec
UPDATE course_documents SET deleted_at=now() WHERE id=$1;

-- name: CourseLearners :many
SELECT p.user_id, COALESCE(pr.name,'')::text AS name, u.member_id,
count(*) FILTER (WHERE p.completed_at IS NOT NULL AND l.deleted_at IS NULL)::integer AS completed_lessons,
(SELECT count(*) FROM course_lessons cl WHERE cl.course_id = @course_id AND cl.deleted_at IS NULL)::integer AS total_lessons,
min(p.first_visited_at)::timestamptz AS started_at, max(p.last_visited_at)::timestamptz AS last_activity_at
FROM course_lesson_progress p JOIN course_lessons l ON l.id=p.lesson_id
JOIN public_users u ON u.id=p.user_id
LEFT JOIN LATERAL (SELECT name FROM profiles WHERE user_id=p.user_id ORDER BY id LIMIT 1) pr ON true
WHERE l.course_id = @course_id AND (@search::text='' OR pr.name ILIKE '%' || @search || '%' OR u.member_id ILIKE '%' || @search || '%')
GROUP BY p.user_id,pr.name,u.member_id ORDER BY max(p.last_visited_at) DESC,p.user_id
LIMIT @page_limit OFFSET @page_offset;

-- name: CountCourseLearners :one
SELECT count(DISTINCT p.user_id) FROM course_lesson_progress p JOIN course_lessons l ON l.id=p.lesson_id
JOIN public_users u ON u.id=p.user_id
LEFT JOIN LATERAL (SELECT name FROM profiles WHERE user_id=p.user_id ORDER BY id LIMIT 1) pr ON true
WHERE l.course_id = @course_id AND (@search::text='' OR pr.name ILIKE '%' || @search || '%' OR u.member_id ILIKE '%' || @search || '%');
