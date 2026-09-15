package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/strogmv/ang-ir/normalizer"
)

const diagnosticMessageLimit = 300

// reportBuildDiagnostics prints diagnostics in the build's log format and, if
// any is an error, a one-line failure as the last line of the output (a JSON
// event in JSON mode). It reports whether the build must stop.
func reportBuildDiagnostics(jsonLogs, verbose bool, diagnostics []normalizer.Warning) bool {
	var errs []normalizer.Warning
	if jsonLogs {
		errs = emitBuildDiagnosticEvents(diagnostics)
	} else {
		errs = writeDiagnostics(os.Stderr, diagnostics, verbose)
	}
	if len(errs) == 0 {
		return false
	}
	summary := diagnosticFailureSummary(errs)
	if jsonLogs {
		first := errs[0]
		emitBuildEvent(buildEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			Stage:     "build",
			Status:    "error",
			Message:   summary,
			Code:      first.Code,
			CUEFile:   first.File,
			Line:      first.Line,
			Column:    first.Column,
		})
	} else {
		fmt.Println(summary)
	}
	return true
}

func diagnosticFailureSummary(errs []normalizer.Warning) string {
	noun := "errors"
	if len(errs) == 1 {
		noun = "error"
	}
	first := errs[0]
	short, _ := shortDiagnosticMessage(first.Message)
	where := diagnosticLocation(first)
	if where != "" {
		where += " "
	}
	return fmt.Sprintf("Build FAILED: %d %s; first: %s%s: %s", len(errs), noun, where, first.Code, short)
}

// visibleDiagnostics drops duplicates and diagnostics silenced by
// //ang:nolint, keeping order, and counts the silenced ones.
func visibleDiagnostics(diagnostics []normalizer.Warning) ([]normalizer.Warning, int) {
	seen := make(map[string]struct{}, len(diagnostics))
	out := make([]normalizer.Warning, 0, len(diagnostics))
	suppressed := 0
	for _, d := range diagnostics {
		key := strings.Join([]string{d.Code, d.File, fmt.Sprint(d.Line), d.CUEPath, d.Op, d.Action, d.Message}, "|")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		if diagnosticSuppressed(d) {
			suppressed++
			continue
		}
		out = append(out, d)
	}
	return out, suppressed
}

func diagnosticSeverity(d normalizer.Warning) string {
	if s := strings.TrimSpace(d.Severity); s != "" {
		return strings.ToUpper(s)
	}
	return "WARN"
}

func diagnosticLocation(d normalizer.Warning) string {
	switch {
	case d.File != "" && d.Line > 0 && d.Column > 0:
		return fmt.Sprintf("%s:%d:%d", d.File, d.Line, d.Column)
	case d.File != "" && d.Line > 0:
		return fmt.Sprintf("%s:%d", d.File, d.Line)
	case d.File != "":
		return d.File
	default:
		return d.Path
	}
}

// shortDiagnosticMessage returns the first line of a message, at most
// diagnosticMessageLimit characters, and whether that dropped anything.
func shortDiagnosticMessage(message string) (string, bool) {
	message = strings.TrimSpace(message)
	short := message
	if i := strings.IndexByte(short, '\n'); i >= 0 {
		short = strings.TrimSpace(short[:i])
	}
	if runes := []rune(short); len(runes) > diagnosticMessageLimit {
		short = string(runes[:diagnosticMessageLimit]) + "…"
	}
	return short, short != message
}
