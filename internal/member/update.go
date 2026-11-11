package member

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"reflect"
	"strconv"
	"unicode/utf16"
)

func (s Service) UpdateCredentials(ctx context.Context, identifier string, data CredentialRequest) (PublicResponse, error) {
	q := dbgen.New(s.Pool)
	user, err := q.PublicUserByIdentifier(ctx, identifier)
	err = database.LegacyQueryError(err, `select * from "public_users" where "id" = $1 limit $2`)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicResponse{}, domain.Fail(404, "USER_NOT_FOUND")
	}
	if err != nil {
		return PublicResponse{}, err
	}
	data.Email = NormalizeEmail(data.Email)
	if text(data.Email) != "" {
		_, err = q.OtherPublicUserByEmail(ctx, dbgen.OtherPublicUserByEmailParams{ID: user.ID, Email: data.Email})
		if err == nil {
			return PublicResponse{}, domain.Fail(409, "EMAIL_ALREADY_REGISTERED")
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return PublicResponse{}, err
		}
	}
	if text(data.Password) != "" {
		hash, err := auth.HashPassword(*data.Password)
		if err != nil {
			return PublicResponse{}, err
		}
		data.Password = &hash
	}
	// The email change and the cancelled self-service request commit together.
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return PublicResponse{}, err
	}
	defer tx.Rollback(ctx)
	q = dbgen.New(tx)
	updated, err := q.UpdateMemberCredentials(ctx, dbgen.UpdateMemberCredentialsParams{ID: user.ID, Email: data.Email, Password: data.Password})
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicView(user), nil
	}
	if emailTaken(err) {
		return PublicResponse{}, domain.Fail(409, "EMAIL_ALREADY_REGISTERED")
	}
	if err != nil {
		return PublicResponse{}, err
	}
	if text(user.Email) != text(updated.Email) {
		if err := q.CancelOpenEmailChanges(ctx, user.ID); err != nil {
			return PublicResponse{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return PublicResponse{}, err
	}
	return PublicView(updated), nil
}

func jsonField[T any](value *T) []byte {
	if value == nil {
		return nil
	}
	// All DTO fields have JSON-native types and have already passed validation.
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

// Existing JSONB values can be objects, arrays or legacy JSON strings. Preserve
// JavaScript object-spread semantics when merging a partial extra_data object.
func spreadJSON(raw []byte) map[string]json.RawMessage {
	result := map[string]json.RawMessage{}
	if json.Unmarshal(raw, &result) == nil && result != nil {
		return result
	}
	result = map[string]json.RawMessage{}
	var values []json.RawMessage
	if json.Unmarshal(raw, &values) == nil {
		for i, value := range values {
			result[strconv.Itoa(i)] = value
		}
		return result
	}
	var value string
	if json.Unmarshal(raw, &value) == nil {
		for i, unit := range utf16.Encode([]rune(value)) {
			result[strconv.Itoa(i)] = json.RawMessage(fmt.Sprintf(`"\u%04x"`, unit))
		}
	}
	return result
}
func mergeExtra(current, incoming []byte) []byte {
	values := spreadJSON(current)
	for key, value := range spreadJSON(incoming) {
		values[key] = value
	}
	raw, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	return raw
}

func (s Service) UpdateProfile(ctx context.Context, identifier string, data ProfileUpdate) (ProfileResponse, error) {
	q := dbgen.New(s.Pool)
	original, err := q.ProfileByIdentifier(ctx, identifier)
	err = database.LegacyQueryError(err, `select * from "profiles" where "id" = $1 limit $2`)
	if err != nil {
		return ProfileResponse{}, err
	}
	if data.EducationHistory != nil {
		names := make([]string, 0, len(*data.EducationHistory))
		for _, entry := range *data.EducationHistory {
			names = append(names, entry.Institution)
		}
		listed, err := q.UniversityNamesIn(ctx, names)
		if err != nil {
			return ProfileResponse{}, err
		}
		if unlisted := UnlistedInstitution(*data.EducationHistory, listed, StoredInstitutions(original.EducationHistory)); unlisted != nil {
			return ProfileResponse{}, unlisted
		}
	}
	// The legacy workflow saves a password separately from the profile update.
	// Preserve that transaction boundary and ignore a missing associated user.
	if text(data.Password) != "" && original.UserID != nil {
		hash, err := auth.HashPassword(*data.Password)
		if err != nil {
			return ProfileResponse{}, err
		}
		if err = q.UpdateMemberPassword(ctx, dbgen.UpdateMemberPasswordParams{ID: *original.UserID, Password: &hash}); err != nil {
			return ProfileResponse{}, err
		}
	}
	params := dbgen.UpdateMemberProfileParams{ID: original.ID, Name: data.Name, Gender: data.Gender, PersonalID: data.PersonalID, Whatsapp: data.Whatsapp, Line: data.Line, Instagram: data.Instagram, Tiktok: data.Tiktok, Linkedin: data.Linkedin, ProvinceID: number(data.ProvinceID), CityID: number(data.CityID), Level: number(data.Level), BirthDate: data.BirthDate, OriginProvinceID: number(data.OriginProvinceID), OriginCityID: number(data.OriginCityID), Country: data.Country, Badges: jsonField(data.Badges), EducationHistory: jsonField(data.EducationHistory), WorkHistory: jsonField(data.WorkHistory)}
	if data.Badges != nil && reflect.DeepEqual(normalizeBadges(original.Badges), *data.Badges) {
		params.Badges = nil
	}
	if data.ExtraData != nil {
		params.ExtraData = mergeExtra(original.ExtraData, jsonField(data.ExtraData))
	}
	updated, err := q.UpdateMemberProfile(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		updated = original
		err = nil
	}
	if err != nil {
		return ProfileResponse{}, err
	}
	view := ProfileView(updated)
	// A merged string date is returned as submitted by Lucid; a later read uses
	// the PostgreSQL date representation instead.
	if data.BirthDate != nil {
		view.BirthDate = data.BirthDate
	}
	return view, nil
}

func (s Service) UpdateRegionalAssignment(ctx context.Context, identifier string, data RegionalRequest) error {
	q := dbgen.New(s.Pool)
	original, err := q.ProfileByIdentifier(ctx, identifier)
	err = database.LegacyQueryError(err, `select * from "profiles" where "id" = $1 limit $2`)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "PROFILE_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	_, err = q.UpdateMemberProfile(ctx, dbgen.UpdateMemberProfileParams{ID: original.ID, ExtraData: mergeExtra(original.ExtraData, jsonField(&data))})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}
