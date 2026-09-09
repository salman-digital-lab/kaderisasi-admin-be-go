//go:build integration

package main

import (
	"errors"
	"kaderisasi/admin/internal/config"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"
)

// This transport is excluded from production builds. It changes only the key
// response location; the unmodified idtoken validator still verifies signatures.
type fixtureKeyTransport struct{ endpoint *url.URL }

func (t fixtureKeyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.String() != "https://www.googleapis.com/oauth2/v3/certs" {
		return nil, errors.New("unexpected Google fixture key request")
	}
	request = request.Clone(request.Context())
	request.URL = t.endpoint
	return http.DefaultTransport.RoundTrip(request)
}

func googleHTTPClient(c config.Config) (*http.Client, error) {
	address := os.Getenv("GO_REWRITE_GOOGLE_KEYS_URL")
	if address == "" {
		return nil, nil
	}
	endpoint, err := url.Parse(address)
	if err != nil || endpoint.Scheme != "http" || endpoint.Hostname() != "127.0.0.1" || endpoint.User != nil || c.Environment != "test" || !regexp.MustCompile(`^go_rewrite_[a-f0-9]{16}_(baseline|candidate|cross)$`).MatchString(c.DBSchema) {
		return nil, errors.New("Google fixtures require a loopback key server and isolated test schema")
	}
	return &http.Client{Transport: fixtureKeyTransport{endpoint}, Timeout: 5 * time.Second}, nil
}
