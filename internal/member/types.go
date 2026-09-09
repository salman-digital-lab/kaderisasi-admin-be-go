package member

import (
	"encoding/json"
	"github.com/jackc/pgx/v5/pgtype"
	"kaderisasi/admin/internal/dbgen"
	"time"
)

type CreateRequest struct {
	Name       string       `json:"name"`
	Email      *string      `json:"email"`
	Password   *string      `json:"password"`
	MemberID   *string      `json:"member_id"`
	Gender     *string      `json:"gender"`
	PersonalID *string      `json:"personal_id"`
	Whatsapp   *string      `json:"whatsapp"`
	Instagram  *string      `json:"instagram"`
	Tiktok     *string      `json:"tiktok"`
	Linkedin   *string      `json:"linkedin"`
	Line       *string      `json:"line"`
	BirthDate  *string      `json:"birth_date"`
	ProvinceID *json.Number `json:"province_id"`
	CityID     *json.Number `json:"city_id"`
	Country    *string      `json:"country"`
}

type PublicResponse struct {
	ID            int32   `json:"id"`
	Email         *string `json:"email"`
	CreatedAt     *string `json:"created_at"`
	UpdatedAt     *string `json:"updated_at"`
	MemberID      *string `json:"member_id"`
	AccountStatus string  `json:"account_status"`
}

type CreatedProfileResponse struct {
	ID         int32   `json:"id"`
	UserID     *int32  `json:"user_id"`
	Name       string  `json:"name"`
	Gender     *string `json:"gender"`
	PersonalID *string `json:"personal_id"`
	Whatsapp   *string `json:"whatsapp"`
	Instagram  *string `json:"instagram"`
	Tiktok     *string `json:"tiktok"`
	Linkedin   *string `json:"linkedin"`
	Line       *string `json:"line"`
	BirthDate  *string `json:"birth_date"`
	ProvinceID *int32  `json:"province_id"`
	CityID     *int32  `json:"city_id"`
	Country    *string `json:"country"`
	CreatedAt  *string `json:"created_at"`
	UpdatedAt  *string `json:"updated_at"`
}

type Created struct {
	User    PublicResponse         `json:"user"`
	Profile CreatedProfileResponse `json:"profile"`
}

func memberTime(value pgtype.Timestamptz) *string {
	if !value.Valid {
		return nil
	}
	text := value.Time.In(time.Local).Format("2006-01-02T15:04:05.000-07:00")
	return &text
}

func PublicView(user dbgen.PublicUser) PublicResponse {
	return PublicResponse{ID: user.ID, Email: user.Email, CreatedAt: memberTime(user.CreatedAt), UpdatedAt: memberTime(user.UpdatedAt), MemberID: user.MemberID, AccountStatus: user.AccountStatus}
}

func createdProfile(row dbgen.CreateMemberProfileRow) CreatedProfileResponse {
	var birthDate *string
	if row.BirthDate.Valid {
		text := row.BirthDate.Time.Format("2006-01-02")
		birthDate = &text
	}
	return CreatedProfileResponse{ID: row.ID, UserID: row.UserID, Name: row.Name, Gender: row.Gender, PersonalID: row.PersonalID, Whatsapp: row.Whatsapp, Instagram: row.Instagram, Tiktok: row.Tiktok, Linkedin: row.Linkedin, Line: row.Line, BirthDate: birthDate, ProvinceID: row.ProvinceID, CityID: row.CityID, Country: row.Country, CreatedAt: memberTime(row.CreatedAt), UpdatedAt: memberTime(row.UpdatedAt)}
}

func text(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func number(value *json.Number) *string {
	if value == nil {
		return nil
	}
	raw := value.String()
	return &raw
}
