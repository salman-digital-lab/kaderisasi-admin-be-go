//go:build integration

package httpapi

import (
	"context"
	"fmt"
	"io"
	"kaderisasi/admin/internal/certificate"
	"kaderisasi/admin/internal/database"
	"log/slog"
	"reflect"
	"sync"
	"testing"
	"time"
)

type issuanceLogHook struct {
	slog.Handler
	after func() error
}

func (h issuanceLogHook) Handle(ctx context.Context, record slog.Record) error {
	if record.Message == "certificate_issued" {
		return h.after()
	}
	return nil
}
func TestCertificateBulkPausesOnConcurrentTemplateChange(t *testing.T) {
	fixture := newCertificateFixture(t, 4)
	f := fixture.f
	ctx := context.Background()
	expected := certificate.Expectation{ActivityID: float64(fixture.activity.ID("id")), TemplateID: float64(fixture.template.ID("id")), TemplateVersion: 2}
	var once sync.Once
	var mutationErr error
	hook := issuanceLogHook{Handler: slog.NewTextHandler(io.Discard, nil), after: func() error {
		once.Do(func() {
			_, mutationErr = f.pool.Exec(ctx, "UPDATE certificate_templates SET version=version+1 WHERE id=$1", int32(expected.TemplateID))
		})
		return mutationErr
	}}
	service := certificate.Issuance{Pool: f.pool, Location: time.Local, Logger: slog.New(hook)}
	result, err := service.Bulk(ctx, wideIDs(fixture.ids), &f.adminID, "bulk-template-change", &expected)
	if err != nil || mutationErr != nil {
		t.Fatal(err, mutationErr)
	}
	if !result.Paused || result.TotalCreated != 1 || !reflect.DeepEqual(result.RemainingIDs, wideIDs(fixture.ids[1:])) {
		t.Fatalf("pause lost work %+v", result)
	}
	if _, err = f.pool.Exec(ctx, "UPDATE activity_registrations SET status='BELUM LULUS' WHERE id=$1", fixture.ids[1]); err != nil {
		t.Fatal(err)
	}
	expected.TemplateVersion = 3
	resumed, err := service.Bulk(ctx, result.RemainingIDs, &f.adminID, "bulk-resume", &expected)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Paused || resumed.TotalCreated != 2 || resumed.TotalSkipped != 1 {
		t.Fatalf("resume %+v", resumed)
	}
	frozen, err := service.ByID(ctx, float64(result.Created[0].Certificate.ID))
	if err != nil || frozen.Template.Version != 2 {
		t.Fatal("prior snapshot changed", err)
	}
	f.call("PUT", fmt.Sprintf("/v2/activities/%d", int32(expected.ActivityID)), map[string]interface{}{"certificate_template_id": nil}, f.token, 200)
	changed, err := service.Bulk(ctx, wideIDs(fixture.ids), &f.adminID, "assignment-change", &expected)
	if err != nil || !changed.Paused || !reflect.DeepEqual(changed.RemainingIDs, wideIDs(fixture.ids)) {
		t.Fatal("assignment change", changed, err)
	}
}

