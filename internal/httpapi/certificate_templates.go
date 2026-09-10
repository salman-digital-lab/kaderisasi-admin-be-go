package httpapi

import (
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/certificate"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/validation"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var positiveIDPattern = regexp.MustCompile(`^[1-9][0-9]*$`)

func certificateID(r *http.Request) string {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || !positiveIDPattern.MatchString(raw) || id > 9007199254740991 {
		return ""
	}
	return raw
}
func certificateInput(w http.ResponseWriter, r *http.Request, schema string) (database.Object, bool) {
	data, issues := validation.Validate(schema, requestData(r))
	if len(issues) > 0 {
		write(w, 422, struct {
			Message string             `json:"message"`
			Errors  []validation.Issue `json:"errors"`
		}{"VALIDATION_ERROR", issues})
		return nil, false
	}
	return data, true
}
func certificateInputAs[T any](w http.ResponseWriter, r *http.Request, schema string) (T, bool) {
	data, ok := certificateInput(w, r, schema)
	if !ok {
		var result T
		return result, false
	}
	return decodeInputAs[T](w, data)
}
func (s *Server) templateAudit(r *http.Request, event string, template dbgen.CertificateTemplate) {
	s.Logger.Info(event, "event", event, "actor_admin_id", actor(r).ID, "template_id", template.ID, "template_version", template.Version, "request_id", r.Header.Get("X-Request-ID"))
}
func (s *Server) registerCertificateTemplates() {
	controller := "certificate_templates_controller"
	service := certificate.Templates{Pool: s.Pool, Storage: s.Storage}
	s.register(controller, "index", func(w http.ResponseWriter, r *http.Request) error {
		params := r.URL.Query()
		page, size := boundedPageParams(r, 10)
		filters := certificate.TemplateFilters{Page: page, Size: size}
		if search := strings.TrimSpace(params.Get("search")); search != "" {
			filters.Search = &search
		}
		status := params.Get("status")
		if !slices.Contains([]string{"draft", "published", "archived"}, status) {
			status = ""
			if params.Has("is_active") {
				status = "archived"
				if params.Get("is_active") == "true" {
					status = "published"
				}
			}
		}
		if status != "" {
			filters.Status = &status
		}
		result, err := service.List(r.Context(), filters)
		if err != nil {
			return err
		}
		if params.Get("view") == "summary" {
			summary := certificate.TemplatePage[certificate.TemplateSummaryResponse]{Meta: result.Meta, Data: []certificate.TemplateSummaryResponse{}}
			for _, row := range result.Data {
				summary.Data = append(summary.Data, certificate.SummaryView(row.CertificateTemplate))
			}
			reply(w, 200, "GET_DATA_SUCCESS", summary)
		} else {
			reply(w, 200, "GET_DATA_SUCCESS", result)
		}
		return nil
	})
	s.register(controller, "show", func(w http.ResponseWriter, r *http.Request) error {
		id := certificateID(r)
		if id == "" {
			return domain.Fail(400, "INVALID_CERTIFICATE_TEMPLATE_ID")
		}
		row, err := service.Show(r.Context(), id)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", row)
		return nil
	})
	s.register(controller, "store", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := certificateInputAs[certificate.TemplateInput](w, r, "certificateTemplateValidator")
		if !ok {
			return nil
		}
		row, err := service.Create(r.Context(), data)
		if err != nil {
			return err
		}
		s.templateAudit(r, "certificate_template_created", row)
		reply(w, 201, "CERTIFICATE_TEMPLATE_CREATED_SUCCESS", certificate.TemplateView(row, 0, 0))
		return nil
	})
	for _, action := range []string{"update", "publish", "archive"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			id := certificateID(r)
			if id == "" {
				if action == "update" {
					return domain.Fail(400, "INVALID_CERTIFICATE_TEMPLATE_ID")
				}
				return domain.Fail(404, "CERTIFICATE_TEMPLATE_NOT_FOUND")
			}
			schema := "mutateCertificateTemplateLifecycleValidator"
			if action == "update" {
				schema = "updateCertificateTemplateValidator"
			}
			data, ok := certificateInputAs[certificate.TemplateInput](w, r, schema)
			if !ok {
				return nil
			}
			row, err := service.Mutate(r.Context(), id, data, action)
			if err != nil {
				return err
			}
			event := map[string]string{"update": "updated", "publish": "published", "archive": "archived"}[action]
			s.templateAudit(r, "certificate_template_"+event, row)
			msg := "CERTIFICATE_TEMPLATE_" + strings.ToUpper(event)
			if action == "update" {
				msg += "_SUCCESS"
			}
			reply(w, 200, msg, certificate.TemplateView(row, 0, 0))
			return nil
		})
	}
	s.register(controller, "duplicate", func(w http.ResponseWriter, r *http.Request) error {
		id := certificateID(r)
		if id == "" {
			return domain.Fail(400, "INVALID_CERTIFICATE_TEMPLATE_ID")
		}
		row, err := service.Duplicate(r.Context(), id)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Fail(404, "CERTIFICATE_TEMPLATE_NOT_FOUND")
		}
		if err != nil {
			return domain.Fail(422, "CERTIFICATE_ASSET_COPY_FAILED")
		}
		s.templateAudit(r, "certificate_template_duplicated", row)
		reply(w, 201, "CERTIFICATE_TEMPLATE_CREATED_SUCCESS", certificate.TemplateView(row, 0, 0))
		return nil
	})
	s.register(controller, "destroy", func(w http.ResponseWriter, r *http.Request) error {
		id := certificateID(r)
		if id == "" {
			return domain.Fail(404, "CERTIFICATE_TEMPLATE_NOT_FOUND")
		}
		if err := service.Delete(r.Context(), id); err != nil {
			return err
		}
		message(w, 200, "CERTIFICATE_TEMPLATE_DELETED_SUCCESS")
		return nil
	})
	s.register(controller, "uploadBackground", s.uploadTemplateAsset)
	s.register(controller, "uploadAsset", s.uploadTemplateAsset)
}
func (s *Server) uploadTemplateAsset(w http.ResponseWriter, r *http.Request) error {
	body, err := readImage(w, r, 5<<20)
	if err != nil {
		return err
	}
	id := certificateID(r)
	if id == "" {
		return domain.Fail(404, "CERTIFICATE_TEMPLATE_NOT_FOUND")
	}
	background := strings.HasSuffix(r.URL.Path, "/background")
	result, err := (certificate.Templates{Pool: s.Pool, Storage: s.Storage}).Upload(r.Context(), id, body, background)
	if err != nil {
		return err
	}
	if background {
		s.templateAudit(r, "certificate_template_background_uploaded", result.Template)
		reply(w, 200, "UPLOAD_BACKGROUND_SUCCESS", result.Background)
	} else {
		s.templateAudit(r, "certificate_template_asset_uploaded", result.Template)
		reply(w, 201, "UPLOAD_CERTIFICATE_ASSET_SUCCESS", result.Asset)
	}
	return nil
}
