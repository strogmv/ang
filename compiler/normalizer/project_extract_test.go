package normalizer

import (
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractProject_ParsesPlugins(t *testing.T) {
	t.Parallel()

	val := cuecontext.New().CompileString(`
		#Project: {
			name: "demo"
			version: "0.1.0"
			plugins: ["shared", "go_legacy"]
		}
	`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	n := New()
	project, err := n.ExtractProject(val)
	if err != nil {
		t.Fatalf("extract project: %v", err)
	}
	if project == nil {
		t.Fatalf("expected project")
	}
	if len(project.Plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(project.Plugins))
	}
	if project.Plugins[0] != "shared" || project.Plugins[1] != "go_legacy" {
		t.Fatalf("unexpected plugins: %+v", project.Plugins)
	}
}
