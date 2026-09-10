package auth

import (
	"encoding/json"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"time"
)

// AdminModelResponse is the public Lucid relation shape, with no password or
// additional account-management projections.
type AdminModelResponse struct {
	ID              int32   `json:"id"`
	Email           string  `json:"email"`
	NormalizedEmail string  `json:"normalized_email"`
	DisplayName     *string `json:"display_name"`
	CreatedAt       *string `json:"created_at"`
	UpdatedAt       *string `json:"updated_at"`
	RoleCode        *string `json:"role_code"`
	IsActive        bool    `json:"is_active"`
}

func AdminModelFromRelation(raw []byte, location *time.Location) (*AdminModelResponse, error) {
	var row *dbgen.AdminUser
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
	}
	if row == nil {
		return nil, nil
	}
	return &AdminModelResponse{ID: row.ID, Email: row.Email, NormalizedEmail: row.NormalizedEmail, DisplayName: row.DisplayName, RoleCode: row.RoleCode, IsActive: row.IsActive, CreatedAt: domain.ModelTimestamp(row.CreatedAt, location), UpdatedAt: domain.ModelTimestamp(row.UpdatedAt, location)}, nil
}
