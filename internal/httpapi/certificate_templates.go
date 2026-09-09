package httpapi

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/certificate"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/media"
	"kaderisasi/admin/internal/validation"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var positiveIDPattern = regexp.MustCompile(`^[1-9][0-9]*$`)

func certificateID(r *http.Request) int32 {
	raw := r.PathValue("id")
	if !positiveIDPattern.MatchString(raw) {
		return 0
	}
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0
	}
	return int32(id)
}
func certificateInput(w http.ResponseWriter, r *http.Request, schema string) (database.Object, bool) {
	body := requestData(r)
	data, issues := validation.Validate(schema, body)
	if len(issues) > 0 {
		write(w, 422, struct {
			Message string             `json:"message"`
			Errors  []validation.Issue `json:"errors"`
		}{"VALIDATION_ERROR", issues})
		return nil, false
	}
	if !certificateIDsFit(data, schema != "issueBulkCertificateValidator") {
		message(w, 500, "GENERAL_ERROR")
		return nil, false
	}
	return data, true
}

const templateCountsSQL = "SELECT t.*,(SELECT count(*)::int FROM activities WHERE certificate_template_id=t.id) AS activity_usage_count,(SELECT count(*)::int FROM issued_certificates WHERE template_id=t.id) AS issued_certificate_count FROM certificate_templates t"

