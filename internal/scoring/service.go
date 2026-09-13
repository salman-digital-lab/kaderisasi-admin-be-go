package scoring

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"reflect"
	"sort"
	"strconv"
	"time"
)

type Service struct{ Pool *pgxpool.Pool }

func rubric(ctx context.Context, q *dbgen.Queries, id int32) (*Rubric, error) {
	row, err := q.ScoringRubric(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r := &Rubric{Revision: row.Revision, Locked: row.LockedAt.Valid}
	if err = json.Unmarshal(row.Definition, &r.Definition); err != nil {
		return nil, err
	}
	return r, nil
}
func (s Service) Rubric(ctx context.Context, id int32) (*Rubric, error) {
	q := dbgen.New(s.Pool)
	if _, err := q.ActivityDetails(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.Fail(404, "ACTIVITY_NOT_FOUND")
	} else if err != nil {
		return nil, err
	}
	return rubric(ctx, q, id)
}
func lockActivity(ctx context.Context, q *dbgen.Queries, id int32) error {
	_, err := q.LockActivityByIdentifier(ctx, strconv.Itoa(int(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "ACTIVITY_NOT_FOUND")
	}
	return err
}
func (s Service) SaveRubric(ctx context.Context, id, actor int32, in Rubric) (*Rubric, error) {
	if err := ValidateDefinition(in.Definition); err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	if err = lockActivity(ctx, q, id); err != nil {
		return nil, err
	}
	old, err := rubric(ctx, q, id)
	if err != nil {
		return nil, err
	}
	if old != nil && old.Locked {
		return nil, domain.Fail(409, "SCORING_RUBRIC_LOCKED")
	}
	if old == nil && in.Revision != 0 || old != nil && old.Revision != in.Revision {
		return nil, domain.Fail(409, "SCORING_REVISION_CONFLICT")
	}
	if in.Grades == nil {
		in.Grades = []Grade{}
	}
	raw, err := json.Marshal(in.Definition)
	if err != nil {
		return nil, err
	}
	if err = q.SaveScoringRubric(ctx, dbgen.SaveScoringRubricParams{ActivityID: id, Definition: raw, UpdatedBy: actor}); err != nil {
		return nil, err
	}
	result, err := rubric(ctx, q, id)
	if err != nil {
		return nil, err
	}
	return result, tx.Commit(ctx)
}
func decode(raw []byte) (*Data, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var d Data
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	if d.SchemaVersion != 1 {
		return nil, domain.Fail(409, "UNSUPPORTED_SCORING_VERSION")
	}
	return &d, nil
}
func load(ctx context.Context, q *dbgen.Queries, activityID, registrationID int32) (*Data, error) {
	row, err := q.ScoringRegistration(ctx, dbgen.ScoringRegistrationParams{ID: registrationID, ActivityID: &activityID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.Fail(404, "REGISTRATION_NOT_FOUND")
	}
	if err != nil {
		return nil, err
	}
	return decode(row.ScoringData)
}
func persist(ctx context.Context, q *dbgen.Queries, id int32, d *Data) error {
	raw, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return q.SaveScoringData(ctx, dbgen.SaveScoringDataParams{ID: id, ScoringData: raw})
}
func revision(d *Data) int32 {
	if d == nil {
		return 0
	}
	return d.Revision
}
func setState(d *Data, result Result) {
	d.State = "incomplete"
	if result.Complete {
		d.State = "complete"
	}
	if d.Published != nil {
		d.State = "published"
		if !reflect.DeepEqual(d.Draft, d.Published.Draft) {
			d.State = "changed"
		}
	}
}
func saveDraft(ctx context.Context, q *dbgen.Queries, r *Rubric, activityID, id, actor int32, in SaveInput) (*Data, error) {
	if in.Revision < 0 || r.Revision != in.RubricRevision {
		return nil, domain.Fail(409, "SCORING_REVISION_CONFLICT")
	}
	result, err := Calculate(r.Definition, in.Draft)
	if err != nil {
		return nil, err
	}
	d, err := load(ctx, q, activityID, id)
	if err != nil {
		return nil, err
	}
	if revision(d) != in.Revision {
		return nil, domain.Fail(409, "SCORING_REVISION_CONFLICT")
	}
	if d == nil {
		d = &Data{SchemaVersion: 1}
	}
	// Canonical keys make blank and explicitly cleared values compare equally.
	scores := map[string]*float64{}
	hasScore := false
	for _, c := range Criteria(r.Definition) {
		if value := in.Draft.Scores[c.ID]; value != nil {
			scores[c.ID] = value
			hasScore = true
		}
	}
	d.Draft = Draft{Scores: scores, Note: in.Draft.Note}
	d.Revision++
	d.UpdatedBy = actor
	d.UpdatedAt = time.Now().UTC()
	setState(d, result)
	if hasScore {
		if err = q.LockScoringRubric(ctx, activityID); err != nil {
			return nil, err
		}
	}
	if err = persist(ctx, q, id, d); err != nil {
		return nil, err
	}
	return d, nil
}
func (s Service) Save(ctx context.Context, activityID, id, actor int32, in SaveInput) (*Data, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	if err = lockActivity(ctx, q, activityID); err != nil {
		return nil, err
	}
	r, err := rubric(ctx, q, activityID)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, domain.Fail(422, "SCORING_RUBRIC_REQUIRED")
	}
	d, err := saveDraft(ctx, q, r, activityID, id, actor, in)
	if err != nil {
		return nil, err
	}
	return d, tx.Commit(ctx)
}
func (s Service) Publish(ctx context.Context, activityID, actor int32, in Batch, withdraw bool) error {
	if len(in.Selections) == 0 || len(in.Selections) > 1000 {
		return domain.Fail(422, "INVALID_SCORING_SELECTION")
	}
	sort.Slice(in.Selections, func(i, j int) bool { return in.Selections[i].RegistrationID < in.Selections[j].RegistrationID })
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	if err = lockActivity(ctx, q, activityID); err != nil {
		return err
	}
	r, err := rubric(ctx, q, activityID)
	if err != nil {
		return err
	}
	if r == nil || r.Revision != in.RubricRevision {
		return domain.Fail(409, "SCORING_REVISION_CONFLICT")
	}
	for i, item := range in.Selections {
		if item.RegistrationID <= 0 || i > 0 && item.RegistrationID == in.Selections[i-1].RegistrationID {
			return domain.Fail(422, "INVALID_SCORING_SELECTION")
		}
		d, err := load(ctx, q, activityID, item.RegistrationID)
		if err != nil {
			return err
		}
		if d == nil || d.Revision != item.Revision {
			return domain.Fail(409, "SCORING_REVISION_CONFLICT")
		}
		result, err := Calculate(r.Definition, d.Draft)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		d.Revision++
		d.UpdatedBy = actor
		d.UpdatedAt = now
		action := "publish"
		var raw []byte
		if withdraw {
			if d.Published == nil {
				return domain.Fail(409, "SCORING_NOT_PUBLISHED")
			}
			d.Published = nil
			action = "withdraw"
		} else {
			if !result.Complete {
				return domain.Fail(422, "SCORING_INCOMPLETE")
			}
			d.Published = &Snapshot{SchemaVersion: 1, ActivityID: activityID, RegistrationID: item.RegistrationID, Revision: d.Revision, Rubric: r.Definition, Draft: d.Draft, Result: result, PublishedBy: actor, PublishedAt: now}
			raw, err = json.Marshal(d.Published)
			if err != nil {
				return err
			}
		}
		setState(d, result)
		if err = q.AddScoringPublication(ctx, dbgen.AddScoringPublicationParams{ActivityID: activityID, RegistrationID: item.RegistrationID, Revision: d.Revision, Action: action, Snapshot: raw, ActorID: actor, CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}}); err != nil {
			return err
		}
		if err = persist(ctx, q, item.RegistrationID, d); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func list(ctx context.Context, q *dbgen.Queries, r *Rubric, id int32, search, state string, page, size int32) (Page, error) {
	result := Page{Entries: []Entry{}, Page: page, PerPage: size}
	rows, err := q.ListScoringRegistrations(ctx, dbgen.ListScoringRegistrationsParams{ActivityID: &id, Search: search, State: state, PageSize: size, PageOffset: (page - 1) * size})
	if err != nil {
		return result, err
	}
	if len(rows) == 0 {
		result.Total, err = q.CountScoringRegistrations(ctx, dbgen.CountScoringRegistrationsParams{ActivityID: &id, Search: search, State: state})
		if err != nil {
			return result, err
		}
	}
	for _, row := range rows {
		result.Total = row.Total
		d, err := decode(row.ScoringData)
		if err != nil {
			return result, err
		}
		e := Entry{RegistrationID: row.ID, Name: row.Name, Data: d}
		if d != nil && r != nil {
			calculated, err := Calculate(r.Definition, d.Draft)
			if err != nil {
				return result, err
			}
			e.Result = &calculated
		}
		result.Entries = append(result.Entries, e)
	}
	return result, nil
}
func (s Service) List(ctx context.Context, id int32, search, state string, page, size int32) (Page, error) {
	switch state {
	case "", "unscored", "incomplete", "complete", "published", "changed":
	default:
		return Page{}, domain.Fail(422, "INVALID_SCORING_STATE")
	}
	r, err := s.Rubric(ctx, id)
	if err != nil {
		return Page{}, err
	}
	return list(ctx, dbgen.New(s.Pool), r, id, search, state, page, size)
}
