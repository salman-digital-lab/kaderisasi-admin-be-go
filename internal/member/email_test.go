package member

import "testing"

func TestNormalizeEmail(t *testing.T) {
	if NormalizeEmail(nil) != nil {
		t.Fatal("nil stays nil")
	}
	for in, want := range map[string]string{
		"  Budi@Gmail.COM\n":             "budi@gmail.com",
		"\xef\xbb\xbfbudi@example.test ": "budi@example.test",
		"plain@example.test":             "plain@example.test",
	} {
		if got := *NormalizeEmail(&in); got != want {
			t.Errorf("NormalizeEmail(%q) = %q, want %q", in, got, want)
		}
	}
}
