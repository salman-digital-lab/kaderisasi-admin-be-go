package scoring

import (
	"github.com/xuri/excelize/v2"
	"testing"
)

func testWorkbook(t *testing.T, edit func(*excelize.File)) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", "Scores")
	f.NewSheet("Metadata")
	for cell, value := range map[string]interface{}{"B1": 1, "B2": 7, "B3": 1, "B4": "template"} {
		if err := f.SetCellValue("Metadata", cell, value); err != nil {
			t.Fatal(err)
		}
	}
	for i, row := range [][]interface{}{{"registration_id", "name", "revision", "note", "criterion:honesty", "criterion:trust"}, {"id", "name", "rev", "note", "honesty", "trust"}, {9, "Guest", 2, "", 0, "35,5"}} {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow("Scores", cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	if edit != nil {
		edit(f)
	}
	b, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestExcelPreview(t *testing.T) {
	r := &Rubric{Definition: testDefinition(), Revision: 1}
	p := Page{Entries: []Entry{{RegistrationID: 9, Name: "Guest", Data: &Data{Revision: 2, Draft: Draft{Scores: map[string]*float64{"honesty": ptr(80)}, Note: "Keep note"}}}}}
	preview, err := parseWorkbook(testWorkbook(t, nil), r, 7, p)
	if err != nil || len(preview.Errors) != 0 || len(preview.Changes) != 1 {
		t.Fatalf("preview %#v %v", preview, err)
	}
	if *preview.Changes[0].Draft.Scores["honesty"] != 0 || *preview.Changes[0].Draft.Scores["trust"] != 35.5 || preview.Changes[0].Draft.Note != "Keep note" {
		t.Fatal(preview.Changes)
	}
	for _, tc := range []struct {
		name string
		edit func(*excelize.File)
	}{
		{"formula", func(f *excelize.File) { f.SetCellFormula("Scores", "E3", "1+1") }},
		{"trailing uncached formula", func(f *excelize.File) { f.SetCellValue("Scores", "F3", ""); f.SetCellFormula("Scores", "F3", "1+1") }},
		{"stale", func(f *excelize.File) { f.SetCellValue("Scores", "C3", 1) }},
		{"unknown", func(f *excelize.File) { f.SetCellValue("Scores", "A3", 99) }},
		{"range", func(f *excelize.File) { f.SetCellValue("Scores", "F3", 51) }},
		{"formatted number retains raw value", func(f *excelize.File) {
			f.SetCellValue("Scores", "F3", 1000)
			style, err := f.NewStyle(&excelize.Style{NumFmt: 3})
			if err != nil {
				t.Fatal(err)
			}
			f.SetCellStyle("Scores", "F3", "F3", style)
		}},
		{"duplicate", func(f *excelize.File) {
			row := []interface{}{9, "Guest", 2, "", 5, 10}
			f.SetSheetRow("Scores", "A4", &row)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := parseWorkbook(testWorkbook(t, tc.edit), r, 7, p)
			if err == nil && len(v.Errors) == 0 {
				t.Fatal("accepted invalid import")
			}
		})
	}
	if _, err := parseWorkbook(testWorkbook(t, nil), r, 8, p); err == nil {
		t.Fatal("wrong activity")
	}
	blank, err := parseWorkbook(testWorkbook(t, func(f *excelize.File) { f.SetCellValue("Scores", "E3", ""); f.SetCellValue("Scores", "F3", "") }), r, 7, p)
	if err != nil || len(blank.Changes) != 0 {
		t.Fatal("blank import changes scores")
	}
}
