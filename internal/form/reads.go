package form

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"strings"
	"time"
)

type Filters struct {
	Search      *string
	FeatureType *string
	FeatureID   *string
	IsActive    *bool
	Unattached  bool
	Page, Size  float64
}
type Page struct {
	Meta database.Pagination `json:"meta"`
	Data []Response          `json:"data"`
}
type Choice struct {
	ID   int32   `json:"id"`
	Name *string `json:"name"`
}

func countStatement(filters Filters) string {
	conditions := []string{}
	n := 0
	arg := func() string { n++; return fmt.Sprintf("$%d", n) }
	if filters.Unattached {
		conditions = append(conditions, `"feature_id" is null`)
	}
	if filters.Search != nil {
		conditions = append(conditions, `"form_name" ilike `+arg())
	}
	if filters.FeatureType != nil {
		conditions = append(conditions, `"feature_type" = `+arg())
	}
	if filters.FeatureID != nil {
		conditions = append(conditions, `"feature_id" = `+arg())
	}
	if filters.IsActive != nil {
		conditions = append(conditions, `"is_active" = `+arg())
	}
	statement := `select count(*) as "total" from "custom_forms"`
	if len(conditions) > 0 {
		statement += " where " + strings.Join(conditions, " and ")
	}
	return statement
}
func (s Service) List(ctx context.Context, filters Filters, location *time.Location) (Page, error) {
	q := dbgen.New(s.Pool)
	total, err := q.CountFormsFiltered(ctx, dbgen.CountFormsFilteredParams{Search: filters.Search, FeatureType: filters.FeatureType, FeatureID: filters.FeatureID, IsActive: filters.IsActive, Unattached: filters.Unattached})
	result := Page{Meta: database.Meta(total, filters.Page, filters.Size), Data: []Response{}}
	if err != nil {
		return result, database.LegacyQueryError(err, countStatement(filters))
	}
	if total == 0 {
		return result, nil
	}
	limit, offset, err := database.SQLPage(filters.Page, filters.Size)
	if err != nil {
		return result, err
	}
	rows, err := q.ListFormsFiltered(ctx, dbgen.ListFormsFilteredParams{Search: filters.Search, FeatureType: filters.FeatureType, FeatureID: filters.FeatureID, IsActive: filters.IsActive, Unattached: filters.Unattached, PageSize: limit, PageOffset: offset})
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		result.Data = append(result.Data, View(row, location))
	}
	return result, nil
}
func (s Service) Show(ctx context.Context, id string, location *time.Location) (Response, error) {
	row, err := dbgen.New(s.Pool).FormByIdentifier(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Response{}, domain.Fail(404, "CUSTOM_FORM_NOT_FOUND")
	}
	return View(row, location), database.LegacyQueryError(err, `select * from "custom_forms" where "id" = $1 limit $2`)
}
func (s Service) ByFeature(ctx context.Context, kind, id string, location *time.Location) (*Response, error) {
	row, err := dbgen.New(s.Pool).ActiveFormByFeature(ctx, dbgen.ActiveFormByFeatureParams{FeatureType: kind, Identifier: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, database.LegacyQueryError(err, `select * from "custom_forms" where "feature_type" = $1 and "feature_id" = $2 and "is_active" = $3 order by "updated_at" desc, "id" desc limit $4`)
	}
	response := View(row, location)
	return &response, nil
}
func (s Service) Available(ctx context.Context, clubs bool, current *string) ([]Choice, error) {
	q := dbgen.New(s.Pool)
	result := []Choice{}
	var err error
	if clubs {
		var rows []dbgen.FormAvailableClubsRow
		rows, err = q.FormAvailableClubs(ctx, current)
		for _, row := range rows {
			name := row.Name
			result = append(result, Choice{ID: row.ID, Name: &name})
		}
	} else {
		var rows []dbgen.FormAvailableActivitiesRow
		rows, err = q.FormAvailableActivities(ctx, current)
		for _, row := range rows {
			result = append(result, Choice{ID: row.ID, Name: &row.Name})
		}
	}
	statement := `select "feature_id" from "custom_forms" where "feature_type" = $1 and "feature_id" is not null`
	if current != nil {
		statement += ` and not "id" = $2`
	}
	return result, database.LegacyQueryError(err, statement)
}
