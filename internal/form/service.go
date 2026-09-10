package form

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	Pool     *pgxpool.Pool
	Location *time.Location
}

func EqualJSON(a, b []byte) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	var av, bv interface{}
	if json.Unmarshal(a, &av) != nil || json.Unmarshal(b, &bv) != nil {
		return false
	}
	return reflect.DeepEqual(av, bv)
}
func numberText(value *json.Number) *string {
	if value == nil {
		return nil
	}
	text := value.String()
	return &text
}
func storedNumber(value *int32) *string {
	if value == nil {
		return nil
	}
	text := strconv.FormatInt(int64(*value), 10)
	return &text
}
func attachedClub(kind, id *string) *string {
	if kind == nil || *kind != "club_registration" {
		return nil
	}
	return id
}
func currentClub(row dbgen.CustomForm) *string {
	return attachedClub(row.FeatureType, storedNumber(row.FeatureID))
}
func merged(row dbgen.CustomForm, input Input) (dbgen.UpdateFormParams, error) {
	result := dbgen.UpdateFormParams{ID: row.ID, FormName: row.FormName, FormDescription: row.FormDescription, PostSubmissionInfo: row.PostSubmissionInfo, FeatureType: row.FeatureType, FeatureID: storedNumber(row.FeatureID), FormSchema: row.FormSchema, IsActive: row.IsActive}
	if input.FormName != nil {
		result.FormName = *input.FormName
	}
	if input.FormDescription.Present {
		result.FormDescription = input.FormDescription.Value
	}
	if input.PostSubmissionInfo.Present {
		result.PostSubmissionInfo = input.PostSubmissionInfo.Value
	}
	if input.FeatureType != nil {
		result.FeatureType = input.FeatureType
	}
	if input.FeatureID.Present {
		result.FeatureID = numberText(input.FeatureID.Value)
	}
	if input.IsActive != nil {
		result.IsActive = input.IsActive
	}
	var err error
	if input.FormSchema != nil {
		result.FormSchema, err = json.Marshal(input.FormSchema)
	}
	return result, err
}
func changesOpen(row dbgen.CustomForm, next dbgen.UpdateFormParams) bool {
	return row.IsActive != nil && *row.IsActive && next.IsActive != nil && !*next.IsActive || !reflect.DeepEqual(storedNumber(row.FeatureID), next.FeatureID) || !reflect.DeepEqual(row.FeatureType, next.FeatureType) || !EqualJSON(row.FormSchema, next.FormSchema)
}
func lockClubs(ctx context.Context, q *dbgen.Queries, ids ...*string) (map[string]dbgen.Club, error) {
	identifiers := []string{}
	for _, id := range ids {
		if id != nil {
			identifiers = append(identifiers, *id)
		}
	}
	slices.Sort(identifiers)
	identifiers = slices.Compact(identifiers)
	result := map[string]dbgen.Club{}
	if len(identifiers) == 0 {
		return result, nil
	}
	rows, err := q.LockFormClubs(ctx, identifiers)
	// The generated query orders rows numerically before taking locks, matching
	// the source service even when target and current clubs appear in reverse.
	if err != nil {
		params := []string{}
		for i := range identifiers {
			params = append(params, fmt.Sprintf("$%d", i+1))
		}
		return nil, database.LegacyQueryError(err, `select * from "clubs" where "id" in (`+strings.Join(params, ", ")+`) order by "id" asc for update`)
	}
	for _, row := range rows {
		result[strconv.FormatInt(int64(row.ID), 10)] = row
	}
	return result, nil
}
func noOtherForm(ctx context.Context, q *dbgen.Queries, clubID string, formID int32) error {
	exists, err := q.OtherClubFormExists(ctx, dbgen.OtherClubFormExistsParams{Identifier: clubID, FormID: formID})
	if err != nil {
		return err
	}
	if exists {
		return domain.Fail(409, "CLUB_ALREADY_HAS_FORM")
	}
	return nil
}
func (s Service) Create(ctx context.Context, input Input) (Created, error) {
	params := dbgen.CreateFormParams{FormName: *input.FormName, FormDescription: input.FormDescription.Value, PostSubmissionInfo: input.PostSubmissionInfo.Value, FeatureType: input.FeatureType, FeatureID: numberText(input.FeatureID.Value), IsActive: input.IsActive}
	if input.FormSchema != nil {
		var err error
		params.FormSchema, err = json.Marshal(input.FormSchema)
		if err != nil {
			return Created{}, err
		}
	}
	q := dbgen.New(s.Pool)
	var tx pgx.Tx
	if id := attachedClub(params.FeatureType, params.FeatureID); id != nil {
		var err error
		tx, err = s.Pool.Begin(ctx)
		if err != nil {
			return Created{}, err
		}
		defer tx.Rollback(ctx)
		q = q.WithTx(tx)
		clubs, err := lockClubs(ctx, q, id)
		if err != nil {
			return Created{}, err
		}
		if _, ok := clubs[*id]; !ok {
			return Created{}, domain.Fail(404, "CLUB_NOT_FOUND")
		}
		if err = noOtherForm(ctx, q, *id, 0); err != nil {
			return Created{}, err
		}
	}
	row, err := q.CreateForm(ctx, params)
	if err != nil {
		return Created{}, database.LegacyQueryError(err, insertStatement(input))
	}
	if tx != nil {
		if err = tx.Commit(ctx); err != nil {
			return Created{}, err
		}
	}
	return Created{ID: row.ID, FormName: row.FormName, FormDescription: input.FormDescription, PostSubmissionInfo: input.PostSubmissionInfo, FeatureType: input.FeatureType, FeatureID: input.FeatureID, FormSchema: input.FormSchema, IsActive: input.IsActive, CreatedAt: domain.ModelTimestamp(row.CreatedAt, s.Location), UpdatedAt: domain.ModelTimestamp(row.UpdatedAt, s.Location)}, nil
}
func safeIdentifier(raw string) (string, error) {
	id := database.NumberIdentifier(raw)
	value, err := strconv.ParseFloat(id, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 || value > 9007199254740991 || math.Trunc(value) != value {
		return "", domain.Fail(404, "CUSTOM_FORM_NOT_FOUND")
	}
	return id, nil
}
func (s Service) Update(ctx context.Context, rawID string, input Input, attach bool) (Response, error) {
	id, err := safeIdentifier(rawID)
	if err != nil {
		return Response{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Response{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	current, err := q.LockFormByIdentifier(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Response{}, domain.Fail(404, "CUSTOM_FORM_NOT_FOUND")
	}
	if err != nil {
		return Response{}, database.LegacyQueryError(err, `select * from "custom_forms" where "id" = $1 limit $2 for update`)
	}
	if attach && current.FeatureID != nil {
		return Response{}, domain.Fail(409, "FORM_ALREADY_ATTACHED")
	}
	next, err := merged(current, input)
	if err != nil {
		return Response{}, err
	}
	oldClub, newClub := currentClub(current), attachedClub(next.FeatureType, next.FeatureID)
	protect := changesOpen(current, next)
	var protectClub *string
	if protect {
		protectClub = oldClub
	}
	clubs, err := lockClubs(ctx, q, newClub, protectClub)
	if err != nil {
		return Response{}, err
	}
	if protect && oldClub != nil && clubs[*oldClub].IsRegistrationOpen != nil && *clubs[*oldClub].IsRegistrationOpen {
		return Response{}, domain.Fail(400, "CLOSE_REGISTRATION_BEFORE_FORM_CHANGE")
	}
	if newClub != nil {
		if _, ok := clubs[*newClub]; !ok {
			return Response{}, domain.Fail(404, "CLUB_NOT_FOUND")
		}
		if err = noOtherForm(ctx, q, *newClub, current.ID); err != nil {
			return Response{}, err
		}
	}
	row, err := q.UpdateForm(ctx, next)
	if err != nil {
		return Response{}, database.LegacyQueryError(err, updateStatement(current, next))
	}
	if err = tx.Commit(ctx); err != nil {
		return Response{}, err
	}
	return View(row, s.Location), nil
}

// Knex sorts insert columns, while a Lucid save updates dirty attributes in
// model order. These diagnostic statements contain placeholders only.
func insertStatement(input Input) string {
	columns := []string{"created_at", "form_name", "updated_at"}
	if input.FormDescription.Present {
		columns = append(columns, "form_description")
	}
	if input.PostSubmissionInfo.Present {
		columns = append(columns, "post_submission_info")
	}
	if input.FeatureType != nil {
		columns = append(columns, "feature_type")
	}
	if input.FeatureID.Present {
		columns = append(columns, "feature_id")
	}
	if input.FormSchema != nil {
		columns = append(columns, "form_schema")
	}
	if input.IsActive != nil {
		columns = append(columns, "is_active")
	}
	slices.Sort(columns)
	quoted, params := []string{}, []string{}
	for i, column := range columns {
		quoted = append(quoted, `"`+column+`"`)
		params = append(params, fmt.Sprintf("$%d", i+1))
	}
	return `insert into "custom_forms" (` + strings.Join(quoted, ", ") + `) values (` + strings.Join(params, ", ") + `) returning "id"`
}
func updateStatement(current dbgen.CustomForm, next dbgen.UpdateFormParams) string {
	columns := []string{}
	if current.FormName != next.FormName {
		columns = append(columns, "form_name")
	}
	if !reflect.DeepEqual(current.FormDescription, next.FormDescription) {
		columns = append(columns, "form_description")
	}
	if !reflect.DeepEqual(storedNumber(current.FeatureID), next.FeatureID) {
		columns = append(columns, "feature_id")
	}
	if !EqualJSON(current.FormSchema, next.FormSchema) {
		columns = append(columns, "form_schema")
	}
	if !reflect.DeepEqual(current.IsActive, next.IsActive) {
		columns = append(columns, "is_active")
	}
	columns = append(columns, "updated_at")
	if !reflect.DeepEqual(current.FeatureType, next.FeatureType) {
		columns = append(columns, "feature_type")
	}
	if !reflect.DeepEqual(current.PostSubmissionInfo, next.PostSubmissionInfo) {
		columns = append(columns, "post_submission_info")
	}
	setters := []string{}
	for i, column := range columns {
		setters = append(setters, fmt.Sprintf(`"%s" = $%d`, column, i+1))
	}
	return `update "custom_forms" set ` + strings.Join(setters, ", ") + fmt.Sprintf(` where "id" = $%d`, len(columns)+1)
}
