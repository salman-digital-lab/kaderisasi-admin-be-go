package httpapi

import (
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/member"
	"net/http"
)

func clubRegistrationSQL(profile, roles, club bool, extraRoleOrder string) string {
	query := "SELECT cr.*,to_jsonb(u)-'password'"
	if profile {
		query += "||jsonb_build_object('profile',to_jsonb(p))"
	}
	query += " AS member"
	if roles {
		query += ",COALESCE((SELECT jsonb_agg(role ORDER BY role.sort_order ASC,role.is_primary DESC" + extraRoleOrder + ") FROM club_member_roles role WHERE role.club_registration_id=cr.id),'[]'::jsonb) AS roles"
	}
	if club {
		query += ",row_to_json(c) AS club"
	}
	query += " FROM club_registrations cr LEFT JOIN public_users u ON u.id=cr.member_id"
	if profile {
		query += " LEFT JOIN profiles p ON p.user_id=u.id"
	}
	if club {
		query += " LEFT JOIN clubs c ON c.id=cr.club_id"
	}
	return query
}
func (s *Server) normalizeClubRegistration(row database.Object) database.Object {
	if row == nil {
		return row
	}
	m := nestedObject(row, "member")
	if raw := m["profile"]; len(raw) > 0 && string(raw) != "null" {
		p := nestedObject(m, "profile")
		m.Set("profile", member.Profile(p))
		row.Set("member", m)
	}
	for _, relation := range []string{"member", "club"} {
		if row.Has(relation) && !row.Null(relation) {
			row.Set(relation, database.Timestamps(nestedObject(row, relation), s.Config.Location, "created_at", "updated_at"))
		}
	}
	if row.Has("roles") {
		var roles []database.Object
		_ = json.Unmarshal(row["roles"], &roles)
		for _, role := range roles {
			database.Timestamps(role, s.Config.Location, "created_at", "updated_at")
		}
		row.Set("roles", roles)
	}
	return database.Timestamps(row, s.Config.Location, "created_at", "updated_at")
}
func (s *Server) registerClubRegistrations() {
	controller := "club_registrations_controller"
	s.register(controller, "export", s.exportClubRegistrations)
	for _, action := range []string{"index", "members"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			id := pathID(r, "id")
			if _, err := s.queries().One(r.Context(), "SELECT id FROM clubs WHERE id=$1", id); err != nil {
				legacyFailure(w, err)
				return nil
			}
			extra := ""
			if action == "members" {
				extra = ",role.created_at ASC"
			}
			query := clubRegistrationSQL(true, true, false, extra) + " WHERE cr.club_id=$1"
			args := []interface{}{id}
			params := r.URL.Query()
			if action == "members" {
				query += " AND cr.status='APPROVED'"
				if search := params.Get("search"); search != "" {
					args = append(args, "%"+search+"%")
					query += " AND (u.email ILIKE $2 OR p.name ILIKE $2)"
				}
			} else if status := params.Get("status"); status != "" {
				args = append(args, status)
				query += " AND cr.status=$2"
			}
			direction := "DESC"
			if params.Get("sort_order") == "asc" {
				direction = "ASC"
			}
			query += " ORDER BY cr.created_at " + direction + " NULLS LAST,cr.id " + direction
			page, size := queryNumber(r, "page", 1), queryNumber(r, "limit", 20)
			data, err := s.queries().Paginate(r.Context(), query, args, page, size)
			if err != nil {
				legacyFailure(w, err)
				return nil
			}
			for i, row := range data.Data {
				data.Data[i] = s.normalizeClubRegistration(row)
			}
			msg := "CLUB_REGISTRATIONS_RETRIEVED"
			if action == "members" {
				msg = "CLUB_MEMBERS_RETRIEVED"
			}
			reply(w, 200, msg, data)
			return nil
		})
	}
	s.register(controller, "show", func(w http.ResponseWriter, r *http.Request) error {
		row, err := s.queries().One(r.Context(), clubRegistrationSQL(true, true, true, "")+" WHERE cr.id=$1", pathID(r, "id"))
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		reply(w, 200, "CLUB_REGISTRATION_RETRIEVED", s.normalizeClubRegistration(row))
		return nil
	})
	for _, action := range []string{"store", "update"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			schema := "storeClubRegistrationValidator"
			if action == "update" {
				schema = "updateClubRegistrationValidator"
			}
			data, ok := caughtValidationInput(w, r, schema)
			if !ok {
				return nil
			}
			id := pathID(r, "id")
			ctx := r.Context()
			var row database.Object
			var err error
			if action == "store" {
				if _, err = s.queries().One(ctx, "SELECT id FROM clubs WHERE id=$1", id); err != nil {
					legacyFailure(w, err)
					return nil
				}
				if _, err = s.queries().One(ctx, "SELECT id FROM public_users WHERE id=$1", data.ID("member_id")); err != nil {
					legacyFailure(w, err)
					return nil
				}
				count, err := s.queries().Count(ctx, "SELECT id FROM club_registrations WHERE club_id=$1 AND member_id=$2", id, data.ID("member_id"))
				if err != nil {
					return err
				}
				if count > 0 {
					return domain.Fail(409, "MEMBER_ALREADY_REGISTERED")
				}
				data.Set("club_id", id)
				data.Set("status", "PENDING")
				if !data.Has("additional_data") {
					data.Set("additional_data", database.Object{})
				}
				row, err = s.queries().Insert(ctx, "club_registrations", data)
			} else {
				row, err = s.queries().Update(ctx, "club_registrations", id, data)
			}
			if err != nil {
				legacyFailure(w, err)
				return nil
			}
			row, err = s.queries().One(ctx, clubRegistrationSQL(false, false, true, "")+" WHERE cr.id=$1", row.ID("id"))
			if err != nil {
				return err
			}
			msg := "CLUB_REGISTRATION_CREATED"
			if action == "update" {
				msg = "CLUB_REGISTRATION_UPDATED"
			}
			reply(w, 200, msg, s.normalizeClubRegistration(row))
			return nil
		})
	}
	s.register(controller, "bulkUpdate", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := caughtValidationInput(w, r, "bulkUpdateClubRegistrationsValidator")
		if !ok {
			return nil
		}
		var changes []database.Object
		_ = json.Unmarshal(data["registrations"], &changes)
		ids := []int32{}
		seen := map[int32]bool{}
		for _, change := range changes {
			id := change.ID("id")
			if seen[id] {
				return domain.Fail(400, "DUPLICATE_REGISTRATION_IDS")
			}
			seen[id] = true
			ids = append(ids, id)
		}
		ctx := r.Context()
		tx, err := s.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		q := database.JSONQueries{DB: tx}
		locked, err := q.All(ctx, "SELECT * FROM club_registrations WHERE id=ANY($1::int[]) FOR UPDATE", ids)
		if err != nil {
			return err
		}
		found := map[int32]bool{}
		for _, row := range locked {
			found[row.ID("id")] = true
		}
		missing := []int32{}
		for _, id := range ids {
			if !found[id] {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			write(w, 404, struct {
				Message string  `json:"message"`
				IDs     []int32 `json:"ids"`
			}{"REGISTRATIONS_NOT_FOUND", missing})
			return nil
		}
		result := []database.Object{}
		for _, change := range changes {
			id := change.ID("id")
			delete(change, "id")
			if _, err = q.Update(ctx, "club_registrations", id, change); err != nil {
				return err
			}
			row, err := q.One(ctx, clubRegistrationSQL(false, false, true, "")+" WHERE cr.id=$1", id)
			if err != nil {
				return err
			}
			result = append(result, s.normalizeClubRegistration(row))
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
		reply(w, 200, "CLUB_REGISTRATIONS_BULK_UPDATED", result)
		return nil
	})
	s.register(controller, "delete", func(w http.ResponseWriter, r *http.Request) error {
		removed, err := s.queries().Delete(r.Context(), "club_registrations", pathID(r, "id"))
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		if !removed {
			legacyFailure(w, pgx.ErrNoRows)
			return nil
		}
		message(w, 200, "CLUB_REGISTRATION_DELETED")
		return nil
	})
}

