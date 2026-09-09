package config

import (
	"fmt"
	"io"
	"log/slog"
)

// Preserve the configured Pino level names, including trace, fatal and silent.
func NewLogger(output io.Writer, name string) (*slog.Logger, error) {
	levels := map[string]slog.Level{"trace": -8, "debug": slog.LevelDebug, "info": slog.LevelInfo, "warn": slog.LevelWarn, "error": slog.LevelError, "fatal": 12, "silent": 100}
	level, ok := levels[name]
	if !ok {
		return nil, fmt.Errorf("invalid LOG_LEVEL %q", name)
	}
	return slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: level})), nil
}
