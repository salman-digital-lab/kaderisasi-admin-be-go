package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	loginWindow      = 15 * time.Minute
	loginEmailLimit  = 10
	loginClientLimit = 30
	loginMaxKeys     = 10000
)

// loginLimiter counts failed credential attempts per email and per client so a
// password can't be guessed at network speed. State is per process: a restart or
// a second replica resets it, which is acceptable for a brute-force brake.
type loginLimiter struct {
	mu       sync.Mutex
	failures map[string]*loginFailures
	now      func() time.Time
}

type loginFailures struct {
	count int
	start time.Time
}

func (l *loginLimiter) clock() time.Time {
	if l.now != nil {
		return l.now()
	}
	return time.Now()
}

// blocked reports whether either key has used up its failures in the window.
func (l *loginLimiter) blocked(email, client string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return (email != "" && l.over("email:"+email, loginEmailLimit)) || l.over("client:"+client, loginClientLimit)
}

func (l *loginLimiter) over(key string, limit int) bool {
	entry, ok := l.failures[key]
	if !ok {
		return false
	}
	if l.clock().Sub(entry.start) >= loginWindow {
		delete(l.failures, key)
		return false
	}
	return entry.count >= limit
}

func (l *loginLimiter) fail(email, client string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failures == nil {
		l.failures = map[string]*loginFailures{}
	}
	now := l.clock()
	if len(l.failures) >= loginMaxKeys {
		for key, entry := range l.failures {
			if now.Sub(entry.start) >= loginWindow {
				delete(l.failures, key)
			}
		}
	}
	keys := []string{"client:" + client}
	if email != "" {
		keys = append(keys, "email:"+email)
	}
	for _, key := range keys {
		entry, ok := l.failures[key]
		if !ok || now.Sub(entry.start) >= loginWindow {
			if !ok && len(l.failures) >= loginMaxKeys {
				continue
			}
			entry = &loginFailures{start: now}
			l.failures[key] = entry
		}
		entry.count++
	}
}

// reset clears the email counter after a successful login.
func (l *loginLimiter) reset(email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, "email:"+email)
}

// limiterClient keys on the address the reverse proxy appended to
// X-Forwarded-For, falling back to the socket peer.
func limiterClient(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if last := strings.TrimSpace(parts[len(parts)-1]); last != "" {
			return last
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func limiterEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
