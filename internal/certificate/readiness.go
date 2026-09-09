package certificate

import (
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/database"
	"net/url"
	"strings"
)

type Readiness struct {
	Ready  bool     `json:"ready"`
	Errors []string `json:"errors"`
}
type Element struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	X        *float64 `json:"x"`
	Y        *float64 `json:"y"`
	Width    *float64 `json:"width"`
	Height   *float64 `json:"height"`
	Variable string   `json:"variable"`
	Visible  *bool    `json:"visible"`
	ImageURL string   `json:"imageUrl"`
}
type TemplateData struct {
	BackgroundURL *string   `json:"backgroundUrl"`
	CanvasWidth   float64   `json:"canvasWidth"`
	CanvasHeight  float64   `json:"canvasHeight"`
	Elements      []Element `json:"elements"`
}

var variables = map[string]bool{"name": true, "activity_name": true, "activity_date": true, "date": true, "certificate_code": true, "certificate_id": true, "university": true, "gender": true}

func ManagedAsset(id int32, value string) bool {
	if strings.HasPrefix(value, "data:") {
		return false
	}
	prefix := fmt.Sprintf("certificate/templates/%d/", id)
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		u, err := url.Parse(value)
		return err == nil && strings.Contains(u.Path, "/"+prefix)
	}
	return strings.Contains(value, prefix)
}
func CheckReadiness(template database.Object) Readiness {
	result := Readiness{Errors: []string{}}
	seen := map[string]bool{}
	add := func(message string) {
		if !seen[message] {
			result.Errors = append(result.Errors, message)
			seen[message] = true
		}
	}
	if strings.TrimSpace(template.String("name")) == "" {
		add("TEMPLATE_NAME_REQUIRED")
	}
	var data *TemplateData
	if err := json.Unmarshal(template["template_data"], &data); err != nil || data == nil {
		add("TEMPLATE_DATA_REQUIRED")
		return result
	}
	if data.CanvasWidth < 100 || data.CanvasHeight < 100 || data.CanvasWidth > 5000 || data.CanvasHeight > 5000 {
		add("INVALID_CANVAS_SIZE")
	}
	if data.BackgroundURL != nil && strings.HasPrefix(*data.BackgroundURL, "data:") {
		add("BACKGROUND_MUST_USE_MANAGED_ASSET")
	}
	if len(data.Elements) == 0 {
		add("ELEMENTS_REQUIRED")
		return result
	}
	if len(data.Elements) > 200 {
		add("TOO_MANY_ELEMENTS")
	}
	ids := map[string]bool{}
	hasName := false
	for _, e := range data.Elements {
		if e.ID == "" || ids[e.ID] {
			add("ELEMENT_IDS_MUST_BE_UNIQUE")
		}
		ids[e.ID] = true
		if e.X == nil || e.Y == nil || e.Width == nil || e.Height == nil || *e.X < 0 || *e.Y < 0 || *e.Width <= 0 || *e.Height <= 0 || *e.X+*e.Width > data.CanvasWidth || *e.Y+*e.Height > data.CanvasHeight {
			add("ELEMENT_OUTSIDE_CANVAS")
		}
		if e.Type == "variable-text" {
			v := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(e.Variable, "{{", ""), "}}", ""))
			if !variables[v] {
				add("UNSUPPORTED_VARIABLE")
			}
			if v == "name" && (e.Visible == nil || *e.Visible) {
				hasName = true
			}
		}
		if e.Type == "image" || e.Type == "signature" {
			if e.ImageURL == "" {
				add("ELEMENT_ASSET_REQUIRED")
			} else if !ManagedAsset(template.ID("id"), e.ImageURL) {
				add("ELEMENT_MUST_USE_MANAGED_ASSET")
			}
		}
	}
	if !hasName {
		add("PARTICIPANT_NAME_VARIABLE_REQUIRED")
	}
	result.Ready = len(result.Errors) == 0
	return result
}
