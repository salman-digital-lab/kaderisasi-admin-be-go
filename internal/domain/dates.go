package domain

import (
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

func Timestamp(value pgtype.Timestamptz, location *time.Location) *string {
	if !value.Valid {
		return nil
	}
	text := value.Time.In(location).Format("2006-01-02T15:04:05.000Z07:00")
	return &text
}

// Lucid's local DateTime retains an explicit offset even on a UTC host. Raw
// PostgreSQL/JavaScript Date projections use Timestamp and retain the Z suffix.
func ModelTimestamp(value pgtype.Timestamptz, location *time.Location) *string {
	if !value.Valid {
		return nil
	}
	text := value.Time.In(location).Format("2006-01-02T15:04:05.000-07:00")
	return &text
}