func TestCertificateRecipientPaginationWithDuplicateProfiles(t *testing.T) {
	fixture := newCertificateFixture(t, 101)
	f := fixture.f
	ctx := context.Background()
	var userID int32
	if err := f.pool.QueryRow(ctx, "INSERT INTO public_users(created_at) VALUES(now()) RETURNING id").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := f.pool.Exec(ctx, "DELETE FROM public_users WHERE id=$1", userID); err != nil {
			t.Error(err)
		}
	})
	if _, err := f.pool.Exec(ctx, "INSERT INTO profiles(user_id,name) VALUES($1,'Peserta 1'),($1,'Legacy duplicate profile')", userID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, "UPDATE activity_registrations SET user_id=$1 WHERE id=$2", userID, fixture.ids[0]); err != nil {
		t.Fatal(err)
	}
	service := certificate.Issuance{Pool: f.pool, Location: time.Local}
	page, err := service.Recipients(ctx, float64(fixture.activity.ID("id")), certificate.RecipientOptions{Page: 2, PerPage: 50})
	if err != nil {
		t.Fatal(err)
	}
	if page.Meta.Total != 101 || len(page.Data) != 50 || page.Counts.Eligible != 101 {
		t.Fatal("recipient multiplication", page.Meta, page.Counts, len(page.Data))
	}
	filtered, err := service.Recipients(ctx, float64(fixture.activity.ID("id")), certificate.RecipientOptions{Page: 1, PerPage: 50, Search: "Peserta 101"})
	if err != nil || len(filtered.Data) != 1 || filtered.Counts.Eligible != 101 {
		t.Fatal("search affected aggregate counts", err)
	}
	all, err := service.Prepare(ctx, float64(fixture.activity.ID("id")), nil)
	if err != nil || !reflect.DeepEqual(all.RegistrationIDs, fixture.ids) {
		t.Fatal("whole-activity preparation", err)
	}
	selected, err := service.Prepare(ctx, float64(fixture.activity.ID("id")), []float64{float64(fixture.ids[0]), float64(fixture.ids[100])})
	if err != nil || !reflect.DeepEqual(selected.RegistrationIDs, []int32{fixture.ids[0], fixture.ids[100]}) {
		t.Fatal("selected preparation", err)
	}
}

func TestCertificateThousandRecipientBatches(t *testing.T) {
	fixture := newCertificateFixture(t, 1000)
	f := fixture.f
	ctx := context.Background()
	expected := certificate.Expectation{ActivityID: float64(fixture.activity.ID("id")), TemplateID: float64(fixture.template.ID("id")), TemplateVersion: 2}
	service := certificate.Issuance{Pool: f.pool, Location: time.Local}
	empty := newCertificateFixture(t, 0)
	review, err := (certificate.Issuance{Pool: empty.f.pool, Location: time.Local}).Prepare(ctx, float64(empty.activity.ID("id")), nil)
	if err != nil || len(review.RegistrationIDs) != 0 || review.Preview != nil {
		t.Fatal("empty preparation", err)
	}
	for offset := 0; offset < len(fixture.ids); offset += 100 {
		result, err := service.Bulk(ctx, wideIDs(fixture.ids[offset:offset+100]), &f.adminID, "thousand-recipient-fixture", &expected)
		if err != nil || result.TotalCreated != 100 || result.Paused || result.TotalFailed != 0 || result.TotalSkipped != 0 {
			t.Fatalf("batch %d: %+v %v", offset, result, err)
		}
		t.Logf("issued %d of 1000 synthetic recipients", offset+100)
	}
	retry, err := service.Bulk(ctx, wideIDs([]int32{fixture.ids[0], fixture.ids[0], fixture.ids[999]}), &f.adminID, "retry-fixture", &expected)
	if err != nil || retry.TotalCreated != 0 || retry.TotalAlreadyIssued != 2 {
		t.Fatal("deduplicated retries", retry, err)
	}
	first := retry.AlreadyIssued[0]
	if _, err = service.Revoke(ctx, float64(first.Certificate.ID), "Synthetic fixture revocation", f.adminID, "fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, "UPDATE activity_registrations SET status='BELUM LULUS' WHERE id=$1", fixture.ids[1]); err != nil {
		t.Fatal(err)
	}
	final, err := service.Prepare(ctx, expected.ActivityID, nil)
	if err != nil || len(final.RegistrationIDs) != 0 || final.Excluded.Revoked != 1 || final.Excluded.AlreadyIssued != 999 {
		t.Fatal("final exclusions", final.Excluded, err)
	}
	count, err := (database.JSONQueries{DB: f.pool}).Count(ctx, "SELECT id FROM issued_certificates WHERE activity_id=$1", int32(expected.ActivityID))
	if err != nil || count != 1000 {
		t.Fatal("exactly one certificate per recipient", count, err)
	}
}

func wideIDs(ids []int32) []float64 {
	out := make([]float64, len(ids))
	for i, id := range ids {
		out[i] = float64(id)
	}
	return out
}
