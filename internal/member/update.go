package member

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"reflect"
	"strconv"
	"unicode/utf16"
)

func (s Service) UpdateCredentials(ctx context.Context, id int32, data CredentialRequest) (PublicResponse, error) {
	q := dbgen.New(s.Pool)
	user, err := q.PublicUserByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicResponse{}, domain.Fail(404, "USER_NOT_FOUND")
	}
	if err != nil {
		return PublicResponse{}, err
	}
	if text(data.Email) != "" {
		_, err = q.OtherPublicUserByEmail(ctx, dbgen.OtherPublicUserByEmailParams{ID: id, Email: data.Email})
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
	updated, err := q.UpdateMemberCredentials(ctx, dbgen.UpdateMemberCredentialsParams{ID: id, Email: data.Email, Password: data.Password})
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicView(user), nil
	}
	return PublicView(updated), err
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

func (s Service) UpdateProfile(ctx context.Context, id int32, data ProfileUpdate) (ProfileResponse, error) {
	q := dbgen.New(s.Pool)
	original, err := q.ProfileByID(ctx, id)
	if err != nil {
		return ProfileResponse{}, err
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
	params := dbgen.UpdateMemberProfileParams{ID: id, Name: data.Name, Gender: data.Gender, PersonalID: data.PersonalID, Whatsapp: data.Whatsapp, Line: data.Line, Instagram: data.Instagram, Tiktok: data.Tiktok, Linkedin: data.Linkedin, ProvinceID: number(data.ProvinceID), CityID: number(data.CityID), Level: number(data.Level), BirthDate: data.BirthDate, OriginProvinceID: number(data.OriginProvinceID), OriginCityID: number(data.OriginCityID), Country: data.Country, Badges: jsonField(data.Badges), EducationHistory: jsonField(data.EducationHistory), WorkHistory: jsonField(data.WorkHistory)}
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

func (s Service) UpdateRegionalAssignment(ctx context.Context, id int32, data RegionalRequest) error {
	q := dbgen.New(s.Pool)
	original, err := q.ProfileByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "PROFILE_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	_, err = q.UpdateMemberProfile(ctx, dbgen.UpdateMemberProfileParams{ID: id, ExtraData: mergeExtra(original.ExtraData, jsonField(&data))})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}
