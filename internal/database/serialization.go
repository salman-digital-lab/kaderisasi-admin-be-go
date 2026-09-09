package database

import "time"

// CreatedModel reflects Lucid's in-memory create result. Database defaults are
// persisted but are absent from the response until the model is fetched again.
func CreatedModel(row, assigned Object) Object {
	for key := range row {
		if !assigned.Has(key) && key != "id" && key != "created_at" && key != "updated_at" {
			delete(row, key)
		}
	}
	return row
}

// Timestamps serializes known timestamp columns using the caller's Adonis
// source: Lucid uses its local timezone, raw pg query results become UTC Dates.
// It intentionally does not traverse arbitrary form answers or JSON settings.
func Timestamps(row Object, location *time.Location, columns ...string) Object {
	if location == nil {
		location = time.Local
	}
	for _, column := range columns {
		if !row.Has(column) || row.Null(column) {
			continue
		}
		if value, err := time.Parse(time.RFC3339Nano, row.String(column)); err == nil {
			row.Set(column, value.In(location).Format("2006-01-02T15:04:05.000-07:00"))
		}
	}
	return row
}

func UTCTimestamps(row Object, columns ...string) Object {
	for _, column := range columns {
		if !row.Has(column) || row.Null(column) {
			continue
		}
		if value, err := time.Parse(time.RFC3339Nano, row.String(column)); err == nil {
			row.Set(column, value.UTC().Format("2006-01-02T15:04:05.000Z"))
		}
	}
	return row
}
