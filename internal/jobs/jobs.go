package jobs

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/storage"
	"log/slog"
	"time"
)

var Names = []string{"close:registration", "clubs:close-registration", "clubs:update-visibility", "forms:clean-uploads"}

type Runner struct {
	Storage  storage.Store
	DB       dbgen.DBTX
	Location *time.Location
	Logger   *slog.Logger
}
type Result struct {
	Job    string  `json:"job"`
	Cutoff string  `json:"cutoff"`
	Count  int     `json:"count"`
	IDs    []int32 `json:"ids"`
}

func Cutoff(now time.Time, location *time.Location, month bool) time.Time {
	if location == nil {
		location = time.Local
	}
	now = now.In(location)
	day := now.Day()
	if month {
		day = 1
	}
	return time.Date(now.Year(), now.Month(), day, 0, 0, 0, 0, location)
}
func (r Runner) Run(ctx context.Context, name string, now time.Time) (Result, error) {
	location := r.Location
	if name == "clubs:close-registration" {
		location = domain.Jakarta()
	}
	cutoff := Cutoff(now, location, name == "clubs:update-visibility")
	date := pgtype.Date{Time: cutoff, Valid: true}
	result := Result{Job: name, Cutoff: cutoff.Format("2006-01-02"), IDs: []int32{}}
	q := dbgen.New(r.DB)
	switch name {
	case "forms:clean-uploads":
		if r.Storage == nil {
			return result, fmt.Errorf("form upload storage unavailable")
		}
		for {
			files, err := q.ExpiredFormAttachments(ctx)
			if err != nil {
				return result, r.failure(name, err)
			}
			if len(files) == 0 {
				return result, nil
			}
			for _, file := range files {
				if err := r.Storage.Delete(ctx, file.StorageKey); err != nil {
					return result, r.failure(name, err)
				}
				if err := q.DeleteExpiredFormAttachment(ctx, file.ID); err != nil {
					return result, r.failure(name, err)
				}
				result.Count++
			}
		}
	case "close:registration":
		rows, err := q.CloseActivityRegistration(ctx, date)
		if err != nil {
			return result, r.failure(name, err)
		}
		for _, row := range rows {
			result.IDs = append(result.IDs, row.ID)
		}
	case "clubs:close-registration":
		rows, err := q.CloseClubRegistration(ctx, date)
		if err != nil {
			return result, r.failure(name, err)
		}
		for _, row := range rows {
			result.IDs = append(result.IDs, row.ID)
		}
	case "clubs:update-visibility":
		rows, err := q.UpdateClubVisibility(ctx, date)
		if err != nil {
			return result, r.failure(name, err)
		}
		for _, row := range rows {
			result.IDs = append(result.IDs, row.ID)
		}
	default:
		return result, fmt.Errorf("unknown job %q", name)
	}
	result.Count = len(result.IDs)
	if r.Logger != nil {
		r.Logger.Info("job completed", "job", name, "cutoff", result.Cutoff, "count", result.Count, "ids", result.IDs)
	}
	return result, nil
}
func (r Runner) failure(name string, err error) error {
	if r.Logger != nil {
		r.Logger.Error("job failed", "job", name, "error", err)
	}
	return fmt.Errorf("%s: %w", name, err)
}
