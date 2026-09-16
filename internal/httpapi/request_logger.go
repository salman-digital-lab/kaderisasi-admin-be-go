package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

type requestObservation struct {
	status         int
	actorID        int32
	panicRecovered bool
}

type responseRecorder struct {
	http.ResponseWriter
	observation *requestObservation
}

func (w *responseRecorder) WriteHeader(status int) {
	if w.observation.status == 0 {
		w.observation.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(body []byte) (int, error) {
	if w.observation.status == 0 {
		w.observation.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

// Unwrap allows http.ResponseController to retain access to the original writer.
func (w *responseRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func logRequest(logger *slog.Logger, request *http.Request, observation requestObservation, started time.Time) {
	status := observation.status
	if status == 0 {
		status = http.StatusOK
	}
	attributes := []any{
		"request_id", request.Header.Get("X-Request-ID"),
		"method", request.Method,
		"path", request.URL.Path,
		"status", status,
		"duration_ms", float64(time.Since(started).Microseconds()) / 1000,
	}
	if observation.actorID != 0 {
		attributes = append(attributes, "actor_admin_id", observation.actorID)
	}

	switch {
	case observation.panicRecovered || status >= http.StatusInternalServerError:
		logger.Error("HTTP request failed", attributes...)
	case status >= http.StatusBadRequest:
		logger.Warn("HTTP request rejected", attributes...)
	default:
		logger.Info("HTTP request completed", attributes...)
	}
}
