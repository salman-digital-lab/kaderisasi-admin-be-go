package member

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"

	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
)

// Audit actions, as stored in member_audit_logs.action.
const (
	AuditCreated            = "created"
	AuditProfileUpdated     = "profile_updated"
	AuditRegionalAssignment = "regional_assignment_updated"
	AuditCredentialsUpdated = "credentials_updated"
	AuditAccountGenerated   = "account_generated"
	AuditDeleted            = "deleted"
)

// Actor is the admin a member change is attributed to. Source names the screen
// the change came from ("admin" by default).
type Actor struct {
	AdminID   int32
	Name      string
	RequestID string
	Source    string
}

// AuditChange is one field of a change. A password only ever reports Changed.
type AuditChange struct {
	Before  json.RawMessage `json:"before,omitempty"`
	After   json.RawMessage `json:"after,omitempty"`
	Changed bool            `json:"changed,omitempty"`
}

// AuditChanges maps a field name to its change. Keys of extra_data are
// reported one by one as "extra_data.<key>".
type AuditChanges map[string]AuditChange

var passwordChanged = AuditChange{Changed: true}

var null = json.RawMessage("null")

func auditValue(value any) json.RawMessage {
	// Every audited value is a JSON-native column value.
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

func auditJSON(raw []byte) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 || !json.Valid(raw) {
		return null
	}
	return json.RawMessage(raw)
}

// profileFields lists the audited columns of a profile. Row identity and
// timestamps are left out.
func profileFields(p dbgen.Profile) map[string]json.RawMessage {
	fields := map[string]json.RawMessage{
		"name": auditValue(p.Name), "gender": auditValue(p.Gender), "personal_id": auditValue(p.PersonalID),
		"picture": auditValue(p.Picture), "whatsapp": auditValue(p.Whatsapp), "line": auditValue(p.Line),
		"instagram": auditValue(p.Instagram), "tiktok": auditValue(p.Tiktok), "linkedin": auditValue(p.Linkedin),
		"province_id": auditValue(p.ProvinceID), "city_id": auditValue(p.CityID), "university_id": auditValue(p.UniversityID),
		"major": auditValue(p.Major), "intake_year": auditValue(p.IntakeYear), "level": auditValue(p.Level),
		"birth_date": auditValue(p.BirthDate), "origin_province_id": auditValue(p.OriginProvinceID),
		"origin_city_id": auditValue(p.OriginCityID), "country": auditValue(p.Country),
		"badges": auditJSON(p.Badges), "education_history": auditJSON(p.EducationHistory), "work_history": auditJSON(p.WorkHistory),
	}
	extra := map[string]json.RawMessage{}
	if err := json.Unmarshal(p.ExtraData, &extra); err == nil && extra != nil {
		for key, value := range extra {
			fields["extra_data."+key] = value
		}
	} else {
		fields["extra_data"] = auditJSON(p.ExtraData)
	}
	return fields
}

// createdFields lists the values a new profile starts with.
func createdFields(row dbgen.CreateMemberProfileRow) map[string]json.RawMessage {
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(auditValue(row), &fields); err != nil {
		panic(err)
	}
	for _, key := range []string{"id", "user_id", "created_at", "updated_at"} {
		delete(fields, key)
	}
	return fields
}

func sameJSON(a, b json.RawMessage) bool {
	var left, right any
	if json.Unmarshal(a, &left) != nil || json.Unmarshal(b, &right) != nil {
		return bytes.Equal(a, b)
	}
	return reflect.DeepEqual(left, right)
}

// diffFields reports the fields whose value differs; a missing field is null.
func diffFields(before, after map[string]json.RawMessage) AuditChanges {
	changes := AuditChanges{}
	record := func(key string) {
		old, current := before[key], after[key]
		if old == nil {
			old = null
		}
		if current == nil {
			current = null
		}
		if !sameJSON(old, current) {
			changes[key] = AuditChange{Before: old, After: current}
		}
	}
	for key := range before {
		record(key)
	}
	for key := range after {
		record(key)
	}
	return changes
}

func emailChange(changes AuditChanges, before, after *string) {
	if text(before) != text(after) {
		changes["email"] = AuditChange{Before: auditValue(before), After: auditValue(after)}
	}
}

// record writes one audit row through q, which must be the transaction of the
// change itself. Nothing is written for an empty change.
func (a Actor) record(ctx context.Context, q *dbgen.Queries, action string, profileID, userID *int32, name string, changes AuditChanges) error {
	if len(changes) == 0 || (profileID == nil && userID == nil) {
		return nil
	}
	raw, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	params := dbgen.InsertMemberAuditLogParams{ProfileID: profileID, UserID: userID, MemberName: name, Action: action, Changes: raw, Source: a.Source, ActorName: a.Name}
	if params.Source == "" {
		params.Source = "admin"
	}
	if a.AdminID != 0 {
		params.ActorAdminUserID = &a.AdminID
	}
	if id := strings.TrimSpace(a.RequestID); id != "" {
		if len(id) > 100 {
			id = id[:100]
		}
		params.RequestID = &id
	}
	return q.InsertMemberAuditLog(ctx, params)
}

// recordAccount attributes an account change to the member's profile when one exists.
func (a Actor) recordAccount(ctx context.Context, q *dbgen.Queries, action string, userID int32, changes AuditChanges) error {
	if len(changes) == 0 {
		return nil
	}
	var profileID *int32
	name := ""
	if profile, err := q.ProfileRefByUser(ctx, &userID); err == nil {
		profileID, name = &profile.ID, profile.Name
	}
	return a.record(ctx, q, action, profileID, &userID, name, changes)
}

type AuditLog struct {
	ID        int64           `json:"id"`
	Action    string          `json:"action"`
	Changes   json.RawMessage `json:"changes"`
	Source    string          `json:"source"`
	ActorID   *int32          `json:"actor_admin_user_id"`
	ActorName string          `json:"actor_name"`
	CreatedAt *string         `json:"created_at"`
}

type AuditLogList struct {
	Meta database.Pagination `json:"meta"`
	Data []AuditLog          `json:"data"`
}

// AuditLogs lists a member's trail, newest first. The trail of a deleted
// profile stays readable by its former id.
func (s Service) AuditLogs(ctx context.Context, profileID, page, perPage int32) (AuditLogList, error) {
	q := dbgen.New(s.Pool)
	var userID *int32
	if profile, err := q.ProfileByID(ctx, profileID); err == nil {
		userID = profile.UserID
	}
	rows, err := q.ListMemberAuditLogs(ctx, dbgen.ListMemberAuditLogsParams{ProfileID: profileID, UserID: userID, RowLimit: perPage, RowOffset: (page - 1) * perPage})
	if err != nil {
		return AuditLogList{}, err
	}
	total, err := q.CountMemberAuditLogs(ctx, dbgen.CountMemberAuditLogsParams{ProfileID: profileID, UserID: userID})
	if err != nil {
		return AuditLogList{}, err
	}
	out := AuditLogList{Meta: database.Meta(total, int(page), int(perPage)), Data: make([]AuditLog, 0, len(rows))}
	for _, row := range rows {
		out.Data = append(out.Data, AuditLog{ID: row.ID, Action: row.Action, Changes: auditJSON(row.Changes), Source: row.Source, ActorID: row.ActorAdminUserID, ActorName: row.ActorName, CreatedAt: memberTime(row.CreatedAt)})
	}
	return out, nil
}
