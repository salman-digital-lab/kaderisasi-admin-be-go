package certificate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func (s Templates) List(ctx context.Context, filters TemplateFilters) (TemplatePage[TemplateResponse], error) {
	q := dbgen.New(s.Pool)
	total, err := q.CountCertificateTemplates(ctx, dbgen.CountCertificateTemplatesParams{Search: filters.Search, Status: filters.Status})
	result := TemplatePage[TemplateResponse]{Meta: database.Meta(total, filters.Page, filters.Size), Data: []TemplateResponse{}}
	if err != nil || total == 0 {
		return result, err
	}
	size, offset, err := database.SQLPage(filters.Page, filters.Size)
	if err != nil {
		return result, err
	}
	rows, err := q.ListCertificateTemplates(ctx, dbgen.ListCertificateTemplatesParams{Search: filters.Search, Status: filters.Status, PageSize: size, PageOffset: offset})
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		result.Data = append(result.Data, TemplateView(row.CertificateTemplate, row.ActivityUsageCount, row.IssuedCertificateCount))
	}
	return result, nil
}
func (s Templates) Show(ctx context.Context, id string) (TemplateResponse, error) {
	row, err := dbgen.New(s.Pool).CertificateTemplateCounts(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return TemplateResponse{}, domain.Fail(404, "CERTIFICATE_TEMPLATE_NOT_FOUND")
	}
	return TemplateView(row.CertificateTemplate, row.ActivityUsageCount, row.IssuedCertificateCount), err
}
func (s Templates) Create(ctx context.Context, input TemplateInput) (dbgen.CertificateTemplate, error) {
	if input.Status != nil && *input.Status == "published" {
		return dbgen.CertificateTemplate{}, domain.Fail(422, "USE_TEMPLATE_PUBLISH_ENDPOINT")
	}
	params := dbgen.CreateCertificateTemplateParams{Name: *input.Name, Description: input.Description.Value, LifecycleStatus: "draft", TemplateData: DefaultTemplate}
	if input.Status != nil && *input.Status == "archived" {
		params.LifecycleStatus = "archived"
		params.ArchivedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}
	if input.Data != nil {
		var err error
		params.TemplateData, err = json.Marshal(input.Data)
		if err != nil {
			return dbgen.CertificateTemplate{}, err
		}
	}
	return dbgen.New(s.Pool).CreateCertificateTemplate(ctx, params)
}
func templateParams(row dbgen.CertificateTemplate) dbgen.UpdateCertificateTemplateParams {
	return dbgen.UpdateCertificateTemplateParams{ID: row.ID, Name: row.Name, Description: row.Description, BackgroundImage: row.BackgroundImage, TemplateData: row.TemplateData, LifecycleStatus: row.LifecycleStatus, IsActive: row.IsActive, Version: strconv.FormatInt(int64(row.Version)+1, 10), BackgroundAssetVersion: strconv.FormatInt(int64(row.BackgroundAssetVersion), 10), PublishedAt: row.PublishedAt, ArchivedAt: row.ArchivedAt}
}
func typedVersionConflict(row dbgen.CertificateTemplate) error {
	if !row.UpdatedAt.Valid {
		return errors.New("Cannot read properties of null (reading 'toISO')")
	}
	return domain.Details(409, "CERTIFICATE_TEMPLATE_VERSION_CONFLICT", map[string]interface{}{"currentVersion": row.Version, "updatedAt": domain.ModelTimestamp(row.UpdatedAt, time.Local)})
}
func (s Templates) Mutate(ctx context.Context, id string, input TemplateInput, action string) (dbgen.CertificateTemplate, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return dbgen.CertificateTemplate{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	current, err := q.LockCertificateTemplate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return current, domain.Fail(404, "CERTIFICATE_TEMPLATE_NOT_FOUND")
	}
	if err != nil {
		return current, err
	}
	if strconv.FormatInt(int64(current.Version), 10) != input.ExpectedVersion.String() {
		return current, typedVersionConflict(current)
	}
	params := templateParams(current)
	status := action
	if action == "update" {
		if current.LifecycleStatus != "draft" && (input.Data != nil || input.Background.Present || input.Name != nil || input.Description.Present || input.Status != nil && *input.Status == "draft") {
			return current, domain.Fail(409, "CERTIFICATE_TEMPLATE_USE_DRAFT_COPY")
		}
		if input.Name != nil {
			params.Name = *input.Name
		}
		if input.Description.Present {
			params.Description = input.Description.Value
		}
		if input.Data != nil {
			params.TemplateData, err = json.Marshal(input.Data)
			if err != nil {
				return current, err
			}
		}
		if input.Background.Value != nil && *input.Background.Value != "" {
			key := *input.Background.Value
			if !strings.HasPrefix(key, fmt.Sprintf("certificate/templates/%d/", current.ID)) {
				return current, domain.Fail(422, "INVALID_CERTIFICATE_ASSET_KEY")
			}
			if _, err = s.Storage.Get(ctx, key); err != nil {
				return current, domain.Fail(422, "INVALID_CERTIFICATE_ASSET_KEY")
			}
			design := rawObject(params.TemplateData)
			design.Set("backgroundUrl", nil)
			params.TemplateData, err = json.Marshal(design)
			if err != nil {
				return current, err
			}
		}
		if input.Background.Present && !reflect.DeepEqual(current.BackgroundImage, input.Background.Value) {
			params.BackgroundImage = input.Background.Value
			params.BackgroundAssetVersion = strconv.FormatInt(int64(current.BackgroundAssetVersion)+1, 10)
		}
		status = ""
		if input.Status != nil {
			status = *input.Status
		} else if input.IsActive != nil {
			if *input.IsActive {
				status = "published"
			} else if current.LifecycleStatus == "published" {
				status = "archived"
			}
		}
	}
	switch status {
	case "published", "publish":
		ready := CheckReadinessValues(current.ID, params.Name, params.TemplateData)
		if !ready.Ready {
			return current, notReady(ready)
		}
		params.LifecycleStatus = "published"
		params.IsActive = new(true)
		params.PublishedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		params.ArchivedAt = pgtype.Timestamptz{}
	case "archived", "archive":
		params.LifecycleStatus = "archived"
		params.IsActive = new(false)
		params.ArchivedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	case "draft":
		params.LifecycleStatus = "draft"
		params.IsActive = new(false)
		params.PublishedAt = pgtype.Timestamptz{}
		params.ArchivedAt = pgtype.Timestamptz{}
	}
	if params.LifecycleStatus == "published" {
		ready := CheckReadinessValues(current.ID, params.Name, params.TemplateData)
		if !ready.Ready {
			return current, notReady(ready)
		}
	}
	row, err := q.UpdateCertificateTemplate(ctx, params)
	if err != nil {
		return current, err
	}
	return row, tx.Commit(ctx)
}
func (s Templates) Delete(ctx context.Context, id string) error {
	q := dbgen.New(s.Pool)
	row, err := q.CertificateTemplateCounts(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "CERTIFICATE_TEMPLATE_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	if row.ActivityUsageCount > 0 || row.IssuedCertificateCount > 0 {
		return domain.Fail(409, "CERTIFICATE_TEMPLATE_IN_USE")
	}
	return q.DeleteCertificateTemplate(ctx, row.CertificateTemplate.ID)
}
