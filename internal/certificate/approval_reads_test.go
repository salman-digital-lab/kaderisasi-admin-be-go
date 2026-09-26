package certificate

import (
	"encoding/json"
	"kaderisasi/admin/internal/dbgen"
	"testing"
)

func TestApprovalDetailSerializesSnapshotAsObject(t *testing.T) {
	snapshot := []byte(`{"participant":{"name":"Peserta"},"activity":{"name":"Kegiatan"},"template":{"template_data":{"elements":[]}}}`)
	row := dbgen.CertificateApproval{ID: 12, Snapshot: snapshot, ContentHash: "original-hash"}
	raw, err := json.Marshal(ApprovalDetail{CertificateApproval: row, Snapshot: json.RawMessage(snapshot)})
	if err != nil {
		t.Fatal(err)
	}
	var detail struct {
		ID          int32    `json:"id"`
		ContentHash string   `json:"content_hash"`
		Snapshot    Response `json:"snapshot"`
	}
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatalf("snapshot must be an object, not encoded bytes: %v", err)
	}
	if detail.ID != 12 || detail.ContentHash != row.ContentHash || detail.Snapshot.Participant.Name != "Peserta" || detail.Snapshot.Activity.Name != "Kegiatan" {
		t.Fatalf("approval detail lost fields: %+v", detail)
	}
}
