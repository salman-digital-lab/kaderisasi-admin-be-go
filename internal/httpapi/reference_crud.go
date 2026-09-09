package httpapi

import (
	"context"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"net/http"
)

func referenceHandler[T any](text string, pluralEnvelope bool, fetch func(*http.Request) (T, error)) Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		value, err := fetch(r)
		if err != nil {
			legacyFailure(w, paginationError(r, err))
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
			legacyFailure(w, paginationError(r, err))
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
		return q.ProvinceByID(r.Context(), pathID(r, "id"))
	}))
	s.register("provinces_controller", "store", referenceMutation("provinceValidator", "CREATE_DATA_SUCCESS", func(r *http.Request, request provinceRequest) (dbgen.CreateProvinceRow, error) {
		return q.CreateProvince(r.Context(), &request.Name)
	}))
	s.register("provinces_controller", "update", referenceMutation("provinceValidator", "UPDATE_DATA_SUCCESS", func(r *http.Request, request provinceRequest) (dbgen.Province, error) {
		return q.UpdateProvince(r.Context(), dbgen.UpdateProvinceParams{ID: pathID(r, "id"), Name: &request.Name})
	}))
	s.register("cities_controller", "index", referenceHandler("GET_DATA_SUCCESS", true, func(r *http.Request) ([]dbgen.City, error) {
		return q.ListCities(r.Context())
	}))
	s.register("cities_controller", "getByProvinceId", referenceHandler("GET_DATA_SUCCESS", true, func(r *http.Request) ([]dbgen.City, error) {
		return q.CitiesByProvince(r.Context(), pathID(r, "id"))
	}))
	s.register("cities_controller", "show", referenceHandler("GET_DATA_SUCCESS", false, func(r *http.Request) (dbgen.City, error) {
		return q.CityByID(r.Context(), pathID(r, "id"))
	}))
	s.register("cities_controller", "store", referenceMutation("cityValidator", "CREATE_DATA_SUCCESS", func(r *http.Request, request cityRequest) (dbgen.CreateCityRow, error) {
		return q.CreateCity(r.Context(), dbgen.CreateCityParams{Name: &request.Name, ProvinceID: numberText(request.ProvinceID)})
	}))
	s.register("cities_controller", "update", referenceMutation("cityValidator", "UPDATE_DATA_SUCCESS", func(r *http.Request, request cityRequest) (dbgen.City, error) {
		return q.UpdateCity(r.Context(), dbgen.UpdateCityParams{ID: pathID(r, "id"), Name: &request.Name, ProvincePresent: request.ProvinceID != nil, ProvinceID: numberText(request.ProvinceID)})
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
		row, err := q.UniversityByID(r.Context(), pathID(r, "id"))
		return universityResponse{University: row.University, Province: referenceProvince(row.ParentID, row.ParentName, row.ParentActive)}, err
	}))
	s.register("universities_controller", "store", referenceMutation("UniversityValidator", "CREATE_DATA_SUCCESS", func(r *http.Request, request universityRequest) (dbgen.CreateUniversityRow, error) {
		return q.CreateUniversity(r.Context(), dbgen.CreateUniversityParams{Name: request.Name, ProvinceID: request.ProvinceID.String()})
	}))
	s.register("universities_controller", "update", referenceMutation("UniversityValidator", "UPDATE_DATA_SUCCESS", func(r *http.Request, request universityRequest) (dbgen.University, error) {
		return q.UpdateUniversity(r.Context(), dbgen.UpdateUniversityParams{ID: pathID(r, "id"), Name: request.Name, ProvinceID: request.ProvinceID.String()})
	}))
	for _, resource := range []struct {
		controller, missing string
		remove              func(context.Context, int32) (int64, error)
	}{
		{"provinces_controller", "PROVINCE_NOT_FOUND", q.DeleteProvince},
		{"cities_controller", "CITY_NOT_FOUND", q.DeleteCity},
		{"universities_controller", "UNIVERSITY_NOT_FOUND", q.DeleteUniversity},
	} {
		s.register(resource.controller, "delete", func(w http.ResponseWriter, r *http.Request) error {
			count, err := resource.remove(r.Context(), pathID(r, "id"))
			if err != nil {
				legacyFailure(w, paginationError(r, err))
			} else if count == 0 {
				message(w, 200, resource.missing)
			} else {
				message(w, 200, "DELETE_DATA_SUCCESS")
			}
			return nil
		})
	}
}
