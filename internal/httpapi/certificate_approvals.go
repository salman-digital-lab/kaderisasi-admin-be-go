package httpapi

import (
	"encoding/json"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/certificate"
	"kaderisasi/admin/internal/domain"
	"net/http"
	"strconv"
)

func approvalInput[T any](r *http.Request) (T, error) {
	var out T
	raw, err := json.Marshal(requestData(r))
	if err == nil {
		err = json.Unmarshal(raw, &out)
	}
	if err != nil {
		return out, domain.Fail(422, "INVALID_APPROVAL_REQUEST")
	}
	return out, nil
}
func (s *Server) registerCertificateApprovals() {
	service := certificate.Issuance{Pool: s.Pool, Location: s.Config.Location, Logger: s.Logger}
	const controller = "certificate_approvals_controller"
	s.register(controller, "signers", func(w http.ResponseWriter, r *http.Request) error {
		data, err := service.Signers(r.Context())
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", data)
		return nil
	})
	s.register(controller, "create", func(w http.ResponseWriter, r *http.Request) error {
		data, err := approvalInput[certificate.ApprovalRequestInput](r)
		if err != nil {
			return err
		}
		out, err := service.RequestApprovals(r.Context(), data, actor(r).ID)
		if err != nil {
			return err
		}
		reply(w, 200, "CERTIFICATE_APPROVALS_REQUESTED", out)
		return nil
	})
	s.register(controller, "decide", func(w http.ResponseWriter, r *http.Request) error {
		data, err := approvalInput[certificate.ApprovalDecisionInput](r)
		if err != nil {
			return err
		}
		user := actor(r)
		permission := "certificate.approve"
		if data.Action == "cancel" {
			permission = "certificate.issue"
		}
		if !auth.ForRole(user.RoleCode, user.IsActive).Allows(permission) {
			return domain.Fail(403, "FORBIDDEN")
		}
		out, err := service.DecideApprovals(r.Context(), data, user.ID)
		if err != nil {
			return err
		}
		reply(w, 200, "CERTIFICATE_APPROVALS_PROCESSED", out)
		return nil
	})
	s.register(controller, "index", func(w http.ResponseWriter, r *http.Request) error {
		var activityID *int32
		if value := r.URL.Query().Get("activity_id"); value != "" {
			id, err := strconv.ParseInt(value, 10, 32)
			if err != nil || id <= 0 {
				return domain.Fail(422, "INVALID_ACTIVITY_ID")
			}
			v := int32(id)
			activityID = &v
		}
		var status *string
		if value := r.URL.Query().Get("status"); value != "" {
			if value != "pending" && value != "approved" && value != "rejected" && value != "cancelled" {
				return domain.Fail(422, "INVALID_APPROVAL_STATUS")
			}
			status = &value
		}
		page, size := boundedPageParams(r, 20)
		out, err := service.Approvals(r.Context(), actor(r).ID, activityID, status, page, size)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", out)
		return nil
	})
	s.register(controller, "show", func(w http.ResponseWriter, r *http.Request) error {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
		if err != nil || id <= 0 {
			return domain.Fail(422, "INVALID_APPROVAL_ID")
		}
		out, err := service.Approval(r.Context(), int32(id), actor(r).ID)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", out)
		return nil
	})
}
