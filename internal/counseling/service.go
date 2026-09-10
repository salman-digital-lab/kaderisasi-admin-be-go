package counseling

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	Pool     *pgxpool.Pool
	Location *time.Location
}

func eligibleCounselor(roleCode *string, active bool) bool {
	return active && auth.ForRole(roleCode, true).Allows("counseling.manage")
}

func (s Service) List(ctx context.Context, filters Filters) (Page, error) {
	q := dbgen.New(s.Pool)
	total, err := q.CountCounseling(ctx, dbgen.CountCounselingParams{Status: filters.Status, Name: filters.Name, Gender: filters.Gender, AdminName: filters.AdminName})
	result := Page{Meta: database.Meta(total, filters.Page, filters.Size), Data: []Detail{}}
	if err != nil {
		return result, database.LegacyQueryError(err, filterStatement(filters, true))
	}
	if total == 0 {
		return result, nil
	}
	size, offset, err := database.SQLPage(filters.Page, filters.Size)
	if err != nil {
		return result, err
	}
	rows, err := q.ListCounseling(ctx, dbgen.ListCounselingParams{Status: filters.Status, Name: filters.Name, Gender: filters.Gender, AdminName: filters.AdminName, PageSize: size, PageOffset: offset})
	if err != nil {
		return result, database.LegacyQueryError(err, filterStatement(filters, false))
	}
	for _, row := range rows {
		detail, err := details(row.RuangCurhat, row.PublicUser, row.Profile, row.AdminUser, nil, s.Location)
		if err != nil {
			return result, err
		}
		result.Data = append(result.Data, detail)
	}
	return result, nil
}
func (s Service) Show(ctx context.Context, id string) (Detail, error) {
	q := dbgen.New(s.Pool)
	record, err := q.CounselingByIdentifier(ctx, id)
	if err != nil {
		return Detail{}, database.LegacyQueryError(err, `select * from "ruang_curhats" where "id" = $1 limit $2`)
	}
	row, err := q.CounselingDetails(ctx, record.ID)
	if err != nil {
		return Detail{}, err
	}
	return details(row.RuangCurhat, row.PublicUser, row.Profile, row.AdminUser, row.University, s.Location)
}
func (s Service) CounselorOptions(ctx context.Context) ([]CounselorOption, error) {
	rows, err := dbgen.New(s.Pool).ListCounselingAdministrators(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]CounselorOption, 0, len(rows))
	for _, row := range rows {
		if !eligibleCounselor(row.RoleCode, true) {
			continue
		}
		result = append(result, CounselorOption{ID: row.ID, Email: row.Email, DisplayName: row.DisplayName})
	}
	return result, nil
}
func number(value *int32) *string {
	if value == nil {
		return nil
	}
	text := strconv.FormatInt(int64(*value), 10)
	return &text
}
func (s Service) Update(ctx context.Context, id string, input Input) (Response, error) {
	q := dbgen.New(s.Pool)
	row, err := q.CounselingByIdentifier(ctx, id)
	if err != nil {
		return Response{}, database.LegacyQueryError(err, `select * from "ruang_curhats" where "id" = $1 limit $2`)
	}
	params := dbgen.UpdateCounselingParams{ID: row.ID, CounselorID: number(row.CounselorID), Status: number(row.Status), AdditionalNotes: row.AdditionalNotes}
	if input.CounselorID != nil {
		value := input.CounselorID.String()
		candidate, candidateErr := q.FindAdminByIdentifier(ctx, value)
		if errors.Is(candidateErr, pgx.ErrNoRows) || candidateErr == nil && !eligibleCounselor(candidate.RoleCode, candidate.IsActive) {
			return Response{}, domain.Fail(422, "INVALID_COUNSELOR")
		}
		if candidateErr != nil {
			return Response{}, candidateErr
		}
		params.CounselorID = &value
	}
	if input.Status != nil {
		value := input.Status.String()
		params.Status = &value
	}
	if input.AdditionalNotes != nil {
		params.AdditionalNotes = input.AdditionalNotes
	}
	updated, err := q.UpdateCounseling(ctx, params)
	if err != nil {
		columns := []string{}
		if !reflect.DeepEqual(number(row.CounselorID), params.CounselorID) {
			columns = append(columns, "counselor_id")
		}
		if !reflect.DeepEqual(number(row.Status), params.Status) {
			columns = append(columns, "status")
		}
		if !reflect.DeepEqual(row.AdditionalNotes, params.AdditionalNotes) {
			columns = append(columns, "additional_notes")
		}
		columns = append(columns, "updated_at")
		setters := []string{}
		for i, column := range columns {
			setters = append(setters, fmt.Sprintf(`"%s" = $%d`, column, i+1))
		}
		statement := `update "ruang_curhats" set ` + strings.Join(setters, ", ") + fmt.Sprintf(` where "id" = $%d`, len(columns)+1)
		return Response{}, database.LegacyQueryError(err, statement)
	}
	return view(updated, s.Location), nil
}
func filterStatement(filters Filters, count bool) string {
	conditions := []string{}
	n := 0
	arg := func() string { n++; return fmt.Sprintf("$%d", n) }
	if filters.Status != nil {
		conditions = append(conditions, `"status" = `+arg())
	}
	if filters.Name != nil || filters.Gender != nil {
		profile := []string{}
		if filters.Name != nil {
			profile = append(profile, `"name" ilike `+arg())
		}
		if filters.Gender != nil {
			profile = append(profile, `"gender" = `+arg())
		}
		conditions = append(conditions, `exists (select * from "public_users" where (exists (select * from "profiles" where (`+strings.Join(profile, " and ")+`) and ("public_users"."id" = "profiles"."user_id"))) and ("public_users"."id" = "ruang_curhats"."user_id"))`)
	}
	if filters.AdminName != nil {
		conditions = append(conditions, `exists (select * from "admin_users" where ("display_name" ilike `+arg()+`) and ("admin_users"."id" = "ruang_curhats"."counselor_id"))`)
	}
	selectExpr := "*"
	if count {
		selectExpr = `count(*) as "total"`
	}
	statement := `select ` + selectExpr + ` from "ruang_curhats"`
	if len(conditions) > 0 {
		statement += " where " + strings.Join(conditions, " and ")
	}
	if !count {
		statement += ` order by "created_at" desc limit ` + arg()
		if filters.Page != 1 {
			statement += " offset " + arg()
		}
	}
	return statement
}
