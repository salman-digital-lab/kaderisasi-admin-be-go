package form

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/xuri/excelize/v2"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"strings"
	"time"
)

type Attachment struct {
	ID           pgtype.UUID `json:"id"`
	Name         string      `json:"name"`
	DownloadName string      `json:"download_name"`
	MIMEType     string      `json:"mime_type"`
	Size         int32       `json:"size"`
}
type SavedResponse struct {
	ID          pgtype.UUID     `json:"id"`
	UserID      *int32          `json:"user_id"`
	Snapshot    json.RawMessage `json:"form_snapshot"`
	Answers     json.RawMessage `json:"answers"`
	CreatedAt   time.Time       `json:"created_at"`
	Attachments []Attachment    `json:"attachments"`
}
type ResponsePage struct {
	Data  []SavedResponse `json:"data"`
	Total int64           `json:"total"`
}

func responseView(row dbgen.CustomFormResponse) SavedResponse {
	return SavedResponse{ID: row.ID, UserID: row.UserID, Snapshot: row.FormSnapshot, Answers: row.Answers, CreatedAt: row.CreatedAt.Time, Attachments: []Attachment{}}
}
func (s Service) Responses(ctx context.Context, formID int32, page, size int32) (ResponsePage, error) {
	return listResponses(ctx, dbgen.New(s.Pool), formID, page, size)
}
func listResponses(ctx context.Context, q *dbgen.Queries, formID int32, page, size int32) (ResponsePage, error) {
	total, err := q.CountFormResponses(ctx, formID)
	if err != nil {
		return ResponsePage{}, err
	}
	rows, err := q.ListFormResponses(ctx, dbgen.ListFormResponsesParams{FormID: formID, Limit: size, Offset: (page - 1) * size})
	result := ResponsePage{Data: []SavedResponse{}, Total: total}
	for _, row := range rows {
		result.Data = append(result.Data, responseView(row))
	}
	return result, err
}
func (s Service) Response(ctx context.Context, formID int32, id pgtype.UUID) (SavedResponse, error) {
	return getResponse(ctx, dbgen.New(s.Pool), formID, id)
}
func getResponse(ctx context.Context, q *dbgen.Queries, formID int32, id pgtype.UUID) (SavedResponse, error) {
	row, err := q.FormResponseByID(ctx, dbgen.FormResponseByIDParams{FormID: formID, ResponseID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return SavedResponse{}, domain.Fail(404, "FORM_RESPONSE_NOT_FOUND")
	}
	if err != nil {
		return SavedResponse{}, err
	}
	result := responseView(row)
	files, err := q.ResponseAttachments(ctx, row.SessionID)
	for _, file := range files {
		result.Attachments = append(result.Attachments, Attachment{file.ID, file.OriginalName, file.DownloadName, file.MimeType, file.SizeBytes})
	}
	return result, err
}
func safeCell(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}
func (s Service) ExportResponses(ctx context.Context, formID int32, downloadBase string) ([]byte, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	queries := dbgen.New(tx)
	book := excelize.NewFile()
	defer book.Close()
	sheet := "Respons"
	if err := book.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	headings := []interface{}{"ID respons", "Dikirim", "ID anggota", "Bagian", "Pertanyaan", "Jawaban"}
	if err := book.SetSheetRow(sheet, "A1", &headings); err != nil {
		return nil, err
	}
	rowNumber := 2
	if _, err := book.NewSheet("Berkas"); err != nil {
		return nil, err
	}
	fileHeadings := []interface{}{"ID respons", "Pertanyaan", "Nama berkas", "Unduh (login admin)"}
	if err := book.SetSheetRow("Berkas", "A1", &fileHeadings); err != nil {
		return nil, err
	}
	fileRow := 2
	for page := int32(1); ; page++ {
		responses, err := listResponses(ctx, queries, formID, page, 100)
		if err != nil {
			return nil, err
		}
		for _, response := range responses.Data {
			detail, err := getResponse(ctx, queries, formID, response.ID)
			if err != nil {
				return nil, err
			}
			var snapshot struct {
				Schema Schema `json:"schema"`
			}
			var answers map[string]json.RawMessage
			if err = json.Unmarshal(detail.Snapshot, &snapshot); err != nil {
				return nil, err
			}
			if err = json.Unmarshal(detail.Answers, &answers); err != nil {
				return nil, err
			}
			firstRow := rowNumber
			user := ""
			if detail.UserID != nil {
				user = fmt.Sprint(*detail.UserID)
			}
			for _, section := range snapshot.Schema.Fields {
				for _, field := range section.Fields {
					raw, exists := answers[field.Key]
					if !exists {
						continue
					}
					text := string(raw)
					var scalar string
					if json.Unmarshal(raw, &scalar) == nil {
						text = scalar
					}
					if field.Type == "file" {
						var ids []string
						_ = json.Unmarshal(raw, &ids)
						lines := []string{}
						for _, id := range ids {
							for _, file := range detail.Attachments {
								if file.ID.String() == id {
									lines = append(lines, file.Name+"\n"+downloadBase+"/"+id)
									cells := []interface{}{detail.ID.String(), safeCell(field.Label), safeCell(file.Name), "Unduh berkas"}
									if err = book.SetSheetRow("Berkas", fmt.Sprintf("A%d", fileRow), &cells); err != nil {
										return nil, err
									}
									if err = book.SetCellHyperLink("Berkas", fmt.Sprintf("D%d", fileRow), downloadBase+"/"+id, "External"); err != nil {
										return nil, err
									}
									fileRow++
								}
							}
						}
						text = strings.Join(lines, "\n")
					}
					cells := []interface{}{detail.ID.String(), detail.CreatedAt.Format(time.RFC3339), user, safeCell(section.Name), safeCell(field.Label), safeCell(text)}
					if err = book.SetSheetRow(sheet, fmt.Sprintf("A%d", rowNumber), &cells); err != nil {
						return nil, err
					}
					rowNumber++
				}
			}
			if rowNumber == firstRow {
				// An intentionally empty response still has an identity and timestamp.
				cells := []interface{}{detail.ID.String(), detail.CreatedAt.Format(time.RFC3339), user}
				if err = book.SetSheetRow(sheet, fmt.Sprintf("A%d", rowNumber), &cells); err != nil {
					return nil, err
				}
				rowNumber++
			}
		}
		if int64(page)*100 >= responses.Total {
			break
		}
	}
	_ = book.SetColWidth(sheet, "A", "E", 24)
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	_ = book.SetColWidth("Berkas", "A", "D", 35)
	_ = book.SetColWidth(sheet, "F", "F", 65)
	_ = book.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	body, err := book.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}
