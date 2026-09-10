package httpapi

import (
	"kaderisasi/admin/internal/domain"
	"strconv"
)

func issuedPathID(raw, invalid string) (float64, error) {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || !positiveIDPattern.MatchString(raw) || value > 9007199254740991 {
		return 0, domain.Fail(400, invalid)
	}
	return float64(value), nil
}
