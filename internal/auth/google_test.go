package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type googleKeyTransport struct{ endpoint *url.URL }

func (t googleKeyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.String() != googleKeysURL {
		panic("unexpected Google key request: " + r.URL.String())
	}
	r = r.Clone(r.Context())
	r.URL = t.endpoint
	return http.DefaultTransport.RoundTrip(r)
}

type googleFixture struct {
	key       *rsa.PrivateKey
	validator *GoogleValidator
	requests  atomic.Int32
}

func newGoogleFixture(t *testing.T) *googleFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &googleFixture{key: key}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		w.Header().Set("Cache-Control", "public, max-age=600")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"keys": []interface{}{map[string]string{"kid": "synthetic-key", "alg": "RS256", "kty": "RSA", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
	}))
	t.Cleanup(server.Close)
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	f.validator, err = NewGoogleValidator(context.Background(), &http.Client{Transport: googleKeyTransport{endpoint}, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *googleFixture) token(t *testing.T, overrides jwt.MapClaims) string {
	t.Helper()
	now := time.Now().Unix()
	claims := jwt.MapClaims{"iss": "https://accounts.google.com", "aud": "synthetic-client", "sub": "synthetic-subject", "email": "fixture@example.test", "email_verified": true, "iat": now - 10, "exp": now + 3600}
	for key, value := range overrides {
		if value == nil {
			delete(claims, key)
		} else {
			claims[key] = value
		}
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "synthetic-key"
	encoded, err := token.SignedString(f.key)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
func TestGoogleSignedTokensAndKeyResponses(t *testing.T) {
	f := newGoogleFixture(t)
	ctx := context.Background()
	now := time.Now().Unix()
	for _, test := range []struct {
		name   string
		claims jwt.MapClaims
		valid  bool
	}{
		{"valid", nil, true}, {"legacy issuer", jwt.MapClaims{"iss": "accounts.google.com"}, true},
		{"wrong issuer", jwt.MapClaims{"iss": "https://example.test"}, false}, {"wrong audience", jwt.MapClaims{"aud": "wrong"}, false},
		{"expired beyond skew", jwt.MapClaims{"exp": now - 360}, false}, {"expired within skew", jwt.MapClaims{"exp": now - 60}, true},
		{"issued in future", jwt.MapClaims{"iat": now + 360}, false}, {"issued within skew", jwt.MapClaims{"iat": now + 60}, true},
		{"missing issued at", jwt.MapClaims{"iat": nil}, false}, {"missing expiry", jwt.MapClaims{"exp": nil}, false},
		{"excessive lifetime", jwt.MapClaims{"exp": now + 90000}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := f.validator.Validate(ctx, f.token(t, test.claims), "synthetic-client")
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v: %v", test.valid, err)
			}
		})
	}
	for _, claims := range []jwt.MapClaims{nil, {"exp": now - 60}} {
		other := newGoogleFixture(t)
		if _, err := f.validator.Validate(ctx, other.token(t, claims), "synthetic-client"); err == nil {
			t.Fatal("forged signature accepted")
		}
	}
	if f.requests.Load() == 0 {
		t.Fatal("no cryptographic key response consumed")
	}
	if _, err := f.validator.Validate(ctx, "malformed", "synthetic-client"); err == nil {
		t.Fatal("malformed accepted")
	}
}
