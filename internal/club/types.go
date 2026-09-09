package club

import (
	"encoding/json"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"time"
)

type Response struct {
	dbgen.Club
	Media            json.RawMessage `json:"media"`
	RegistrationInfo json.RawMessage `json:"registration_info"`
	CreatedAt        *string         `json:"created_at"`
	UpdatedAt        *string         `json:"updated_at"`
}

func View(row dbgen.Club) Response {
	return Response{Club: row, Media: row.Media, RegistrationInfo: row.RegistrationInfo, CreatedAt: domain.ModelTimestamp(row.CreatedAt, time.Local), UpdatedAt: domain.ModelTimestamp(row.UpdatedAt, time.Local)}
}
func FromRelation(raw []byte) (*Response, error) {
	var row *struct {
		dbgen.Club
		Media            json.RawMessage `json:"media"`
		RegistrationInfo json.RawMessage `json:"registration_info"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
	}
	if row == nil {
		return nil, nil
	}
	row.Club.Media = row.Media
	row.Club.RegistrationInfo = row.RegistrationInfo
	view := View(row.Club)
	return &view, nil
}
