package database

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// LegacyQueryError preserves an exposed Adonis SQL diagnostic independently of
// the generated query used for execution. Parameter values are never included.
func LegacyQueryError(err error, statement string) error {
	if pg, ok := errors.AsType[*pgconn.PgError](err); ok {
		return fmt.Errorf("%s - %s", statement, pg.Message)
	}
	return err
}
