package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

func TestWriteDiagnosticsFoldsRepeatedWarningsAndPutsErrorsLast(t *testing.T) {
	var diagnostics []normalizer.Warning
	for i := 1; i <= 5; i++ {
		diagnostics = append(diagnostics, normalizer.Warning{Code: "LARGE_CUE_FILE", Severity: "warn", Message: "CUE file is large", File: "cue/api/f.cue", Line: i})
	}
	longGo := "logic.Call arg 'func' has invalid Go expression \"(func() {\\n" + strings.Repeat("x := 1\\n", 80) + "})\": string literal not terminated"
	diagnostics = append([]normalizer.Warning{{Code: "INVALID_GO_EXPR", Severity: "error", Message: longGo, File: "cue/api/impl_x.cue", Line: 39, Column: 4}}, diagnostics...)

	var out bytes.Buffer
	errs := writeDiagnostics(&out, diagnostics, false)
	text := out.String()
	if len(errs) != 1 {
		t.Fatalf("errors = %d", len(errs))
	}
	if !strings.Contains(text, "⚠️  WARN [LARGE_CUE_FILE] ×5") || !strings.Contains(text, "… and 2 more") || strings.Count(text, "cue/api/f.cue:") != 3 {
		t.Fatalf("warnings are not folded:\n%s", text)
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if last := lines[len(lines)-1]; last != "5 warnings, 0 suppressed, 1 errors" {
		t.Fatalf("last line = %q", last)
	}
	errorLine := ""
	for _, l := range lines {
		if strings.HasPrefix(l, "❌ ") {
			errorLine = l
		}
	}
	if !strings.HasPrefix(errorLine, "❌ cue/api/impl_x.cue:39:4: ERROR [INVALID_GO_EXPR]: ") || len([]rune(errorLine)) > 400 {
		t.Fatalf("error line = %q", errorLine)
	}
	if strings.Index(text, "❌ ") < strings.Index(text, "LARGE_CUE_FILE") {
		t.Fatalf("errors must come after warnings:\n%s", text)
	}

	out.Reset()
	writeDiagnostics(&out, diagnostics, true)
	if strings.Count(out.String(), "[LARGE_CUE_FILE]") != 5 || !strings.Contains(out.String(), "   | ") || !strings.Contains(out.String(), "string literal not terminated") {
		t.Fatalf("verbose must list every warning and the full error:\n%s", out.String())
	}
}

func TestEmitBuildDiagnosticEventsShortenMessageAndEmitErrorsLast(t *testing.T) {
	diagnostics := []normalizer.Warning{
		{Code: "INVALID_GO_EXPR", Severity: "error", Message: "bad Go\n" + strings.Repeat("line\n", 20), File: "cue/api/impl_x.cue", Line: 39, Column: 4},
		{Code: "ORPHAN_PUBLISH", Severity: "warn", Message: "Event X is published but nothing consumes it", File: "cue/api/x.cue", Line: 5},
	}
	output := captureStdout(t, func() {
		if !reportBuildDiagnostics(true, false, diagnostics) {
			t.Fatal("an error must stop the build")
		}
	})
	var events []buildEvent
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		var ev buildEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("not JSON: %q", line)
		}
		events = append(events, ev)
	}
	if len(events) != 3 {
		t.Fatalf("events = %#v", events)
	}
	if events[0].Code != "ORPHAN_PUBLISH" || events[1].Code != "INVALID_GO_EXPR" {
		t.Fatalf("errors must come after warnings: %#v", events)
	}
	if events[1].Message != "bad Go" || !strings.Contains(events[1].Detail, "line\nline") {
		t.Fatalf("error event message=%q detail=%q", events[1].Message, events[1].Detail)
	}
	last := events[2]
	if last.Stage != "build" || last.Status != "error" || last.CUEFile != "cue/api/impl_x.cue" || !strings.HasPrefix(last.Message, "Build FAILED: 1 error; first: cue/api/impl_x.cue:39:4 INVALID_GO_EXPR: bad Go") {
		t.Fatalf("last event = %#v", last)
	}
}

func captureStdout(t *testing.T, run func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	run()
	os.Stdout = original
	_ = w.Close()
	return <-done
}
