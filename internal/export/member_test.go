package export

import (
	"bytes"
	"encoding/csv"
	"github.com/xuri/excelize/v2"
	"reflect"
	"strings"
	"testing"
)

func TestMemberFiles(t *testing.T) {
	rows := [][]string{{"001234567890", "=1+1", "Nama, dengan\nbaris baru", " +SUM(A1:A2)"}}
	body, err := MemberFile([]string{"Phone", "Name", "History", "Extra"}, rows, "csv")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(body), "\xef\xbb\xbf"))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parsed[1], []string{rows[0][0], "'=1+1", rows[0][2], "' +SUM(A1:A2)"}) {
		t.Fatalf("CSV: %v", parsed)
	}
	body, err = MemberFile([]string{"Phone", "Name", "History", "Extra"}, rows, "xlsx")
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	parsed, err = book.GetRows("Anggota")
	if err != nil || !reflect.DeepEqual(parsed[1], rows[0]) {
		t.Fatalf("XLSX: %v %v", parsed, err)
	}
	formula, err := book.GetCellFormula("Anggota", "B2")
	if err != nil || formula != "" {
		t.Fatalf("unexpected formula %q %v", formula, err)
	}
}
