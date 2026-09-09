package httpapi

import (
	"github.com/jackc/pgx/v5/pgtype"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
	"time"
)

type adminCreateRequest struct {
	DisplayName string  `json:"displayName"`
	Email       string  `json:"email"`
	Password    string  `json:"password"`
	RoleCode    *string `json:"role_code"`
}

type adminUpdateRequest struct {
	RoleCode domain.Optional[string] `json:"role_code"`
	IsActive domain.Optional[bool]   `json:"isActive"`
}

type passwordRequest struct {
	Password string `json:"password"`
}

type adminIdentityResponse struct {
	Provider   string  `json:"provider"`
	Email      string  `json:"email"`
	LastUsedAt *string `json:"last_used_at"`
	CreatedAt  *string `json:"created_at"`
}

type adminResponse struct {
	ID                    int32                   `json:"id"`
	Email                 string                  `json:"email"`
	NormalizedEmail       string                  `json:"normalized_email"`
	DisplayName           *string                 `json:"display_name"`
	CreatedAt             *string                 `json:"created_at"`
	UpdatedAt             *string                 `json:"updated_at"`
	IsActive              bool                    `json:"is_active"`
	RoleCode              *string                 `json:"role_code"`
	Role                  *auth.AssignedRole      `json:"role"`
	EffectivePermissions  []string                `json:"effective_permissions"`
	IsSuperAdmin          bool                    `json:"is_super_admin"`
	AuthenticationMethods []string                `json:"authentication_methods"`
	GoogleLinked          bool                    `json:"google_linked"`
	Identities            []adminIdentityResponse `json:"identities"`
}

type adminPage struct {
	Meta database.Pagination `json:"meta"`
	Data []adminResponse     `json:"data"`
}

func timestamp(value pgtype.Timestamptz, location *time.Location) *string {
	if !value.Valid {
		return nil
	}
	if location == nil {
		location = time.Local
	}
	text := value.Time.In(location).Format("2006-01-02T15:04:05.000Z07:00")
	return &text
}
