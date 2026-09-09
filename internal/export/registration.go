package export

import (
	"bytes"
	"encoding/json"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/member"
	"strings"
)

var RegistrationHeaders = []string{"No", "Nama Lengkap", "Jenis Kelamin", "Email", "Foto Profil", "Whatsapp", "Nomor Identitas", "Tanggal Lahir", "Line ID", "Instagram", "TikTok", "LinkedIn", "Provinsi Domisili", "Kota Domisili", "Negara Domisili", "Provinsi Asal", "Kota Asal", "Kampus/Universitas Profil", "Jurusan Profil", "Tahun Masuk Profil", "Institusi Pendidikan Saat Ini", "Fakultas Pendidikan Saat Ini", "Jurusan Pendidikan Saat Ini", "Tahun Masuk Pendidikan Saat Ini", "Riwayat Pendidikan", "Riwayat Pekerjaan", "Data Tambahan", "Jenjang", "Lencana Profil", "Lencana Kegiatan"}

type Question struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}
type FormSchema struct {
	Fields []struct {
		SectionName string     `json:"section_name"`
		Fields      []Question `json:"fields"`
	} `json:"fields"`
}

func FormQuestions(raw []byte) []Question {
	var form FormSchema
	_ = json.Unmarshal(raw, &form)
	questions := []Question{}
	for _, section := range form.Fields {
		if section.SectionName != "profile_data" {
			questions = append(questions, section.Fields...)
		}
	}
	return questions
}
func Text(value json.RawMessage) string {
	if len(value) == 0 || bytes.Equal(value, []byte("null")) {
		return ""
	}
	var text string
	if json.Unmarshal(value, &text) == nil {
		return text
	}
	var compact bytes.Buffer
	if json.Compact(&compact, value) == nil {
		return compact.String()
	}
	return ""
}
func Object(data database.Object, key string) database.Object {
	var result database.Object
	_ = json.Unmarshal(data[key], &result)
	if result == nil {
		result = database.Object{}
	}
	return result
}
func history(raw json.RawMessage, keys []string) string {
	var entries []database.Object
	if json.Unmarshal(raw, &entries) != nil {
		return Text(raw)
	}
	lines := []string{}
	for _, entry := range entries {
		parts := []string{}
		for _, key := range keys {
			if text := Text(entry[key]); text != "" && text != "0" {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			lines = append(lines, strings.Join(parts, " - "))
		}
	}
	return strings.Join(lines, "; ")
}
func RegistrationRow(number int, registration database.Object, questions []Question, badge string) []interface{} {
	guest := Object(registration, "guest_data")
	profile := Object(registration, "profile")
	user := Object(registration, "public_user")
	isGuest := registration.Null("user_id")
	identity := profile
	if isGuest {
		identity = guest
	}
	education := []database.Object{}
	_ = json.Unmarshal(profile["education_history"], &education)
	current := Object(guest, "current_education")
	if len(education) > 0 {
		current = education[len(education)-1]
	}
	email := user.String("email")
	level := Text(profile["level"])
	if label, ok := map[int32]string{0: "JAMAAH", 3: "AKTIVIS", 6: "KADER", 10: "KADER LANJUT"}[profile.ID("level")]; ok {
		level = label
	}
	if isGuest {
		email = guest.String("email")
		level = "Tamu"
	}
	badges := []string{}
	if len(profile) > 0 {
		profile = member.Profile(profile)
		_ = json.Unmarshal(profile["badges"], &badges)
	}
	location := func(key string) string { return registration.String("location_" + key) }
	row := []interface{}{number, identity.String("name"), profile.String("gender"), email, profile.String("picture"), identity.String("whatsapp"), profile.String("personal_id"), strings.Split(identity.String("birth_date"), "T")[0], profile.String("line"), profile.String("instagram"), profile.String("tiktok"), profile.String("linkedin"), location("province"), location("city"), identity.String("country"), location("origin_province"), location("origin_city"), location("university"), identity.String("major"), Text(identity["intake_year"]), current.String("institution"), current.String("faculty"), current.String("major"), "", history(identity["education_history"], []string{"degree", "institution", "faculty", "major", "intake_year"}), history(profile["work_history"], []string{"job_title", "company", "start_year", "end_year"}), Text(profile["extra_data"]), level, strings.Join(badges, ", "), badge}
	if current.ID("intake_year") != 0 {
		row[23] = current.ID("intake_year")
	}
	answers := Object(registration, "questionnaire_answer")
	for _, question := range questions {
		row = append(row, Text(answers[question.Key]))
	}
	return row
}
