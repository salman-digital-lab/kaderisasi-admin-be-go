package httpapi

import (
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/activity"
	"kaderisasi/admin/internal/club"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"net/http"
)

type activityPage struct {
	Meta database.Pagination `json:"meta"`
	Data []activity.Summary  `json:"data"`
}

func (s *Server) registerActivityReads() {
	q := dbgen.New(s.Pool)
	s.register("activities_controller", "index", func(w http.ResponseWriter, r *http.Request) error {
		params := r.URL.Query()
		optional := func(key string) *string {
			value := params.Get(key)
			if value == "" {
				return nil
			}
			return &value
		}
		filters := dbgen.CountActivitiesFilteredParams{Search: params.Get("search"), Category: optional("category"), MinimumLevel: optional("minimum_level"), ActivityType: optional("activity_type"), IsPublished: optional("is_published"), ClubID: optional("club_id")}
		total, err := q.CountActivitiesFiltered(r.Context(), filters)
		if err != nil {
			legacyFailure(w, paginationError(r, err))
			return nil
		}
		page, size := pageParams(r, 10, 0)
		data := activityPage{Meta: database.Meta(total, page, size), Data: []activity.Summary{}}
		if total > 0 {
			limit, offset, err := database.SQLPage(page, size)
			if err != nil {
				legacyFailure(w, err)
				return nil
			}
			rows, err := q.ListActivitiesFiltered(r.Context(), dbgen.ListActivitiesFilteredParams{Search: filters.Search, Category: filters.Category, MinimumLevel: filters.MinimumLevel, ActivityType: filters.ActivityType, IsPublished: filters.IsPublished, ClubID: filters.ClubID, PageSize: limit, PageOffset: offset})
			if err != nil {
				legacyFailure(w, paginationError(r, err))
				return nil
			}
			for _, row := range rows {
				relation, err := club.FromRelation(row.Club)
				if err != nil {
					return err
				}
				data.Data = append(data.Data, activity.Summary{ListActivitiesFilteredRow: row, Club: relation})
			}
		}
		plural(w, "GET_DATA_SUCCESS", data)
		return nil
	})
	s.register("activities_controller", "show", func(w http.ResponseWriter, r *http.Request) error {
		row, err := q.ActivityDetails(r.Context(), pathID(r, "id"))
		if errors.Is(err, pgx.ErrNoRows) {
			reply(w, 200, "GET_DATA_SUCCESS", nil)
			return nil
		}
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		relation, err := club.FromRelation(row.Club)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", activity.Detail{Response: activity.View(row.Activity), Club: relation})
		return nil
	})
}
