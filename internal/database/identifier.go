package database

import "kaderisasi/admin/internal/jscompat"

// NumberIdentifier preserves Number(params.id), including non-finite values,
// without narrowing to a machine integer before PostgreSQL validates the value.
func NumberIdentifier(raw string) string { return JSNumber(jscompat.Number(raw)) }
