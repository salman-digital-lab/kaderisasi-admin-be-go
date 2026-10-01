package talent

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
)

type Service struct{ Pool *pgxpool.Pool }
type Draft struct {
	ID              string    `json:"draft_id"`
	Version         string    `json:"definition_version"`
	Answers         []int     `json:"answers"`
	CurrentQuestion int       `json:"current_question"`
	Revision        int32     `json:"revision"`
	UpdatedAt       time.Time `json:"updated_at"`
}
type Result struct {
	SubmissionID string    `json:"submission_id"`
	Version      string    `json:"definition_version"`
	SubmittedAt  time.Time `json:"submitted_at"`
	Scores
}
type State struct {
	Draft  *Draft  `json:"draft"`
	Result *Result `json:"result"`
}
type SaveRequest struct {
	DraftID         string `json:"draft_id"`
	Revision        int32  `json:"revision"`
	Answers         []int  `json:"answers"`
	CurrentQuestion int    `json:"current_question"`
}
type SubmitRequest struct {
	DraftID  string `json:"draft_id"`
	Revision int32  `json:"revision"`
}

func draftView(row dbgen.TalentAssessmentDraft) (*Draft, error) {
	result := &Draft{ID: row.DraftID, Version: row.DefinitionVersion, CurrentQuestion: int(row.CurrentQuestion), Revision: row.Revision, UpdatedAt: row.UpdatedAt.Time}
	err := json.Unmarshal(row.Answers, &result.Answers)
	return result, err
}
func resultView(row dbgen.TalentAssessmentResult) (*Result, error) {
	result := &Result{SubmissionID: row.SubmissionID, Version: row.DefinitionVersion, SubmittedAt: row.SubmittedAt.Time}
	err := json.Unmarshal(row.Result, &result.Scores)
	return result, err
}

// Account locking serializes draft creation, saves and replacement without changing
// any existing account data or locking another participant's assessment.
func (s Service) locked(ctx context.Context, id int32, fn func(*dbgen.Queries) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := dbgen.New(tx)
	if _, err = q.LockAdmin(ctx, id); err != nil {
		return err
	}
	if err = fn(q); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s Service) State(ctx context.Context, id int32) (State, error) {
	result := State{}
	err := s.locked(ctx, id, func(q *dbgen.Queries) error {
		row, err := q.GetTalentDraft(ctx, id)
		if err == nil {
			result.Draft, err = draftView(row)
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		completed, err := q.GetTalentResult(ctx, id)
		if err == nil {
			result.Result, err = resultView(completed)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	})
	return result, err
}
func (s Service) Start(ctx context.Context, id int32) (*Draft, error) {
	var result *Draft
	err := s.locked(ctx, id, func(q *dbgen.Queries) error {
		row, err := q.GetTalentDraft(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			draftID, uuidErr := auth.UUID()
			if uuidErr != nil {
				return uuidErr
			}
			answers, marshalErr := json.Marshal(make([]int, QuestionCount))
			if marshalErr != nil {
				return marshalErr
			}
			row, err = q.CreateTalentDraft(ctx, dbgen.CreateTalentDraftParams{AdminUserID: id, DraftID: draftID, DefinitionVersion: definition.Version, Answers: answers})
		}
		if err != nil {
			return err
		}
		result, err = draftView(row)
		return err
	})
	return result, err
}
func (s Service) Save(ctx context.Context, id int32, input SaveRequest) (*Draft, error) {
	if input.DraftID == "" || input.Revision < 1 || input.CurrentQuestion < 1 || input.CurrentQuestion > QuestionCount || ValidateAnswers(input.Answers, false) != nil {
		return nil, domain.Fail(422, "TALENT_INVALID_ANSWERS")
	}
	var result *Draft
	err := s.locked(ctx, id, func(q *dbgen.Queries) error {
		row, err := q.GetTalentDraft(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Fail(409, "TALENT_DRAFT_CHANGED")
		}
		if err != nil {
			return err
		}
		if row.DraftID != input.DraftID || row.Revision != input.Revision {
			return domain.Fail(409, "TALENT_DRAFT_CHANGED")
		}
		if row.DefinitionVersion != definition.Version {
			return domain.Fail(409, "TALENT_VERSION_CHANGED")
		}
		answers, err := json.Marshal(input.Answers)
		if err != nil {
			return err
		}
		row, err = q.SaveTalentDraft(ctx, dbgen.SaveTalentDraftParams{AdminUserID: id, Answers: answers, CurrentQuestion: int32(input.CurrentQuestion)})
		if err != nil {
			return err
		}
		result, err = draftView(row)
		return err
	})
	return result, err
}
func (s Service) Submit(ctx context.Context, id int32, input SubmitRequest) (*Result, error) {
	if input.DraftID == "" || input.Revision < 1 {
		return nil, domain.Fail(422, "TALENT_INVALID_SUBMISSION")
	}
	var result *Result
	err := s.locked(ctx, id, func(q *dbgen.Queries) error {
		previous, err := q.GetTalentResult(ctx, id)
		if err == nil && previous.SubmissionID == input.DraftID {
			result, err = resultView(previous)
			return err
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		row, err := q.GetTalentDraft(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Fail(409, "TALENT_DRAFT_CHANGED")
		}
		if err != nil {
			return err
		}
		if row.DraftID != input.DraftID || row.Revision != input.Revision {
			return domain.Fail(409, "TALENT_DRAFT_CHANGED")
		}
		if row.DefinitionVersion != definition.Version {
			return domain.Fail(409, "TALENT_VERSION_CHANGED")
		}
		var answers []int
		if err = json.Unmarshal(row.Answers, &answers); err != nil {
			return err
		}
		scores, err := Score(answers)
		if err != nil {
			return domain.Fail(422, "TALENT_INCOMPLETE")
		}
		encoded, err := json.Marshal(scores)
		if err != nil {
			return err
		}
		completed, err := q.ReplaceTalentResult(ctx, dbgen.ReplaceTalentResultParams{AdminUserID: id, SubmissionID: row.DraftID, DefinitionVersion: row.DefinitionVersion, Answers: row.Answers, Result: encoded})
		if err != nil {
			return err
		}
		if err = q.DeleteTalentDraft(ctx, id); err != nil {
			return err
		}
		result, err = resultView(completed)
		return err
	})
	return result, err
}
func (s Service) Result(ctx context.Context, id int32) (*Result, error) {
	row, err := dbgen.New(s.Pool).GetTalentResult(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.Fail(404, "TALENT_RESULT_NOT_FOUND")
	}
	if err != nil {
		return nil, err
	}
	return resultView(row)
}
