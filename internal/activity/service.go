package activity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/certificate"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"time"
)

func numberText(value *json.Number) *string {
	if value == nil {
		return nil
	}
	text := value.String()
	return &text
}
func (s Service) checkTemplate(ctx context.Context, id *json.Number, canManage bool) error {
	if !canManage {
		return domain.Fail(403, "FORBIDDEN")
	}
	if id == nil {
		return nil
	}
	template, err := dbgen.New(s.Pool).CertificateTemplateByIdentifier(ctx, id.String())
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(422, "CERTIFICATE_TEMPLATE_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	readiness := certificate.CheckReadinessValues(template.ID, template.Name, template.TemplateData)
	if template.LifecycleStatus != "published" || !readiness.Ready {
		return domain.Details(422, "CERTIFICATE_TEMPLATE_NOT_READY", map[string]interface{}{"errors": readiness.Errors})
	}
	return nil
}
func (s Service) Create(ctx context.Context, data Input, permissions auth.Authorization) (Created, error) {
	if err := authorizeTransition(dbgen.Activity{}, data, permissions); err != nil {
		return Created{}, err
	}
	if data.IsPublished != nil && data.IsPublished.String() != "0" || data.IsRegistrationOpen != nil && *data.IsRegistrationOpen {
		return Created{}, domain.Fail(422, "CREATE_ACTIVITY_AS_DRAFT")
	}
	closed := false
	unpublished := json.Number("0")
	data.IsRegistrationOpen = &closed
	data.IsPublished = &unpublished
	canManage := permissions.Allows("certificate.template.manage")
	templateID := data.CertificateTemplateID.Value
	if templateID == nil && data.AdditionalConfig != nil {
		templateID = data.AdditionalConfig.CertificateTemplateID.Value
	}
	if templateID != nil {
		if err := s.checkTemplate(ctx, templateID, canManage); err != nil {
			return Created{}, err
		}
	}
	data.CertificateTemplateID = domain.Optional[json.Number]{Present: true, Value: templateID}
	data.ClubID.Present = true
	var config []byte
	if data.AdditionalConfig != nil {
		data.AdditionalConfig.CertificateTemplateID = data.CertificateTemplateID
		var err error
		config, err = json.Marshal(data.AdditionalConfig)
		if err != nil {
			return Created{}, err
		}
	}
	q := dbgen.New(s.Pool)
	base := Slug(*data.Name)
	slug := base
	for n := 2; ; n++ {
		exists, err := q.ActivitySlugExists(ctx, slug)
		if err != nil {
			return Created{}, err
		}
		if !exists {
			break
		}
		slug = fmt.Sprintf("%s-%d", base, n)
	}
	row, err := q.CreateActivity(ctx, dbgen.CreateActivityParams{Name: data.Name, Description: data.Description, Badge: data.Badge, ActivityStart: data.ActivityStart, ActivityEnd: data.ActivityEnd, RegistrationStart: data.RegistrationStart, RegistrationEnd: data.RegistrationEnd, SelectionStart: data.SelectionStart, SelectionEnd: data.SelectionEnd, MinimumLevel: numberText(data.MinimumLevel), ActivityType: numberText(data.ActivityType), ActivityCategory: numberText(data.ActivityCategory), IsPublished: numberText(data.IsPublished), IsRegistrationOpen: data.IsRegistrationOpen, Slug: slug, ClubID: numberText(data.ClubID.Value), CertificateTemplateID: numberText(templateID), AdditionalConfig: config})
	if err != nil {
		return Created{}, err
	}
	return Created{Input: data, ID: row.ID, Slug: row.Slug, CreatedAt: domain.ModelTimestamp(row.CreatedAt, time.Local), UpdatedAt: domain.ModelTimestamp(row.UpdatedAt, time.Local)}, nil
}
func sameTemplate(value *json.Number, current *int32) bool {
	if value == nil || current == nil {
		return value == nil && current == nil
	}
	parsed, err := value.Float64()
	return err == nil && parsed == float64(*current)
}
func (s Service) Update(ctx context.Context, identifier string, data Input, permissions auth.Authorization) (Updated, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Updated{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	old, err := q.LockActivityByIdentifier(ctx, identifier)
	err = database.LegacyQueryError(err, `select * from "activities" where "id" = $1 limit $2`)
	if err != nil {
		return Updated{}, err
	}
	if err = authorizeTransition(old, data, permissions); err != nil {
		return Updated{}, err
	}
	canManage := permissions.Allows("certificate.template.manage")
	if data.IsPublished != nil && data.IsPublished.String() == "0" && old.IsPublished != nil && *old.IsPublished {
		closed := false
		data.IsRegistrationOpen = &closed
	}
	assignment := data.CertificateTemplateID
	if !assignment.Present && data.AdditionalConfig != nil {
		assignment = data.AdditionalConfig.CertificateTemplateID
	}
	if assignment.Present && !sameTemplate(assignment.Value, old.CertificateTemplateID) {
		if err := s.checkTemplate(ctx, assignment.Value, canManage); err != nil {
			return Updated{}, err
		}
	}
	if len(old.AdditionalConfig) == 0 || string(old.AdditionalConfig) == "null" {
		return Updated{}, errors.New("Cannot read properties of null (reading 'images')")
	}
	config := map[string]json.RawMessage{}
	if err = json.Unmarshal(old.AdditionalConfig, &config); err != nil {
		return Updated{}, err
	}
	images := config["images"]
	if len(images) == 0 || string(images) == "null" {
		images = json.RawMessage("[]")
	}
	if data.AdditionalConfig != nil {
		raw, err := json.Marshal(data.AdditionalConfig)
		if err != nil {
			return Updated{}, err
		}
		var incoming map[string]json.RawMessage
		if err = json.Unmarshal(raw, &incoming); err != nil {
			return Updated{}, err
		}
		for key, value := range incoming {
			config[key] = value
		}
	}
	config["images"] = images
	if assignment.Present {
		config["certificate_template_id"], err = json.Marshal(assignment.Value)
		if err != nil {
			return Updated{}, err
		}
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return Updated{}, err
	}
	row, err := q.UpdateActivity(ctx, dbgen.UpdateActivityParams{ID: old.ID, Name: data.Name, Description: data.Description, Badge: data.Badge, ActivityStart: data.ActivityStart, ActivityEnd: data.ActivityEnd, RegistrationStart: data.RegistrationStart, RegistrationEnd: data.RegistrationEnd, SelectionStart: data.SelectionStart, SelectionEnd: data.SelectionEnd, MinimumLevel: numberText(data.MinimumLevel), ActivityType: numberText(data.ActivityType), ActivityCategory: numberText(data.ActivityCategory), IsPublished: numberText(data.IsPublished), IsRegistrationOpen: data.IsRegistrationOpen, ClubPresent: data.ClubID.Present, ClubID: numberText(data.ClubID.Value), TemplatePresent: assignment.Present, CertificateTemplateID: numberText(assignment.Value), AdditionalConfig: raw})
	if err != nil {
		return Updated{}, err
	}
	result := Updated{Response: View(row)}
	if row.IsPublished != nil && *row.IsPublished && data.Description != nil && !hasDescription(row.Description) {
		return Updated{}, domain.Fail(422, "ACTIVITY_DESCRIPTION_REQUIRED")
	}
	becomingPublic := row.IsPublished != nil && *row.IsPublished && (old.IsPublished == nil || !*old.IsPublished)
	opening := row.IsRegistrationOpen && !old.IsRegistrationOpen
	if becomingPublic || opening {
		form, lookupErr := activeRegistrationForm(ctx, q, row.ID)
		if lookupErr != nil {
			return Updated{}, lookupErr
		}
		location, _ := time.LoadLocation("Asia/Jakarta")
		readiness := EvaluateReadiness(row, form, time.Now().In(location))
		if becomingPublic && !readiness.CanPublish || opening && (!readiness.CanOpenRegistration || row.IsPublished == nil || !*row.IsPublished) {
			return Updated{}, domain.Details(422, "ACTIVITY_NOT_READY", map[string]interface{}{"readiness": readiness})
		}
	}
	if data.IsPublished != nil {
		result.IsPublished, err = json.Marshal(data.IsPublished)
	} else {
		result.IsPublished, err = json.Marshal(row.IsPublished)
	}
	if data.ActivityStart != nil {
		result.ActivityStart = &row.ActivityStart
	}
	if data.ActivityEnd != nil {
		result.ActivityEnd = &row.ActivityEnd
	}
	if data.RegistrationStart != nil {
		result.RegistrationStart = &row.RegistrationStart
	}
	if data.RegistrationEnd != nil {
		result.RegistrationEnd = &row.RegistrationEnd
	}
	if data.SelectionStart != nil {
		result.SelectionStart = &row.SelectionStart
	}
	if data.SelectionEnd != nil {
		result.SelectionEnd = &row.SelectionEnd
	}
	if err != nil {
		return Updated{}, err
	}
	return result, tx.Commit(ctx)
}
