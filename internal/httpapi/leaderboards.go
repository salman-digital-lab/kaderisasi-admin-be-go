package httpapi

import (
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/achievement"
	"kaderisasi/admin/internal/export"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const achievementSQL = "SELECT a.*,(to_jsonb(u)-'password')||jsonb_build_object('profile',to_jsonb(p)) AS user,to_jsonb(admin)-'password' AS approver FROM achievements a LEFT JOIN public_users u ON u.id=a.user_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN admin_users admin ON admin.id=a.approver_id"

func achievementMissing(w http.ResponseWriter, err error) {
	message := err.Error()
	if errors.Is(err, pgx.ErrNoRows) {
		message = "Row not found"
	}
	write(w, 404, struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}{"ACHIEVEMENT_NOT_FOUND", message})
}
func (s *Server) registerLeaderboards() {
	controller := "leaderboards_controller"
	service := achievement.Service{Pool: s.Pool, Location: s.Config.Location}
	s.register(controller, "index", func(w http.ResponseWriter, r *http.Request) error {
		params := r.URL.Query()
		query := achievementSQL + " WHERE true"
		args := []interface{}{}
		for _, key := range []string{"status", "type"} {
			if params.Has(key) {
				args = append(args, params.Get(key))
				query += fmt.Sprintf(" AND a.%s=$%d", key, len(args))
			}
		}
		if email := params.Get("email"); email != "" {
			args = append(args, email)
			query += fmt.Sprintf(" AND u.email=$%d", len(args))
		}
		if name := params.Get("name"); name != "" {
			args = append(args, "%"+name+"%")
			query += fmt.Sprintf(" AND p.name ILIKE $%d", len(args))
		}
		sort := "created_at"
		if params.Get("sort_by") == "achievement_date" {
			sort = "achievement_date"
		}
		direction := "DESC"
		if params.Get("sort_order") == "asc" {
			direction = "ASC"
		}
		page, size := pageParams(r, 10, 0)
		data, err := s.queries().Paginate(r.Context(), query+" ORDER BY a."+sort+" "+direction, args, page, size)
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		for i, row := range data.Data {
			data.Data[i] = s.normalizeUserProfile(row, "user")
		}
		reply(w, 200, "GET_DATA_SUCCESS", data)
		return nil
	})
	s.register(controller, "show", func(w http.ResponseWriter, r *http.Request) error {
		row, err := s.queries().One(r.Context(), "SELECT a.*,to_jsonb(u)-'password' AS user,to_jsonb(admin)-'password' AS approver FROM achievements a LEFT JOIN public_users u ON u.id=a.user_id LEFT JOIN admin_users admin ON admin.id=a.approver_id WHERE a.id=$1", pathID(r, "id"))
		if err != nil {
			achievementMissing(w, err)
			return nil
		}
		reply(w, 200, "GET_DATA_SUCCESS", s.normalizeUserProfile(row, "user"))
		return nil
	})
	for _, action := range []string{"update", "approveReject"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			old, err := s.queries().One(r.Context(), "SELECT * FROM achievements WHERE id=$1", pathID(r, "id"))
			if err != nil {
				if action == "update" {
					achievementMissing(w, err)
				} else {
					legacyFailure(w, err)
				}
				return nil
			}
			style := "caught"
			if action == "update" {
				style = "achievement"
			}
			data, ok := inputWithErrors(w, r, "updateAchievementValidator", style)
			if !ok {
				return nil
			}
			if action == "update" {
				if data.Has("status") && data.ID("status") == 1 && old.ID("status") != 1 {
					data.Set("approver_id", actor(r).ID)
					data.Set("approved_at", time.Now().In(s.Config.Location).Format("2006-01-02"))
				}
				row, err := s.queries().Update(r.Context(), "achievements", pathID(r, "id"), data)
				if err != nil {
					achievementMissing(w, err)
					return nil
				}
				reply(w, 200, "UPDATE_DATA_SUCCESS", s.normalizeUserProfile(row, "user"))
				return nil
			}
			if !data.Has("status") {
				write(w, 400, struct {
					Message string `json:"message"`
					Error   string `json:"error"`
				}{"INVALID_STATUS", "Status must be a number"})
				return nil
			}
			row, err := service.Review(r.Context(), old, data, actor(r).ID)
			if err != nil {
				legacyFailure(w, err)
				return nil
			}
			msg := "ACHIEVEMENT_REJECTED"
			if data.ID("status") == 1 {
				msg = "ACHIEVEMENT_APPROVED"
			}
			reply(w, 200, msg, s.normalizeUserProfile(row, "user"))
			return nil
		})
	}
	for _, action := range []string{"monthlyLeaderboard", "lifetimeLeaderboard"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			table := "lifetime_leaderboards"
			if action == "monthlyLeaderboard" {
				table = "monthly_leaderboards"
			}
			query := "SELECT b.*,(to_jsonb(u)-'password')||jsonb_build_object('profile',to_jsonb(p)||jsonb_build_object('university',to_jsonb(university))) AS user FROM " + table + " b LEFT JOIN public_users u ON u.id=b.user_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN universities university ON university.id=p.university_id WHERE true"
			args := []interface{}{}
			params := r.URL.Query()
			if action == "monthlyLeaderboard" && params.Get("year") != "" {
				year, _ := strconv.Atoi(params.Get("year"))
				month, _ := strconv.Atoi(params.Get("month"))
				if params.Get("month") != "" {
					args = append(args, fmt.Sprintf("%04d-%02d-01", year, month))
					query += " AND b.month=$1"
				} else {
					args = append(args, fmt.Sprintf("%04d-01-01", year), fmt.Sprintf("%04d-12-31", year))
					query += " AND b.month BETWEEN $1 AND $2"
				}
			}
			for _, pair := range [][2]string{{"email", "u.email"}, {"name", "p.name"}} {
				if v := params.Get(pair[0]); v != "" {
					args = append(args, "%"+v+"%")
					query += fmt.Sprintf(" AND %s ILIKE $%d", pair[1], len(args))
				}
			}
			page, size := pageParams(r, 10, 0)
			data, err := s.queries().Paginate(r.Context(), query+" ORDER BY b.score DESC", args, page, size)
			if err != nil {
				legacyFailure(w, err)
				return nil
			}
			for i, row := range data.Data {
				data.Data[i] = s.normalizeUserProfile(row, "user")
			}
			reply(w, 200, "GET_DATA_SUCCESS", data)
			return nil
		})
	}
	s.register(controller, "export", s.exportAchievements)
}
func (s *Server) exportAchievements(w http.ResponseWriter, r *http.Request) error {
	achievements, err := s.queries().All(r.Context(), achievementSQL+" ORDER BY a.created_at DESC")
	if err != nil {
		return err
	}
	headers := []string{"No", "Nama Prestasi", "Nama", "Email", "Kategori", "Skor", "Status", "Tanggal Prestasi", "Tanggal Dibuat", "Disetujui Oleh", "Tanggal Persetujuan", "Catatan", "Deskripsi", "Bukti"}
	formatDate := func(raw string) string {
		if raw == "" {
			return ""
		}
		date, err := time.Parse("2006-01-02", raw)
		if strings.Contains(raw, "T") {
			date, err = time.Parse(time.RFC3339Nano, raw)
			date = date.In(s.Config.Location)
		}
		if err != nil {
			return ""
		}
		return date.Format("02 January 2006")
	}
	label := func(labels []string, value int32) string {
		if value >= 0 && int(value) < len(labels) {
			return labels[value]
		}
		return strconv.Itoa(int(value))
	}
	rows := [][]interface{}{}
	for i, a := range achievements {
		u := nestedObject(a, "user")
		p := nestedObject(u, "profile")
		admin := nestedObject(a, "approver")
		rows = append(rows, []interface{}{i + 1, a.String("name"), p.String("name"), u.String("email"), label([]string{"Kompetensi", "Organisasi", "Akademik"}, a.ID("type")), a.Number("score"), label([]string{"Menunggu Persetujuan", "Diterima", "Ditolak"}, a.ID("status")), formatDate(a.String("achievement_date")), formatDate(a.String("created_at")), admin.String("display_name"), formatDate(a.String("approved_at")), a.String("remark"), a.String("description"), a.String("proof")})
	}
	body, err := export.Workbook("Achievements", headers, rows)
	if err != nil {
		return err
	}
	sendWorkbook(w, "achievements-"+time.Now().In(s.Config.Location).Format("2006-01-02")+".xlsx", body)
	return nil
}
