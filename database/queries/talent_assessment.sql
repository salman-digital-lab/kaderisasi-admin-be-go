-- name: GetTalentDraft :one
SELECT * FROM talent_assessment_drafts WHERE admin_user_id=$1;

-- name: CreateTalentDraft :one
INSERT INTO talent_assessment_drafts(admin_user_id,draft_id,definition_version,answers)
VALUES ($1,$2,$3,$4) RETURNING *;

-- name: SaveTalentDraft :one
UPDATE talent_assessment_drafts SET answers=$2,current_question=$3,revision=revision+1,updated_at=now()
WHERE admin_user_id=$1 RETURNING *;

-- name: DeleteTalentDraft :exec
DELETE FROM talent_assessment_drafts WHERE admin_user_id=$1;

-- name: GetTalentResult :one
SELECT * FROM talent_assessment_results WHERE admin_user_id=$1;

-- name: ReplaceTalentResult :one
INSERT INTO talent_assessment_results(admin_user_id,submission_id,definition_version,answers,result)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT(admin_user_id) DO UPDATE SET submission_id=EXCLUDED.submission_id,
definition_version=EXCLUDED.definition_version,answers=EXCLUDED.answers,result=EXCLUDED.result,submitted_at=now()
RETURNING *;
