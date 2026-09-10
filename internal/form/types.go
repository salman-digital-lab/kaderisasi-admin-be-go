package form

import (
	"encoding/json"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"time"
)

type Response struct {
	dbgen.CustomForm
	FormSchema json.RawMessage `json:"form_schema"`
	CreatedAt  *string         `json:"created_at"`
	UpdatedAt  *string         `json:"updated_at"`
}

func View(row dbgen.CustomForm, location *time.Location) Response {
	return Response{CustomForm: row, FormSchema: row.FormSchema, CreatedAt: domain.ModelTimestamp(row.CreatedAt, location), UpdatedAt: domain.ModelTimestamp(row.UpdatedAt, location)}
}
