package club

import "kaderisasi/admin/internal/database"

func updateIdentifier(raw string) string { return database.NumberIdentifier(raw) }
