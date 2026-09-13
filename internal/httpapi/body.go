package httpapi

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"io"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/validation"
	"mime"
	"net/http"
	"strings"
)

type requestDataKey struct{}

func requestData(r *http.Request) validation.Object {
	data, _ := r.Context().Value(requestDataKey{}).(validation.Object)
	if data == nil {
		return validation.Object{}
	}
	return data
}

// Adonis runs its body parser after route matching, before named authentication
// and origin middleware. Query fields override body fields in request.all().
func prepareRequest(w http.ResponseWriter, r *http.Request, route Route) (*http.Request, error) {
	if route.Controller == "short_links_controller" {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, 65536)
		}
		return r, nil
	}
	body := map[string]any{}
	kind, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	allowed := r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" || r.Method == "DELETE"
	if allowed && r.Body != nil && r.ContentLength != 0 {
		jsonType := kind == "application/json" || kind == "application/json-patch+json" || kind == "application/vnd.api+json" || kind == "application/csp-report"
		formType := kind == "application/x-www-form-urlencoded"
		if jsonType || formType || strings.HasPrefix(kind, "text/") {
			raw, err := readRequestBody(w, r, 1<<20)
			if err != nil {
				return r, err
			}
			if jsonType && len(raw) > 0 {
				trimmed := bytes.TrimLeft(raw, " \t\r\n")
				if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
					return r, domain.Fail(422, "Invalid JSON, only supports object and array")
				}
				var value any
				decoder := json.NewDecoder(bytes.NewReader(raw))
				decoder.UseNumber()
				// Unmarshal first to reject trailing data as well as malformed input.
				var check json.RawMessage
				if err = json.Unmarshal(raw, &check); err != nil {
					return r, domain.Fail(400, jsonDiagnostic(raw, err))
				}
				if err = decoder.Decode(&value); err != nil {
					return r, err
				}
				if object, ok := cleanJSON(value).(map[string]any); ok {
					body = object
				}
			} else if formType {
				body = parseFields(string(raw), true)
			}
		} else if kind == "multipart/form-data" && isUploadRoute(route) {
			limit := int64(6 << 20)
			if route.Controller == "courses_controller" {
				limit = 21 << 20
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			if err := r.ParseMultipartForm(6 << 20); err != nil {
				var large *http.MaxBytesError
				if errors.As(err, &large) {
					return r, domain.Fail(413, "request entity too large")
				}
				return r, domain.Fail(400, "Invalid multipart request")
			}
			for key, values := range r.MultipartForm.Value {
				if len(values) == 1 {
					body[key] = values[0]
				} else {
					body[key] = values
				}
			}
		}
	}
	for key, value := range parseFields(r.URL.RawQuery, false) {
		body[key] = value
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return r, err
	}
	var data validation.Object
	if err = json.Unmarshal(encoded, &data); err != nil {
		return r, err
	}
	return r.WithContext(context.WithValue(r.Context(), requestDataKey{}, data)), nil
}
func isUploadRoute(route Route) bool {
	return route.Controller == "courses_controller" && route.Action == "uploadDocument" ||
		route.Controller == "activities_controller" && route.Action == "uploadImage" ||
		route.Controller == "clubs_controller" && (route.Action == "uploadLogo" || route.Action == "uploadImageMedia") ||
		route.Controller == "certificate_templates_controller" && (route.Action == "uploadBackground" || route.Action == "uploadAsset")
}
func readRequestBody(w http.ResponseWriter, r *http.Request, limit int64) ([]byte, error) {
	var reader io.Reader = http.MaxBytesReader(w, r.Body, limit)
	encoding := r.Header.Get("Content-Encoding")
	if encoding != "" && encoding != "identity" {
		switch encoding {
		case "gzip", "deflate":
			// inflation accepts either zlib or gzip framing for both encoding names.
			compressed, err := io.ReadAll(reader)
			if err != nil {
				return nil, domain.Fail(413, "request entity too large")
			}
			var stream io.ReadCloser
			if len(compressed) > 1 && compressed[0] == 0x1f && compressed[1] == 0x8b {
				stream, err = gzip.NewReader(bytes.NewReader(compressed))
			} else {
				stream, err = zlib.NewReader(bytes.NewReader(compressed))
			}
			if err != nil {
				return nil, domain.Fail(400, "incorrect header check")
			}
			defer stream.Close()
			raw, err := io.ReadAll(io.LimitReader(stream, limit+1))
			if err != nil {
				return nil, domain.Fail(400, "incorrect header check")
			}
			if int64(len(raw)) > limit {
				return nil, domain.Fail(413, "request entity too large")
			}
			// The installed raw-body config retains length=0 for compressed bodies.
			if len(raw) > 0 {
				return nil, domain.Fail(400, "request size did not match content length")
			}
			return raw, nil
		default:
			return nil, domain.Fail(415, "Unsupported Content-Encoding: "+encoding)
		}
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			return nil, domain.Fail(413, "request entity too large")
		}
		return nil, err
	}
	return raw, nil
}
func cleanJSON(value any) any {
	switch value := value.(type) {
	case string:
		if value == "" {
			return nil
		}
		return value
	case []any:
		for i, entry := range value {
			value[i] = cleanJSON(entry)
		}
		return value
	case map[string]any:
		delete(value, "__proto__")
		if constructor, ok := value["constructor"].(map[string]any); ok {
			if _, exists := constructor["prototype"]; exists {
				delete(value, "constructor")
			}
		}
		for key, entry := range value {
			value[key] = cleanJSON(entry)
		}
		return value
	default:
		return value
	}
}
