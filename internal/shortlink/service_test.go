package shortlink

import "testing"

func TestDestinations(t *testing.T) {
	for _, value := range []string{"https://salmanitb.com/path?q=a%20b#section", "http://example.com", "https://example.com/é"} {
		if err := ValidateDestination(value, "https://s.salmanitb.com"); err != nil {
			t.Errorf("valid %q: %v", value, err)
		}
	}
	for _, value := range []string{"", "//example.com", "javascript:alert(1)", "https:example.com", "https://user@example.com", "https://@example.com", "https://S.SALMANITB.COM/a", "https://s.salmanitb.com./a", "https://example.com\n", "https://example.com\\@s.salmanitb.com", "https://example.com/a b"} {
		if err := ValidateDestination(value, "https://s.salmanitb.com"); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
}
func TestCodes(t *testing.T) {
	for _, code := range []string{"abc", "aB_01-xyz9"} {
		if err := ValidateCode(code); err != nil {
			t.Fatal(err)
		}
	}
	for _, code := range []string{"ab", "abcdefghijk", "health", "a/b", "éab", " abc"} {
		if ValidateCode(code) == nil {
			t.Errorf("accepted %q", code)
		}
	}
	for range 100 {
		code, err := generateCode()
		if err != nil || len(code) != 6 || ValidateCode(code) != nil {
			t.Fatalf("invalid generated code %q: %v", code, err)
		}
	}
}
