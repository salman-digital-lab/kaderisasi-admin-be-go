package achievement

import (
	"context"
	"github.com/jackc/pgx/v5/pgtype"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/export"
	"strconv"
	"time"
)

type Export struct {
	Filename string
	Body     []byte
}

func dateLabel(value pgtype.Date) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("02 January 2006")
}
func label(labels []string, value *int32) string {
	if value == nil {
		return "null"
	}
	if *value >= 0 && int(*value) < len(labels) {
		return labels[*value]
	}
	return strconv.FormatInt(int64(*value), 10)
}
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func (s Service) Export(ctx context.Context) (Export, error) {
	rows, err := dbgen.New(s.Pool).AchievementExport(ctx)
	if err != nil {
		return Export{}, err
	}
	headers := []string{"No", "Nama Prestasi", "Nama", "Email", "Kategori", "Skor", "Status", "Tanggal Prestasi", "Tanggal Dibuat", "Disetujui Oleh", "Tanggal Persetujuan", "Catatan", "Deskripsi", "Bukti"}
	data := make([][]interface{}, 0, len(rows))
	for i, row := range rows {
		detail, err := s.details(row.Achievement, row.PublicUser, row.Profile, row.Approver, true)
		if err != nil {
			return Export{}, err
		}
		name, email, approver, createdAt := "", "", "", ""
		if detail.User != nil {
			email = stringValue(detail.User.Email)
			if detail.User.Profile.Value != nil {
				name = detail.User.Profile.Value.Name
			}
		}
		if detail.Approver != nil {
			approver = stringValue(detail.Approver.DisplayName)
		}
		if row.Achievement.CreatedAt.Valid {
			createdAt = row.Achievement.CreatedAt.Time.In(locationOrLocal(s.Location)).Format("02 January 2006")
		}
		a := row.Achievement
		var title interface{}
		if a.Name != nil {
			title = *a.Name
		}
		var score interface{}
		if a.Score != nil {
			score = *a.Score
		}
		data = append(data, []interface{}{i + 1, title, name, email, label([]string{"Kompetensi", "Organisasi", "Akademik"}, a.Type), score, label([]string{"Menunggu Persetujuan", "Diterima", "Ditolak"}, a.Status), dateLabel(a.AchievementDate), createdAt, approver, dateLabel(a.ApprovedAt), stringValue(a.Remark), stringValue(a.Description), stringValue(a.Proof)})
	}
	body, err := export.Workbook("Achievements", headers, data)
	return Export{Filename: "achievements-" + time.Now().In(locationOrLocal(s.Location)).Format("2006-01-02") + ".xlsx", Body: body}, err
}
