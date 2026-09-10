package auth

import (
	"bytes"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
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

func TestAccessTokenAudienceIsolation(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	key := "synthetic-audience-test-key"
	for _, tc := range []struct {
		name     string
		audience jwt.ClaimStrings
		lifetime time.Duration
		valid    bool
	}{
		{"admin", jwt.ClaimStrings{AdminAudience}, AccessTTL, true},
		{"legacy admin", nil, AccessTTL, true},
		{"learner", jwt.ClaimStrings{"kaderisasi-public"}, 24 * time.Hour, false},
		{"legacy learner", nil, 24 * time.Hour, false},
		{"mixed audiences", jwt.ClaimStrings{AdminAudience, "kaderisasi-public"}, AccessTTL, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := Claims{UserID: 1, Email: "shared-id@example.test", RegisteredClaims: jwt.RegisteredClaims{Audience: tc.audience, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(tc.lifetime))}}
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(key))
			if err != nil {
				t.Fatal(err)
			}
			_, err = VerifyAccess(key, token, now)
			if (err == nil) != tc.valid {
				t.Fatalf("audience validation: %v", err)
			}
		})
	}
	access, err := SignAccess(key, 1, "admin@example.test", now)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := VerifyAccess(key, access, now)
	if err != nil || len(claims.Audience) != 1 || claims.Audience[0] != AdminAudience {
		t.Fatal("new admin tokens must identify their audience", err)
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
