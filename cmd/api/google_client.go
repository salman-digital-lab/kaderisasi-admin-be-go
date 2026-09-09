//go:build !integration

package main

import (
	"kaderisasi/admin/internal/config"
	"net/http"
)

// Production builds always use Google's fixed key endpoints.
func googleHTTPClient(config.Config) (*http.Client, error) { return nil, nil }
