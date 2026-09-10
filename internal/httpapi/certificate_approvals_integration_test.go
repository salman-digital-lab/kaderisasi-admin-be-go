//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/certificate"
	"kaderisasi/admin/internal/config"
	"kaderisasi/admin/internal/database"
	"testing"
)

func approvalOutcomes(t *testing.T, value database.Object) []certificate.ApprovalOutcome {
	t.Helper()
	var out []certificate.ApprovalOutcome
	if err := json.Unmarshal(value["data"], &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestCertificateApprovalWorkflow(t *testing.T) {
	fixture := newCertificateFixture(t, 4)
	f := fixture.f
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := f.pool.Exec(context.Background(), "DELETE FROM certificate_approvals WHERE activity_id=$1", fixture.activity.ID("id")); err != nil {
			t.Error(err)
		}
	})
	_, err := f.pool.Exec(ctx, `UPDATE certificate_templates SET template_data=jsonb_set(template_data,'{elements}',template_data->'elements'||'[{"id":"approval","type":"variable-text","variable":"{{approval}}","x":80,"y":250,"width":320,"height":120},{"id":"qr","type":"qr-code","x":500,"y":250,"width":100,"height":100}]'::jsonb) WHERE id=$1`, fixture.template.ID("id"))
	if err != nil {
		t.Fatal(err)
	}
	signer := f.admin("certificate_manager")
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	service := auth.Service{Pool: f.pool, Key: c.AppKey}
	session, err := service.Build(ctx, mustUser(t, f.pool, signer))
	if err != nil {
		t.Fatal(err)
	}
	expected := certificate.Expectation{ActivityID: float64(fixture.activity.ID("id")), TemplateID: float64(fixture.template.ID("id")), TemplateVersion: float64(fixture.template.ID("version"))}
	input := certificate.ApprovalRequestInput{RegistrationIDs: fixture.ids, SignerID: signer, SignerTitle: "Ketua kegiatan", Expected: expected}
	f.call("POST", "/v2/certificates/issue-single", map[string]int32{"registration_id": fixture.ids[0]}, f.token, 409)
	f.call("POST", "/v2/certificates/approvals", input, "", 401)
	created := approvalOutcomes(t, f.call("POST", "/v2/certificates/approvals", input, f.token, 200))
	if len(created) != 4 {
		t.Fatal(created)
	}
	for _, row := range created {
		if row.Status != "pending" {
			t.Fatalf("request %+v", row)
		}
	}
	repeated := approvalOutcomes(t, f.call("POST", "/v2/certificates/approvals", input, f.token, 200))
	if repeated[0].ID != created[0].ID {
		t.Fatal("duplicate request created")
	}
	var count int
	if err = f.pool.QueryRow(ctx, "SELECT count(*) FROM issued_certificates WHERE activity_id=$1", fixture.activity.ID("id")).Scan(&count); err != nil || count != 0 {
		t.Fatalf("pending requests were issued: %d %v", count, err)
	}
	details := []certificate.ApprovalDecisionItem{}
	for _, row := range created {
		detail := objectData(t, f.call("GET", fmt.Sprintf("/v2/certificates/approvals/%d", row.ID), nil, session.AccessToken, 200))
		details = append(details, certificate.ApprovalDecisionItem{ID: row.ID, ContentHash: detail.String("content_hash")})
	}
	decision := certificate.ApprovalDecisionInput{Items: details[:2], Action: "approve", Consent: true}
	unauthorized := approvalOutcomes(t, f.call("POST", "/v2/certificates/approvals/decide", decision, f.token, 200))
	if unauthorized[0].Reason != "APPROVAL_SIGNER_REQUIRED" {
		t.Fatal("requester approved for signer", unauthorized)
	}
	decision.Consent = false
	f.call("POST", "/v2/certificates/approvals/decide", decision, session.AccessToken, 422)
	decision.Consent = true
	// Recipient changes invalidate the reviewed snapshot, while other batch items can succeed.
	if _, err = f.pool.Exec(ctx, `UPDATE activity_registrations SET guest_data=jsonb_set(guest_data,'{name}','"Changed name"') WHERE id=$1`, fixture.ids[1]); err != nil {
		t.Fatal(err)
	}
	approved := approvalOutcomes(t, f.call("POST", "/v2/certificates/approvals/decide", decision, session.AccessToken, 200))
	if approved[0].Status != "approved" || approved[0].CertificateID == nil || approved[1].Reason != "CERTIFICATE_CONTEXT_CHANGED" {
		t.Fatalf("approval batch %+v", approved)
	}
	payload := certificateResponse(t, f.call("GET", fmt.Sprintf("/v2/certificates/%d", *approved[0].CertificateID), nil, f.token, 200))
	if payload.Certificate.Approval == nil || payload.Certificate.Approval.SignerID != signer || payload.Certificate.Approval.ContentHash != details[0].ContentHash {
		t.Fatalf("missing approval evidence %+v", payload.Certificate)
	}
	decision.Items = details[:1]
	replay := approvalOutcomes(t, f.call("POST", "/v2/certificates/approvals/decide", decision, session.AccessToken, 200))
	if replay[0].CertificateID == nil || *replay[0].CertificateID != *approved[0].CertificateID {
		t.Fatal("approval retry duplicated issuance")
	}
	// Cancellation permits a fresh request for changed data.
	cancel := certificate.ApprovalDecisionInput{Items: details[1:2], Action: "cancel"}
	cancelled := approvalOutcomes(t, f.call("POST", "/v2/certificates/approvals/decide", cancel, f.token, 200))
	if cancelled[0].Status != "cancelled" {
		t.Fatal(cancelled)
	}
	input.RegistrationIDs = fixture.ids[1:2]
	fresh := approvalOutcomes(t, f.call("POST", "/v2/certificates/approvals", input, f.token, 200))
	if fresh[0].ID == created[1].ID || fresh[0].Status != "pending" {
		t.Fatal("resubmission failed", fresh)
	}
	rejection := certificate.ApprovalDecisionInput{Items: details[2:3], Action: "reject", Reason: "Periksa jabatan"}
	rejected := approvalOutcomes(t, f.call("POST", "/v2/certificates/approvals/decide", rejection, session.AccessToken, 200))
	if rejected[0].Status != "rejected" {
		t.Fatal(rejected)
	}
	// Deactivated signers cannot use an existing authenticated session.
	if _, err = f.pool.Exec(ctx, "UPDATE admin_users SET is_active=false WHERE id=$1", signer); err != nil {
		t.Fatal(err)
	}
	decision.Items = details[3:]
	f.call("POST", "/v2/certificates/approvals/decide", decision, session.AccessToken, 403)
	f.call("POST", fmt.Sprintf("/v2/certificates/%d/revoke", *approved[0].CertificateID), map[string]string{"reason": "Fixture revocation"}, f.token, 200)
}
