package certificate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/storage"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf16"
)

type Templates struct {
	Pool    *pgxpool.Pool
	Storage storage.Store
}

var DefaultTemplate = json.RawMessage(`{"backgroundUrl":null,"elements":[],"canvasWidth":800,"canvasHeight":566}`)

func rawObject(raw []byte) database.Object {
	out := database.Object{}
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		out = database.Object{}
	}
	return out
}
func SerializeTemplate(template database.Object) database.Object {
	database.Timestamps(template, time.Local, "created_at", "updated_at", "published_at", "archived_at")
	for _, key := range []string{"published_at", "archived_at"} {
		if !template.Has(key) {
			template.Set(key, nil)
		}
	}
	template.Set("status", template.String("lifecycle_status"))
	template.Set("is_active", template.String("lifecycle_status") == "published")
	template.Set("readiness", CheckReadiness(template))
	if !template.Has("activity_usage_count") {
		template.Set("activity_usage_count", 0)
	}
	if !template.Has("issued_certificate_count") {
		template.Set("issued_certificate_count", 0)
	}
	return template
}
func TemplateSummary(template database.Object) database.Object {
	out := database.Object{}
	for _, key := range []string{"id", "name", "description", "version"} {
		out[key] = template[key]
	}
	out.Set("status", template.String("lifecycle_status"))
	out.Set("readiness", CheckReadiness(template))
	return out
}
func (s Templates) Create(ctx context.Context, payload database.Object) (database.Object, error) {
	if payload.String("status") == "published" {
		return nil, domain.Fail(422, "USE_TEMPLATE_PUBLISH_ENDPOINT")
	}
	status := "draft"
	if payload.String("status") == "archived" {
		status = "archived"
	}
	data := database.Object{}
	data["name"] = payload["name"]
	data.Set("description", nil)
	if payload.Has("description") {
		data["description"] = payload["description"]
	}
	data["template_data"] = DefaultTemplate
	if payload.Has("templateData") {
		data["template_data"] = payload["templateData"]
	}
	data.Set("background_image", nil)
	data.Set("lifecycle_status", status)
	data.Set("is_active", false)
	data.Set("version", 1)
	data.Set("background_asset_version", 0)
	data.Set("published_at", nil)
	data.Set("archived_at", nil)
	if status == "archived" {
		data.Set("archived_at", time.Now())
	}
	return (database.JSONQueries{DB: s.Pool}).Insert(ctx, "certificate_templates", data)
}
func VersionConflict(template database.Object) error {
	database.Timestamps(template, time.Local, "updated_at")
	return domain.Details(409, "CERTIFICATE_TEMPLATE_VERSION_CONFLICT", map[string]interface{}{"currentVersion": template.ID("version"), "updatedAt": template["updated_at"]})
}
func notReady(readiness Readiness) error {
	return domain.Details(422, "CERTIFICATE_TEMPLATE_NOT_READY", map[string]interface{}{"errors": readiness.Errors})
}
func (s Templates) Mutate(ctx context.Context, id int32, payload database.Object, action string) (database.Object, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := database.JSONQueries{DB: tx}
	template, err := q.One(ctx, "SELECT * FROM certificate_templates WHERE id=$1 FOR UPDATE", id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.Fail(404, "CERTIFICATE_TEMPLATE_NOT_FOUND")
	}
	if err != nil {
		return nil, err
	}
	if template.ID("version") != payload.ID("expectedVersion") {
		return nil, VersionConflict(template)
	}
	change := database.Object{}
	requestedStatus := action
	if action == "update" {
		if template.String("lifecycle_status") != "draft" && (payload.Has("templateData") || payload.Has("backgroundImage") || payload.Has("name") || payload.Has("description") || payload.String("status") == "draft") {
			return nil, domain.Fail(409, "CERTIFICATE_TEMPLATE_USE_DRAFT_COPY")
		}
		for _, pair := range [][2]string{{"name", "name"}, {"description", "description"}, {"templateData", "template_data"}} {
			if payload.Has(pair[0]) {
				change[pair[1]] = payload[pair[0]]
			}
		}
		if background := payload.String("backgroundImage"); background != "" {
			if !strings.HasPrefix(background, fmt.Sprintf("certificate/templates/%d/", id)) {
				return nil, domain.Fail(422, "INVALID_CERTIFICATE_ASSET_KEY")
			}
			if _, err = s.Storage.Get(ctx, background); err != nil {
				return nil, domain.Fail(422, "INVALID_CERTIFICATE_ASSET_KEY")
			}
			raw := template["template_data"]
			if change.Has("template_data") {
				raw = change["template_data"]
			}
			data := rawObject(raw)
			data.Set("backgroundUrl", nil)
			change.Set("template_data", data)
		}
		if payload.Has("backgroundImage") && string(payload["backgroundImage"]) != string(template["background_image"]) {
			change["background_image"] = payload["backgroundImage"]
			change.Set("background_asset_version", template.ID("background_asset_version")+1)
		}
		requestedStatus = payload.String("status")
		if requestedStatus == "" && payload.Has("isActive") {
			if payload.Bool("isActive") {
				requestedStatus = "published"
			} else if template.String("lifecycle_status") == "published" {
				requestedStatus = "archived"
			}
		}
	}
	if requestedStatus == "publish" {
		requestedStatus = "published"
	}
	if requestedStatus == "archive" {
		requestedStatus = "archived"
	}
	for key, value := range change {
		template[key] = value
	}
	if requestedStatus == "published" {
		ready := CheckReadiness(template)
		if !ready.Ready {
			return nil, notReady(ready)
		}
		change.Set("lifecycle_status", "published")
		change.Set("is_active", true)
		change.Set("published_at", time.Now())
		change.Set("archived_at", nil)
	} else if requestedStatus == "archived" {
		change.Set("lifecycle_status", "archived")
		change.Set("is_active", false)
		change.Set("archived_at", time.Now())
	} else if requestedStatus == "draft" {
		change.Set("lifecycle_status", "draft")
		change.Set("is_active", false)
		change.Set("published_at", nil)
		change.Set("archived_at", nil)
	}
	if action == "update" && (requestedStatus == "published" || (requestedStatus == "" && template.String("lifecycle_status") == "published")) {
		ready := CheckReadiness(template)
		if !ready.Ready {
			return nil, notReady(ready)
		}
	}
	change.Set("version", template.ID("version")+1)
	row, err := q.Update(ctx, "certificate_templates", id, change)
	if err != nil {
		return nil, err
	}
	return row, tx.Commit(ctx)
}
func AssetKey(value string, id int32) (string, error) {
	prefix := fmt.Sprintf("certificate/templates/%d/", id)
	path := value
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		u, err := url.Parse(value)
		if err != nil {
			return "", err
		}
		decoded, err := url.PathUnescape(u.EscapedPath())
		if err != nil {
			return "", err
		}
		path = strings.TrimPrefix(decoded, "/")
	}
	key := ""
	if strings.HasPrefix(path, prefix) {
		key = path
	} else if i := strings.Index(path, "/"+prefix); i >= 0 {
		key = path[i+1:]
	}
	if !strings.HasPrefix(key, prefix) || strings.Contains(key, "?") {
		return "", errors.New("INVALID_CERTIFICATE_ASSET_KEY")
	}
	for _, part := range strings.Split(key, "/") {
		if part == "." || part == ".." {
			return "", errors.New("INVALID_CERTIFICATE_ASSET_KEY")
		}
	}
	return key, nil
}

