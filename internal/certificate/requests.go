package certificate

import "context"

type RegistrationInput struct {
	RegistrationID float64 `json:"registration_id"`
}
type GenerationInput struct {
	ActivityID float64 `json:"activity_id"`
	Status     string  `json:"status"`
}
type SelectionInput struct {
	ActivityID      float64   `json:"activity_id"`
	RegistrationIDs []float64 `json:"registration_ids"`
}
type BulkInput struct {
	Expected        *Expectation `json:"expected"`
	ResponseMode    string       `json:"response_mode"`
	RegistrationIDs []float64    `json:"registration_ids"`
}
type RevocationInput struct {
	Reason string `json:"reason"`
}
type CompactItem struct {
	RegistrationID  float64 `json:"registration_id"`
	Name            string  `json:"name"`
	State           string  `json:"state"`
	CertificateID   *int32  `json:"certificate_id,omitempty"`
	CertificateCode *string `json:"certificate_code,omitempty"`
	Reason          string  `json:"reason,omitempty"`
}
type CompactResult struct {
	Results      []CompactItem `json:"results"`
	Paused       bool          `json:"paused"`
	RemainingIDs []float64     `json:"remaining_ids"`
}

func (s Issuance) Compact(ctx context.Context, result BulkResult, ids []float64) (CompactResult, error) {
	names, err := s.RecipientNames(ctx, ids)
	if err != nil {
		return CompactResult{}, err
	}
	out := CompactResult{Results: []CompactItem{}, Paused: result.Paused, RemainingIDs: result.RemainingIDs}
	for _, group := range []struct {
		items []Response
		state string
	}{{result.Created, "created"}, {result.AlreadyIssued, "already_issued"}} {
		for _, item := range group.items {
			row := CompactItem{RegistrationID: float64(item.Participant.RegistrationID), Name: item.Participant.Name, State: group.state, CertificateID: &item.Certificate.ID, CertificateCode: &item.Certificate.Code}
			if group.state == "already_issued" && item.Certificate.RevokedAt != nil {
				row.State = "skipped"
				row.Reason = "CERTIFICATE_ALREADY_REVOKED"
			}
			out.Results = append(out.Results, row)
		}
	}
	for _, group := range []struct {
		items []Failure
		state string
	}{{result.Skipped, "skipped"}, {result.Failed, "failed"}} {
		for _, item := range group.items {
			name := "Peserta"
			if item.RegistrationID <= 2147483647 {
				if value := names[int32(item.RegistrationID)]; value != "" {
					name = value
				}
			}
			out.Results = append(out.Results, CompactItem{RegistrationID: item.RegistrationID, Reason: item.Reason, Name: name, State: group.state})
		}
	}
	return out, nil
}
