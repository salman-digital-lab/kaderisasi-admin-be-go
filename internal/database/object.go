package database

import "kaderisasi/admin/internal/validation"

type Object = validation.Object

type Pagination struct {
	Total           int64      `json:"total"`
	PerPage         PageNumber `json:"per_page"`
	CurrentPage     PageNumber `json:"current_page"`
	LastPage        PageNumber `json:"last_page"`
	FirstPage       int        `json:"first_page"`
	FirstPageURL    string     `json:"first_page_url"`
	LastPageURL     string     `json:"last_page_url"`
	NextPageURL     *string    `json:"next_page_url"`
	PreviousPageURL *string    `json:"previous_page_url"`
}
type RawPagination struct {
	Total           int64      `json:"total"`
	PerPage         PageNumber `json:"perPage"`
	CurrentPage     PageNumber `json:"currentPage"`
	LastPage        PageNumber `json:"lastPage"`
	FirstPage       int        `json:"firstPage"`
	FirstPageURL    string     `json:"firstPageUrl"`
	LastPageURL     string     `json:"lastPageUrl"`
	NextPageURL     *string    `json:"nextPageUrl"`
	PreviousPageURL *string    `json:"previousPageUrl"`
}

func (m Pagination) Raw() RawPagination {
	return RawPagination(m)
}
