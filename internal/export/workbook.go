package export

import (
	"fmt"
	"github.com/xuri/excelize/v2"
	"regexp"
	"unicode/utf16"
)

var filenameDisallowed = regexp.MustCompile(`[^\w\s-]`)
var whitespace = regexp.MustCompile(`\s+`)

func Filename(name string) string {
	return whitespace.ReplaceAllString(filenameDisallowed.ReplaceAllString(name, ""), "-") + ".xlsx"
}
func Workbook(sheet string, headers []string, rows [][]interface{}) ([]byte, error) {
	book := excelize.NewFile()
	defer book.Close()
	if err := book.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	header := make([]interface{}, len(headers))
	widths := make([]int, len(headers))
	for i, value := range headers {
		header[i] = value
		widths[i] = len(utf16.Encode([]rune(value)))
	}
	if err := book.SetSheetRow(sheet, "A1", &header); err != nil {
		return nil, err
	}
	style, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E0E0E0"}}})
	if err != nil {
		return nil, err
	}
	end, _ := excelize.CoordinatesToCellName(len(headers), 1)
	if err = book.SetCellStyle(sheet, "A1", end, style); err != nil {
		return nil, err
	}
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		if err = book.SetSheetRow(sheet, cell, &row); err != nil {
			return nil, err
		}
		for j, value := range row {
			if j >= len(widths) {
				break
			}
			length := len(utf16.Encode([]rune(fmt.Sprint(value))))
			if value == nil || value == "" || value == 0 {
				length = 10
			}
			if length > widths[j] {
				widths[j] = length
			}
		}
	}
	for i, length := range widths {
		width := float64(length + 2)
		if length < 10 {
			width = 10
		}
		if length > 50 {
			width = 50
		}
		column, _ := excelize.ColumnNumberToName(i + 1)
		if err = book.SetColWidth(sheet, column, column, width); err != nil {
			return nil, err
		}
	}
	buffer, err := book.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
