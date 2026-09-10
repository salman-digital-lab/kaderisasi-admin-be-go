package main

import (
	"context"
	"errors"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/config"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/httpapi"
	"kaderisasi/admin/internal/storage"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func run() error {
	c, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := database.Open(ctx, c)
	if err != nil {
		return err
	}
	defer pool.Close()
	googleClient, err := googleHTTPClient(c)
	if err != nil {
		return err
	}
	google, err := auth.NewGoogleValidator(ctx, googleClient)
	if err != nil {
		return err
	}
	logger, err := config.NewLogger(os.Stdout, c.LogLevel)
	if err != nil {
		return err
	}
	slog.SetDefault(logger)
	objects := storage.New(c)
	if path := os.Getenv("GO_REWRITE_STORAGE_LEDGER"); path != "" {
		if !storageFixtureAllowed(c) {
			return errors.New("storage instrumentation requires an isolated test schema")
		}
		objects.Created, err = storage.Journal(path)
		if err != nil {
			return err
		}
	}
	app := &httpapi.Server{Config: c, Pool: pool, Auth: &auth.Service{Pool: pool, Key: c.AppKey, GoogleClientID: c.GoogleClientID, Google: google}, Storage: objects, Logger: logger}
	app.CourseStorage = storage.NewCourseDocuments(c)
	if path := os.Getenv("GO_COURSE_STORAGE_LEDGER"); path != "" {
		if !storageFixtureAllowed(c) {
			return errors.New("course storage instrumentation requires an isolated test schema")
		}
		store, ok := app.CourseStorage.(*storage.S3)
		if !ok {
			return errors.New("course storage instrumentation requires a private bucket")
		}
		store.Created, err = storage.Journal(path)
		if err != nil {
			return err
		}
	}
	server := &http.Server{Addr: c.Address(), Handler: app.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second}
	failure := make(chan error, 1)
	go func() { logger.Info("admin API listening", "address", c.Address()); failure <- server.ListenAndServe() }()
	select {
	case err := <-failure:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		slog.Error("API stopped", "error", err.Error())
		os.Exit(1)
	}
}
