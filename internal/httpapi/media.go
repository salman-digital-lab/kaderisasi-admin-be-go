package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"io"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/media"
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
func imagesFrom(raw []byte) (map[string]json.RawMessage, []string, error) {
	data := map[string]json.RawMessage{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, nil, err
		}
	}
	images := []string{}
	if value := data["images"]; len(value) > 0 {
		if err := json.Unmarshal(value, &images); err != nil {
			return nil, nil, err
		}
	}
	return data, images, nil
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
	activity, err := dbgen.New(s.Pool).ActivityByID(r.Context(), int32(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	_, images, err := imagesFrom(activity.AdditionalConfig)
	if err != nil {
		return err
	}
	if len(images) >= 8 {
		return domain.Fail(409, "ACTIVITY_IMAGE_LIMIT_REACHED")
	}
	uuid, err := auth.UUID()
	if err != nil {
		return err
	}
	key, err := media.Upload(r.Context(), s.Storage, body, "activity/"+strconv.FormatInt(id, 10)+"/"+uuid, media.Gallery)
	if errors.Is(err, media.ErrInvalidImage) {
		return domain.Fail(422, "INVALID_IMAGE")
	}
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.Storage.Delete(context.WithoutCancel(r.Context()), key)
		}
	}()
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	q := dbgen.New(tx)
	locked, err := q.LockActivity(r.Context(), int32(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	data, images, err := imagesFrom(locked.AdditionalConfig)
	if err != nil {
		return err
	}
	if len(images) >= 8 {
		return domain.Fail(409, "ACTIVITY_IMAGE_LIMIT_REACHED")
	}
	images = append(images, key)
	data["images"], _ = json.Marshal(images)
	raw, _ := json.Marshal(data)
	if err = q.SetActivityConfig(r.Context(), dbgen.SetActivityConfigParams{ID: int32(id), AdditionalConfig: raw}); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	committed = true
	reply(w, 200, "UPLOAD_IMAGE_SUCCESS", struct {
		Image  string   `json:"image"`
		Images []string `json:"images"`
	}{key, images})
	return nil
}
