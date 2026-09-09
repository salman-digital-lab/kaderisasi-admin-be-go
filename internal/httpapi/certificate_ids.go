package httpapi

import (
	"encoding/json"
	"errors"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
	"math"
	"strconv"
)

// Vine accepts JavaScript integers wider than PostgreSQL's integer columns.
// Preserve their error outcome instead of narrowing them into another row ID.
func issuedPathID(raw, invalid string) (int32, error) {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || !positiveIDPattern.MatchString(raw) || value > 9007199254740991 {
		return 0, domain.Fail(400, invalid)
	}
	if value > math.MaxInt32 {
		return 0, errors.New("integer out of range")
	}
	return int32(value), nil
}

func certificateIDsFit(data database.Object, arrays bool) bool {
	for _, field := range []string{"activity_id", "registration_id"} {
		if data.Has(field) && data.Number(field) > math.MaxInt32 {
			return false
		}
	}
	if arrays && data.Has("registration_ids") {
		var ids []int64
		if json.Unmarshal(data["registration_ids"], &ids) != nil {
			return false
		}
		for _, id := range ids {
			if id > math.MaxInt32 {
				return false
			}
		}
	}
	return true
}
