package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"kaderisasi/admin/internal/activity"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/validation"
	"net/http"
	"strconv"
	"strings"
)

func readImage(w http.ResponseWriter, r *http.Request, limit int64) ([]byte, error) {
	var jsonBody validation.Object
	invalidFile := func(rule, text string) error {
		issue := validation.Issue{Message: text, Rule: rule, Field: "file"}
		if rule == "file.size" || rule == "file.extname" {
			issue.Meta = map[string]interface{}{"size": fmt.Sprintf("%dmb", limit>>20), "extnames": []string{"jpg", "png", "jpeg", "webp"}}
		}
		message := ""
		if strings.HasPrefix(r.URL.Path, "/v2/certificate-templates/") {
			message = "VALIDATION_ERROR"
		}
		issues := []validation.Issue{issue}
		if strings.HasSuffix(r.URL.Path, "/media/image") {
			raw := jsonBody["media_type"]
			if jsonBody == nil {
				raw, _ = json.Marshal(r.FormValue("media_type"))
			}
			_, other := validation.ValidateField("imageMediaValidator", "media_type", raw)
			issues = append(issues, other...)
		}
		return domain.Details(422, message, map[string]interface{}{"errors": issues})
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		jsonBody = requestData(r)
		if raw := jsonBody["file"]; len(raw) > 0 && string(raw) != "null" && string(raw) != `""` {
			return nil, invalidFile("file", "The file must be a file")
		}
		return nil, invalidFile("required", "The file field must be defined")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	if err := r.ParseMultipartForm(6 << 20); err != nil {
		var oversized *http.MaxBytesError
		if errors.As(err, &oversized) {
			return nil, domain.Fail(413, "request entity too large")
		}
		return nil, invalidFile("required", "The file field must be defined")
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, invalidFile("required", "The file field must be defined")
	}
	defer file.Close()
	if header.Size > limit {
		return nil, invalidFile("file.size", fmt.Sprintf("File size should be less than %dMB", limit>>20))
	}
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(body)) > limit {
		return nil, domain.Fail(422, "INVALID_IMAGE")
	}
	switch http.DetectContentType(body) {
	case "image/jpeg", "image/png", "image/webp":
		return body, nil
	case "image/gif":
		return nil, invalidFile("file.extname", "Invalid file extension gif. Only jpg, png, jpeg, webp are allowed")
	default:
		return nil, invalidFile("file.extname", "Invalid file extension undefined. Only jpg, png, jpeg, webp are allowed")
	}
}
func (s *Server) registerMedia() {
	s.register("activities_controller", "uploadImage", s.uploadActivityImage)
}
func (s *Server) uploadActivityImage(w http.ResponseWriter, r *http.Request) error {
	body, err := readImage(w, r, 1<<20)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if err != nil {
		return domain.Fail(404, "ACTIVITY_NOT_FOUND")
	}
	result, err := (activity.Service{Pool: s.Pool, Storage: s.Storage}).UploadImage(r.Context(), int32(id), body)
	if err != nil {
		return err
	}
	reply(w, 200, "UPLOAD_IMAGE_SUCCESS", result)
	return nil
}
