package export

import (
	"bytes"
	"encoding/csv"
	"strings"
)

func MemberFile(headers []string, rows [][]string, format string) ([]byte, error) {
	if format == "csv" {
		var buffer bytes.Buffer
		buffer.WriteString("\xef\xbb\xbf")
		writer := csv.NewWriter(&buffer)
		if err := writer.Write(headers); err != nil {
			return nil, err
		}
		for _, row := range rows {
			cells := make([]string, len(row))
			for i, value := range row {
				trimmed := strings.TrimLeft(value, " \t\r\n")
				if strings.HasPrefix(value, "\t") || strings.HasPrefix(value, "\r") || strings.HasPrefix(value, "\n") || (len(trimmed) > 0 && strings.ContainsRune("=+-@", rune(trimmed[0]))) {
					value = "'" + value
				}
				cells[i] = value
			}
			if err := writer.Write(cells); err != nil {
				return nil, err
			}
		}
		writer.Flush()
		return buffer.Bytes(), writer.Error()
	}
	values := make([][]interface{}, len(rows))
	for i, row := range rows {
		values[i] = make([]interface{}, len(row))
		for j, cell := range row {
			values[i][j] = cell
		}
	}
	return Workbook("Anggota", headers, values)
}
