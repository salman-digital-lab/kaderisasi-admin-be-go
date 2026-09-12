package club

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/formschema"
	"kaderisasi/admin/internal/storage"
	"time"
)

type Service struct {
	Pool     *pgxpool.Pool
	Location *time.Location
	Storage  storage.Store
}

func (s Service) location() *time.Location {
	if s.Location != nil {
		return s.Location
	}
	return time.Local
}
func (s Service) view(row dbgen.Club) Response {
	response := View(row)
	response.CreatedAt = domain.ModelTimestamp(row.CreatedAt, s.location())
	response.UpdatedAt = domain.ModelTimestamp(row.UpdatedAt, s.location())
	return response
}
func lookupError(err error, locked bool) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "CLUB_NOT_FOUND")
	}
	statement := `select * from "clubs" where "id" = $1 limit $2`
	if locked {
		statement += " for update"
	}
	return database.LegacyQueryError(err, statement)
}
func (s Service) Create(ctx context.Context, data Input) (Created, error) {
	result := Created{}
	media := Media{Items: []Item{}}
	if data.Media != nil {
		media = *data.Media
	}
	if Duplicates(media.Items) {
		return result, domain.Fail(409, "MEDIA_ALREADY_EXISTS")
	}
	kind := "UNIT"
	if data.ClubType != nil {
		kind = *data.ClubType
	}
	raw, err := json.Marshal(media)
	if err != nil {
		return result, err
	}
	row, err := dbgen.New(s.Pool).CreateClub(ctx, dbgen.CreateClubParams{Name: *data.Name, ClubType: kind, Description: data.Description, ShortDescription: data.ShortDescription, Media: raw, StartPeriod: dateValue(data.StartPeriod), EndPeriod: dateValue(data.EndPeriod), RegistrationEndDate: dateValue(data.RegistrationEndDate)})
	if err != nil {
		return result, err
	}
	return Created{ID: row.ID, Name: row.Name, ClubType: row.ClubType, Description: data.Description, ShortDescription: data.ShortDescription, Media: media, StartPeriod: row.StartPeriod, EndPeriod: row.EndPeriod, RegistrationEndDate: row.RegistrationEndDate, CreatedAt: domain.ModelTimestamp(row.CreatedAt, s.location()), UpdatedAt: domain.ModelTimestamp(row.UpdatedAt, s.location())}, nil
}
func (s Service) Update(ctx context.Context, id string, data Input) (Response, error) {
	if data.Media != nil && Duplicates(data.Media.Items) {
		return Response{}, domain.Fail(409, "MEDIA_ALREADY_EXISTS")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Response{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	old, err := q.LockClubByIdentifier(ctx, updateIdentifier(id))
	if err != nil {
		return Response{}, lookupError(err, true)
	}
	if data.IsRegistrationOpen != nil && *data.IsRegistrationOpen {
		active, err := q.ActiveFormByFeature(ctx, dbgen.ActiveFormByFeatureParams{FeatureType: "club_registration", Identifier: database.JSNumber(float64(old.ID))})
		if errors.Is(err, pgx.ErrNoRows) {
			return Response{}, domain.Fail(400, "ACTIVE_CUSTOM_FORM_REQUIRED")
		}
		if err != nil {
			return Response{}, err
		}
		var schema formschema.Schema
		if json.Unmarshal(active.FormSchema, &schema) != nil || !formschema.ValidRouting(schema) {
			return Response{}, domain.Fail(400, "INVALID_FORM_SCHEMA")
		}
		end := old.RegistrationEndDate
		if data.RegistrationEndDate.Present {
			end = dateValue(data.RegistrationEndDate)
		}
		if end.Valid && end.Time.Format("2006-01-02") < time.Now().In(s.location()).Format("2006-01-02") {
			return Response{}, domain.Fail(400, "REGISTRATION_END_DATE_PASSED")
		}
	}
	if data.Name != nil {
		old.Name = *data.Name
	}
	if data.ClubType != nil {
		old.ClubType = *data.ClubType
	}
	if data.Description != nil {
		old.Description = data.Description
	}
	if data.ShortDescription != nil {
		old.ShortDescription = data.ShortDescription
	}
	if data.Media != nil {
		old.Media, err = json.Marshal(data.Media)
		if err != nil {
			return Response{}, err
		}
	}
	if data.StartPeriod.Present {
		old.StartPeriod = dateValue(data.StartPeriod)
	}
	if data.EndPeriod.Present {
		old.EndPeriod = dateValue(data.EndPeriod)
	}
	if data.RegistrationEndDate.Present {
		old.RegistrationEndDate = dateValue(data.RegistrationEndDate)
	}
	if data.IsShow != nil {
		old.IsShow = data.IsShow
	}
	if data.IsRegistrationOpen != nil {
		old.IsRegistrationOpen = data.IsRegistrationOpen
	}
	row, err := q.UpdateClub(ctx, dbgen.UpdateClubParams{ID: old.ID, Name: old.Name, ClubType: old.ClubType, Description: old.Description, ShortDescription: old.ShortDescription, Media: old.Media, StartPeriod: old.StartPeriod, EndPeriod: old.EndPeriod, IsShow: old.IsShow, IsRegistrationOpen: old.IsRegistrationOpen, RegistrationEndDate: old.RegistrationEndDate})
	if err != nil {
		return Response{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Response{}, err
	}
	return s.view(row), nil
}
func (s Service) UpdateRegistrationInfo(ctx context.Context, id string, data RegistrationInfo) (RegistrationInfoResponse, error) {
	q := dbgen.New(s.Pool)
	row, err := q.ClubByIdentifier(ctx, id)
	// This source action catches findOrFail as a generic error, unlike show/update.
	if err != nil {
		return RegistrationInfoResponse{}, database.LegacyQueryError(err, `select * from "clubs" where "id" = $1 limit $2`)
	}
	raw, err := json.Marshal(data)
	if err == nil {
		_, err = q.UpdateClubRegistrationInfo(ctx, dbgen.UpdateClubRegistrationInfoParams{ID: row.ID, RegistrationInfo: raw})
	}
	return RegistrationInfoResponse{Info: data}, err
}
