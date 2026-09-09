package auth

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestAdonisInteroperability(t *testing.T) {
	command := exec.Command("node", "../../scripts/crypto-fixture.cjs")
	raw, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture: %v: %s", err, raw)
	}
	var f struct{ Key, Password, Refresh, Hash, Cookie, Access string }
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000100, 0)
	if !VerifyPassword(f.Hash, f.Password) || VerifyPassword(f.Hash, "wrong") {
		t.Fatal("Adonis password interoperability")
	}
	value, err := VerifyRefreshCookie(f.Key, f.Cookie, now)
	if err != nil || value != f.Refresh {
		t.Fatal("Adonis signed cookie interoperability", err)
	}
	claims, err := VerifyAccess(f.Key, f.Access, now)
	if err != nil || claims.UserID != 42 {
		t.Fatal("Adonis JWT interoperability", err)
	}
	if _, err = VerifyAccess(f.Key, f.Access, now.Add(time.Hour)); err == nil {
		t.Fatal("expired token accepted")
	}
	if _, err = VerifyRefreshCookie("different-test-signing-key", f.Cookie, now); err == nil {
		t.Fatal("tampered cookie accepted")
	}
	hash, err := HashPassword(f.Password)
	if err != nil {
		t.Fatal(err)
	}
	access, err := SignAccess(f.Key, 42, "fixture@example.test", time.Unix(1700000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(struct {
		Hash   string `json:"hash"`
		Access string `json:"access"`
		Cookie string `json:"cookie"`
	}{hash, access, SignRefreshCookie(f.Key, f.Refresh)})
	command = exec.Command("node", "../../scripts/crypto-fixture.cjs", "verify")
	command.Stdin = bytes.NewReader(data)
	if output, err := command.CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != "verified" {
		t.Fatalf("reverse interoperability: %v %s", err, output)
	}
}

func TestUnknownRolesFailClosed(t *testing.T) {
	for _, code := range []string{"missing", "toString", "0"} {
		a := ForRole(&code, true)
		if len(a.Permissions) != 0 || a.Role != nil || a.IsSuperAdmin {
			t.Fatal(code)
		}
	}
	super := "super_admin"
	if !ForRole(&super, true).Allows("admin_users.manage") {
		t.Fatal("super admin permissions")
	}
	if ForRole(&super, false).Allows("admin_users.manage") {
		t.Fatal("inactive administrator")
	}
}
