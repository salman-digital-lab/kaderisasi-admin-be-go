package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/export"
	"net/http"
)

func sendWorkbook(w http.ResponseWriter, filename string, body []byte) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(200)
	_, _ = w.Write(body)
}
func (s *Server) exportRegistrations(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	id := pathID(r, "id")
	a, err := s.queries().One(ctx, "SELECT * FROM activities WHERE id=$1", id)
	if err != nil {
		exportFailure(w, err)
		return nil
	}
	questions := []export.Question{}
	form, err := s.queries().One(ctx, "SELECT form_schema FROM custom_forms WHERE feature_type='activity_registration' AND feature_id=$1 AND is_active=true LIMIT 1", id)
	if err == nil {
		questions = export.FormQuestions(form["form_schema"])
	} else if errors.Is(err, pgx.ErrNoRows) {
		config := nestedObject(a, "additional_config")
		var legacy []struct {
			Name  string `json:"name"`
			Label string `json:"label"`
		}
		if err = json.Unmarshal(config["additional_questionnaire"], &legacy); err != nil {
			legacyFailure(w, err)
			return nil
		}
		for _, q := range legacy {
			questions = append(questions, export.Question{Key: q.Name, Label: q.Label})
		}
	} else {
		return err
	}
	query := "SELECT ar.*,row_to_json(p) AS profile,to_jsonb(u)-'password' AS public_user"
	joins := " FROM activity_registrations ar LEFT JOIN public_users u ON u.id=ar.user_id LEFT JOIN profiles p ON p.user_id=ar.user_id"
	for _, pair := range [][2]string{{"province", "provinces"}, {"city", "cities"}, {"origin_province", "provinces"}, {"origin_city", "cities"}, {"university", "universities"}} {
		key, table := pair[0], pair[1]
		query += ",loc_" + key + ".name AS location_" + key
		// Guest IDs may be either strings or JSON numbers. Invalid IDs have no match.
		joins += " LEFT JOIN " + table + " loc_" + key + " ON loc_" + key + ".id=COALESCE(p." + key + "_id,CASE WHEN ar.guest_data->>'" + key + "_id' ~ '^[0-9]+$' THEN (ar.guest_data->>'" + key + "_id')::numeric ELSE NULL END)"
	}
	// Preserve the source registration query's order before loading relations.
	// Lucid loads these relations separately; a joined query can reorder rows.
	order, err := s.queries().All(ctx, "SELECT id FROM activity_registrations WHERE activity_id=$1", id)
	if err != nil {
		return err
	}
	joined, err := s.queries().All(ctx, query+joins+" WHERE ar.activity_id=$1", id)
	if err != nil {
		legacyFailure(w, err)
		return nil
	}
	byID := map[int32]int{}
	for i, row := range joined {
		if _, exists := byID[row.ID("id")]; !exists {
			byID[row.ID("id")] = i
		}
	}
	// Use a separate slice because reordering in place would overwrite rows still
	// needed by another registration.
	registrations := make([]database.Object, 0, len(order))
	for _, row := range order {
		if index, exists := byID[row.ID("id")]; exists {
			registrations = append(registrations, joined[index])
		}
	}
	headers := append([]string{}, export.RegistrationHeaders...)
	for _, q := range questions {
		headers = append(headers, q.Label)
	}
	rows := make([][]interface{}, len(registrations))
	for i, registration := range registrations {
		rows[i] = export.RegistrationRow(i+1, registration, questions, a.String("badge"))
	}
	body, err := export.Workbook("Registrations", headers, rows)
	if err != nil {
		return err
	}
	sendWorkbook(w, export.Filename(a.String("name")), body)
	return nil
}
