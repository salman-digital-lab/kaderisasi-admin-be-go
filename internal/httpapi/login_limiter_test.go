package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginLimiterBlocksAfterRepeatedFailures(t *testing.T) {
	now := time.Now()
	l := loginLimiter{now: func() time.Time { return now }}
	for i := 0; i < loginEmailLimit; i++ {
		if l.blocked("a@example.test", "1.1.1.1") {
			t.Fatalf("blocked after %d failures", i)
		}
		l.fail("a@example.test", "1.1.1.1")
	}
	if !l.blocked("a@example.test", "9.9.9.9") {
		t.Fatal("email not blocked from another client")
	}
	if l.blocked("b@example.test", "9.9.9.9") {
		t.Fatal("unrelated email blocked")
	}
	now = now.Add(loginWindow)
	if l.blocked("a@example.test", "1.1.1.1") {
		t.Fatal("still blocked after the window")
	}
}

func TestLoginLimiterResetAndClientLimit(t *testing.T) {
	l := loginLimiter{}
	l.fail("a@example.test", "1.1.1.1")
	l.reset("a@example.test")
	if _, ok := l.failures["email:a@example.test"]; ok {
		t.Fatal("reset kept email counter")
	}
	for i := 0; i < loginClientLimit; i++ {
		l.fail("", "2.2.2.2")
	}
	if !l.blocked("", "2.2.2.2") || l.blocked("", "3.3.3.3") {
		t.Fatal("client limit wrong")
	}
}

func TestLimiterClientUsesLastForwardedAddress(t *testing.T) {
	r := httptest.NewRequest("POST", "/", nil)
	r.RemoteAddr = "10.0.0.1:5555"
	if got := limiterClient(r); got != "10.0.0.1" {
		t.Fatalf("got %q", got)
	}
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 7.7.7.7")
	if got := limiterClient(r); got != "7.7.7.7" {
		t.Fatalf("got %q", got)
	}
}

func TestAccessTokenAndMutatingMethods(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	if accessToken(r) != "" {
		t.Fatal("expected no token")
	}
	r.Header.Set("Authorization", "Bearer abc")
	if accessToken(r) != "abc" {
		t.Fatal("bearer not read")
	}
	r.AddCookie(&http.Cookie{Name: "admin_access_jwt", Value: "cookie"})
	if accessToken(r) != "cookie" {
		t.Fatal("cookie should win")
	}
	if !mutatesState("DELETE") || mutatesState("GET") {
		t.Fatal("mutatesState wrong")
	}
}
