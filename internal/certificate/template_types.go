package certificate

import (
	"encoding/json"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"time"
)

type TemplateInput struct {
	Name            *string                 `json:"name,omitempty"`
	Description     domain.Optional[string] `json:"description,omitzero"`
	Data            *Design                 `json:"templateData,omitempty"`
	Background      domain.Optional[string] `json:"backgroundImage,omitzero"`
	Status          *string                 `json:"status,omitempty"`
	IsActive        *bool                   `json:"isActive,omitempty"`
	ExpectedVersion json.Number             `json:"expectedVersion"`
}
type Design struct {
	BackgroundURL domain.Optional[string] `json:"backgroundUrl,omitzero"`
	Elements      *[]DesignElement        `json:"elements,omitempty"`
	CanvasWidth   *float64                `json:"canvasWidth,omitempty"`
	CanvasHeight  *float64                `json:"canvasHeight,omitempty"`
}
type DesignElement struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	Name           *string  `json:"name,omitempty"`
	X              float64  `json:"x"`
	Y              float64  `json:"y"`
	Width          float64  `json:"width"`
	Height         float64  `json:"height"`
	Content        *string  `json:"content,omitempty"`
	Variable       *string  `json:"variable,omitempty"`
	FontSize       *float64 `json:"fontSize,omitempty"`
	FontFamily     *string  `json:"fontFamily,omitempty"`
	Color          *string  `json:"color,omitempty"`
	TextAlign      *string  `json:"textAlign,omitempty"`
	VerticalAlign  *string  `json:"verticalAlign,omitempty"`
	FontWeight     *string  `json:"fontWeight,omitempty"`
	FontStyle      *string  `json:"fontStyle,omitempty"`
	TextDecoration *string  `json:"textDecoration,omitempty"`
	LineHeight     *float64 `json:"lineHeight,omitempty"`
	LetterSpacing  *float64 `json:"letterSpacing,omitempty"`
	ImageURL       *string  `json:"imageUrl,omitempty"`
	Opacity        *float64 `json:"opacity,omitempty"`
	Rotation       *float64 `json:"rotation,omitempty"`
	BorderRadius   *float64 `json:"borderRadius,omitempty"`
	ObjectFit      *string  `json:"objectFit,omitempty"`
	Visible        *bool    `json:"visible,omitempty"`
	Locked         *bool    `json:"locked,omitempty"`
}
type TemplateResponse struct {
	dbgen.CertificateTemplate
	TemplateData           json.RawMessage `json:"template_data"`
	IsActive               bool            `json:"is_active"`
	Status                 string          `json:"status"`
	Readiness              Readiness       `json:"readiness"`
	ActivityUsageCount     int64           `json:"activity_usage_count"`
	IssuedCertificateCount int64           `json:"issued_certificate_count"`
	CreatedAt              *string         `json:"created_at"`
	UpdatedAt              *string         `json:"updated_at"`
	PublishedAt            *string         `json:"published_at"`
	ArchivedAt             *string         `json:"archived_at"`
}
type TemplateSummaryResponse struct {
	ID          int32     `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Version     int32     `json:"version"`
	Status      string    `json:"status"`
	Readiness   Readiness `json:"readiness"`
}
type TemplateFilters struct {
	Search, Status *string
	Page, Size     float64
}
type TemplatePage[T any] struct {
	Meta database.Pagination `json:"meta"`
	Data []T                 `json:"data"`
}

func TemplateView(row dbgen.CertificateTemplate, activityCount, certificateCount int64) TemplateResponse {
	return TemplateResponse{CertificateTemplate: row, TemplateData: row.TemplateData, IsActive: row.LifecycleStatus == "published", Status: row.LifecycleStatus, Readiness: CheckReadinessValues(row.ID, row.Name, row.TemplateData), ActivityUsageCount: activityCount, IssuedCertificateCount: certificateCount, CreatedAt: domain.ModelTimestamp(row.CreatedAt, time.Local), UpdatedAt: domain.ModelTimestamp(row.UpdatedAt, time.Local), PublishedAt: domain.ModelTimestamp(row.PublishedAt, time.Local), ArchivedAt: domain.ModelTimestamp(row.ArchivedAt, time.Local)}
}
func SummaryView(row dbgen.CertificateTemplate) TemplateSummaryResponse {
	return TemplateSummaryResponse{ID: row.ID, Name: row.Name, Description: row.Description, Version: row.Version, Status: row.LifecycleStatus, Readiness: CheckReadinessValues(row.ID, row.Name, row.TemplateData)}
}

type AssetUploaded struct {
	Key      string `json:"asset_key"`
	URL      string `json:"url"`
	AssetKey string `json:"assetKey"`
}
type BackgroundUploaded struct {
	Background      string `json:"backgroundImage"`
	Key             string `json:"asset_key"`
	URL             string `json:"url"`
	AssetVersion    int32  `json:"assetVersion"`
	TemplateVersion int32  `json:"templateVersion"`
}
