package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestRenderFlow_ExecRunUsesTimeoutDefault(t *testing.T) {
	t.Parallel()

	code := renderFlow([]normalizer.FlowStep{
		{
			Action: "exec.Run",
			Args: map[string]any{
				"cmd":    `"ang"`,
				"args":   []string{`"build"`},
				"output": "out",
			},
		},
	})

	if !strings.Contains(code, "context.WithTimeout(ctx, 120 * time.Second)") {
		t.Fatalf("expected default exec timeout in generated code, got:\n%s", code)
	}
}

func TestRenderFlow_ExecRunUsesExplicitTimeout(t *testing.T) {
	t.Parallel()

	code := renderFlow([]normalizer.FlowStep{
		{
			Action: "exec.Run",
			Args: map[string]any{
				"cmd":     `"ang"`,
				"args":    []string{`"build"`},
				"timeout": "15 * time.Second",
				"output":  "out",
			},
		},
	})

	if !strings.Contains(code, "context.WithTimeout(ctx, 15 * time.Second)") {
		t.Fatalf("expected explicit exec timeout in generated code, got:\n%s", code)
	}
}

func TestRenderFlow_ExecStreamUsesPipeAndScanner(t *testing.T) {
	t.Parallel()

	code := renderFlow([]normalizer.FlowStep{
		{
			Action: "exec.Stream",
			Args: map[string]any{
				"cmd":     `"ang"`,
				"args":    []string{`"build"`},
				"timeout": "15 * time.Second",
				"output":  "streamOut",
			},
		},
	})

	if !strings.Contains(code, "context.WithTimeout(ctx, 15 * time.Second)") {
		t.Fatalf("expected timeout in exec.Stream generated code, got:\n%s", code)
	}
	if !strings.Contains(code, "io.Pipe()") || !strings.Contains(code, "bufio.NewScanner(") {
		t.Fatalf("expected pipe+scanner streaming code, got:\n%s", code)
	}
	if !strings.Contains(code, "slog.Info(\"exec.stream\"") {
		t.Fatalf("expected per-line stream log emission, got:\n%s", code)
	}
	if _, err := parseFlowStmtList(code); err != nil {
		t.Fatalf("generated code must be valid Go: %v\n%s", err, code)
	}
}

func TestRenderFlow_FlowDeferPredeclaresCleanupVar(t *testing.T) {
	t.Parallel()

	code := renderFlow([]normalizer.FlowStep{
		{
			Action: "flow.Defer",
			Args: map[string]any{
				"_do": []normalizer.FlowStep{
					{Action: "fs.Remove", Args: map[string]any{"path": "workDir"}},
				},
			},
		},
		{
			Action: "fs.TempDir",
			Args: map[string]any{
				"output":  "workDir",
				"pattern": `"sendbox-build-*"`,
			},
		},
	})

	if !strings.Contains(code, "var workDir string") {
		t.Fatalf("expected predeclared cleanup variable in generated code, got:\n%s", code)
	}
	if !strings.Contains(code, "defer func()") {
		t.Fatalf("expected defer wrapper in generated code, got:\n%s", code)
	}
	if !strings.Contains(code, "workDir = _tmpDir") {
		t.Fatalf("expected fs.TempDir to assign into predeclared var, got:\n%s", code)
	}
	if _, err := parseFlowStmtList(code); err != nil {
		t.Fatalf("generated code must be valid Go: %v\n%s", err, code)
	}
}

func TestRenderFlow_FSRemoveIsImmediate(t *testing.T) {
	t.Parallel()

	code := renderFlow([]normalizer.FlowStep{
		{Action: "fs.Remove", Args: map[string]any{"path": "workDir"}},
	})

	if strings.Contains(code, "defer os.RemoveAll(") {
		t.Fatalf("fs.Remove must be immediate, got deferred cleanup:\n%s", code)
	}
	if !strings.Contains(code, "os.RemoveAll(workDir)") {
		t.Fatalf("expected os.RemoveAll call in generated code, got:\n%s", code)
	}
}
