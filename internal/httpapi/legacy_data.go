package httpapi

import (
	"encoding/json"
	"kaderisasi/admin/internal/database"
)

func nestedObject(data database.Object, key string) database.Object {
	out := database.Object{}
	_ = json.Unmarshal(data[key], &out)
	if out == nil {
		out = database.Object{}
	}
	return out
}
