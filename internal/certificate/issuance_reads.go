package certificate

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"time"
)

type RecipientOptions struct {
	Page            float64   `json:"page"`
	PerPage         float64   `json:"per_page"`
	SortOrder       string    `json:"sort_order"`
	Search          string    `json:"search"`
	State           string    `json:"state"`
	RegistrationIDs []float64 `json:"registration_ids"`
}
type ActivityIdentity struct {
	ID   int32  `json:"id"`
	Name string `json:"name"`
}
type RecipientTemplate struct {
	ID        int32     `json:"id"`
	Name      string    `json:"name"`
	Version   int32     `json:"version"`
	Status    string    `json:"status"`
	Readiness Readiness `json:"readiness"`
}
type Recipient struct {
	dbgen.ListCertificateRecipientsRow
	CreatedAt *string `json:"created_at"`
}
type RecipientCounts struct {
	Eligible   int64 `json:"eligible_not_issued"`
	Ineligible int64 `json:"not_eligible"`
	Issued     int64 `json:"issued_active"`
	Revoked    int64 `json:"issued_revoked"`
}
type Recipients struct {
	Activity ActivityIdentity       `json:"activity"`
	Template *RecipientTemplate     `json:"template"`
	Counts   RecipientCounts        `json:"counts"`
	Meta     database.RawPagination `json:"meta"`
	Data     []Recipient            `json:"data"`
}