const clubRoleSQL = "SELECT role.*,to_jsonb(cr)||jsonb_build_object('member',(to_jsonb(u)-'password')||jsonb_build_object('profile',to_jsonb(p))) AS registration FROM club_member_roles role JOIN club_registrations cr ON cr.id=role.club_registration_id LEFT JOIN public_users u ON u.id=cr.member_id LEFT JOIN profiles p ON p.user_id=u.id"

func (s *Server) normalizeClubRole(row database.Object) database.Object {
	registration := s.normalizeClubRegistration(nestedObject(row, "registration"))
	row.Set("registration", registration)
	return database.Timestamps(row, s.Config.Location, "created_at", "updated_at")
}
func (s *Server) registerClubRoles() {
	controller := "club_member_roles_controller"
	for _, action := range []string{"index", "suggestions"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			id := pathID(r, "id")
			if _, err := s.queries().One(r.Context(), "SELECT id FROM clubs WHERE id=$1", id); err != nil {
				legacyFailure(w, err)
				return nil
			}
			if action == "suggestions" {
				rows, err := s.queries().All(r.Context(), "SELECT DISTINCT role_name FROM club_member_roles role JOIN club_registrations cr ON cr.id=role.club_registration_id WHERE cr.club_id=$1 ORDER BY role_name ASC", id)
				if err != nil {
					return err
				}
				names := []string{}
				for _, row := range rows {
					names = append(names, row.String("role_name"))
				}
				reply(w, 200, "CLUB_MEMBER_ROLE_SUGGESTIONS_RETRIEVED", names)
			} else {
				rows, err := s.queries().All(r.Context(), clubRoleSQL+" WHERE cr.club_id=$1 AND cr.status='APPROVED' ORDER BY role.sort_order ASC,role.is_primary DESC,role.created_at ASC", id)
				if err != nil {
					return err
				}
				for i, row := range rows {
					rows[i] = s.normalizeClubRole(row)
				}
				reply(w, 200, "CLUB_MEMBER_ROLES_RETRIEVED", rows)
			}
			return nil
		})
	}
	for _, action := range []string{"store", "update"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			ctx := r.Context()
			id := pathID(r, "id")
			schema := "updateClubMemberRoleValidator"
			if action == "store" {
				if _, err := s.queries().One(ctx, "SELECT id FROM clubs WHERE id=$1", id); err != nil {
					legacyFailure(w, err)
					return nil
				}
				schema = "storeClubMemberRoleValidator"
			}
			data, ok := caughtValidationInput(w, r, schema)
			if !ok {
				return nil
			}
			var registrationID int32
			if action == "store" {
				registration, err := s.queries().One(ctx, "SELECT id FROM club_registrations WHERE id=$1 AND club_id=$2 AND status='APPROVED'", data.ID("club_registration_id"), id)
				if errors.Is(err, pgx.ErrNoRows) {
					return domain.Fail(400, "APPROVED_CLUB_MEMBER_REQUIRED")
				}
				if err != nil {
					return err
				}
				registrationID = registration.ID("id")
				for _, key := range []string{"start_date", "end_date"} {
					if !data.Has(key) {
						data.Set(key, nil)
					}
				}
				if !data.Has("is_primary") {
					data.Set("is_primary", false)
				}
				if !data.Has("sort_order") {
					data.Set("sort_order", 0)
				}
			} else {
				old, err := s.queries().One(ctx, "SELECT * FROM club_member_roles WHERE id=$1", id)
				if err != nil {
					legacyFailure(w, err)
					return nil
				}
				registrationID = old.ID("club_registration_id")
			}
			if data.Bool("is_primary") {
				sql := "UPDATE club_member_roles SET is_primary=false WHERE club_registration_id=$1"
				args := []interface{}{registrationID}
				if action == "update" {
					sql += " AND id<>$2"
					args = append(args, id)
				}
				if _, err := s.Pool.Exec(ctx, sql, args...); err != nil {
					return err
				}
			}
			var row database.Object
			var err error
			if action == "store" {
				row, err = s.queries().Insert(ctx, "club_member_roles", data)
			} else {
				row, err = s.queries().Update(ctx, "club_member_roles", id, data)
			}
			if err != nil {
				legacyFailure(w, err)
				return nil
			}
			row, err = s.queries().One(ctx, clubRoleSQL+" WHERE role.id=$1", row.ID("id"))
			if err != nil {
				return err
			}
			status := 200
			msg := "CLUB_MEMBER_ROLE_UPDATED"
			if action == "store" {
				status = 201
				msg = "CLUB_MEMBER_ROLE_CREATED"
			}
			reply(w, status, msg, s.normalizeClubRole(row))
			return nil
		})
	}
	s.register(controller, "destroy", func(w http.ResponseWriter, r *http.Request) error {
		removed, err := s.queries().Delete(r.Context(), "club_member_roles", pathID(r, "id"))
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		if !removed {
			legacyFailure(w, pgx.ErrNoRows)
			return nil
		}
		message(w, 200, "CLUB_MEMBER_ROLE_DELETED")
		return nil
	})
}
