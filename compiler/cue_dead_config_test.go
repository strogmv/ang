package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

func TestEmitDeadCUEDirectoryDiagnostics(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"cue/api/http.cue", "cue/events_meta/annotations.cue", "cue/schedules/jobs.cue"} {
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

// A folder another CUE file imports holds a project's own definitions (such as
// dealingi-back's cue/enums next to the vendored schema): it is loaded through
// the import, so it is not dead.
func TestImportedCUEDirectoryIsNotDead(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"cue.mod/module.cue":    "module: \"example.com/app\"\nlanguage: version: \"v0.9.0\"\n",
		"cue/events/n.cue":      "package events\n\nimport \"example.com/app/cue/enums\"\n\n#N: {type: enums.#Kind}\n",
		"cue/enums/kinds.cue":   "package enums\n\n#Kind: \"a\" | \"b\"\n",
		"cue/leftovers/old.cue": "package leftovers\n",
		"cue/api/other.cue":     "package api\n\n// \"example.com/other/cue/leftovers\" is another module\n",
	}
	for path, content := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var got []normalizer.Warning
	emitDeadCUEDirectoryDiagnostics(root, "cue", PipelineOptions{WarningSink: func(w normalizer.Warning) { got = append(got, w) }})
	if len(got) != 1 || got[0].File != filepath.Join(root, "cue/leftovers/old.cue") {
		t.Fatalf("only cue/leftovers is dead: %#v", got)
	}
}
