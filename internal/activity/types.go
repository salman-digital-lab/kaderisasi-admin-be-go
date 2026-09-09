package activity

import (
	"encoding/json"
	"kaderisasi/admin/internal/club"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"time"
)

type Response struct {
	dbgen.Activity
	AdditionalConfig json.RawMessage `json:"additional_config"`
	CreatedAt        *string         `json:"created_at"`
	UpdatedAt        *string         `json:"updated_at"`
}
type Detail struct {
	Response
	Club *club.Response `json:"club"`
}
type Summary struct {
	dbgen.ListActivitiesFilteredRow
	Club *club.Response `json:"club"`
}

func View(row dbgen.Activity) Response {
	return Response{Activity: row, AdditionalConfig: row.AdditionalConfig, CreatedAt: domain.ModelTimestamp(row.CreatedAt, time.Local), UpdatedAt: domain.ModelTimestamp(row.UpdatedAt, time.Local)}
}

// FromRelation decodes JSONB columns as JSON, rather than Go's []byte base64 encoding.
func FromRelation(raw []byte) (*Response, error) {
	var row *struct {
		dbgen.Activity
		AdditionalConfig json.RawMessage `json:"additional_config"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
	}
	if row == nil {
		return nil, nil
	}
	row.Activity.AdditionalConfig = row.AdditionalConfig
	view := View(row.Activity)
	return &view, nil
}
