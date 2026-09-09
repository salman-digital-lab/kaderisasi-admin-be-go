package domain

type Error struct {
	Status  int
	Message string
	Fields  map[string]interface{}
}

func (e *Error) Error() string              { return e.Message }
func Fail(status int, message string) error { return &Error{Status: status, Message: message} }
func Details(status int, message string, fields map[string]interface{}) error {
	return &Error{Status: status, Message: message, Fields: fields}
}
