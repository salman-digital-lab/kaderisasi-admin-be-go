package database

import (
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
)

// LegacyQueryError preserves an exposed Adonis SQL diagnostic independently of
// the generated query used for execution. Parameter values are never included.
func LegacyQueryError(err error, statement string) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return fmt.Errorf("%s - %s", statement, pg.Message)
	}
	return err
}
