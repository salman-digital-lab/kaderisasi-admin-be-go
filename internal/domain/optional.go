package domain

import "encoding/json"

// Optional preserves all three PATCH states: omitted, explicit null, and value.
type Optional[T any] struct {
	Present bool
	Value   *T
}

func (field *Optional[T]) UnmarshalJSON(raw []byte) error {
	field.Present = true
	return json.Unmarshal(raw, &field.Value)
}

func (field Optional[T]) MarshalJSON() ([]byte, error) { return json.Marshal(field.Value) }
func (field Optional[T]) IsZero() bool                 { return !field.Present }
func Value[T any](value T) Optional[T]                 { return Optional[T]{Present: true, Value: &value} }
