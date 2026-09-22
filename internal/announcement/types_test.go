package announcement

import (
	"strings"
	"testing"
	"time"
)

func TestValidateAnnouncement(t *testing.T) {
	good := func() Input { return Input{Title: " Pesan ", Body: "Isi", Audience: Audience{AllMembers: true}} }
	for _, mutate := range []func(*Input){
		func(i *Input) { i.Title = " " }, func(i *Input) { i.Title = strings.Repeat("文", 161) }, func(i *Input) { i.Body = strings.Repeat("文", 10001) },
		func(i *Input) { v := "javascript:alert(1)"; l := "Buka"; i.LinkURL = &v; i.LinkLabel = &l }, func(i *Input) { v := "https://user:pass@example.com"; l := "Buka"; i.LinkURL = &v; i.LinkLabel = &l },
		func(i *Input) { v := "https://example.com"; i.LinkURL = &v }, func(i *Input) { i.Audience.MemberIDs = []int32{-1} }, func(i *Input) { i.Audience.RoleCodes = []string{"unknown"} },
	} {
		in := good()
		mutate(&in)
		if Validate(&in) == nil {
			t.Fatalf("invalid input accepted: %#v", in)
		}
	}
	in := good()
	if e := Validate(&in); e != nil {
		t.Fatal(e)
	}
	if in.Title != "Pesan" || len(in.Audience.ClubStatuses) != 1 || in.Audience.ClubStatuses[0] != "APPROVED" {
		t.Fatal("normalization")
	}
	in = good()
	in.Title = strings.Repeat("文", 160)
	if e := Validate(&in); e != nil {
		t.Fatal("unicode title rejected", e)
	}
}
func TestCursorPrecision(t *testing.T) {
	stamp := time.Date(2026, 9, 22, 1, 2, 3, 123456000, time.UTC)
	encoded := EncodeCursor(22, stamp)
	c, e := DecodeCursor(encoded)
	if e != nil || c.ID != 22 || !c.Time.Equal(stamp) {
		t.Fatal("cursor precision", c, e)
	}
	for _, s := range []string{"garbage", "e30", "W10"} {
		if _, e := DecodeCursor(s); e == nil {
			t.Fatal("invalid cursor accepted")
		}
	}
}
