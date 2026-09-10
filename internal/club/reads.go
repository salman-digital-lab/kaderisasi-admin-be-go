package club

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/form"
)

type Filters struct {
	Search             string
	ClubType           *string
	IsShow             *bool
	IsRegistrationOpen *bool
	Page, Size         float64
}
type Summary struct {
	dbgen.ListClubsFilteredRow
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}
type Page struct {
	Meta database.Pagination `json:"meta"`
	Data []Summary           `json:"data"`
}
type Detail struct {
	Response
	AttachedCustomForm *form.Response `json:"attachedCustomForm"`
}

func (s Service) List(ctx context.Context, filters Filters) (Page, error) {
	q := dbgen.New(s.Pool)
	total, err := q.CountClubsFiltered(ctx, dbgen.CountClubsFilteredParams{Search: filters.Search, ClubType: filters.ClubType, IsShow: filters.IsShow, IsRegistrationOpen: filters.IsRegistrationOpen})
	result := Page{Meta: database.Meta(total, filters.Page, filters.Size), Data: []Summary{}}
	if err != nil || total == 0 {
		return result, err
	}
	limit, offset, err := database.SQLPage(filters.Page, filters.Size)
	if err != nil {
		return result, err
	}
	rows, err := q.ListClubsFiltered(ctx, dbgen.ListClubsFilteredParams{Search: filters.Search, ClubType: filters.ClubType, IsShow: filters.IsShow, IsRegistrationOpen: filters.IsRegistrationOpen, PageSize: limit, PageOffset: offset})
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		result.Data = append(result.Data, Summary{ListClubsFilteredRow: row, CreatedAt: domain.ModelTimestamp(row.CreatedAt, s.location()), UpdatedAt: domain.ModelTimestamp(row.UpdatedAt, s.location())})
	}
	return result, nil
}
func (s Service) Show(ctx context.Context, id string) (Detail, error) {
	q := dbgen.New(s.Pool)
	row, err := q.ClubByIdentifier(ctx, id)
	if err != nil {
		return Detail{}, lookupError(err, false)
	}
	result := Detail{Response: s.view(row)}
	attached, err := q.LatestClubForm(ctx, &row.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	view := form.View(attached, s.location())
	result.AttachedCustomForm = &view
	return result, nil
}