var extensionPattern = regexp.MustCompile(`\.[a-zA-Z0-9]+$`)

func (s Templates) Duplicate(ctx context.Context, id int32) (result database.Object, err error) {
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
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := database.JSONQueries{DB: tx}
	source, err := q.One(ctx, "SELECT * FROM certificate_templates WHERE id=$1 FOR UPDATE", id)
	if err != nil {
		return nil, err
	}
	data := database.Object{}
	name := utf16.Encode([]rune(source.String("name")))
	if len(name) > 245 {
		name = name[:245]
	}
	data.Set("name", string(utf16.Decode(name))+" (Salinan)")
	data["description"] = source["description"]
	design := rawObject(source["template_data"])
	design.Set("backgroundUrl", nil)
	design.Set("elements", []interface{}{})
	data.Set("template_data", design)
	data.Set("background_image", nil)
	data.Set("background_asset_version", 0)
	data.Set("lifecycle_status", "draft")
	data.Set("is_active", false)
	data.Set("version", 1)
	data.Set("published_at", nil)
	data.Set("archived_at", nil)
	copy, err := q.Insert(ctx, "certificate_templates", data)
	if err != nil {
		return nil, err
	}
	replacements := map[string]string{}
	copyAsset := func(value string) (string, error) {
		key, err := AssetKey(value, id)
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
		target := fmt.Sprintf("certificate/templates/%d/assets/%s%s", copy.ID("id"), uuid, extension)
		copied = append(copied, target)
		if err = s.Storage.Copy(ctx, key, target); err != nil {
			return "", err
		}
		replacements[key] = target
		return target, nil
	}
	original := rawObject(source["template_data"])
	background := source.String("background_image")
	if background == "" {
		background = original.String("backgroundUrl")
	}
	change := database.Object{}
	change.Set("background_image", nil)
	if background != "" {
		key, err := copyAsset(background)
		if err != nil {
			return nil, err
		}
		change.Set("background_image", key)
	}
	var elements []database.Object
	if err = json.Unmarshal(original["elements"], &elements); err != nil {
		return nil, err
	}
	for _, element := range elements {
		if image := element.String("imageUrl"); image != "" {
			key, err := copyAsset(image)
			if err != nil {
				return nil, err
			}
			element.Set("imageUrl", key)
		}
	}
	design.Set("elements", elements)
	change.Set("template_data", design)
	result, err = q.Update(ctx, "certificate_templates", copy.ID("id"), change)
	if err != nil {
		return nil, err
	}
	return result, tx.Commit(ctx)
}
