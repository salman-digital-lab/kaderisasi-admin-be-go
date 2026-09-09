package export

import (
	"encoding/json"
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
	text := Text(raw)
	if text == "0" || text == "false" {
		return ""
	}
	return text
}
func historyParts(parts ...json.RawMessage) string {
	values := []string{}
	for _, part := range parts {
		if text := truthyText(part); text != "" {
			values = append(values, text)
		}
	}
	return strings.Join(values, " - ")
}
func educationHistory(raw json.RawMessage) string {
	var entries []EducationEntry
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	if json.Unmarshal(raw, &entries) != nil {
		return Text(raw)
	}
	lines := []string{}
	for _, entry := range entries {
		if line := historyParts(entry.Degree, entry.Institution, entry.Faculty, entry.Major, entry.IntakeYear); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "; ")
}
func workHistory(raw json.RawMessage) string {
	var entries []WorkEntry
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	if json.Unmarshal(raw, &entries) != nil {
		return Text(raw)
	}
	lines := []string{}
	for _, entry := range entries {
		if line := historyParts(entry.JobTitle, entry.Company, entry.StartYear, entry.EndYear); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "; ")
}
func RegistrationRow(number int, registration Registration, questions []Question, badge string) []interface{} {
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
	current := guest.CurrentEducation
	var history []EducationEntry
	if json.Unmarshal(profile.EducationHistory, &history) == nil && len(history) > 0 {
		current = &history[len(history)-1]
	}
	if current == nil {
		current = &EducationEntry{}
	}
	var currentYear interface{} = ""
	if truthyText(current.IntakeYear) != "" {
		var value interface{}
		if json.Unmarshal(current.IntakeYear, &value) == nil {
			currentYear = value
		}
	}
	locations := registration.Locations
	row := []interface{}{number, name, stringValue(profile.Gender), email, stringValue(profile.Picture), whatsapp, stringValue(profile.PersonalID), strings.Split(birthDate, "T")[0], stringValue(profile.Line), stringValue(profile.Instagram), stringValue(profile.Tiktok), stringValue(profile.Linkedin), stringValue(locations.Province), stringValue(locations.City), country, stringValue(locations.OriginProvince), stringValue(locations.OriginCity), stringValue(locations.University), major, intakeYear, truthyText(current.Institution), truthyText(current.Faculty), truthyText(current.Major), currentYear, educationHistory(education), workHistory(profile.WorkHistory), Text(profile.ExtraData), level, strings.Join(profile.Badges, ", "), badge}
	for _, question := range questions {
		row = append(row, Text(registration.Answers[question.Key]))
	}
	return row
}
