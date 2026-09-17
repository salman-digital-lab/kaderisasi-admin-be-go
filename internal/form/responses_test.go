package form

import "testing"

func TestResponseExportEscapesFormulaPrefixes(t *testing.T) {
	for _, input := range []string{"=SUM(1,2)", "\t+1", "\n-2", " @name"} {
		if got := safeCell(input); got != "'"+input {
			t.Fatalf("unsafe value: %q", got)
		}
	}
	for _, input := range []string{"", " ", "Nama peserta", "2026-09-16"} {
		if got := safeCell(input); got != input {
			t.Fatalf("changed ordinary value: %q", got)
		}
	}
}
