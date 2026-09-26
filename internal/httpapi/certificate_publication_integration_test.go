//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/certificate"
	"kaderisasi/admin/internal/config"
	"testing"
)

func TestCertificateDirectPublicationAndCorrection(t *testing.T) {
	fixture := newCertificateFixture(t, 1)
	f := fixture.f
	ctx := context.Background()
	t.Cleanup(func() {
		_, err := f.pool.Exec(context.Background(), "DELETE FROM certificate_approvals WHERE activity_id=$1", fixture.activity.ID("id"))
		if err != nil {
			t.Error(err)
		}
	})
	_, err := f.pool.Exec(ctx, `UPDATE certificate_templates SET template_data=jsonb_set(template_data,'{elements}',template_data->'elements'||'[{"id":"approval","type":"variable-text","variable":"{{approval}}","x":80,"y":250,"width":320,"height":120},{"id":"qr","type":"qr-code","x":500,"y":250,"width":100,"height":100}]'::jsonb) WHERE id=$1`, fixture.template.ID("id"))
	if err != nil {
		t.Fatal(err)
	}
	admin := f.admin("admin")
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	authService := auth.Service{Pool: f.pool, Key: c.AppKey}
	session, err := authService.Build(ctx, mustUser(t, f.pool, admin))
	if err != nil {
		t.Fatal(err)
	}
	input := certificate.ApprovalRequestInput{DocumentSignerKey: "oktofa-yudha-sudrajad", RegistrationIDs: fixture.ids, SignerID: admin, Expected: certificate.Expectation{ActivityID: float64(fixture.activity.ID("id")), TemplateID: float64(fixture.template.ID("id")), TemplateVersion: float64(fixture.template.ID("version"))}}
	pending := approvalOutcomes(t, f.call("POST", "/v2/certificates/approvals", input, f.token, 200))
	body := map[string]int32{"registration_id": fixture.ids[0]}
	f.call("POST", "/v2/certificates/issue-single", body, "", 401)
	first := certificateResponse(t, f.call("POST", "/v2/certificates/issue-single", body, session.AccessToken, 201))
	evidence := first.Certificate.Approval
	if evidence == nil || evidence.SignerName != certificate.DocumentSigners()[0].Name || evidence.ApprovedBy == nil || *evidence.ApprovedBy != admin || evidence.RequestID != 0 {
		t.Fatal("publication must be directly signed and attributed to the publishing Asmen", evidence)
	}
	var status string
	if err := f.pool.QueryRow(ctx, "SELECT status FROM certificate_approvals WHERE id=$1", pending[0].ID).Scan(&status); err != nil || status != "cancelled" {
		t.Fatal("pending approval not superseded", err, status)
	}
	f.call("POST", "/v2/certificates/issue-single", body, session.AccessToken, 200)
	f.call("POST", fmt.Sprintf("/v2/certificates/%d/revoke", first.Certificate.ID), map[string]string{"reason": "Correct certificate"}, session.AccessToken, 200)
	verification := objectData(t, f.call("GET", "/v2/certificates/verify/"+first.Certificate.Code, nil, f.token, 200))
	if verification.Bool("valid") {
		t.Fatal("withdrawn certificate must be invalid")
	}
	if _, err := f.pool.Exec(ctx, `UPDATE activity_registrations SET guest_data=jsonb_set(guest_data,'{name}','"Corrected participant"'), certificate_group='Group B' WHERE id=$1`, fixture.ids[0]); err != nil {
		t.Fatal(err)
	}
	prepared := objectData(t, f.call("POST", "/v2/certificates/prepare-issuance", map[string]interface{}{"activity_id": fixture.activity.ID("id"), "registration_ids": fixture.ids}, session.AccessToken, 200))
	var ids []int32
	_ = json.Unmarshal(prepared["registration_ids"], &ids)
	if len(ids) != 1 {
		t.Fatal("withdrawn recipient must be eligible again")
	}
	second := certificateResponse(t, f.call("POST", "/v2/certificates/issue-single", body, session.AccessToken, 201))
	if second.Certificate.ID == first.Certificate.ID || second.Certificate.Code == first.Certificate.Code || second.Participant.Name != "Corrected participant" {
		t.Fatal("republication must create a corrected version with a fresh code")
	}
	old := certificateResponse(t, f.call("GET", fmt.Sprintf("/v2/certificates/%d", first.Certificate.ID), nil, f.token, 200))
	if old.Participant.Name != first.Participant.Name || old.Certificate.RevokedAt == nil {
		t.Fatal("previous version must remain immutable and withdrawn")
	}
	lookup := f.call("POST", "/v2/certificates/lookup", map[string]interface{}{"activity_id": fixture.activity.ID("id"), "registration_ids": fixture.ids}, f.token, 200)
	var rows []certificate.IssuedListItem
	_ = json.Unmarshal(lookup["data"], &rows)
	if len(rows) != 1 || rows[0].ID != second.Certificate.ID {
		t.Fatal("lookup must return latest version", rows)
	}
	var active, total int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FILTER (WHERE revoked_at IS NULL),count(*) FROM issued_certificates WHERE registration_id=$1", fixture.ids[0]).Scan(&active, &total); err != nil || active != 1 || total != 2 {
		t.Fatal("version uniqueness/history", active, total, err)
	}
}
