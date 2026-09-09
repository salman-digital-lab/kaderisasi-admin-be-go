package form

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
	"reflect"
	"slices"
)

type Service struct{ Pool *pgxpool.Pool }

func Canonical(data database.Object) database.Object {
	out := database.Object{}
	for key, value := range data {
		out[database.Snake(key)] = value
	}
	return out
}
func ClubID(state database.Object) int32 {
	if state.String("feature_type") != "club_registration" {
		return 0
	}
	return state.ID("feature_id")
}
func UpdatedClubID(current, changes database.Object) int32 {
	merged := database.Object{}
	for key, value := range current {
		merged[key] = value
	}
	for key, value := range changes {
		merged[key] = value
	}
	return ClubID(merged)
}
func EqualJSON(a, b []byte) bool {
	var av, bv interface{}
	if json.Unmarshal(a, &av) != nil || json.Unmarshal(b, &bv) != nil {
		return false
	}
	return reflect.DeepEqual(av, bv)
}
func ChangesOpen(current, changes database.Object) bool {
	return changes.Has("is_active") && !changes.Bool("is_active") && current.Bool("is_active") || changes.Has("feature_id") && !EqualJSON(changes["feature_id"], current["feature_id"]) || changes.Has("feature_type") && !EqualJSON(changes["feature_type"], current["feature_type"]) || changes.Has("form_schema") && !EqualJSON(changes["form_schema"], current["form_schema"])
}
func lockClubs(ctx context.Context, q database.JSONQueries, ids ...int32) (map[int32]database.Object, error) {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	ids = slices.DeleteFunc(ids, func(id int32) bool { return id == 0 })
	clubs := map[int32]database.Object{}
	if len(ids) == 0 {
		return clubs, nil
	}
	rows, err := q.All(ctx, "SELECT * FROM clubs WHERE id=ANY($1::int[]) ORDER BY id ASC FOR UPDATE", ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		clubs[row.ID("id")] = row
	}
	return clubs, nil
}
func noOtherForm(ctx context.Context, q database.JSONQueries, clubID, formID int32) error {
	count, err := q.Count(ctx, "SELECT id FROM custom_forms WHERE feature_type='club_registration' AND feature_id=$1 AND id<>$2", clubID, formID)
	if err != nil {
		return err
	}
	if count > 0 {
		return domain.Fail(409, "CLUB_ALREADY_HAS_FORM")
	}
	return nil
}
func (s Service) Create(ctx context.Context, payload database.Object) (database.Object, error) {
	data := Canonical(payload)
	id := ClubID(data)
	if id == 0 {
		return (database.JSONQueries{DB: s.Pool}).Insert(ctx, "custom_forms", data)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := database.JSONQueries{DB: tx}
	clubs, err := lockClubs(ctx, q, id)
	if err != nil {
		return nil, err
	}
	if clubs[id] == nil {
		return nil, domain.Fail(404, "CLUB_NOT_FOUND")
	}
	if err = noOtherForm(ctx, q, id, 0); err != nil {
		return nil, err
	}
	row, err := q.Insert(ctx, "custom_forms", data)
	if err != nil {
		return nil, err
	}
	return row, tx.Commit(ctx)
}
func (s Service) Update(ctx context.Context, id int32, payload database.Object, attach bool) (database.Object, error) {
	if id <= 0 {
		return nil, domain.Fail(404, "CUSTOM_FORM_NOT_FOUND")
	}
	data := Canonical(payload)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := database.JSONQueries{DB: tx}
	current, err := q.One(ctx, "SELECT * FROM custom_forms WHERE id=$1 FOR UPDATE", id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.Fail(404, "CUSTOM_FORM_NOT_FOUND")
	}
	if err != nil {
		return nil, err
	}
	if attach && !current.Null("feature_id") {
		return nil, domain.Fail(409, "FORM_ALREADY_ATTACHED")
	}
	currentClub, targetClub := ClubID(current), UpdatedClubID(current, data)
	protect := ChangesOpen(current, data)
	lockCurrent := int32(0)
	if protect {
		lockCurrent = currentClub
	}
	clubs, err := lockClubs(ctx, q, targetClub, lockCurrent)
	if err != nil {
		return nil, err
	}
	if protect && currentClub != 0 && clubs[currentClub].Bool("is_registration_open") {
		return nil, domain.Fail(400, "CLOSE_REGISTRATION_BEFORE_FORM_CHANGE")
	}
	if targetClub != 0 {
		if clubs[targetClub] == nil {
			return nil, domain.Fail(404, "CLUB_NOT_FOUND")
		}
		if err = noOtherForm(ctx, q, targetClub, id); err != nil {
			return nil, err
		}
	}
	row, err := q.Update(ctx, "custom_forms", id, data)
	if err != nil {
		return nil, err
	}
	return row, tx.Commit(ctx)
}
