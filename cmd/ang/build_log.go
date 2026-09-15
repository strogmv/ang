package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/strogmv/ang/angir/normalizer"
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
	// Detail is the full message when Message holds only its first line.
	Detail string `json:"detail,omitempty"`
}

func emitBuildDiagnostics(diagnostics []normalizer.Warning) bool {
	return len(emitBuildDiagnosticEvents(diagnostics)) > 0
}

// emitBuildDiagnosticEvents writes one event per visible diagnostic, warnings
// first and errors last, and returns the errors. message is the first line of
// the diagnostic, at most 300 characters; detail carries the full text when
// that shortened it (an invalid Go expression used to put a whole function on
// one line).
func emitBuildDiagnosticEvents(diagnostics []normalizer.Warning) []normalizer.Warning {
	visible, _ := visibleDiagnostics(diagnostics)
	emit := func(diagnostic normalizer.Warning) {
		short, cut := shortDiagnosticMessage(diagnostic.Message)
		event := buildEvent{
			Timestamp:    time.Now().UTC().Format(time.RFC3339Nano),
			Stage:        "diagnostic",
			Status:       strings.ToLower(diagnosticSeverity(diagnostic)),
			Code:         diagnostic.Code,
			CUEFile:      diagnostic.File,
			Line:         diagnostic.Line,
			Column:       diagnostic.Column,
			Message:      short,
			SuggestedFix: diagnostic.SuggestedFix,
			DocsURL:      firstNonEmpty(diagnostic.DocsURL, compiler.DiagnosticDocsURL(diagnostic.Code)),
		}
		if cut {
			event.Detail = diagnostic.Message
		}
		emitBuildEvent(event)
	}
	var errs []normalizer.Warning
	for _, diagnostic := range visible {
		if diagnosticSeverity(diagnostic) == "ERROR" {
			errs = append(errs, diagnostic)
			continue
		}
		emit(diagnostic)
	}
	for _, diagnostic := range errs {
		emit(diagnostic)
	}
	return errs
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
