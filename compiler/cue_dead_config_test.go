package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestEmitDeadCUEDirectoryDiagnostics(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"cue/api/http.cue", "cue/events_meta/annotations.cue", "cue/expert/security.cue", "cue/schedules/jobs.cue"} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("package test\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var got []normalizer.Warning
	emitDeadCUEDirectoryDiagnostics(root, "cue", PipelineOptions{WarningSink: func(w normalizer.Warning) { got = append(got, w) }})
	if len(got) != 1 || got[0].Code != codeCUEDeadDirectory || got[0].Line != 1 {
		t.Fatalf("unexpected diagnostics: %#v", got)
	}
	if got[0].File != filepath.Join(root, "cue/schedules/jobs.cue") {
		t.Fatalf("unexpected dead CUE directory: %#v", got[0])
	}
}
