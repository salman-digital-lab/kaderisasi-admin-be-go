package p3ri

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/export"
	"kaderisasi/admin/internal/storage"
)

type Service struct {
	Pool    *pgxpool.Pool
	Storage storage.Store
}

// File is an uploaded file referenced by a file question.
type File struct {
	Field string `json:"field"`
	Key   string `json:"key"`
	Name  string `json:"name"`
	Mime  string `json:"mime"`
	Size  int64  `json:"size"`
}

type Answers struct {
	Common    map[string]any            `json:"common"`
	Divisions map[string]map[string]any `json:"divisions"`
}

// Registration is one registration as the review shows it.
type Registration struct {
	ID                  int32      `json:"id"`
	UserID              int32      `json:"user_id"`
	Email               string     `json:"email"`
	Status              string     `json:"status"`
	Choices             []string   `json:"choices"`
	AcceptedDivision    *string    `json:"accepted_division"`
	Answers             Answers    `json:"answers"`
	Files               []File     `json:"files"`
	AdminNote           *string    `json:"admin_note"`
	StatusUpdatedByName *string    `json:"status_updated_by_name,omitempty"`
	StatusUpdatedAt     *time.Time `json:"status_updated_at"`
	SubmittedAt         time.Time  `json:"submitted_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func view(r dbgen.P3riRegistration, email *string) Registration {
	out := Registration{
		ID: r.ID, UserID: r.UserID, Status: r.Status, Choices: r.Choices, AcceptedDivision: r.AcceptedDivision,
		Files: []File{}, AdminNote: r.AdminNote, SubmittedAt: r.SubmittedAt.Time, UpdatedAt: r.UpdatedAt.Time,
	}
	if email != nil {
		out.Email = *email
	}
	_ = json.Unmarshal(r.Answers, &out.Answers)
	if out.Answers.Common == nil {
		out.Answers.Common = map[string]any{}
	}
	if out.Answers.Divisions == nil {
		out.Answers.Divisions = map[string]map[string]any{}
	}
	_ = json.Unmarshal(r.Files, &out.Files)
	if r.StatusUpdatedAt.Valid {
		at := r.StatusUpdatedAt.Time
		out.StatusUpdatedAt = &at
	}
	return out
}

// Filter narrows the list and the export. Empty fields do not filter.
type Filter struct {
	Status     string
	Division   string
	ChoiceRank int32
	Gender     string
	Search     string
}

func optional(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func (f Filter) params(limit, offset int32) (dbgen.ListP3riRegistrationsParams, error) {
	p := dbgen.ListP3riRegistrationsParams{
		Status: optional(f.Status), Division: optional(f.Division), Gender: optional(f.Gender),
		Search: optional(f.Search), PageLimit: limit, PageOffset: offset,
	}
	if p.Status != nil && !slices.Contains(Statuses, *p.Status) {
		return p, domain.Fail(422, "Status tidak dikenal.")
	}
	if f.ChoiceRank != 0 {
		if f.ChoiceRank < 1 || f.ChoiceRank > 3 {
			return p, domain.Fail(422, "Urutan pilihan tidak dikenal.")
		}
		rank := f.ChoiceRank
		p.ChoiceRank = &rank
	}
	return p, nil
}

type ListResult struct {
	Meta   database.Pagination `json:"meta"`
	Data   []Registration      `json:"data"`
	Counts map[string]int64    `json:"counts"`
}

func (s *Service) List(ctx context.Context, f Filter, page, perPage int32) (ListResult, error) {
	q := dbgen.New(s.Pool)
	p, err := f.params(perPage, (page-1)*perPage)
	if err != nil {
		return ListResult{}, err
	}
	rows, err := q.ListP3riRegistrations(ctx, p)
	if err != nil {
		return ListResult{}, err
	}
	total, err := q.CountP3riRegistrations(ctx, dbgen.CountP3riRegistrationsParams{
		Status: p.Status, Division: p.Division, ChoiceRank: p.ChoiceRank, Gender: p.Gender, Search: p.Search,
	})
	if err != nil {
		return ListResult{}, err
	}
	counts := map[string]int64{}
	for _, status := range Statuses {
		counts[status] = 0
	}
	byStatus, err := q.P3riStatusCounts(ctx)
	if err != nil {
		return ListResult{}, err
	}
	for _, c := range byStatus {
		counts[c.Status] = c.Total
	}
	out := ListResult{Meta: database.Meta(total, int(page), int(perPage)), Data: make([]Registration, 0, len(rows)), Counts: counts}
	for _, row := range rows {
		out.Data = append(out.Data, view(row.P3riRegistration, row.Email))
	}
	return out, nil
}

func notFound() error { return domain.Fail(404, "Pendaftar tidak ditemukan.") }

func (s *Service) Detail(ctx context.Context, id int32) (Registration, error) {
	row, err := dbgen.New(s.Pool).P3riRegistration(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Registration{}, notFound()
	}
	if err != nil {
		return Registration{}, err
	}
	out := view(row.P3riRegistration, row.Email)
	out.StatusUpdatedByName = row.StatusUpdatedByName
	return out, nil
}

type StatusInput struct {
	Status           string  `json:"status"`
	AcceptedDivision *string `json:"accepted_division"`
	AdminNote        *string `json:"admin_note"`
}

// SetStatus moves a registration through the pipeline. Diterima needs the
// accepted division, which must be one of the applicant's choices.
func (s *Service) SetStatus(ctx context.Context, actor, id int32, in StatusInput) (Registration, error) {
	if !slices.Contains(Statuses, in.Status) {
		return Registration{}, domain.Fail(422, "Status tidak dikenal.")
	}
	current, err := s.Detail(ctx, id)
	if err != nil {
		return Registration{}, err
	}
	division := optional(deref(in.AcceptedDivision))
	if in.Status == "diterima" {
		if division == nil || !slices.Contains(current.Choices, *division) {
			return Registration{}, domain.Fail(422, "Pilih divisi penerimaan dari pilihan pendaftar.")
		}
	} else {
		division = nil
	}
	note := optional(deref(in.AdminNote))
	if note != nil && len([]rune(*note)) > 2000 {
		return Registration{}, domain.Fail(422, "Catatan maksimal 2000 karakter.")
	}
	if _, err := dbgen.New(s.Pool).UpdateP3riRegistrationStatus(ctx, dbgen.UpdateP3riRegistrationStatusParams{
		ID: id, Status: in.Status, AcceptedDivision: division, AdminNote: note, StatusUpdatedBy: actor,
	}); err != nil {
		return Registration{}, err
	}
	return s.Detail(ctx, id)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// File returns an uploaded file and its type when it belongs to a registration.
func (s *Service) File(ctx context.Context, key string) ([]byte, string, error) {
	if !strings.HasPrefix(key, "p3ri/") {
		return nil, "", domain.Fail(404, "Berkas tidak ditemukan.")
	}
	rows, err := dbgen.New(s.Pool).P3riRegistrationFiles(ctx, key)
	if err != nil {
		return nil, "", err
	}
	mime := ""
	for _, raw := range rows {
		var files []File
		_ = json.Unmarshal(raw, &files)
		for _, f := range files {
			if f.Key == key {
				mime = f.Mime
			}
		}
	}
	if mime == "" {
		return nil, "", domain.Fail(404, "Berkas tidak ditemukan.")
	}
	data, err := s.Storage.Get(ctx, key)
	if err != nil {
		return nil, "", domain.Fail(404, "Berkas tidak ditemukan.")
	}
	return data, mime, nil
}

// Export builds an xlsx with one row per registrant: common answers, then
// each choice with its division answers.
func (s *Service) Export(ctx context.Context, f Filter) (string, []byte, error) {
	p, err := f.params(1_000_000, 0)
	if err != nil {
		return "", nil, err
	}
	rows, err := dbgen.New(s.Pool).ListP3riRegistrations(ctx, p)
	if err != nil {
		return "", nil, err
	}
	form := Current()
	headers := []string{"No", "Email", "Status", "Diterima di", "Waktu daftar", "Terakhir diubah"}
	var common []Question
	for _, section := range form.Sections {
		for _, q := range section.Questions {
			if q.Type == "file" {
				continue
			}
			common = append(common, q)
			headers = append(headers, q.Label)
		}
	}
	// Division questions differ per division, so each choice gets its
	// division name, the shared choice questions, and a combined answer cell.
	for n := 1; n <= 3; n++ {
		headers = append(headers, fmt.Sprintf("Pilihan %d", n))
		for _, q := range form.ChoiceQuestions {
			headers = append(headers, fmt.Sprintf("Pilihan %d – %s", n, q.Label))
		}
		headers = append(headers, fmt.Sprintf("Pilihan %d – Jawaban divisi", n))
	}
	headers = append(headers, "Catatan panitia")

	data := make([][]interface{}, 0, len(rows))
	for i, row := range rows {
		r := view(row.P3riRegistration, row.Email)
		accepted := ""
		if r.AcceptedDivision != nil {
			accepted = divisionName(*r.AcceptedDivision)
		}
		line := []interface{}{i + 1, r.Email, StatusLabels[r.Status], accepted,
			r.SubmittedAt.In(wib).Format("2006-01-02 15:04"), r.UpdatedAt.In(wib).Format("2006-01-02 15:04")}
		for _, q := range common {
			line = append(line, cell(q, r.Answers.Common[q.ID]))
		}
		for n := 0; n < 3; n++ {
			if n >= len(r.Choices) {
				line = append(line, "")
				for range form.ChoiceQuestions {
					line = append(line, "")
				}
				line = append(line, "")
				continue
			}
			code := r.Choices[n]
			answers := r.Answers.Divisions[code]
			line = append(line, divisionName(code))
			for _, q := range form.ChoiceQuestions {
				line = append(line, cell(q, answers[q.ID]))
			}
			division, _ := form.Division(code)
			parts := []string{}
			for _, q := range division.Questions {
				if v := cell(q, answers[q.ID]); v != "" {
					parts = append(parts, q.Label+": "+v)
				}
			}
			line = append(line, strings.Join(parts, "\n"))
		}
		line = append(line, deref(r.AdminNote))
		data = append(data, line)
	}
	body, err := export.Workbook("Pendaftar P3RI", headers, data)
	if err != nil {
		return "", nil, err
	}
	return export.Filename("Pendaftar P3RI 1448 H"), body, nil
}

var wib = time.FixedZone("WIB", 7*60*60)

func divisionName(code string) string {
	if d, ok := Current().Division(code); ok {
		return d.Name
	}
	return code
}

// cell renders an answer as text.
func cell(q Question, v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case float64:
		return fmt.Sprintf("%g", value)
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		parts := []string{}
		for _, row := range q.Rows {
			if col, ok := value[row]; ok {
				parts = append(parts, fmt.Sprintf("%s: %v", row, col))
			}
		}
		return strings.Join(parts, "; ")
	}
	return fmt.Sprint(v)
}
