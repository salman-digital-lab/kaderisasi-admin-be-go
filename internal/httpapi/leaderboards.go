package httpapi

import (
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/achievement"
	"net/http"
)

func achievementMissing(w http.ResponseWriter, err error) {
	text := err.Error()
	if errors.Is(err, pgx.ErrNoRows) {
		text = "Row not found"
	}
	write(w, 404, struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}{"ACHIEVEMENT_NOT_FOUND", text})
}
func (s *Server) registerLeaderboards() {
	controller := "leaderboards_controller"
	service := achievement.Service{Pool: s.Pool, Location: s.Config.Location}
	s.register(controller, "index", func(w http.ResponseWriter, r *http.Request) error {
		params := r.URL.Query()
		optional := func(key string, empty bool) *string {
			value := params.Get(key)
			if !params.Has(key) || !empty && value == "" {
				return nil
			}
			return &value
		}
		page, size := pageParams(r, 10, 0)
		result, err := service.List(r.Context(), achievement.Filters{Status: optional("status", true), Email: optional("email", false), Name: optional("name", false), Type: optional("type", true), DateOrder: params.Get("sort_by") == "achievement_date", Ascending: params.Get("sort_order") == "asc", Page: page, Size: size})
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register(controller, "show", func(w http.ResponseWriter, r *http.Request) error {
		result, err := service.Show(r.Context(), r.PathValue("id"))
		if err != nil {
			achievementMissing(w, err)
			return nil
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	for _, action := range []string{"update", "approveReject"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			failure := legacyFailure
			if action == "update" {
				failure = achievementMissing
			}
			old, err := service.Existing(r.Context(), r.PathValue("id"))
			if err != nil {
				failure(w, err)
				return nil
			}
			style := "caught"
			if action == "update" {
				style = "achievement"
			}
			validated, ok := inputWithErrors(w, r, "updateAchievementValidator", style)
			if !ok {
				return nil
			}
			input, ok := decodeInputAs[achievement.Input](w, validated)
			if !ok {
				return nil
			}
			var result achievement.Response
			success := "UPDATE_DATA_SUCCESS"
			if action == "update" {
				result, err = service.Update(r.Context(), old, input, actor(r).ID)
			} else {
				if input.Status == nil {
					write(w, 400, struct {
						Message string `json:"message"`
						Error   string `json:"error"`
					}{"INVALID_STATUS", "Status must be a number"})
					return nil
				}
				result, err = service.Review(r.Context(), old, input, actor(r).ID)
				success = "ACHIEVEMENT_REJECTED"
				if input.Status.String() == "1" {
					success = "ACHIEVEMENT_APPROVED"
				}
			}
			if err != nil {
				failure(w, err)
				return nil
			}
			reply(w, 200, success, result)
			return nil
		})
	}
	for _, action := range []string{"monthlyLeaderboard", "lifetimeLeaderboard"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			params := r.URL.Query()
			optional := func(key string) *string {
				value := params.Get(key)
				if value == "" {
					return nil
				}
				return &value
			}
			page, size := pageParams(r, 10, 0)
			filters := achievement.LeaderboardFilters{Month: params.Get("month"), Year: params.Get("year"), Email: optional("email"), Name: optional("name"), Page: page, Size: size}
			if action == "monthlyLeaderboard" {
				result, err := service.Monthly(r.Context(), filters)
				if err != nil {
					legacyFailure(w, err)
					return nil
				}
				reply(w, 200, "GET_DATA_SUCCESS", result)
			} else {
				result, err := service.Lifetime(r.Context(), filters)
				if err != nil {
					legacyFailure(w, err)
					return nil
				}
				reply(w, 200, "GET_DATA_SUCCESS", result)
			}
			return nil
		})
	}
	s.register(controller, "export", func(w http.ResponseWriter, r *http.Request) error {
		result, err := service.Export(r.Context())
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		sendWorkbook(w, result.Filename, result.Body)
		return nil
	})
}
