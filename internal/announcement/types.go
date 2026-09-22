package announcement

import (
	"encoding/base64"
	"encoding/json"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/domain"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

type Audience struct {
	AllMembers       bool     `json:"all_members"`
	MemberIDs        []int32  `json:"member_ids"`
	ActivityIDs      []int32  `json:"activity_ids"`
	ActivityStatuses []string `json:"activity_statuses"`
	ClubIDs          []int32  `json:"club_ids"`
	ClubStatuses     []string `json:"club_statuses"`
	AllAdmins        bool     `json:"all_admins"`
	AdminIDs         []int32  `json:"admin_ids"`
	RoleCodes        []string `json:"role_codes"`
}
type Input struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	LinkLabel *string  `json:"link_label"`
	LinkURL   *string  `json:"link_url"`
	Audience  Audience `json:"audience"`
	Version   int32    `json:"version"`
}
type Preview struct {
	Eligible int32 `json:"eligible"`
	Excluded int32 `json:"excluded"`
	Members  int32 `json:"members"`
	Admins   int32 `json:"admins"`
}
type Cursor struct {
	ID   int32     `json:"id"`
	Time time.Time `json:"time"`
}

func EncodeCursor(id int32, t time.Time) string {
	b, _ := json.Marshal(Cursor{id, t})
	return base64.RawURLEncoding.EncodeToString(b)
}
func DecodeCursor(value string) (Cursor, error) {
	if value == "" {
		return Cursor{}, nil
	}
	var c Cursor
	b, e := base64.RawURLEncoding.DecodeString(value)
	if e != nil || json.Unmarshal(b, &c) != nil || c.ID <= 0 || c.Time.IsZero() {
		return c, domain.Fail(422, "INVALID_CURSOR")
	}
	return c, nil
}
func Validate(in *Input) error {
	in.Title = strings.TrimSpace(in.Title)
	in.Body = strings.TrimSpace(in.Body)
	invalid := func() error { return domain.Fail(422, "INVALID_ANNOUNCEMENT") }
	if in.Title == "" || utf8.RuneCountInString(in.Title) > 160 || in.Body == "" || utf8.RuneCountInString(in.Body) > 10000 {
		return invalid()
	}
	if (in.LinkLabel == nil) != (in.LinkURL == nil) {
		return invalid()
	}
	if in.LinkURL != nil {
		label := strings.TrimSpace(*in.LinkLabel)
		link := strings.TrimSpace(*in.LinkURL)
		u, e := url.Parse(link)
		if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || len(link) > 2048 || label == "" || utf8.RuneCountInString(label) > 100 {
			return invalid()
		}
		in.LinkLabel = &label
		in.LinkURL = &link
	}
	for _, ids := range [][]int32{in.Audience.MemberIDs, in.Audience.AdminIDs, in.Audience.ActivityIDs, in.Audience.ClubIDs} {
		if len(ids) > 1000 {
			return invalid()
		}
		for _, id := range ids {
			if id <= 0 {
				return invalid()
			}
		}
	}
	for _, statuses := range [][]string{in.Audience.ActivityStatuses, in.Audience.ClubStatuses} {
		if len(statuses) > 100 {
			return invalid()
		}
		for _, status := range statuses {
			if strings.TrimSpace(status) == "" || len(status) > 255 {
				return invalid()
			}
		}
	}
	for _, code := range in.Audience.RoleCodes {
		found := false
		for _, role := range auth.Roles() {
			if role.Code == code {
				found = true
			}
		}
		if !found {
			return invalid()
		}
	}
	if len(in.Audience.ClubStatuses) == 0 {
		in.Audience.ClubStatuses = []string{"APPROVED"}
	}
	return nil
}
