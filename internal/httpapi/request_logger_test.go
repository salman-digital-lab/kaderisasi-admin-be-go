package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestResponseRecorderTracksImplicitAndExplicitStatus(t *testing.T) {
	for _, test := range []struct {
		name  string
		write func(http.ResponseWriter)
		want  int
	}{
		{name: "implicit success", write: func(w http.ResponseWriter) { _, _ = w.Write([]byte("ok")) }, want: http.StatusOK},
		{name: "explicit rejection", write: func(w http.ResponseWriter) { w.WriteHeader(http.StatusForbidden) }, want: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			observation := &requestObservation{}
			writer := &responseRecorder{ResponseWriter: httptest.NewRecorder(), observation: observation}
			test.write(writer)
			if observation.status != test.want {
				t.Fatalf("status = %d, want %d", observation.status, test.want)
			}
		})
	}
}

func TestLogRequestIncludesCorrelationAndUsesStatusSeverity(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v2/members?email=private@example.test", nil)
	request.Header.Set("X-Request-ID", "request-123")

	for _, test := range []struct {
		status         int
		panicRecovered bool
		level          string
		message        string
	}{
		{status: http.StatusOK, level: "INFO", message: "HTTP request completed"},
		{status: http.StatusBadRequest, level: "WARN", message: "HTTP request rejected"},
		{status: http.StatusInternalServerError, level: "ERROR", message: "HTTP request failed"},
		{status: http.StatusOK, panicRecovered: true, level: "ERROR", message: "HTTP request failed"},
	} {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&output, nil))
			logRequest(logger, request, requestObservation{status: test.status, actorID: 42, panicRecovered: test.panicRecovered}, time.Now())
			got := output.String()
			for _, value := range []string{"level=" + test.level, "msg=\"" + test.message + "\"", "request_id=request-123", "status=" + strconv.Itoa(test.status), "actor_admin_id=42"} {
				if !strings.Contains(got, value) {
					t.Fatalf("log = %q, missing %q", got, value)
				}
			}
			if strings.Contains(got, "private@example.test") {
				t.Fatalf("log must not include query values: %q", got)
			}
		})
	}
}
