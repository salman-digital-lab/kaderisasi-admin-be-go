package httpapi

import (
	"encoding/json"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
)

type provinceRequest struct {
	Name string `json:"name"`
}

type cityRequest struct {
	Name       string       `json:"name"`
	ProvinceID *json.Number `json:"province_id"`
}

type universityRequest struct {
	Name       string      `json:"name"`
	ProvinceID json.Number `json:"provinceId"`
}

type universityResponse struct {
	dbgen.University
	Province *dbgen.Province `json:"province"`
}

type universityPage struct {
	Meta database.Pagination  `json:"meta"`
	Data []universityResponse `json:"data"`
}

func referenceProvince(id *int32, name *string, active *bool) *dbgen.Province {
	if id == nil {
		return nil
	}
	return &dbgen.Province{ID: *id, Name: name, IsActive: active}
}

func numberText(value *json.Number) *string {
	if value == nil {
		return nil
	}
	text := value.String()
	return &text
}
