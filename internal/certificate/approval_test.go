package certificate

import (
	"encoding/json"
	"testing"
)

func TestApprovalHashBindsContentAndSigner(t *testing.T) {
	data := Response{Template: TemplateSnapshot{ID: 1, Version: 2, Data: json.RawMessage(`{"elements":[],"canvasWidth":800}`)}, Participant: Participant{Name: "Peserta", RegistrationID: 1}}
	original, err := ApprovalHash(data, 2, "Signer", "Ketua")
	if err != nil {
		t.Fatal(err)
	}
	data.Template.Data = json.RawMessage(`{ "canvasWidth":800, "elements":[] }`)
	reordered, _ := ApprovalHash(data, 2, "Signer", "Ketua")
	if reordered != original {
		t.Fatal("JSONB ordering changed the content hash")
	}
	changedSigner, _ := ApprovalHash(data, 3, "Signer", "Ketua")
	if changedSigner == original {
		t.Fatal("signer not bound")
	}
	changedTitle, _ := ApprovalHash(data, 2, "Signer", "Direktur")
	if changedTitle == original {
		t.Fatal("title not bound")
	}
	data.Participant.Name = "Different recipient"
	changedContent, _ := ApprovalHash(data, 2, "Signer", "Ketua")
	if changedContent == original {
		t.Fatal("recipient not bound")
	}
}

func TestApprovalReadinessRequiresVisibleQR(t *testing.T) {
	data := `{"canvasWidth":800,"canvasHeight":566,"elements":[{"id":"name","type":"variable-text","variable":"{{name}}","x":0,"y":0,"width":400,"height":50},{"id":"approval","type":"variable-text","variable":"{{approval}}","x":0,"y":100,"width":320,"height":120}]}`
	if !RequiresApproval([]byte(data)) {
		t.Fatal("approval not detected")
	}
	if CheckReadinessValues(1, "E-sign", []byte(data)).Ready {
		t.Fatal("approval without QR accepted")
	}
	var document map[string]interface{}
	_ = json.Unmarshal([]byte(data), &document)
	document["elements"] = append(document["elements"].([]interface{}), map[string]interface{}{"id": "qr", "type": "qr-code", "x": 500, "y": 100, "width": 100, "height": 100})
	raw, _ := json.Marshal(document)
	if !CheckReadinessValues(1, "E-sign", raw).Ready {
		t.Fatal("valid approval design rejected")
	}
	document["elements"].([]interface{})[2].(map[string]interface{})["visible"] = false
	raw, _ = json.Marshal(document)
	if CheckReadinessValues(1, "E-sign", raw).Ready {
		t.Fatal("hidden QR accepted")
	}
}
