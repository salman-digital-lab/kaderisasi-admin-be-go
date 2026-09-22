package scoring

import (
	"encoding/json"
	"time"
)

// PublishedResult is the participant-facing score sheet, without draft or audit data.
type PublishedResult struct {
	Revision    int32      `json:"revision"`
	PublishedAt time.Time  `json:"published_at"`
	Rubric      Definition `json:"rubric"`
	Note        string     `json:"note"`
	Result      Result     `json:"result"`
}

func PublishedForRegistration(raw []byte, registrationID, activityID int32) (*PublishedResult, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var data Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	published := data.Published
	if data.SchemaVersion != 1 || published == nil || published.SchemaVersion != 1 ||
		published.RegistrationID != registrationID || published.ActivityID != activityID || !published.Result.Complete {
		return nil, nil
	}
	return &PublishedResult{
		Revision: published.Revision, PublishedAt: published.PublishedAt,
		Rubric: published.Rubric, Note: published.Draft.Note, Result: published.Result,
	}, nil
}
