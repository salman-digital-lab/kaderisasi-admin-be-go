package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/scrypt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const RefreshCookie = "admin_refresh_token"
const AccessTTL = 15 * time.Minute
const RefreshTTL = 30 * 24 * time.Hour

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash, err := scrypt.Key([]byte(password), salt, 16384, 8, 1, 64)
	if err != nil {
		return "", err
	}
	return "$scrypt$n=16384,r=8,p=1$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(hash), nil
}

func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[1] != "scrypt" {
		return false
	}
	params := map[string]int{}
	for _, part := range strings.Split(parts[2], ",") {
		pair := strings.SplitN(part, "=", 2)
		if len(pair) != 2 {
			return false
		}
		n, err := strconv.Atoi(pair[1])
		if err != nil {
			return false
		}
		params[pair[0]] = n
	}
	n, r, p := params["n"], params["r"], params["p"]
	if n < 2 || n > 262144 || r < 1 || r > 32 || p < 1 || p > 16 || int64(n)*int64(r)*128 >= 33554432 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(salt) < 8 || len(salt) > 1024 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(expected) < 64 || len(expected) > 128 {
		return false
	}
	got, err := scrypt.Key([]byte(password), salt, n, r, p, len(expected))
	return err == nil && subtle.ConstantTimeCompare(got, expected) == 1
}

type Claims struct {
	UserID int32  `json:"userId"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

const AdminAudience = "kaderisasi-admin"

func SignAccess(key string, userID int32, email string, now time.Time) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{UserID: userID, Email: email, RegisteredClaims: jwt.RegisteredClaims{Audience: jwt.ClaimStrings{AdminAudience}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL))}}).SignedString([]byte(key))
}

func VerifyAccess(key, token string, now time.Time) (Claims, error) {
	var claims Claims
	_, err := jwt.ParseWithClaims(token, &claims, func(token *jwt.Token) (interface{}, error) { return []byte(key), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithTimeFunc(func() time.Time { return now }))
	if err != nil || claims.UserID <= 0 {
		return Claims{}, errors.New("invalid access token")
	}
	// Existing admin access tokens last 15 minutes. Legacy learner tokens last
	// one day and must never be exchanged for an administrator session.
	if len(claims.Audience) == 0 {
		if claims.IssuedAt == nil || claims.ExpiresAt == nil || claims.ExpiresAt.Sub(claims.IssuedAt.Time) != AccessTTL {
			return Claims{}, errors.New("invalid legacy access token")
		}
	} else if len(claims.Audience) != 1 || claims.Audience[0] != AdminAudience {
		return Claims{}, errors.New("invalid access audience")
	}
	return claims, nil
}

type signedMessage struct {
	Message    string     `json:"message"`
	Purpose    string     `json:"purpose"`
	ExpiryDate *time.Time `json:"expiryDate,omitempty"`
}

func SignRefreshCookie(key, token string) string {
	body, _ := json.Marshal(signedMessage{Message: token, Purpose: RefreshCookie})
	encoded := base64.RawURLEncoding.EncodeToString(body)
	secret := sha256.Sum256([]byte(key))
	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write([]byte(encoded))
	return "s:" + encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func VerifyRefreshCookie(key, value string, now time.Time) (string, error) {
	value, err := url.PathUnescape(value)
	if err != nil || !strings.HasPrefix(value, "s:") {
		return "", errors.New("invalid cookie")
	}
	parts := strings.Split(strings.TrimPrefix(value, "s:"), ".")
	if len(parts) != 2 {
		return "", errors.New("invalid cookie")
	}
	secret := sha256.Sum256([]byte(key))
	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write([]byte(parts[0]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(mac.Sum(nil), signature) {
		return "", errors.New("invalid cookie signature")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	var message signedMessage
	if err := json.Unmarshal(body, &message); err != nil || message.Message == "" || message.Purpose != RefreshCookie || (message.ExpiryDate != nil && now.After(*message.ExpiryDate)) {
		return "", errors.New("invalid cookie purpose or expiry")
	}
	return message.Message, nil
}

func RandomToken() (string, error) {
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func HashToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func UUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
