package httpapi

import (
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/access"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"net/http"
)

func (s *Server) ticketReply(w http.ResponseWriter, r *http.Request, status int, msg string, id int32) error {
	row, err := dbgen.New(s.Pool).TicketDetails(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "TICKET_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	reply(w, status, msg, ticketDetailResponse{ticketResponse: ticketView(row.Ticket), RequesterName: row.RequesterName, RequesterEmail: row.RequesterEmail, ReviewerName: row.ReviewerName})
	return nil
}

func (s *Server) registerTickets() {
	q := dbgen.New(s.Pool)
	service := access.Tickets{Pool: s.Pool}
	s.register("access_requests_controller", "store", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := inputAs[ticketCreateRequest](w, r, "createAccessRequestValidator")
		if !ok {
			return nil
		}
		id, err := service.Create(r.Context(), actor(r).ID, data.RoleCode, data.Reason)
		if err != nil {
			return err
		}
		return s.ticketReply(w, r, 201, "ACCESS_REQUEST_CREATED", id)
	})
	s.register("access_requests_controller", "ownIndex", func(w http.ResponseWriter, r *http.Request) error {
		rows, err := q.OwnTickets(r.Context(), actor(r).ID)
		if err != nil {
			return err
		}
		data := make([]ticketResponse, len(rows))
		for i, row := range rows {
			data[i] = ticketView(row)
		}
		reply(w, 200, "GET_DATA_SUCCESS", data)
		return nil
	})
	s.register("access_requests_controller", "ownShow", func(w http.ResponseWriter, r *http.Request) error {
		ticket, err := q.TicketByID(r.Context(), pathID(r, "id"))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Fail(404, "TICKET_NOT_FOUND")
		}
		if err != nil {
			return err
		}
		if ticket.RequesterAdminUserID != actor(r).ID {
			policyDenied(w)
			return nil
		}
		return s.ticketReply(w, r, 200, "GET_DATA_SUCCESS", ticket.ID)
	})
	s.register("access_requests_controller", "reviewIndex", func(w http.ResponseWriter, r *http.Request) error {
		rows, err := q.ReviewTickets(r.Context(), r.URL.Query().Get("status"))
		if err != nil {
			return err
		}
		data := make([]ticketReviewResponse, len(rows))
		for i, row := range rows {
			data[i] = ticketReviewResponse{ticketResponse: ticketView(row.Ticket), RequesterName: row.RequesterName, RequesterEmail: row.RequesterEmail, RequesterID: row.RequesterID}
		}
		reply(w, 200, "GET_DATA_SUCCESS", data)
		return nil
	})
	s.register("access_requests_controller", "reviewShow", func(w http.ResponseWriter, r *http.Request) error {
		return s.ticketReply(w, r, 200, "GET_DATA_SUCCESS", pathID(r, "id"))
	})
	s.register("access_requests_controller", "cancel", func(w http.ResponseWriter, r *http.Request) error {
		id := pathID(r, "id")
		if err := service.Cancel(r.Context(), id, actor(r).ID); err != nil {
			if errors.Is(err, access.ErrTicketAccess) {
				policyDenied(w)
				return nil
			}
			return err
		}
		return s.ticketReply(w, r, 200, "TICKET_CANCELLED", id)
	})
	for _, resolution := range []string{"approved", "rejected"} {
		action, msg := "approve", "TICKET_APPROVED"
		if resolution == "rejected" {
			action, msg = "reject", "TICKET_REJECTED"
		}
		s.register("access_requests_controller", action, func(w http.ResponseWriter, r *http.Request) error {
			var reason *string
			if resolution == "rejected" {
				data, ok := inputAs[ticketRejectRequest](w, r, "rejectTicketValidator")
				if !ok {
					return nil
				}
				reason = &data.Reason
			}
			id := pathID(r, "id")
			if err := service.Resolve(r.Context(), id, actor(r).ID, resolution, reason); err != nil {
				return err
			}
			return s.ticketReply(w, r, 200, msg, id)
		})
	}
}
