package member

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"strings"
)

type Service struct{ Pool *pgxpool.Pool }

func normalizeBadges(raw []byte) []string {
	normalized := []string{}
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err == nil {
		for _, value := range values {
			var text string
			if json.Unmarshal(value, &text) == nil {
				normalized = append(normalized, text)
			}
		}
	} else {
		var value string
		if json.Unmarshal(raw, &value) == nil {
			value = strings.TrimSpace(value)
			if value != "" {
				if json.Unmarshal([]byte(value), &values) == nil {
					for _, entry := range values {
						var text string
						if json.Unmarshal(entry, &text) == nil {
							normalized = append(normalized, text)
						}
					}
				} else {
					normalized = append(normalized, value)
				}
			}
		}
	}
	return normalized
}

func (s Service) Create(ctx context.Context, data CreateRequest) (Created, error) {
	q := dbgen.New(s.Pool)
	if text(data.Email) != "" {
		_, err := q.PublicUserByEmail(ctx, data.Email)
		if err == nil {
			return Created{}, domain.Fail(409, "EMAIL_ALREADY_REGISTERED")
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Created{}, err
		}
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Created{}, err
	}
	defer tx.Rollback(ctx)
	q = dbgen.New(tx)
	var password *string
	status := "no_account"
	if value := text(data.Password); value != "" {
		hash, err := auth.HashPassword(value)
		if err != nil {
			return Created{}, err
		}
		password = &hash
		if text(data.Email) != "" {
			status = "active"
		}
	}
	user, err := q.CreateMemberUser(ctx, dbgen.CreateMemberUserParams{Email: data.Email, Password: password, AccountStatus: status})
	if err != nil {
		return Created{}, err
	}
	memberID := text(data.MemberID)
	if memberID != "" {
		_, err = q.PublicUserByMemberNumber(ctx, &memberID)
		if err == nil {
			return Created{}, domain.Fail(409, "MEMBER_ID_ALREADY_EXISTS")
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Created{}, err
		}
	} else {
		memberID = fmt.Sprintf("%08d", user.ID)
	}
	user, err = q.SetMemberNumber(ctx, dbgen.SetMemberNumberParams{ID: user.ID, MemberID: &memberID})
	if err != nil {
		return Created{}, err
	}
	profile, err := q.CreateMemberProfile(ctx, dbgen.CreateMemberProfileParams{UserID: &user.ID, Name: data.Name, Gender: data.Gender, PersonalID: data.PersonalID, Whatsapp: data.Whatsapp, Instagram: data.Instagram, Tiktok: data.Tiktok, Linkedin: data.Linkedin, Line: data.Line, BirthDate: data.BirthDate, ProvinceID: number(data.ProvinceID), CityID: number(data.CityID), Country: data.Country})
	if err != nil {
		return Created{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Created{}, err
	}
	return Created{User: PublicView(user), Profile: createdProfile(profile)}, nil
}

func (s Service) GenerateAccount(ctx context.Context, identifier string, email, password string) error {
	q := dbgen.New(s.Pool)
	user, err := q.PublicUserByIdentifier(ctx, identifier)
	err = database.LegacyQueryError(err, `select * from "public_users" where "id" = $1 limit $2`)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "MEMBER_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	if user.AccountStatus == "active" {
		return domain.Fail(400, "ACCOUNT_ALREADY_ACTIVE")
	}
	_, err = q.OtherPublicUserByEmail(ctx, dbgen.OtherPublicUserByEmailParams{Email: &email, ID: user.ID})
	if err == nil {
		return domain.Fail(409, "EMAIL_ALREADY_REGISTERED")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	return q.ActivateMemberAccount(ctx, dbgen.ActivateMemberAccountParams{ID: user.ID, Email: &email, Password: &hash})
}
