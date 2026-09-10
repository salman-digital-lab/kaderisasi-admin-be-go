package httpapi

import (
	"context"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"net/http"
	"strings"
)

func referenceHandler[T any](text string, pluralEnvelope bool, fetch func(*http.Request) (T, error)) Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		value, err := fetch(r)
		if err != nil {
			legacyFailure(w, paginationError(r, referencePathError(r, err)))
		} else if pluralEnvelope {
			plural(w, text, value)
		} else {
			reply(w, 200, text, value)
		}
		return nil
	}
}

func referenceMutation[Request, Response any](schema, text string, mutate func(*http.Request, Request) (Response, error)) Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		request, ok := inputAs[Request](w, r, schema)
		if !ok {
			return nil
		}
		value, err := mutate(r, request)
		if err != nil {
			legacyFailure(w, paginationError(r, referencePathError(r, err)))
		} else {
			reply(w, 200, text, value)
		}
		return nil
	}
}

func (s *Server) registerReferenceCRUD() {
	q := dbgen.New(s.Pool)
	s.register("provinces_controller", "index", referenceHandler("GET_DATA_SUCCESS", true, func(r *http.Request) ([]dbgen.Province, error) {
		return q.ListProvinces(r.Context())
	}))
	s.register("provinces_controller", "show", referenceHandler("GET_DATA_SUCCESS", false, func(r *http.Request) (dbgen.Province, error) {
		return q.ProvinceByIdentifier(r.Context(), pathID(r, "id"))
	}))
	s.register("provinces_controller", "store", referenceMutation("provinceValidator", "CREATE_DATA_SUCCESS", func(r *http.Request, request provinceRequest) (dbgen.CreateProvinceRow, error) {
		return q.CreateProvince(r.Context(), &request.Name)
	}))
	s.register("provinces_controller", "update", referenceMutation("provinceValidator", "UPDATE_DATA_SUCCESS", func(r *http.Request, request provinceRequest) (dbgen.Province, error) {
		row, err := q.ProvinceByIdentifier(r.Context(), pathID(r, "id"))
		if err != nil {
			return dbgen.Province{}, database.LegacyQueryError(err, `select * from "provinces" where "id" = $1 limit $2`)
		}
		return q.UpdateProvince(r.Context(), dbgen.UpdateProvinceParams{ID: row.ID, Name: &request.Name})
	}))
	s.register("cities_controller", "index", referenceHandler("GET_DATA_SUCCESS", true, func(r *http.Request) ([]dbgen.City, error) {
		return q.ListCities(r.Context())
	}))
	s.register("cities_controller", "getByProvinceId", referenceHandler("GET_DATA_SUCCESS", true, func(r *http.Request) ([]dbgen.City, error) {
		return q.CitiesByProvinceIdentifier(r.Context(), pathID(r, "id"))
	}))
	s.register("cities_controller", "show", referenceHandler("GET_DATA_SUCCESS", false, func(r *http.Request) (dbgen.City, error) {
		return q.CityByIdentifier(r.Context(), pathID(r, "id"))
	}))
	s.register("cities_controller", "store", referenceMutation("cityValidator", "CREATE_DATA_SUCCESS", func(r *http.Request, request cityRequest) (dbgen.CreateCityRow, error) {
		row, err := q.CreateCity(r.Context(), dbgen.CreateCityParams{Name: &request.Name, ProvinceID: numberText(request.ProvinceID)})
		return row, database.LegacyQueryError(err, `insert into "cities" ("name", "province_id") values ($1, $2) returning "id"`)
	}))
	s.register("cities_controller", "update", referenceMutation("cityValidator", "UPDATE_DATA_SUCCESS", func(r *http.Request, request cityRequest) (dbgen.City, error) {
		row, err := q.CityByIdentifier(r.Context(), pathID(r, "id"))
		if err != nil {
			return dbgen.City{}, database.LegacyQueryError(err, `select * from "cities" where "id" = $1 limit $2`)
		}
		return q.UpdateCity(r.Context(), dbgen.UpdateCityParams{ID: row.ID, Name: &request.Name, ProvincePresent: request.ProvinceID != nil, ProvinceID: numberText(request.ProvinceID)})
	}))
	s.register("universities_controller", "index", referenceHandler("GET_DATA_SUCCESS", true, func(r *http.Request) (universityPage, error) {
		page, size := pageParams(r, 10, 0)
		search := "%" + r.URL.Query().Get("search") + "%"
		total, err := q.CountUniversities(r.Context(), search)
		if err != nil {
			return universityPage{}, err
		}
		rows := []dbgen.ListUniversitiesRow{}
		if total > 0 {
			limit, offset, pageErr := database.SQLPage(page, size)
			if pageErr != nil {
				return universityPage{}, pageErr
			}
			rows, err = q.ListUniversities(r.Context(), dbgen.ListUniversitiesParams{Search: search, PageSize: limit, PageOffset: offset})
			if err != nil {
				return universityPage{}, err
			}
		}
		data := make([]universityResponse, len(rows))
		for i, row := range rows {
			data[i] = universityResponse{University: row.University, Province: referenceProvince(row.ParentID, row.ParentName, row.ParentActive)}
		}
		return universityPage{Meta: database.Meta(total, page, size), Data: data}, nil
	}))
	s.register("universities_controller", "show", referenceHandler("GET_DATA_SUCCESS", false, func(r *http.Request) (universityResponse, error) {
		row, err := q.UniversityByIdentifier(r.Context(), pathID(r, "id"))
		return universityResponse{University: row.University, Province: referenceProvince(row.ParentID, row.ParentName, row.ParentActive)}, err
	}))
	s.register("universities_controller", "store", referenceMutation("UniversityValidator", "CREATE_DATA_SUCCESS", func(r *http.Request, request universityRequest) (dbgen.CreateUniversityRow, error) {
		row, err := q.CreateUniversity(r.Context(), dbgen.CreateUniversityParams{Name: request.Name, ProvinceID: request.ProvinceID.String()})
		return row, database.LegacyQueryError(err, `insert into "universities" ("name", "province_id") values ($1, $2) returning "id"`)
	}))
	s.register("universities_controller", "update", referenceMutation("UniversityValidator", "UPDATE_DATA_SUCCESS", func(r *http.Request, request universityRequest) (dbgen.University, error) {
		row, err := q.UniversityByIdentifier(r.Context(), pathID(r, "id"))
		if err != nil {
			return dbgen.University{}, database.LegacyQueryError(err, `select * from "universities" where "id" = $1 limit $2`)
		}
		return q.UpdateUniversity(r.Context(), dbgen.UpdateUniversityParams{ID: row.University.ID, Name: request.Name, ProvinceID: request.ProvinceID.String()})
	}))
	for _, resource := range []struct {
		controller, missing string
		remove              func(context.Context, string) (int64, error)
	}{
		{"provinces_controller", "PROVINCE_NOT_FOUND", q.DeleteProvinceByIdentifier},
		{"cities_controller", "CITY_NOT_FOUND", q.DeleteCityByIdentifier},
		{"universities_controller", "UNIVERSITY_NOT_FOUND", q.DeleteUniversityByIdentifier},
	} {
		s.register(resource.controller, "delete", func(w http.ResponseWriter, r *http.Request) error {
			count, err := resource.remove(r.Context(), pathID(r, "id"))
			if err != nil {
				legacyFailure(w, paginationError(r, referencePathError(r, err)))
			} else if count == 0 {
				message(w, 200, resource.missing)
			} else {
				message(w, 200, "DELETE_DATA_SUCCESS")
			}
			return nil
		})
	}
}

func referencePathError(r *http.Request, err error) error {
	if r.PathValue("id") == "" {
		return err
	}
	table := strings.Split(strings.TrimPrefix(r.URL.Path, "/v2/"), "/")[0]
	if strings.HasSuffix(r.URL.Path, "/cities") {
		return database.LegacyQueryError(err, `select * from "cities" where "province_id" = $1`)
	}
	switch table {
	case "provinces", "cities", "universities":
		return database.LegacyQueryError(err, `select * from "`+table+`" where "id" = $1 limit $2`)
	}
	return err
}
