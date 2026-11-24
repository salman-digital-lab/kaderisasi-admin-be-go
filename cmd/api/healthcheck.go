package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

// healthcheck probes the local /health endpoint so the runtime image needs no curl.
func healthcheck() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3334"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/health", nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health returned %d", response.StatusCode)
	}
	return nil
}
