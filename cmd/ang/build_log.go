package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/generator"
)

var stdoutRedirectMu sync.Mutex

type buildEvent struct {
	Timestamp      string           `json:"ts"`
	Stage          string           `json:"stage"`
	Target         string           `json:"target,omitempty"`
	Step           string           `json:"step,omitempty"`
	Status         string           `json:"status"`
	DurationMS     int64            `json:"duration_ms,omitempty"`
	MissingCaps    []string         `json:"missing_caps,omitempty"`
	FilesGenerated int              `json:"files_generated,omitempty"`
	Warnings       int              `json:"warnings,omitempty"`
	Error          string           `json:"error,omitempty"`
	Message        string           `json:"message,omitempty"`
	Code           string           `json:"code,omitempty"`
	CUEFile        string           `json:"cueFile,omitempty"`
	Line           int              `json:"line,omitempty"`
	Column         int              `json:"column,omitempty"`
	SuggestedFix   []normalizer.Fix `json:"suggestedFix,omitempty"`
	DocsURL        string           `json:"docsURL,omitempty"`
}

func emitBuildDiagnostics(diagnostics []normalizer.Warning) bool {
	hasErrors := false
	seen := map[string]struct{}{}
	for _, diagnostic := range diagnostics {
		key := fmt.Sprintf("%s|%s|%d|%s|%s", diagnostic.Code, diagnostic.File, diagnostic.Line, diagnostic.CUEPath, diagnostic.Message)
		if _, ok := seen[key]; ok || diagnosticSuppressed(diagnostic) {
			continue
		}
		seen[key] = struct{}{}
		status := strings.ToLower(strings.TrimSpace(diagnostic.Severity))
		if status == "" {
			status = "warn"
		}
		if status == "error" {
			hasErrors = true
		}
		emitBuildEvent(buildEvent{
			Timestamp:    time.Now().UTC().Format(time.RFC3339Nano),
			Stage:        "diagnostic",
			Status:       status,
			Code:         diagnostic.Code,
			CUEFile:      diagnostic.File,
			Line:         diagnostic.Line,
			Column:       diagnostic.Column,
			Message:      diagnostic.Message,
			SuggestedFix: diagnostic.SuggestedFix,
			DocsURL:      firstNonEmpty(diagnostic.DocsURL, compiler.DiagnosticDocsURL(diagnostic.Code)),
		})
	}
	return hasErrors
}

func emitBuildEvent(ev buildEvent) {
	b, _ := json.Marshal(ev)
	fmt.Fprintln(os.Stdout, string(b))
}

func stdoutIsTerminal() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func withSuppressedStdout(run func() error) error {
	stdoutRedirectMu.Lock()
	defer stdoutRedirectMu.Unlock()
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer null.Close()
	original := os.Stdout
	os.Stdout = null
	defer func() { os.Stdout = original }()
	return run()
}

func mapStepEvent(ev generator.StepEvent) buildEvent {
	missing := make([]string, 0, len(ev.MissingCaps))
	for _, c := range ev.MissingCaps {
		missing = append(missing, string(c))
	}
	return buildEvent{
		Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
		Stage:          ev.Stage,
		Target:         ev.Target,
		Step:           ev.Step,
		Status:         ev.Status,
		DurationMS:     ev.DurationMS,
		MissingCaps:    missing,
		FilesGenerated: ev.FilesGenerated,
		Warnings:       ev.Warnings,
		Error:          ev.Error,
	}
}
