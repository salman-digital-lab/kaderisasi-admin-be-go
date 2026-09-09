package httpapi

import (
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/export"
	"net/http"
	"strings"
	"time"
)

func (s *Server) exportClubRegistrations(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	id := pathID(r, "id")
	club, err := s.queries().One(ctx, "SELECT * FROM clubs WHERE id=$1", id)
	if err != nil {
		legacyFailure(w, err)
		return nil
	}
	registrations, err := s.queries().All(ctx, "SELECT cr.*,to_jsonb(u)-'password' AS member,row_to_json(p) AS profile,province.name AS province,university.name AS university FROM club_registrations cr JOIN public_users u ON u.id=cr.member_id LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN provinces province ON province.id=p.province_id LEFT JOIN universities university ON university.id=p.university_id WHERE cr.club_id=$1 ORDER BY cr.created_at DESC", id)
	if err != nil {
		return err
	}
	form, err := s.queries().One(ctx, "SELECT form_schema FROM custom_forms WHERE feature_type='club_registration' AND feature_id=$1 AND is_active=true ORDER BY updated_at DESC,id DESC LIMIT 1", id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	answerSets := [][]byte{}
	for _, registration := range registrations {
		answerSets = append(answerSets, registration["additional_data"])
	}
	questions := export.ClubQuestions(form["form_schema"], answerSets)
	headers := []string{"No", "Nama Lengkap", "Email", "Whatsapp", "Nomor Identitas", "Provinsi", "Universitas", "Jurusan", "Tahun Masuk", "Jenjang", "Status", "Tanggal Pendaftaran"}
	for _, question := range questions {
		headers = append(headers, question.Label)
	}
	rows := [][]interface{}{}
	for i, registration := range registrations {
		p := nestedObject(registration, "profile")
		m := nestedObject(registration, "member")
		level := ""
		if !registration.Null("profile") {
			level = export.Text(p["level"])
			if label, ok := map[int32]string{0: "JAMAAH", 3: "AKTIVIS", 6: "KADER", 10: "KADER LANJUT"}[p.ID("level")]; ok {
				level = label
			}
		}
		registeredAt := ""
		if !registration.Null("created_at") {
			created, err := time.Parse(time.RFC3339Nano, registration.String("created_at"))
			if err != nil {
				return err
			}
			registeredAt = created.In(domain.Jakarta()).Format("2006-01-02 15:04:05")
		}
		var intake interface{} = ""
		if p.ID("intake_year") != 0 {
			intake = p.ID("intake_year")
		}
		row := []interface{}{i + 1, p.String("name"), m.String("email"), p.String("whatsapp"), p.String("personal_id"), registration.String("province"), registration.String("university"), p.String("major"), intake, level, registration.String("status"), registeredAt}
		answers := nestedObject(registration, "additional_data")
		for _, question := range questions {
			row = append(row, export.ClubAnswer(answers[question.Key]))
		}
		rows = append(rows, row)
	}
	body, err := export.Workbook("Registrations", headers, rows)
	if err != nil {
		return err
	}
	sendWorkbook(w, strings.TrimSuffix(export.Filename(club.String("name")), ".xlsx")+"_registrations.xlsx", body)
	return nil
}
