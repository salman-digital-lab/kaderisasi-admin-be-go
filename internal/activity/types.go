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
	return Response{Activity: row, AdditionalConfig: row.AdditionalConfig, CreatedAt: domain.Timestamp(row.CreatedAt, time.Local), UpdatedAt: domain.Timestamp(row.UpdatedAt, time.Local)}
}
