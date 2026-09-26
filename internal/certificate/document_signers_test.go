package certificate

import (
	"encoding/json"
	"testing"
)

func TestDocumentSignerSnapshotHash(t *testing.T) {
	profile := documentSigner("oktofa-yudha-sudrajad")
	if profile == nil || documentSigner("unknown") != nil {
		t.Fatal("invalid signer catalog")
	}
	original := Response{DocumentSigner: profile}
	hash, err := ApprovalHash(original, 17, profile.Name, profile.Title)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(original)
	var restored Response
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	roundtrip, _ := ApprovalHash(restored, 17, profile.Name, profile.Title)
	if hash != roundtrip {
		t.Fatal("profile did not survive snapshot serialization")
	}
	restored.DocumentSigner.Key = "another-person"
	changed, _ := ApprovalHash(restored, 17, profile.Name, profile.Title)
	if changed == hash {
		t.Fatal("document signer key not bound to approval")
	}
	var legacy Response
	if err := json.Unmarshal([]byte(`{"activity":{},"template":{},"participant":{}}`), &legacy); err != nil || legacy.DocumentSigner != nil {
		t.Fatal("legacy snapshot changed")
	}
	bytes, _ := json.Marshal(legacy)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(bytes, &fields)
	if _, exists := fields["document_signer"]; exists {
		t.Fatal("legacy hash must omit document signer")
	}
}
