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
