package certificate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/media"
	"strconv"
	"unicode/utf16"
)

func (s Templates) Duplicate(ctx context.Context, id string) (result dbgen.CertificateTemplate, err error) {
	copied := []string{}
	defer func() {
		if err != nil {
			for _, key := range copied {
				_ = s.Storage.Delete(context.WithoutCancel(ctx), key)
			}
		}
	}()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	source, err := q.LockCertificateTemplate(ctx, id)
	if err != nil {
		return result, err
	}
	name := utf16.Encode([]rune(source.Name))
	if len(name) > 245 {
		name = name[:245]
	}
	original := rawObject(source.TemplateData)
	design := rawObject(source.TemplateData)
	design.Set("backgroundUrl", nil)
	design.Set("elements", []interface{}{})
	raw, err := json.Marshal(design)
	if err != nil {
		return result, err
	}
	copy, err := q.CreateCertificateTemplate(ctx, dbgen.CreateCertificateTemplateParams{Name: string(utf16.Decode(name)) + " (Salinan)", Description: source.Description, TemplateData: raw, LifecycleStatus: "draft"})
	if err != nil {
		return result, err
	}
	replacements := map[string]string{}
	copyAsset := func(value string) (string, error) {
		key, err := AssetKey(value, source.ID)
		if err != nil {
			return "", err
		}
		if existing := replacements[key]; existing != "" {
			return existing, nil
		}
		extension := extensionPattern.FindString(key)
		if extension == "" {
			extension = ".webp"
		}
		uuid, err := auth.UUID()
		if err != nil {
			return "", err
		}
		target := fmt.Sprintf("certificate/templates/%d/assets/%s%s", copy.ID, uuid, extension)
		copied = append(copied, target)
		if err = s.Storage.Copy(ctx, key, target); err != nil {
			return "", err
		}
		replacements[key] = target
		return target, nil
	}
	background := original.String("backgroundUrl")
	if source.BackgroundImage != nil && *source.BackgroundImage != "" {
		background = *source.BackgroundImage
	}
	var backgroundKey *string
	if background != "" {
		key, err := copyAsset(background)
		if err != nil {
			return result, err
		}
		backgroundKey = &key
	}
	var elements []database.Object
	if string(original["elements"]) == "null" {
		return result, errors.New("source.templateData.elements is not iterable")
	}
	if err = json.Unmarshal(original["elements"], &elements); err != nil {
		return result, err
	}
	for _, element := range elements {
		if element == nil {
			return result, errors.New("Cannot read properties of null (reading 'imageUrl')")
		}
		if value := element.String("imageUrl"); value != "" {
			key, err := copyAsset(value)
			if err != nil {
				return result, err
			}
			element.Set("imageUrl", key)
		}
	}
	design.Set("elements", elements)
	raw, err = json.Marshal(design)
	if err != nil {
		return result, err
	}
	result, err = q.CompleteDuplicatedTemplate(ctx, dbgen.CompleteDuplicatedTemplateParams{ID: copy.ID, BackgroundImage: backgroundKey, TemplateData: raw})
	if err != nil {
		return result, err
	}
	err = tx.Commit(ctx)
	return result, err
}

type UploadedTemplate struct {
	Template   dbgen.CertificateTemplate
	Asset      AssetUploaded
	Background BackgroundUploaded
}

func (s Templates) Upload(ctx context.Context, id string, body []byte, background bool) (UploadedTemplate, error) {
	result := UploadedTemplate{}
	template, err := dbgen.New(s.Pool).CertificateTemplateByIdentifier(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, domain.Fail(404, "CERTIFICATE_TEMPLATE_NOT_FOUND")
	}
	if err != nil {
		return result, err
	}
	if template.LifecycleStatus != "draft" {
		return result, domain.Fail(409, "CERTIFICATE_TEMPLATE_USE_DRAFT_COPY")
	}
	version := int64(template.Version) + 1
	kind := "assets"
	if background {
		version = int64(template.BackgroundAssetVersion) + 1
		kind = "background"
	}
	uuid, err := auth.UUID()
	if err != nil {
		return result, err
	}
	key, err := media.Upload(ctx, s.Storage, body, fmt.Sprintf("certificate/templates/%d/%s/v%d-%s", template.ID, kind, version, uuid), media.Certificate)
	if errors.Is(err, media.ErrInvalidImage) {
		return result, domain.Fail(422, "INVALID_IMAGE")
	}
	if err != nil {
		return result, err
	}
	if !background {
		result.Template = template
		result.Asset = AssetUploaded{key, key, key}
		return result, nil
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.Storage.Delete(context.WithoutCancel(ctx), key)
		}
	}()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	current, err := q.LockCertificateTemplate(ctx, id)
	if err != nil {
		return result, err
	}
	if current.LifecycleStatus != "draft" || current.Version != template.Version {
		return result, domain.Fail(409, "CERTIFICATE_TEMPLATE_VERSION_CONFLICT")
	}
	current, err = q.ReplaceCertificateBackground(ctx, dbgen.ReplaceCertificateBackgroundParams{ID: current.ID, BackgroundImage: key, BackgroundAssetVersion: strconv.FormatInt(version, 10), Version: strconv.FormatInt(int64(current.Version)+1, 10)})
	if err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	committed = true
	result.Template = current
	result.Background = BackgroundUploaded{key, key, key, current.BackgroundAssetVersion, current.Version}
	return result, nil
}
