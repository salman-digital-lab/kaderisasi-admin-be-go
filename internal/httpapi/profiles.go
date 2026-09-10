package httpapi

import (
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/member"
	"net/http"
)

type profilePage struct {
	Meta database.Pagination     `json:"meta"`
	Data []member.ProfileSummary `json:"data"`
}

type profileDetailsResponse struct {
	Profile []member.ProfileDetail `json:"profile"`
}

func (s *Server) registerProfileReads() {
	q := dbgen.New(s.Pool)
	s.register("profiles_controller", "index", func(w http.ResponseWriter, r *http.Request) error {
		params := r.URL.Query()
		filters := dbgen.CountProfilesFilteredParams{Search: params.Get("search"), MemberNumber: params.Get("member_id"), Institution: params.Get("education_institution"), Badge: params.Get("badge")}
		total, err := q.CountProfilesFiltered(r.Context(), filters)
		if err != nil {
			legacyFailure(w, paginationError(r, err))
			return nil
		}
		page, size := pageParams(r, 10, 0)
		data := profilePage{Meta: database.Meta(total, page, size), Data: []member.ProfileSummary{}}
		if total > 0 {
			limit, offset, err := database.SQLPage(page, size)
			if err != nil {
				legacyFailure(w, err)
				return nil
			}
			rows, err := q.ListProfilesFiltered(r.Context(), dbgen.ListProfilesFilteredParams{Search: filters.Search, MemberNumber: filters.MemberNumber, Institution: filters.Institution, Badge: filters.Badge, PageSize: limit, PageOffset: offset})
			if err != nil {
				legacyFailure(w, paginationError(r, err))
				return nil
			}
			for _, row := range rows {
				view, err := member.ProfileWithUser(row.Profile, row.PublicUser)
				if err != nil {
					return err
				}
				data.Data = append(data.Data, view)
			}
		}
		plural(w, "GET_DATA_SUCCESS", data)
		return nil
	})
	for _, target := range []struct {
		action string
		byUser bool
	}{{"show", false}, {"showByUserId", true}} {
		s.register("profiles_controller", target.action, func(w http.ResponseWriter, r *http.Request) error {
			rows, err := q.ProfileDetailsByIdentifier(r.Context(), dbgen.ProfileDetailsByIdentifierParams{ID: pathID(r, "id"), ByUser: target.byUser})
			if err != nil {
				column := "id"
				if target.byUser {
					column = "user_id"
				}
				legacyFailure(w, database.LegacyQueryError(err, `select * from "profiles" where "`+column+`" = $1`))
				return nil
			}
			data := profileDetailsResponse{Profile: make([]member.ProfileDetail, len(rows))}
			for i, row := range rows {
				data.Profile[i], err = member.ProfileWithRelations(row)
				if err != nil {
					return err
				}
			}
			reply(w, 200, "GET_DATA_SUCCESS", data)
			return nil
		})
	}
}
