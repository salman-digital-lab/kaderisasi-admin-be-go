package export

import (
	"encoding/json"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/member"
	"strconv"
	"strings"
)

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func truthyText(raw json.RawMessage) string {
	if !database.JSONTruthy(raw) {
		return ""
	}
	return Text(raw)
}
func historyText(raw json.RawMessage, fields ...string) (string, error) {
	var entries []json.RawMessage
	if len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &entries) != nil {
		return Text(raw), nil
	}
	lines := []string{}
	for _, entry := range entries {
		parts := []string{}
		for _, field := range fields {
			part, err := documentProperty(entry, field)
			if err != nil {
				return "", err
			}
			if text := truthyText(part); text != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			lines = append(lines, strings.Join(parts, " - "))
		}
	}
	return strings.Join(lines, "; "), nil
}
func currentEducationCell(raw json.RawMessage, key string) interface{} {
	value, _ := documentProperty(raw, key)
	if !database.JSONTruthy(value) {
		return ""
	}
	var result interface{}
	if json.Unmarshal(value, &result) != nil {
		return ""
	}
	return result
}
func RegistrationRow(number int, registration Registration, questions []Question, badge string) ([]interface{}, error) {
	profile := registration.Profile
	if profile == nil {
		profile = &member.ProfileResponse{}
	}
	guest := registration.Guest
	name, email, whatsapp := profile.Name, stringValue(registration.Email), stringValue(profile.Whatsapp)
	birthDate, country, major := stringValue(profile.BirthDate), stringValue(profile.Country), stringValue(profile.Major)
	intakeYear := ""
	if profile.IntakeYear != nil && *profile.IntakeYear != 0 {
		intakeYear = strconv.FormatInt(int64(*profile.IntakeYear), 10)
	}
	levelValue := int32(0)
	if profile.Level != nil {
		levelValue = *profile.Level
	}
	level := strconv.FormatInt(int64(levelValue), 10)
	if label, ok := map[int32]string{0: "JAMAAH", 3: "AKTIVIS", 6: "KADER", 10: "KADER LANJUT"}[levelValue]; ok {
		level = label
	}
	education := profile.EducationHistory
	if registration.UserID == nil {
		name, email, whatsapp = Text(guest.Name), Text(guest.Email), Text(guest.Whatsapp)
		birthDate, country, major = Text(guest.BirthDate), Text(guest.Country), Text(guest.Major)
		intakeYear, level, education = truthyText(guest.IntakeYear), "Tamu", guest.EducationHistory
	}
	educationText, err := historyText(education, "degree", "institution", "faculty", "major", "intake_year")
	if err != nil {
		return nil, err
	}
	workText, err := historyText(profile.WorkHistory, "job_title", "company", "start_year", "end_year")
	if err != nil {
		return nil, err
	}
	current := guest.CurrentEducation
	var history []json.RawMessage
	if json.Unmarshal(profile.EducationHistory, &history) == nil && len(history) > 0 && database.JSONTruthy(history[len(history)-1]) {
		current = history[len(history)-1]
	}
	locations := registration.Locations
	row := []interface{}{number, name, stringValue(profile.Gender), email, stringValue(profile.Picture), whatsapp, stringValue(profile.PersonalID), strings.Split(birthDate, "T")[0], stringValue(profile.Line), stringValue(profile.Instagram), stringValue(profile.Tiktok), stringValue(profile.Linkedin), stringValue(locations.Province), stringValue(locations.City), country, stringValue(locations.OriginProvince), stringValue(locations.OriginCity), stringValue(locations.University), major, intakeYear, currentEducationCell(current, "institution"), currentEducationCell(current, "faculty"), currentEducationCell(current, "major"), currentEducationCell(current, "intake_year"), educationText, workText, Text(profile.ExtraData), level, strings.Join(profile.Badges, ", "), badge}
	for _, question := range questions {
		answer, err := documentProperty(registration.Answers, question.Key)
		if err != nil {
			return nil, err
		}
		row = append(row, Text(answer))
	}
	return row, nil
}
