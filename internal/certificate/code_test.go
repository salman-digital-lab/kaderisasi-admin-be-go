package certificate

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestCertificateCodeEntropyAndUTCYear(t *testing.T) {
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	code, err := GenerateCode(42, now, bytes.NewReader(bytes.Repeat([]byte{0xab}, 16)))
	if err != nil || code != "CERT-2026-42-"+strings.Repeat("AB", 16) {
		t.Fatal(code, err)
	}
	if _, err = GenerateCode(0, now, bytes.NewReader(make([]byte, 16))); err == nil {
		t.Fatal("invalid activity accepted")
	}
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	code, err = GenerateCode(42, time.Date(2027, 1, 1, 0, 30, 0, 0, jakarta), bytes.NewReader(make([]byte, 16)))
	if err != nil || !strings.HasPrefix(code, "CERT-2026-") {
		t.Fatal("UTC year boundary", code, err)
	}
}
