package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
)

const googleKeysURL = "https://www.googleapis.com/oauth2/v3/certs"

// GoogleValidator adds the issuer and issuance-time checks performed by the
// Adonis google-auth-library. No environment option can replace Google's keys.
type GoogleValidator struct {
	validator *idtoken.Validator
	client    *http.Client
}

func NewGoogleValidator(ctx context.Context, client *http.Client) (*GoogleValidator, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	validator, err := idtoken.NewValidator(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	return &GoogleValidator{validator: validator, client: client}, nil
}

func (v *GoogleValidator) Validate(ctx context.Context, credential, audience string) (*idtoken.Payload, error) {
	payload, err := idtoken.ParsePayload(credential)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	if payload.Issuer != "accounts.google.com" && payload.Issuer != "https://accounts.google.com" {
		return nil, errors.New("invalid Google issuer")
	}
	if audience == "" || payload.Audience != audience || payload.IssuedAt == 0 || payload.Expires == 0 || payload.IssuedAt > now+300 || payload.Expires < now-300 || payload.Expires >= now+86400 {
		return nil, errors.New("invalid Google token claims")
	}
	verified, err := v.validator.Validate(ctx, credential, audience)
	if err == nil {
		return verified, nil
	}
	if payload.Expires >= now {
		return nil, err
	}
	// Google's Go helper has no clock-skew option. Adonis permits 300 seconds.
	// Within that window, still verify the ORIGINAL signature against Google's
	// fixed key endpoint. Never accept the unverified parsed payload alone.
	_, err = jwt.Parse(credential, func(token *jwt.Token) (interface{}, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("missing Google key ID")
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, googleKeysURL, nil)
		if err != nil {
			return nil, err
		}
		response, err := v.client.Do(request)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Google key response: %d", response.StatusCode)
		}
		var keys struct {
			Keys []struct{ Kid, Kty, N, E string }
		}
		if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&keys); err != nil {
			return nil, err
		}
		for _, key := range keys.Keys {
			if key.Kid == kid && key.Kty == "RSA" {
				n, err := base64.RawURLEncoding.DecodeString(key.N)
				if err != nil {
					return nil, err
				}
				e, err := base64.RawURLEncoding.DecodeString(key.E)
				if err != nil || len(e) > 4 {
					return nil, errors.New("invalid Google RSA exponent")
				}
				return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}, nil
			}
		}
		return nil, errors.New("unknown Google key ID")
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithAudience(audience), jwt.WithLeeway(300*time.Second), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil {
		return nil, err
	}
	return payload, nil
}
