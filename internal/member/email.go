package member

import (
	"errors"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5/pgconn"
)

// NormalizeEmail stores member emails the way the web API compares them on
// login, register and Google sign-in: trimmed (JavaScript String#trim) and
// lowercased. An email saved with capitals could otherwise never log in.
func NormalizeEmail(value *string) *string {
	if value == nil {
		return nil
	}
	normalized := strings.ToLower(strings.TrimFunc(*value, func(r rune) bool {
		return unicode.IsSpace(r) || r == '\uFEFF'
	}))
	return &normalized
}

// emailTaken reports a write rejected because another account already has
// the email, exactly or ignoring case.
func emailTaken(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" &&
		(pgErr.ConstraintName == "public_users_email_unique" || pgErr.ConstraintName == "public_users_lower_email_unique")
}
