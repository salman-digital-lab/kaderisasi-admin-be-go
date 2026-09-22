package export

import (
	"encoding/json"
	"kaderisasi/admin/internal/member"
	"strconv"
	"strings"
)

var clubDefaultProfileFields = []ClubField{
	{Key: "name", Label: "Nama Lengkap"}, {Key: "email", Label: "Email"},
	{Key: "whatsapp", Label: "Whatsapp"}, {Key: "personal_id", Label: "Nomor Identitas"},
	{Key: "province_id", Label: "Provinsi"}, {Key: "university_id", Label: "Universitas"},
	{Key: "major", Label: "Jurusan"}, {Key: "intake_year", Label: "Tahun Masuk"},
	{Key: "level", Label: "Jenjang"},
}

var clubEducationColumns = []struct{ Key, Label string }{
	{"institution", "Kampus/Sekolah"}, {"degree", "Jenjang"}, {"faculty", "Fakultas"},
	{"major", "Jurusan"}, {"intake_year", "Tahun Masuk"},
}

var clubDegreeLabels = map[string]string{
	"high_school": "SMA/SMK", "diploma": "D3", "bachelor": "S1", "master": "S2", "doctoral": "S3",
}

func ClubProfileFields(schema []byte) []ClubField {
	var form clubFormSchema
	_ = json.Unmarshal(schema, &form)
	fields := []ClubField{}
	seen := map[string]bool{}
	hasProfile := false
	for _, section := range form.Fields {
		if section.SectionName != "profile_data" {
			continue
		}
		hasProfile = true
		for _, field := range section.Fields {
			if !seen[field.Key] {
				seen[field.Key] = true
				fields = append(fields, field)
			}
		}
	}
	if !hasProfile {
		return clubDefaultProfileFields
	}
	if !seen["email"] {
		fields = append(fields, ClubField{Key: "email", Label: "Email"})
	}
	return fields
}

func ClubProfileHeaders(fields []ClubField) []string {
	headers := []string{}
	for _, field := range fields {
		if field.Key == "current_education" {
			for _, column := range clubEducationColumns {
				headers = append(headers, field.Label+" - "+column.Label)
			}
		} else {
			headers = append(headers, field.Label)
		}
	}
	return headers
}

func clubEducationHistory(raw json.RawMessage) string {
	entries := member.HistoryEntries(member.NormalizeEducationHistory(raw))
	lines := []string{}
	for _, entry := range entries {
		parts := []string{}
		for _, key := range []string{"degree", "institution", "faculty", "major", "intake_year"} {
			value := Text(entry[key])
			if key == "degree" {
				value = clubDegreeLabels[value]
			}
			if value != "" {
				parts = append(parts, value)
			}
		}
		if len(parts) > 0 {
			lines = append(lines, strings.Join(parts, " - "))
		}
	}
	return strings.Join(lines, "; ")
}

func ClubProfileRow(fields []ClubField, profile *member.ProfileResponse, email *string, locations RegistrationLocations) []interface{} {
	if profile == nil {
		values := []interface{}{}
		for _, field := range fields {
			if field.Key == "email" {
				values = append(values, stringValue(email))
			} else if field.Key == "current_education" {
				for range clubEducationColumns {
					values = append(values, "")
				}
			} else {
				values = append(values, "")
			}
		}
		return values
	}
	education := member.HistoryEntries(member.NormalizeEducationHistory(profile.EducationHistory))
	var current map[string]json.RawMessage
	if len(education) > 0 {
		current = education[len(education)-1]
	}
	values := []interface{}{}
	for _, field := range fields {
		if field.Key == "current_education" {
			for _, column := range clubEducationColumns {
				if column.Key == "degree" {
					values = append(values, clubDegreeLabels[Text(current[column.Key])])
				} else {
					values = append(values, ClubAnswer(current[column.Key]))
				}
			}
			continue
		}
		var value interface{} = ""
		switch field.Key {
		case "name":
			value = profile.Name
		case "email":
			value = stringValue(email)
		case "gender":
			value = stringValue(profile.Gender)
			if len(field.Options) == 0 {
				if label, ok := map[string]string{"M": "Laki-laki", "F": "Perempuan"}[stringValue(profile.Gender)]; ok {
					value = label
				}
			}
		case "personal_id":
			value = stringValue(profile.PersonalID)
		case "picture":
			value = stringValue(profile.Picture)
		case "whatsapp":
			value = stringValue(profile.Whatsapp)
		case "line":
			value = stringValue(profile.Line)
		case "instagram":
			value = stringValue(profile.Instagram)
		case "tiktok":
			value = stringValue(profile.Tiktok)
		case "linkedin":
			value = stringValue(profile.Linkedin)
		case "birth_date":
			if profile.Profile.BirthDate.Valid {
				value = profile.Profile.BirthDate.Time.Format("2006-01-02")
			}
		case "country":
			value = stringValue(profile.Country)
		case "province_id":
			value = stringValue(locations.Province)
		case "city_id":
			value = stringValue(locations.City)
		case "origin_province_id":
			value = stringValue(locations.OriginProvince)
		case "origin_city_id":
			value = stringValue(locations.OriginCity)
		case "university_id":
			value = stringValue(locations.University)
		case "major":
			value = stringValue(profile.Major)
		case "intake_year":
			if profile.IntakeYear != nil && *profile.IntakeYear != 0 {
				value = *profile.IntakeYear
			}
		case "level":
			level := int32(0)
			if profile.Level != nil {
				level = *profile.Level
			}
			value = strconv.FormatInt(int64(level), 10)
			if label, ok := map[int32]string{0: "JAMAAH", 3: "AKTIVIS", 6: "KADER", 10: "KADER LANJUT"}[level]; ok {
				value = label
			}
		case "education_history":
			value = clubEducationHistory(profile.EducationHistory)
		case "work_history":
			value, _ = historyText(member.NormalizeWorkHistory(profile.WorkHistory), "job_title", "company", "start_year", "end_year")
		case "badges":
			value = strings.Join(profile.Badges, ", ")
		case "extra_data":
			value = Text(profile.ExtraData)
		}
		if len(field.Options) > 0 {
			raw, _ := json.Marshal(value)
			value = field.Answer(raw)
		}
		values = append(values, value)
	}
	return values
}
