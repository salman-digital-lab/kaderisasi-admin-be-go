package httpapi

import (
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/http"
	"runtime"
	"strings"
)

// The registration export returns a diagnostic stack in its error field. Keep
// the error identity and string shape while reporting this implementation's
// actual call sites, never invented JavaScript frames.
func exportFailure(w http.ResponseWriter, err error) {
	identity := err.Error()
	if errors.Is(err, pgx.ErrNoRows) {
		identity = "E_ROW_NOT_FOUND: Row not found"
	}
	var stack strings.Builder
	stack.WriteString(identity)
	pcs := make([]uintptr, 16)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		fmt.Fprintf(&stack, "\n    at %s (%s:%d)", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
	write(w, 500, struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}{"GENERAL_ERROR", stack.String()})
}