func (s *Server) templateAudit(r *http.Request, event string, template database.Object) {
	s.Logger.Info(event, "event", event, "actor_admin_id", actor(r).ID, "template_id", template.ID("id"), "template_version", template.ID("version"), "request_id", r.Header.Get("X-Request-ID"))
}
func (s *Server) registerCertificateTemplates() {
	controller := "certificate_templates_controller"
	service := certificate.Templates{Pool: s.Pool, Storage: s.Storage}
	s.register(controller, "index", func(w http.ResponseWriter, r *http.Request) error {
		params := r.URL.Query()
		query := templateCountsSQL + " WHERE true"
		args := []interface{}{}
		if search := strings.TrimSpace(params.Get("search")); search != "" {
			args = append(args, "%"+search+"%")
			query += fmt.Sprintf(" AND t.name ILIKE $%d", len(args))
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
			args = append(args, status)
			query += fmt.Sprintf(" AND t.lifecycle_status=$%d", len(args))
		}
		page, size := boundedPageParams(r, 10)
		data, err := s.queries().Paginate(r.Context(), query+" ORDER BY t.created_at DESC", args, page, size)
		if err != nil {
			return err
		}
		for i, row := range data.Data {
			if params.Get("view") == "summary" {
				data.Data[i] = certificate.TemplateSummary(row)
			} else {
				data.Data[i] = certificate.SerializeTemplate(row)
			}
		}
		reply(w, 200, "GET_DATA_SUCCESS", data)
		return nil
	})
	s.register(controller, "show", func(w http.ResponseWriter, r *http.Request) error {
		id := certificateID(r)
		if id == 0 {
			return domain.Fail(400, "INVALID_CERTIFICATE_TEMPLATE_ID")
		}
		row, err := s.queries().One(r.Context(), templateCountsSQL+" WHERE t.id=$1", id)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Fail(404, "CERTIFICATE_TEMPLATE_NOT_FOUND")
		}
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", certificate.SerializeTemplate(row))
		return nil
	})
	s.register(controller, "store", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := certificateInput(w, r, "certificateTemplateValidator")
		if !ok {
			return nil
		}
		row, err := service.Create(r.Context(), data)
		if err != nil {
			return err
		}
		s.templateAudit(r, "certificate_template_created", row)
		reply(w, 201, "CERTIFICATE_TEMPLATE_CREATED_SUCCESS", certificate.SerializeTemplate(row))
		return nil
	})
	for _, action := range []string{"update", "publish", "archive"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			id := certificateID(r)
			if id == 0 {
				if action == "update" {
					return domain.Fail(400, "INVALID_CERTIFICATE_TEMPLATE_ID")
				}
				return domain.Fail(404, "CERTIFICATE_TEMPLATE_NOT_FOUND")
			}
			schema := "mutateCertificateTemplateLifecycleValidator"
			if action == "update" {
				schema = "updateCertificateTemplateValidator"
			}
			data, ok := certificateInput(w, r, schema)
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
			reply(w, 200, msg, certificate.SerializeTemplate(row))
			return nil
		})
	}
	s.register(controller, "duplicate", func(w http.ResponseWriter, r *http.Request) error {
		id := certificateID(r)
		if id == 0 {
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
		reply(w, 201, "CERTIFICATE_TEMPLATE_CREATED_SUCCESS", certificate.SerializeTemplate(row))
		return nil
	})
	s.register(controller, "destroy", func(w http.ResponseWriter, r *http.Request) error {
		id := certificateID(r)
		if id == 0 {
			return domain.Fail(404, "CERTIFICATE_TEMPLATE_NOT_FOUND")
		}
		row, err := s.queries().One(r.Context(), templateCountsSQL+" WHERE t.id=$1", id)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Fail(404, "CERTIFICATE_TEMPLATE_NOT_FOUND")
		}
		if err != nil {
			return err
		}
		if row.ID("activity_usage_count") > 0 || row.ID("issued_certificate_count") > 0 {
			return domain.Fail(409, "CERTIFICATE_TEMPLATE_IN_USE")
		}
		if _, err = s.queries().Delete(r.Context(), "certificate_templates", id); err != nil {
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
	ctx := r.Context()
	id := certificateID(r)
	template, err := s.queries().One(ctx, "SELECT * FROM certificate_templates WHERE id=$1", id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "CERTIFICATE_TEMPLATE_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	if template.String("lifecycle_status") != "draft" {
		return domain.Fail(409, "CERTIFICATE_TEMPLATE_USE_DRAFT_COPY")
	}
	background := strings.HasSuffix(r.URL.Path, "/background")
	version := template.ID("version") + 1
	kind := "assets"
	if background {
		version = template.ID("background_asset_version") + 1
		kind = "background"
	}
	uuid, err := auth.UUID()
	if err != nil {
		return err
	}
	key, err := media.Upload(ctx, s.Storage, body, fmt.Sprintf("certificate/templates/%d/%s/v%d-%s", id, kind, version, uuid), media.Certificate)
	if errors.Is(err, media.ErrInvalidImage) {
		return domain.Fail(422, "INVALID_IMAGE")
	}
	if err != nil {
		return err
	}
	if !background {
		s.templateAudit(r, "certificate_template_asset_uploaded", template)
		reply(w, 201, "UPLOAD_CERTIFICATE_ASSET_SUCCESS", struct {
			Key      string `json:"asset_key"`
			URL      string `json:"url"`
			AssetKey string `json:"assetKey"`
		}{key, key, key})
		return nil
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.Storage.Delete(context.WithoutCancel(ctx), key)
		}
	}()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := database.JSONQueries{DB: tx}
	current, err := q.One(ctx, "SELECT * FROM certificate_templates WHERE id=$1 FOR UPDATE", id)
	if err != nil {
		return err
	}
	if current.String("lifecycle_status") != "draft" || current.ID("version") != template.ID("version") {
		return domain.Fail(409, "CERTIFICATE_TEMPLATE_VERSION_CONFLICT")
	}
	change := database.Object{}
	change.Set("background_image", key)
	change.Set("background_asset_version", version)
	change.Set("version", current.ID("version")+1)
	current, err = q.Update(ctx, "certificate_templates", id, change)
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	s.templateAudit(r, "certificate_template_background_uploaded", current)
	reply(w, 200, "UPLOAD_BACKGROUND_SUCCESS", struct {
		Background      string `json:"backgroundImage"`
		Key             string `json:"asset_key"`
		URL             string `json:"url"`
		AssetVersion    int32  `json:"assetVersion"`
		TemplateVersion int32  `json:"templateVersion"`
	}{key, key, key, version, current.ID("version")})
	return nil
}
