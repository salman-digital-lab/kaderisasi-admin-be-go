package scoring

import "time"

type Criterion struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Maximum float64 `json:"maximum"`
	Weight  float64 `json:"weight"`
}
type Group struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Criteria []Criterion `json:"criteria"`
}
type Grade struct {
	Label   string  `json:"label"`
	Minimum float64 `json:"minimum"`
}
type Definition struct {
	Groups []Group `json:"groups"`
	Grades []Grade `json:"grades"`
	Note   string  `json:"note"`
}
type Rubric struct {
	Definition
	Revision int32 `json:"revision"`
	Locked   bool  `json:"locked"`
}
type Draft struct {
	Scores map[string]*float64 `json:"scores"`
	Note   string              `json:"note"`
}
type CriterionResult struct {
	CriterionID string   `json:"criterion_id"`
	Score       *float64 `json:"score"`
	Normalized  *float64 `json:"normalized"`
	Grade       *string  `json:"grade"`
}
type Result struct {
	Criteria []CriterionResult `json:"criteria"`
	Total    *float64          `json:"total"`
	Grade    *string           `json:"grade"`
	Complete bool              `json:"complete"`
}
type Snapshot struct {
	SchemaVersion  int        `json:"schema_version"`
	ActivityID     int32      `json:"activity_id"`
	RegistrationID int32      `json:"registration_id"`
	Revision       int32      `json:"revision"`
	Rubric         Definition `json:"rubric"`
	Draft          Draft      `json:"draft"`
	Result         Result     `json:"result"`
	PublishedBy    int32      `json:"published_by"`
	PublishedAt    time.Time  `json:"published_at"`
}
type Data struct {
	SchemaVersion int       `json:"schema_version"`
	Revision      int32     `json:"revision"`
	State         string    `json:"state"`
	Draft         Draft     `json:"draft"`
	Published     *Snapshot `json:"published"`
	UpdatedBy     int32     `json:"updated_by"`
	UpdatedAt     time.Time `json:"updated_at"`
}
type SaveInput struct {
	Revision       int32 `json:"revision"`
	RubricRevision int32 `json:"rubric_revision"`
	Draft          Draft `json:"draft"`
}
type Selection struct {
	RegistrationID int32 `json:"registration_id"`
	Revision       int32 `json:"revision"`
}
type Batch struct {
	RubricRevision int32       `json:"rubric_revision"`
	Selections     []Selection `json:"selections"`
}
type Entry struct {
	RegistrationID int32   `json:"registration_id"`
	Name           string  `json:"name"`
	Data           *Data   `json:"scoring_data"`
	Result         *Result `json:"result"`
}
type Page struct {
	Entries []Entry `json:"entries"`
	Total   int64   `json:"total"`
	Page    int32   `json:"page"`
	PerPage int32   `json:"per_page"`
}
