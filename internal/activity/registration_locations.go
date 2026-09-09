package activity

import (
	"context"
	"encoding/json"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/export"
	"math"
	"strconv"
	"strings"
)

func guestLocationNumber(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false
	}
	value := string(raw)
	if raw[0] == '"' {
		if json.Unmarshal(raw, &value) != nil {
			return 0, false
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return 0, false
		}
	}
	if value == "true" || value == "false" {
		return 0, false
	}
	var number float64
	var err error
	if len(value) > 2 && value[0] == '0' && strings.ContainsAny(value[1:2], "xXoObB") {
		var integer uint64
		integer, err = strconv.ParseUint(value, 0, 64)
		number = float64(integer)
	} else {
		number, err = strconv.ParseFloat(value, 64)
	}
	return number, (err == nil || math.IsInf(number, 0)) && !math.IsNaN(number) && number != 0
}

type locationMap map[float64]string

func (locations locationMap) fallback(current *string, raw json.RawMessage) *string {
	if current != nil && *current != "" {
		return current
	}
	if id, ok := guestLocationNumber(raw); ok {
		if value, exists := locations[id]; exists {
			return &value
		}
	}
	return current
}
func registrationExportLocations(ctx context.Context, q *dbgen.Queries, records []export.Registration) error {
	provinceIDs, cityIDs, universityIDs := []string{}, []string{}, []string{}
	add := func(ids *[]string, raw json.RawMessage) {
		if value, ok := guestLocationNumber(raw); ok {
			text := database.JSNumber(value)
			for _, id := range *ids {
				if id == text {
					return
				}
			}
			*ids = append(*ids, text)
		}
	}
	for _, record := range records {
		if record.UserID == nil {
			add(&provinceIDs, record.Guest.ProvinceID)
			add(&provinceIDs, record.Guest.OriginProvinceID)
			add(&cityIDs, record.Guest.CityID)
			add(&cityIDs, record.Guest.OriginCityID)
			add(&universityIDs, record.Guest.UniversityID)
		}
	}
	provinces, cities, universities := locationMap{}, locationMap{}, locationMap{}
	if len(provinceIDs) > 0 {
		rows, err := q.RegistrationExportProvinces(ctx, provinceIDs)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.Name != nil {
				provinces[float64(row.ID)] = *row.Name
			}
		}
	}
	if len(cityIDs) > 0 {
		rows, err := q.RegistrationExportCities(ctx, cityIDs)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.Name != nil {
				cities[float64(row.ID)] = *row.Name
			}
		}
	}
	if len(universityIDs) > 0 {
		rows, err := q.RegistrationExportUniversities(ctx, universityIDs)
		if err != nil {
			return err
		}
		for _, row := range rows {
			universities[float64(row.ID)] = row.Name
		}
	}
	for i, record := range records {
		locations := &records[i].Locations
		locations.Province = provinces.fallback(locations.Province, record.Guest.ProvinceID)
		locations.City = cities.fallback(locations.City, record.Guest.CityID)
		locations.OriginProvince = provinces.fallback(locations.OriginProvince, record.Guest.OriginProvinceID)
		locations.OriginCity = cities.fallback(locations.OriginCity, record.Guest.OriginCityID)
		locations.University = universities.fallback(locations.University, record.Guest.UniversityID)
	}
	return nil
}
