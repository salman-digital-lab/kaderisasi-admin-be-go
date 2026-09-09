package export

import (
	"bytes"
	"encoding/json"
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
