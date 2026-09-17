package httpapi

import (
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/form"
	"kaderisasi/admin/internal/storage"
	"mime"
	"net/http"
	"slices"
	"strconv"
)

func (s *Server) registerFormResponses() {
	service := form.Service{Pool: s.Pool, Location: s.Config.Location}
	formID := func(r *http.Request) (int32, error) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
		if err != nil || id < 1 {
			return 0, domain.Fail(404, "CUSTOM_FORM_NOT_FOUND")
		}
		return int32(id), nil
	}
	for _, action := range []string{"responses", "response", "exportResponses", "downloadAttachment"} {
		s.register("custom_forms_controller", action, func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("Cache-Control", "private, no-store")
			id, err := formID(r)
			if err != nil {
				return err
			}
			switch action {
			case "responses":
				page, size := coursePagination(r)
				result, err := service.Responses(r.Context(), id, page, size)
				if err != nil {
					return err
				}
				reply(w, 200, "GET_DATA_SUCCESS", result)
			case "response":
				var uuid pgtype.UUID
				if uuid.Scan(r.PathValue("responseId")) != nil {
					return domain.Fail(404, "FORM_RESPONSE_NOT_FOUND")
				}
				result, err := service.Response(r.Context(), id, uuid)
				if err != nil {
					return err
				}
				reply(w, 200, "GET_DATA_SUCCESS", result)
			case "exportResponses":
				// Downloads point to the authenticated admin route, never the object store.
				origin := r.Header.Get("Origin")
				if !slices.Contains(s.Config.Origins, origin) {
					if len(s.Config.Origins) == 0 {
						return domain.Fail(503, "ADMIN_ORIGIN_NOT_CONFIGURED")
					}
					origin = s.Config.Origins[0]
				}
				base := fmt.Sprintf("%s/custom-form/%d/files", origin, id)
				body, err := service.ExportResponses(r.Context(), id, base)
				if err != nil {
					return err
				}
				sendWorkbook(w, "respons-formulir.xlsx", body)
			case "downloadAttachment":
				var uuid pgtype.UUID
				if uuid.Scan(r.PathValue("attachmentId")) != nil {
					return domain.Fail(404, "ATTACHMENT_NOT_FOUND")
				}
				file, err := dbgen.New(s.Pool).FormAttachmentByID(r.Context(), dbgen.FormAttachmentByIDParams{FormID: id, AttachmentID: uuid})
				if errors.Is(err, pgx.ErrNoRows) {
					return domain.Fail(404, "ATTACHMENT_NOT_FOUND")
				}
				if err != nil {
					return err
				}
				store := storage.NewCourseDocuments(s.Config)
				if store == nil {
					return domain.Fail(503, "STORAGE_UNAVAILABLE")
				}
				body, err := store.Get(r.Context(), file.StorageKey)
				if err != nil {
					return err
				}
				w.Header().Set("Content-Type", file.MimeType)
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.DownloadName}))
				w.WriteHeader(200)
				_, err = w.Write(body)
				return err
			}
			return nil
		})
	}
}
