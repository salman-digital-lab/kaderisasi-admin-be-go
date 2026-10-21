package main

import (
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
	client := http.Client{Timeout: 4 * time.Second}
	response, err := client.Get("http://127.0.0.1:" + port + "/health")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health returned %d", response.StatusCode)
	}
	return nil
}
