package member

import (
	"context"
	"encoding/json"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
)

type ExportColumn struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

var exportColumns = []ExportColumn{
	{"id", "ID Profil"}, {"user_id", "ID Pengguna"}, {"name", "Nama"}, {"email", "Email"},
	{"gender", "Jenis Kelamin"}, {"personal_id", "NIK"}, {"picture", "Foto"},
	{"whatsapp", "WhatsApp"}, {"line", "LINE"}, {"instagram", "Instagram"}, {"tiktok", "TikTok"}, {"linkedin", "LinkedIn"},
	{"province_id", "ID Provinsi"}, {"city_id", "ID Kota"}, {"university_id", "ID Universitas"},
	{"major", "Jurusan"}, {"intake_year", "Tahun Masuk"}, {"level", "Level"}, {"badges", "Lencana"},
	{"created_at", "Dibuat Pada"}, {"updated_at", "Diperbarui Pada"}, {"birth_date", "Tanggal Lahir"},
	{"origin_province_id", "ID Provinsi Asal"}, {"origin_city_id", "ID Kota Asal"}, {"country", "Negara"},
	{"education_history", "Riwayat Pendidikan"}, {"work_history", "Riwayat Pekerjaan"}, {"extra_data", "Data Tambahan"},
}

type ExportPreview struct {
	Total   int64          `json:"total"`
	Columns []ExportColumn `json:"columns"`
}

func (s Service) PreviewExport(ctx context.Context, filters dbgen.CountProfilesFilteredParams) (ExportPreview, error) {
	total, err := dbgen.New(s.Pool).CountProfilesFiltered(ctx, filters)
	return ExportPreview{Total: total, Columns: exportColumns}, err
}

func exportHeaders(columns []string, format string) ([]string, error) {
	if (format != "xlsx" && format != "csv") || len(columns) == 0 || len(columns) > len(exportColumns) {
		return nil, domain.Fail(422, "INVALID_MEMBER_EXPORT")
	}
	headers := make([]string, 0, len(columns))
	seen := make(map[string]bool, len(columns))
	for _, key := range columns {
		label := ""
		for _, column := range exportColumns {
			if column.Key == key {
				label = column.Label
				break
			}
		}
		if label == "" || seen[key] {
			return nil, domain.Fail(422, "INVALID_MEMBER_EXPORT")
		}
		seen[key] = true
		headers = append(headers, label)
	}
	return headers, nil
}

type ExportData struct {
	Headers []string
	Rows    [][]string
}

// Export uses the list's exact predicate with no pagination limit.
func (s Service) Export(ctx context.Context, filters dbgen.CountProfilesFilteredParams, columns []string, format string) (*ExportData, error) {
	headers, err := exportHeaders(columns, format)
	if err != nil {
		return nil, err
	}
	rows, err := dbgen.New(s.Pool).ListProfilesFiltered(ctx, dbgen.ListProfilesFilteredParams{
		Search: filters.Search, MemberNumber: filters.MemberNumber, Institution: filters.Institution, Badge: filters.Badge,
		PageSize: nil, PageOffset: "0",
	})
	if err != nil {
		return nil, err
	}
	values := make([][]string, 0, len(rows))
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		value, err := exportValues(row, columns)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return &ExportData{Headers: headers, Rows: values}, nil
}

func exportValues(row dbgen.ListProfilesFilteredRow, columns []string) ([]string, error) {
	// Raw JSON projection preserves every profile column, including legacy JSON,
	// without exposing other account fields or converting numeric identifiers.
	profile := struct {
		dbgen.Profile
		Badges           json.RawMessage `json:"badges"`
		EducationHistory json.RawMessage `json:"education_history"`
		WorkHistory      json.RawMessage `json:"work_history"`
		ExtraData        json.RawMessage `json:"extra_data"`
	}{row.Profile, row.Profile.Badges, row.Profile.EducationHistory, row.Profile.WorkHistory, row.Profile.ExtraData}
	encoded, err := json.Marshal(profile)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, err
	}
	var account struct {
		Email *string `json:"email"`
	}
	if len(row.PublicUser) > 0 {
		if err := json.Unmarshal(row.PublicUser, &account); err != nil {
			return nil, err
		}
	}
	values := make([]string, len(columns))
	for i, key := range columns {
		if key == "email" {
			values[i] = text(account.Email)
			continue
		}
		raw := fields[key]
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		if json.Unmarshal(raw, &values[i]) != nil {
			values[i] = string(raw)
		}
	}
	return values, nil
}
