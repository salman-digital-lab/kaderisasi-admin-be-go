package httpapi

import (
	"errors"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
	"path/filepath"
	"runtime"
	"strings"
)

type errorFrame struct {
	File        string   `json:"file"`
	FilePath    string   `json:"filePath"`
	Line        int      `json:"line"`
	Column      int      `json:"column"`
	Callee      string   `json:"callee"`
	CalleeShort string   `json:"calleeShort"`
	Context     struct{} `json:"context"`
	IsApp       bool     `json:"isApp"`
	IsModule    bool     `json:"isModule"`
	IsNative    bool     `json:"isNative"`
}

// Uncaught Adonis errors expose their message in production and add debugging
// frames in development. Report actual Go frames, never source-language frames.
func (s *Server) frameworkError(err error, statement string) error {
	if err == nil {
		return nil
	}
	var domainError *domain.Error
	if errors.As(err, &domainError) {
		return err
	}
	if statement != "" {
		err = database.LegacyQueryError(err, statement)
	}
	if s.Config.Environment == "production" {
		return domain.Fail(500, err.Error())
	}
	pcs := make([]uintptr, 16)
	n := runtime.Callers(2, pcs)
	stack := runtime.CallersFrames(pcs[:n])
	frames := []errorFrame{}
	for {
		frame, more := stack.Next()
		isApp := strings.Contains(frame.Function, "kaderisasi/admin/")
		frames = append(frames, errorFrame{File: filepath.Base(frame.File), FilePath: frame.File, Line: frame.Line, Column: 1, Callee: frame.Function, CalleeShort: frame.Function, IsApp: isApp, IsModule: !isApp, IsNative: false})
		if !more {
			break
		}
	}
	return domain.Details(500, err.Error(), map[string]interface{}{"name": "error", "status": 500, "frames": frames})
}
