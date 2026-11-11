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

func (s Service) Create(ctx context.Context, actor Actor, data CreateRequest) (Created, error) {
	q := dbgen.New(s.Pool)
	data.Email = NormalizeEmail(data.Email)
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
	if emailTaken(err) {
		return Created{}, domain.Fail(409, "EMAIL_ALREADY_REGISTERED")
	}
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
	changes := diffFields(nil, createdFields(profile))
	emailChange(changes, nil, user.Email)
	changes["member_id"] = AuditChange{Before: null, After: auditValue(user.MemberID)}
	changes["account_status"] = AuditChange{Before: null, After: auditValue(user.AccountStatus)}
	if password != nil {
		changes["password"] = passwordChanged
	}
	if err = actor.record(ctx, q, AuditCreated, &profile.ID, &user.ID, profile.Name, changes); err != nil {
		return Created{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Created{}, err
	}
	return Created{User: PublicView(user), Profile: createdProfile(profile)}, nil
}

func (s Service) GenerateAccount(ctx context.Context, actor Actor, identifier string, email, password string) error {
	q := dbgen.New(s.Pool)
	email = *NormalizeEmail(&email)
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
	// The activation and its audit row commit together.
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q = dbgen.New(tx)
	err = q.ActivateMemberAccount(ctx, dbgen.ActivateMemberAccountParams{ID: user.ID, Email: &email, Password: &hash})
	if emailTaken(err) {
		return domain.Fail(409, "EMAIL_ALREADY_REGISTERED")
	}
	if err != nil {
		return err
	}
	changes := AuditChanges{"password": passwordChanged, "account_status": {Before: auditValue(user.AccountStatus), After: auditValue("active")}}
	emailChange(changes, user.Email, &email)
	if err = actor.recordAccount(ctx, q, AuditAccountGenerated, user.ID, changes); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Delete removes a profile and records what was removed in the same transaction.
// It returns pgx.ErrNoRows when the profile does not exist.
func (s Service) Delete(ctx context.Context, actor Actor, identifier string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	removed, err := q.DeleteProfileReturning(ctx, identifier)
	if err != nil {
		return err
	}
	if err = actor.record(ctx, q, AuditDeleted, &removed.ID, removed.UserID, removed.Name, diffFields(profileFields(removed), nil)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
