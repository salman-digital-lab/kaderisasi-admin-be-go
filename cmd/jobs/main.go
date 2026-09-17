// Command jobs invokes one scheduled business job. The API never invokes it.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/config"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/jobs"
	"kaderisasi/admin/internal/storage"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: admin-jobs <%s>", strings.Join(jobs.Names, "|"))
	}
	name := os.Args[1]
	known := false
	for _, candidate := range jobs.Names {
		if name == candidate {
			known = true
		}
	}
	if !known {
		return fmt.Errorf("unknown job %q", name)
	}
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
	logger, err := config.NewLogger(os.Stderr, c.LogLevel)
	if err != nil {
		return err
	}
	slog.SetDefault(logger)
	result, err := (jobs.Runner{DB: pool, Location: c.Location, Logger: logger, Storage: storage.NewCourseDocuments(c)}).Run(ctx, name, time.Now())
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
func main() {
	if err := run(); err != nil {
		slog.Error("scheduled job stopped", "error", err)
		os.Exit(1)
	}
}
