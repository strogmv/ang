package mcp

import (
	"context"
	"slices"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestDefaultWorkflowsIncludeNewMCPTools(t *testing.T) {
	feature := defaultFeatureAddWorkflow()
	bugfix := defaultBugFixWorkflow()

	for _, name := range []string{"ang_ops_context", "ang_template_diff", "ang_template_rebase"} {
		if !slices.Contains(feature, name) {
			t.Fatalf("feature_add workflow missing %q", name)
		}
		if !slices.Contains(bugfix, name) {
			t.Fatalf("bug_fix workflow missing %q", name)
		}
	}
}

func TestRegisterCoreToolsIncludesNewWrappers(t *testing.T) {
	tools := map[string]func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error){}
	addTool := func(name string, tool mcp.Tool, h func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
		tools[name] = h
	}

	registerCoreTools(addTool, coreToolDeps{
		currentProfile:     func() string { return "compact" },
		runtimeConfigPath:  func() string { return ".ang/mcp-runtime.json" },
		runtimeConfigError: func() string { return "" },
		featureAddWorkflow: defaultFeatureAddWorkflow,
		bugFixWorkflow:     defaultBugFixWorkflow,
		bootstrapExempt: func() map[string]bool {
			return map[string]bool{}
		},
		envelopeEnabled:  func() bool { return true },
		searchLimits:     func() (int, int) { return 25, 80 },
		symbolLimits:     func() (int, int) { return 6, 24 },
		snapshotLimits:   func() (int, int) { return 25, 80 },
		mcpSchemaVersion: "mcp-envelope/v1",
	})

	for _, name := range []string{"ang_ops_context", "ang_template_diff", "ang_template_rebase"} {
		if _, ok := tools[name]; !ok {
			t.Fatalf("registerCoreTools missing %q", name)
		}
	}
}

func TestBuildOpsContextArgs(t *testing.T) {
	args := buildOpsContextArgs(".", "prod", true, "tmp/facts.json")
	want := []string{"ops", "context", ".", "--json", "--profile=prod", "--migration-mode", "--facts=tmp/facts.json"}
	if !slices.Equal(args, want) {
		t.Fatalf("buildOpsContextArgs()=%v want %v", args, want)
	}
}

func TestBuildTemplateDiffArgs(t *testing.T) {
	args := buildTemplateDiffArgs(angTemplateToolArgs{
		projectPath:       ".",
		target:            "backend",
		mode:              "release",
		backendDir:        ".",
		frontendDir:       "sdk",
		skipFrontend:      true,
		skipContractTests: true,
		outPath:           "/tmp/report.json",
	})
	want := []string{
		"template", "diff", ".", "--json",
		"--out", "/tmp/report.json",
		"--target", "backend",
		"--mode", "release",
		"--backend-dir", ".",
		"--frontend-dir", "sdk",
		"--skip-frontend",
		"--skip-contract-tests",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("buildTemplateDiffArgs()=%v want %v", args, want)
	}
}

func TestBuildTemplateRebaseArgsDefaultsToPlan(t *testing.T) {
	args := buildTemplateRebaseArgs(angTemplateToolArgs{
		projectPath: ".",
		fromPath:    "/tmp/diff.json",
	})
	want := []string{"template", "rebase", ".", "--json", "--plan", "--from", "/tmp/diff.json"}
	if !slices.Equal(args, want) {
		t.Fatalf("buildTemplateRebaseArgs()=%v want %v", args, want)
	}
}

func TestBuildTemplateRebaseArgsApply(t *testing.T) {
	args := buildTemplateRebaseArgs(angTemplateToolArgs{
		projectPath: ".",
		apply:       true,
		outPath:     "/tmp/plan.json",
		target:      "backend",
	})
	want := []string{"template", "rebase", ".", "--json", "--apply", "--out", "/tmp/plan.json", "--target", "backend"}
	if !slices.Equal(args, want) {
		t.Fatalf("buildTemplateRebaseArgs()=%v want %v", args, want)
	}
}
