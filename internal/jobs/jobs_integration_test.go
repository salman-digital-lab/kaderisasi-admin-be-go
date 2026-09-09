//go:build integration

package jobs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/config"
	"kaderisasi/admin/internal/database"
)

func jobPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.DBSchema, "go_rewrite_") {
		t.Fatal("owned fixture schema required")
	}
	pool, err := database.Open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestJobsDateBoundariesAndRepeatedExecution(t *testing.T) {
	pool := jobPool(t)
	ctx := context.Background()
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	for _, instant := range []string{"2028-02-29T16:59:59Z", "2028-02-29T17:00:00Z", "2026-12-31T17:00:00Z"} {
		t.Run(instant, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, instant)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range Names {
				t.Run(name, func(t *testing.T) {
					tx, err := pool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					defer func() {
						if err := tx.Rollback(ctx); err != nil {
							t.Error(err)
						}
					}()
					cutoff := Cutoff(now, location, name == "clubs:update-visibility")
					ids := make([]int32, 5)
					for i, days := range []int{-1, 0, 1, -1, 0} {
						var date interface{} = cutoff.AddDate(0, 0, days).Format("2006-01-02")
						if i == 4 {
							date = nil
						}
						if name == "close:registration" {
							err = tx.QueryRow(ctx, `INSERT INTO activities(slug,name,is_published,is_registration_open,registration_end,updated_at) VALUES($1,'Scheduled fixture',$2,true,$3,'2020-01-01') RETURNING id`, fmt.Sprintf("job-%d-%d", time.Now().UnixNano(), i), i != 3, date).Scan(&ids[i])
						} else {
							err = tx.QueryRow(ctx, `INSERT INTO clubs(name,is_show,is_registration_open,registration_end_date,end_period,updated_at) VALUES('Scheduled fixture',$1,$1,$2,$2,'2020-01-01') RETURNING id`, i != 3, date).Scan(&ids[i])
						}
						if err != nil {
							t.Fatal(err)
						}
					}
					var output bytes.Buffer
					runner := Runner{DB: tx, Location: location, Logger: slog.New(slog.NewJSONHandler(&output, nil))}
					result, err := runner.Run(ctx, name, now)
					if err != nil {
						t.Fatal(err)
					}
					if result.Cutoff != cutoff.Format("2006-01-02") {
						t.Fatal(result)
					}
					for i, id := range ids {
						found := false
						for _, changed := range result.IDs {
							if changed == id {
								found = true
							}
						}
						if found != (i == 0) {
							t.Fatalf("fixture %d changed=%v", i, found)
						}
						var active, other bool
						var updated time.Time
						if name == "close:registration" {
							err = tx.QueryRow(ctx, "SELECT is_published,is_registration_open,updated_at FROM activities WHERE id=$1", id).Scan(&active, &other, &updated)
						} else if name == "clubs:close-registration" {
							err = tx.QueryRow(ctx, "SELECT is_registration_open,is_show,updated_at FROM clubs WHERE id=$1", id).Scan(&active, &other, &updated)
						} else {
							err = tx.QueryRow(ctx, "SELECT is_show,is_registration_open,updated_at FROM clubs WHERE id=$1", id).Scan(&active, &other, &updated)
						}
						if err != nil {
							t.Fatal(err)
						}
						if active != (i != 0 && i != 3) || other != (name == "close:registration" || i != 3) || updated.Year() != 2020 {
							t.Fatalf("unrelated values changed: %d %v %v %v", i, active, other, updated)
						}
					}
					repeated, err := runner.Run(ctx, name, now)
					if err != nil || repeated.Count != 0 || repeated.IDs == nil {
						t.Fatal("not idempotent", repeated, err)
					}
					if !strings.Contains(output.String(), "job completed") {
						t.Fatal("completion was not logged")
					}
				})
			}
		})
	}
}

func TestJobFailuresAreReturnedAndLogged(t *testing.T) {
	pool := jobPool(t)
	ctx := context.Background()
	for _, name := range Names {
		t.Run(name, func(t *testing.T) {
			tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := tx.Rollback(ctx); err != nil {
					t.Error(err)
				}
			}()
			var output bytes.Buffer
			runner := Runner{DB: tx, Logger: slog.New(slog.NewJSONHandler(&output, nil))}
			result, err := runner.Run(ctx, name, time.Now())
			if err == nil || result.Count != 0 || !strings.Contains(output.String(), "job failed") || !strings.Contains(err.Error(), name) {
				t.Fatal(result, err, output.String())
			}
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			_, err = (Runner{DB: pool}).Run(cancelled, name, time.Now())
			if !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation was lost", err)
			}
		})
	}
	if _, err := (Runner{DB: pool}).Run(ctx, "unknown", time.Now()); err == nil {
		t.Fatal("unknown job accepted")
	}
}
