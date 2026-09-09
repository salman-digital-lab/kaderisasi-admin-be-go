package httpapi

import "net/http"

func policyDenied(w http.ResponseWriter) {
	write(w, 403, struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}{Errors: []struct {
		Message string `json:"message"`
	}{{Message: "Access denied"}}})
}