func identifierStrings(ids []float64) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, database.JSNumber(id))
	}
	return out
}
func nonempty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func (s Issuance) Recipients(ctx context.Context, id float64, options RecipientOptions) (Recipients, error) {
	result := Recipients{Data: []Recipient{}}
	q := dbgen.New(s.Pool)
	activity, err := q.CertificateActivityByIdentifier(ctx, database.JSNumber(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return result, Error("ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return result, err
	}
	filter := dbgen.CountCertificateRecipientsParams{ActivityID: database.JSNumber(id), Search: nonempty(options.Search), State: nonempty(options.State), FilterSelected: options.RegistrationIDs != nil, RegistrationIds: identifierStrings(options.RegistrationIDs)}
	total, err := q.CountCertificateRecipients(ctx, filter)
	if err != nil {
		return result, err
	}
	result.Meta = database.Meta(total, options.Page, options.PerPage).Raw()
	if total > 0 {
		size, offset, err := database.SQLPage(options.Page, options.PerPage)
		if err != nil {
			return result, err
		}
		rows, err := q.ListCertificateRecipients(ctx, dbgen.ListCertificateRecipientsParams{ActivityID: filter.ActivityID, Search: filter.Search, State: filter.State, FilterSelected: filter.FilterSelected, RegistrationIds: filter.RegistrationIds, Ascending: options.SortOrder == "asc", PageSize: size, PageOffset: offset})
		if err != nil {
			return result, err
		}
		for _, row := range rows {
			result.Data = append(result.Data, Recipient{ListCertificateRecipientsRow: row, CreatedAt: domain.Timestamp(row.CreatedAt, time.UTC)})
		}
	}
	counts, err := q.CertificateRecipientCounts(ctx, database.JSNumber(id))
	if err != nil {
		return result, err
	}
	for _, row := range counts {
		switch row.State {
		case "eligible_not_issued":
			result.Counts.Eligible = row.Total
		case "not_eligible":
			result.Counts.Ineligible = row.Total
		case "issued_active":
			result.Counts.Issued = row.Total
		case "issued_revoked":
			result.Counts.Revoked = row.Total
		}
	}
	if templateID := activityTemplateIdentifier(activity); templateID != nil {
		template, err := q.CertificateTemplateByIdentifier(ctx, *templateID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return result, err
		}
		if err == nil {
			result.Template = &RecipientTemplate{ID: template.ID, Name: template.Name, Version: template.Version, Status: template.LifecycleStatus, Readiness: CheckReadinessValues(template.ID, template.Name, template.TemplateData)}
		}
	}
	result.Activity = ActivityIdentity{activity.ID, activity.Name}
	return result, nil
}

type Excluded struct {
	AlreadyIssued int `json:"already_issued"`
	Revoked       int `json:"revoked"`
	NotEligible   int `json:"not_eligible"`
	Missing       int `json:"missing"`
}
type Prepared struct {
	ActivityID      int32     `json:"activity_id"`
	TemplateID      int32     `json:"template_id"`
	TemplateVersion int32     `json:"template_version"`
	RegistrationIDs []int32   `json:"registration_ids"`
	Excluded        Excluded  `json:"excluded"`
	Preview         *Response `json:"preview"`
}

func (s Issuance) Prepare(ctx context.Context, id float64, ids []float64) (Prepared, error) {
	q := dbgen.New(s.Pool)
	activity, err := q.CertificateActivityByIdentifier(ctx, database.JSNumber(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Prepared{}, Error("ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return Prepared{}, err
	}
	templateID := activityTemplateIdentifier(activity)
	if templateID == nil {
		return Prepared{}, Error("CERTIFICATE_TEMPLATE_NOT_READY")
	}
	template, err := q.CertificateTemplateByIdentifier(ctx, *templateID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Prepared{}, err
	}
	if err != nil || template.LifecycleStatus != "published" || !CheckReadinessValues(template.ID, template.Name, template.TemplateData).Ready {
		return Prepared{}, Error("CERTIFICATE_TEMPLATE_NOT_READY")
	}
	rows, err := q.CertificatePreparationRecipients(ctx, dbgen.CertificatePreparationRecipientsParams{ActivityID: database.JSNumber(id), FilterSelected: ids != nil, RegistrationIds: identifierStrings(ids)})
	if err != nil {
		return Prepared{}, err
	}
	out := Prepared{ActivityID: activity.ID, TemplateID: template.ID, TemplateVersion: template.Version, RegistrationIDs: []int32{}}
	for _, row := range rows {
		switch row.State {
		case "eligible_not_issued":
			out.RegistrationIDs = append(out.RegistrationIDs, row.RegistrationID)
		case "issued_active":
			out.Excluded.AlreadyIssued++
		case "issued_revoked":
			out.Excluded.Revoked++
		case "not_eligible":
			out.Excluded.NotEligible++
		}
	}
	if ids != nil {
		out.Excluded.Missing = len(UniqueIDs(ids)) - len(rows)
	}
	if len(out.RegistrationIDs) > 0 {
		preview, err := s.Preview(ctx, float64(out.RegistrationIDs[0]))
		if err != nil {
			return Prepared{}, err
		}
		if preview.Template.ID != template.ID || preview.Template.Version != template.Version {
			return Prepared{}, Error("CERTIFICATE_CONTEXT_CHANGED")
		}
		out.Preview = &preview
	}
	return out, nil
}
func (s Issuance) RecipientNames(ctx context.Context, ids []float64) (map[int32]string, error) {
	names := map[int32]string{}
	if len(ids) == 0 {
		return names, nil
	}
	rows, err := dbgen.New(s.Pool).CertificateRecipientNames(ctx, identifierStrings(ids))
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		names[row.ID] = row.Name
	}
	return names, nil
}

type IssuedListItem struct {
	ID               int32           `json:"id"`
	Code             string          `json:"certificate_code"`
	RegistrationID   int32           `json:"registration_id"`
	ActivityID       int32           `json:"activity_id"`
	ParticipantName  json.RawMessage `json:"participant_name,omitempty"`
	ParticipantEmail json.RawMessage `json:"participant_email,omitempty"`
	ActivityName     json.RawMessage `json:"activity_name,omitempty"`
	TemplateName     string          `json:"template_name"`
	IssuedAt         *string         `json:"issued_at"`
	IssuedBy         *int32          `json:"issued_by"`
	IssuedByName     *string         `json:"issued_by_name"`
	RevokedAt        *string         `json:"revoked_at"`
	RevokedReason    *string         `json:"revoked_reason"`
	RevokedBy        *int32          `json:"revoked_by"`
	RevokedByName    *string         `json:"revoked_by_name"`
	State            string          `json:"state"`
}
type IssuedPage struct {
	Meta database.Pagination `json:"meta"`
	Data []IssuedListItem    `json:"data"`
}

func (s Issuance) List(ctx context.Context, activityID float64, ids []float64, page, size float64) (IssuedPage, error) {
	q := dbgen.New(s.Pool)
	filter := dbgen.CountIssuedCertificateListParams{FilterSelected: ids != nil, RegistrationIds: identifierStrings(ids)}
	if activityID != 0 {
		value := database.JSNumber(activityID)
		filter.ActivityID = &value
	}
	total, err := q.CountIssuedCertificateList(ctx, filter)
	result := IssuedPage{Meta: database.Meta(total, page, size), Data: []IssuedListItem{}}
	if err != nil || total == 0 {
		return result, err
	}
	limit, offset, err := database.SQLPage(page, size)
	if err != nil {
		return result, err
	}
	rows, err := q.ListIssuedCertificates(ctx, dbgen.ListIssuedCertificatesParams{ActivityID: filter.ActivityID, FilterSelected: filter.FilterSelected, RegistrationIds: filter.RegistrationIds, PageSize: limit, PageOffset: offset})
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		var snapshot struct {
			Name         json.RawMessage `json:"name"`
			Email        json.RawMessage `json:"email"`
			ActivityName json.RawMessage `json:"activity_name"`
		}
		if string(row.ParticipantSnapshot) == "null" {
			return result, errors.New("Cannot read properties of null (reading 'name')")
		}
		if err = json.Unmarshal(row.ParticipantSnapshot, &snapshot); err != nil {
			return result, err
		}
		result.Data = append(result.Data, IssuedListItem{ID: row.ID, Code: row.CertificateCode, RegistrationID: row.RegistrationID, ActivityID: row.ActivityID, ParticipantName: snapshot.Name, ParticipantEmail: snapshot.Email, ActivityName: snapshot.ActivityName, TemplateName: row.TemplateName, IssuedAt: domain.ModelTimestamp(row.IssuedAt, s.Location), IssuedBy: row.IssuedBy, IssuedByName: row.IssuedByName, RevokedAt: domain.ModelTimestamp(row.RevokedAt, s.Location), RevokedReason: row.RevokedReason, RevokedBy: row.RevokedBy, RevokedByName: row.RevokedByName, State: row.State})
	}
	return result, nil
}

type PreviewActivity struct {
	dbgen.Activity
	AdditionalConfig json.RawMessage `json:"additional_config"`
	CreatedAt        *string         `json:"created_at"`
	UpdatedAt        *string         `json:"updated_at"`
}
type PreviewTemplate struct {
	dbgen.CertificateTemplate
	TemplateData json.RawMessage `json:"template_data"`
	CreatedAt    *string         `json:"created_at"`
	UpdatedAt    *string         `json:"updated_at"`
	PublishedAt  *string         `json:"published_at"`
	ArchivedAt   *string         `json:"archived_at"`
}
type BulkPreview struct {
	Activity     PreviewActivity `json:"activity"`
	Template     PreviewTemplate `json:"template"`
	Participants []Participant   `json:"participants"`
	Total        int             `json:"total"`
}

func (s Issuance) PreviewBulk(ctx context.Context, id float64, status string) (BulkPreview, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return BulkPreview{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	a, err := q.CertificateActivityByIdentifier(ctx, database.JSNumber(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return BulkPreview{}, Error("ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return BulkPreview{}, err
	}
	templateID := activityTemplateIdentifier(a)
	if templateID == nil {
		return BulkPreview{}, Error("NO_CERTIFICATE_TEMPLATE")
	}
	template, err := q.CertificateTemplateByIdentifier(ctx, *templateID)
	if errors.Is(err, pgx.ErrNoRows) {
		return BulkPreview{}, Error("CERTIFICATE_TEMPLATE_NOT_FOUND")
	}
	if err != nil {
		return BulkPreview{}, err
	}
	if template.LifecycleStatus != "published" {
		return BulkPreview{}, Error("CERTIFICATE_TEMPLATE_NOT_PUBLISHED")
	}
	if ready := CheckReadinessValues(template.ID, template.Name, template.TemplateData); !ready.Ready {
		return BulkPreview{}, Error("CERTIFICATE_TEMPLATE_NOT_READY", ready.Errors)
	}
	rows, err := q.CertificateBulkRegistrations(ctx, dbgen.CertificateBulkRegistrationsParams{ActivityID: database.JSNumber(id), Status: status})
	if err != nil {
		return BulkPreview{}, err
	}
	if len(rows) == 0 {
		return BulkPreview{}, Error("INVALID_STATUS")
	}
	out := BulkPreview{Activity: PreviewActivity{a, a.AdditionalConfig, domain.ModelTimestamp(a.CreatedAt, s.Location), domain.ModelTimestamp(a.UpdatedAt, s.Location)}, Template: PreviewTemplate{template, template.TemplateData, domain.ModelTimestamp(template.CreatedAt, s.Location), domain.ModelTimestamp(template.UpdatedAt, s.Location), domain.ModelTimestamp(template.PublishedAt, s.Location), domain.ModelTimestamp(template.ArchivedAt, s.Location)}, Participants: []Participant{}, Total: len(rows)}
	for _, row := range rows {
		participant, err := s.participant(ctx, q, row, a)
		if err != nil {
			return BulkPreview{}, err
		}
		out.Participants = append(out.Participants, participant)
	}
	return out, tx.Commit(ctx)
}
