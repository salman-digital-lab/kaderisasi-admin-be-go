package httpapi

import (
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"time"
)

type ticketCreateRequest struct {
	RoleCode string `json:"role_code"`
	Reason   string `json:"reason"`
}
type ticketRejectRequest struct {
	Reason string `json:"rejection_reason"`
}

type ticketResponse struct {
	ID                    int32   `json:"id"`
	Number                string  `json:"number"`
	Status                string  `json:"status"`
	Resolution            *string `json:"resolution"`
	RequesterAdminUserID  int32   `json:"requester_admin_user_id"`
	RequestedRoleCode     string  `json:"requested_role_code"`
	Reason                string  `json:"reason"`
	RejectionReason       *string `json:"rejection_reason"`
	ResolvedByAdminUserID *int32  `json:"resolved_by_admin_user_id"`
	ResolvedAt            *string `json:"resolved_at"`
	CancelledAt           *string `json:"cancelled_at"`
	CreatedAt             *string `json:"created_at"`
	UpdatedAt             *string `json:"updated_at"`
	RoleName              string  `json:"role_name"`
}

type ticketDetailResponse struct {
	ticketResponse
	RequesterName  *string `json:"requester_name"`
	RequesterEmail string  `json:"requester_email"`
	ReviewerName   *string `json:"reviewer_name"`
}

type ticketReviewResponse struct {
	ticketResponse
	RequesterName  *string `json:"requester_name"`
	RequesterEmail string  `json:"requester_email"`
	RequesterID    int32   `json:"requester_id"`
}

func ticketView(row dbgen.Ticket) ticketResponse {
	name := row.RequestedRoleCode
	if role := auth.RoleByCode(row.RequestedRoleCode); role != nil {
		name = role.Name
	}
	return ticketResponse{ID: row.ID, Number: row.Number, Status: row.Status, Resolution: row.Resolution, RequesterAdminUserID: row.RequesterAdminUserID, RequestedRoleCode: row.RequestedRoleCode, Reason: row.Reason, RejectionReason: row.RejectionReason, ResolvedByAdminUserID: row.ResolvedByAdminUserID, ResolvedAt: timestamp(row.ResolvedAt, time.UTC), CancelledAt: timestamp(row.CancelledAt, time.UTC), CreatedAt: timestamp(row.CreatedAt, time.UTC), UpdatedAt: timestamp(row.UpdatedAt, time.UTC), RoleName: name}
}
