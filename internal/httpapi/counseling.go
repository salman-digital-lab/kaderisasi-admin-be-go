package httpapi

import (
	"fmt"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/member"
	"net/http"
)

func (s *Server) normalizeUserProfile(row database.Object, key string) database.Object {
	user := nestedObject(row, key)
	if raw := user["profile"]; len(raw) > 0 && string(raw) != "null" {
		profile := nestedObject(user, "profile")
		user.Set("profile", member.Profile(profile))
		row.Set(key, user)
	}
	for _, relation := range []string{key, "adminUser", "approver"} {
		if row.Has(relation) && !row.Null(relation) {
			row.Set(relation, database.Timestamps(nestedObject(row, relation), s.Config.Location, "created_at", "updated_at"))
		}
	}
	return database.Timestamps(row, s.Config.Location, "created_at", "updated_at")
}

const counselingSQL = "SELECT rc.*,(to_jsonb(u)-'password')||jsonb_build_object('profile',to_jsonb(p)) AS \"publicUser\",to_jsonb(a)-'password' AS \"adminUser\" FROM ruang_curhats rc LEFT JOIN public_users u ON u.id=rc.user_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN admin_users a ON a.id=rc.counselor_id"

func (s *Server) registerCounseling() {
	controller := "ruang_curhats_controller"
	s.register(controller, "index", func(w http.ResponseWriter, r *http.Request) error {
		query := counselingSQL + " WHERE true"
		args := []interface{}{}
		params := r.URL.Query()
		for _, filter := range []struct {
			key, expr string
			pattern   bool
		}{{"status", "rc.status", false}, {"name", "p.name", true}, {"gender", "p.gender", false}, {"admin_display_name", "a.display_name", true}} {
			if value := params.Get(filter.key); value != "" {
				op := "="
				if filter.pattern {
					value = "%" + value + "%"
					op = " ILIKE "
				}
				args = append(args, value)
				query += fmt.Sprintf(" AND %s%s$%d", filter.expr, op, len(args))
			}
		}
		page, size := pageParams(r, 10, 0)
		data, err := s.queries().Paginate(r.Context(), query+" ORDER BY rc.created_at DESC", args, page, size)
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		for i, row := range data.Data {
			data.Data[i] = s.normalizeUserProfile(row, "publicUser")
		}
		plural(w, "GET_DATA_SUCCESS", data)
		return nil
	})
	s.register(controller, "show", func(w http.ResponseWriter, r *http.Request) error {
		row, err := s.queries().One(r.Context(), counselingSQL+" WHERE rc.id=$1", pathID(r, "id"))
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		reply(w, 200, "GET_DATA_SUCCESS", s.normalizeUserProfile(row, "publicUser"))
		return nil
	})
	s.register(controller, "update", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := input(w, r, "UpdateRuangCurhatValidator")
		if !ok {
			return nil
		}
		row, err := s.queries().Update(r.Context(), "ruang_curhats", pathID(r, "id"), data)
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", database.Timestamps(row, s.Config.Location, "created_at", "updated_at"))
		return nil
	})
}
