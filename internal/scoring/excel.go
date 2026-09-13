package scoring

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/xuri/excelize/v2"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"sort"
	"strconv"
	"strings"
)

const MaxWorkbookBytes = 5 << 20
const maxExcelRows = 10000

type ImportChange struct {
	Row            int    `json:"row"`
	RegistrationID int32  `json:"registration_id"`
	Name           string `json:"name"`
	SaveInput
}
type ImportError struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
}
type Preview struct {
	Changes []ImportChange `json:"changes"`
	Errors  []ImportError  `json:"errors"`
	Hash    string         `json:"hash"`
}

func workbookRows(ctx context.Context, q *dbgen.Queries, r *Rubric, id int32) (Page, error) {
	p, err := list(ctx, q, r, id, "", "", 1, maxExcelRows+1)
	if err == nil && p.Total > maxExcelRows {
		return p, domain.Fail(422, "SCORING_EXCEL_ROW_LIMIT")
	}
	return p, err
}
func (s Service) Workbook(ctx context.Context, id int32, mode string) ([]byte, error) {
	if mode != "template" && mode != "draft" && mode != "published" {
		return nil, domain.Fail(422, "INVALID_SCORING_EXPORT")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	if err = lockActivity(ctx, q, id); err != nil {
		return nil, err
	}
	r, err := rubric(ctx, q, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, domain.Fail(422, "SCORING_RUBRIC_REQUIRED")
	}
	p, err := workbookRows(ctx, q, r, id)
	if err != nil {
		return nil, err
	}
	f := excelize.NewFile()
	defer f.Close()
	if err = f.SetSheetName("Sheet1", "Scores"); err != nil {
		return nil, err
	}
	if _, err = f.NewSheet("Metadata"); err != nil {
		return nil, err
	}
	metadata := [][]interface{}{{"schema_version", 1}, {"activity_id", id}, {"rubric_revision", r.Revision}, {"mode", mode}, {"instructions", "Isi nilai mentah. Sel kosong tidak mengubah nilai. Jangan ubah ID atau revisi. Catatan kosong tidak mengubah catatan."}}
	for i, row := range metadata {
		if err = f.SetSheetRow("Metadata", fmt.Sprintf("A%d", i+1), &row); err != nil {
			return nil, err
		}
	}
	criteria := Criteria(r.Definition)
	header := []interface{}{"registration_id", "name", "revision", "note"}
	labels := []interface{}{"ID peserta", "Nama peserta", "Revisi", "Catatan peserta"}
	for _, c := range criteria {
		header = append(header, "criterion:"+c.ID)
		labels = append(labels, fmt.Sprintf("%s (maks. %g, bobot %g)", c.Name, c.Maximum, c.Weight))
	}
	if mode != "template" {
		header = append(header, "total", "grade")
		labels = append(labels, "Total (0–100)", "Indeks")
	}
	if err = f.SetSheetRow("Scores", "A1", &header); err != nil {
		return nil, err
	}
	if err = f.SetSheetRow("Scores", "A2", &labels); err != nil {
		return nil, err
	}
	for i, e := range p.Entries {
		draft := Draft{Scores: map[string]*float64{}}
		var result *Result
		if mode == "draft" && e.Data != nil {
			draft = e.Data.Draft
			result = e.Result
		}
		if mode == "published" {
			if e.Data == nil || e.Data.Published == nil {
				continue
			}
			draft = e.Data.Published.Draft
			result = &e.Data.Published.Result
		}
		row := []interface{}{e.RegistrationID, e.Name, revision(e.Data), draft.Note}
		for _, c := range criteria {
			if score := draft.Scores[c.ID]; score != nil {
				row = append(row, *score)
			} else {
				row = append(row, "")
			}
		}
		if mode != "template" {
			var total, grade interface{} = "", ""
			if result != nil && result.Total != nil {
				total = *result.Total
			}
			if result != nil && result.Grade != nil {
				grade = *result.Grade
			}
			row = append(row, total, grade)
		}
		if err = f.SetSheetRow("Scores", fmt.Sprintf("A%d", i+3), &row); err != nil {
			return nil, err
		}
	}
	if err = f.SetColWidth("Scores", "A", "C", 18); err != nil {
		return nil, err
	}
	if err = f.SetColWidth("Scores", "B", "B", 30); err != nil {
		return nil, err
	}
	last, _ := excelize.ColumnNumberToName(len(header))
	if err = f.SetColWidth("Scores", "D", last, 28); err != nil {
		return nil, err
	}
	if err = f.SetPanes("Scores", &excelize.Panes{Freeze: true, YSplit: 2, TopLeftCell: "A3", ActivePane: "bottomLeft"}); err != nil {
		return nil, err
	}
	buffer, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
func parseWorkbook(body []byte, r *Rubric, id int32, p Page) (Preview, error) {
	preview := Preview{Changes: []ImportChange{}, Errors: []ImportError{}}
	if len(body) == 0 || len(body) > MaxWorkbookBytes {
		return preview, domain.Fail(413, "INVALID_SCORING_WORKBOOK_SIZE")
	}
	f, err := excelize.OpenReader(bytes.NewReader(body), excelize.Options{UnzipSizeLimit: 32 << 20, UnzipXMLSizeLimit: 8 << 20})
	if err != nil {
		return preview, domain.Fail(422, "INVALID_SCORING_WORKBOOK")
	}
	defer f.Close()
	for cell, expected := range map[string]string{"B1": "1", "B2": strconv.Itoa(int(id)), "B3": strconv.Itoa(int(r.Revision)), "B4": "template"} {
		value, e := f.GetCellValue("Metadata", cell)
		if e != nil || value != expected {
			return preview, domain.Fail(409, "SCORING_TEMPLATE_MISMATCH")
		}
	}
	rows, err := f.GetRows("Scores", excelize.Options{RawCellValue: true})
	if err != nil || len(rows) < 2 || len(rows) > maxExcelRows+2 {
		return preview, domain.Fail(422, "INVALID_SCORING_WORKBOOK")
	}
	criteria := Criteria(r.Definition)
	expected := []string{"registration_id", "name", "revision", "note"}
	for _, c := range criteria {
		expected = append(expected, "criterion:"+c.ID)
	}
	if len(rows[0]) != len(expected) {
		return preview, domain.Fail(422, "INVALID_SCORING_COLUMNS")
	}
	for i, key := range expected {
		if rows[0][i] != key {
			return preview, domain.Fail(422, "INVALID_SCORING_COLUMNS")
		}
	}
	entries := map[int32]Entry{}
	for _, e := range p.Entries {
		entries[e.RegistrationID] = e
	}
	seen := map[int32]bool{}
	for index, row := range rows[2:] {
		rowNumber := index + 3
		cell := func(i int) string {
			if i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
		if strings.TrimSpace(strings.Join(row, "")) == "" {
			continue
		}
		addError := func(column, message string) {
			preview.Errors = append(preview.Errors, ImportError{Row: rowNumber, Column: column, Message: message})
		}
		before := len(preview.Errors)
		for column := range max(len(row), len(expected)) {
			coord, _ := excelize.CoordinatesToCellName(column+1, rowNumber)
			formula, e := f.GetCellFormula("Scores", coord)
			if e != nil || formula != "" {
				addError(coord, "Formula tidak diperbolehkan")
			}
		}
		parsed, e := strconv.ParseInt(cell(0), 10, 32)
		entry, exists := entries[int32(parsed)]
		if e != nil || !exists {
			addError("A", "Peserta tidak ditemukan dalam kegiatan ini")
			continue
		}
		if seen[entry.RegistrationID] {
			addError("A", "Peserta duplikat")
			continue
		}
		seen[entry.RegistrationID] = true
		rev, e := strconv.ParseInt(cell(2), 10, 32)
		if e != nil || int32(rev) != revision(entry.Data) {
			addError("C", "Revisi berubah. Unduh templat terbaru.")
		}
		draft := Draft{Scores: map[string]*float64{}}
		if entry.Data != nil {
			draft.Note = entry.Data.Draft.Note
			for key, value := range entry.Data.Draft.Scores {
				draft.Scores[key] = value
			}
		}
		changed := false
		if cell(3) != "" {
			draft.Note = cell(3)
			changed = true
		}
		for i, c := range criteria {
			value := cell(i + 4)
			if value == "" {
				continue
			}
			changed = true
			n, e := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
			if e != nil || !validNumber(n, false) || n > c.Maximum {
				column, _ := excelize.ColumnNumberToName(i + 5)
				addError(column, "Nilai harus dalam rentang dan maksimal dua desimal")
				continue
			}
			draft.Scores[c.ID] = &n
		}
		if len(row) > len(expected) {
			addError("", "Kolom tidak dikenal")
		}
		if _, e := Calculate(r.Definition, draft); e != nil {
			addError("", e.Error())
		}
		if changed && before == len(preview.Errors) {
			preview.Changes = append(preview.Changes, ImportChange{Row: rowNumber, RegistrationID: entry.RegistrationID, Name: entry.Name, SaveInput: SaveInput{Revision: int32(rev), RubricRevision: r.Revision, Draft: draft}})
		}
	}
	raw, err := json.Marshal(preview)
	if err != nil {
		return preview, err
	}
	h := sha256.New()
	h.Write(body)
	h.Write(raw)
	preview.Hash = hex.EncodeToString(h.Sum(nil))
	return preview, nil
}
func (s Service) Import(ctx context.Context, id, actor int32, body []byte, expectedHash string, commit bool) (Preview, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Preview{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	if err = lockActivity(ctx, q, id); err != nil {
		return Preview{}, err
	}
	r, err := rubric(ctx, q, id)
	if err != nil {
		return Preview{}, err
	}
	if r == nil {
		return Preview{}, domain.Fail(422, "SCORING_RUBRIC_REQUIRED")
	}
	p, err := workbookRows(ctx, q, r, id)
	if err != nil {
		return Preview{}, err
	}
	preview, err := parseWorkbook(body, r, id, p)
	if err != nil {
		return preview, err
	}
	if !commit {
		return preview, nil
	}
	if expectedHash == "" || expectedHash != preview.Hash {
		return preview, domain.Fail(409, "SCORING_IMPORT_CHANGED_REVIEW_AGAIN")
	}
	if len(preview.Errors) > 0 || len(preview.Changes) == 0 {
		return preview, domain.Fail(422, "SCORING_IMPORT_HAS_ERRORS")
	}
	sort.Slice(preview.Changes, func(i, j int) bool { return preview.Changes[i].RegistrationID < preview.Changes[j].RegistrationID })
	for _, change := range preview.Changes {
		if _, err = saveDraft(ctx, q, r, id, change.RegistrationID, actor, change.SaveInput); err != nil {
			return preview, err
		}
	}
	return preview, tx.Commit(ctx)
}
