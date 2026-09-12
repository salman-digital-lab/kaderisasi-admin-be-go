package form

import "kaderisasi/admin/internal/formschema"

func ValidSchema(raw []byte) bool { return formschema.ValidSchema(raw) }
