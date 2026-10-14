package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/parser"
	"github.com/strogmv/ang/compiler/schemavendor"
)

// The marketplace template imports github.com/strogmv/ang/cue/schema, which a
// new project (module github.com/example/…) could not resolve: its cue/api
// failed to load before ang init vendored the schema.
func TestInitVendorsTheSchemaTheTemplatesImport(t *testing.T) {
	target := filepath.Join(t.TempDir(), "market")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "marketplace",
		TargetDir:    target,
		ProjectName:  "market",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/market",
	}); err != nil {
		t.Fatal(err)
	}
	if status, err := schemavendor.Plan(target, schemavendor.Options{}); err != nil || !status.Applies || !status.InSync() {
		t.Fatalf("a new project has the binary's schema vendored: %+v, %v", status, err)
	}
	if _, err := parser.New().LoadDomain(filepath.Join(target, "cue", "api")); err != nil {
		t.Fatalf("cue/api must load with the vendored schema: %v", err)
	}

	minimal := filepath.Join(t.TempDir(), "minimal")
	if err := initMinimalScaffold(minimal, "github.com/example/minimal", "go", "postgres"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(schemavendor.Dir(minimal), "types.cue")); err != nil {
		t.Fatalf("the minimal scaffold vendors the schema too: %v", err)
	}
	if _, err := os.Stat(filepath.Join(minimal, "cue", "schema")); !os.IsNotExist(err) {
		t.Fatalf("the scaffold no longer creates an empty cue/schema to copy the schema into: %v", err)
	}
}

func TestBuildSyncsTheVendoredSchemaAndCheckFailsOnAnEdit(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "schema-app")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "schema-app",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/schema-app",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	repoRoot, _ := filepath.Abs("../..")
	writeTestFile(t, filepath.Join(projectDir, "go.mod"), "module github.com/example/schema-app\n\ngo 1.25\n\nreplace github.com/strogmv/ang => "+repoRoot+"\n")
	build := []string{projectDir, "--mode=in_place", "--backend-dir=.", "--skip-go-verify", "--skip-frontend"}
	check := append(append([]string(nil), build...), "--check")
	if err := runBuild(build); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	if err := runBuild(check); err != nil {
		t.Fatalf("check right after a build must pass: %v", err)
	}

	vendored := filepath.Join(schemavendor.Dir(projectDir), "types.cue")
	original, err := os.ReadFile(vendored)
	if err != nil {
		t.Fatal(err)
	}
	edited := string(original) + "\n#Mine: string\n"
	writeTestFile(t, vendored, edited)
	if err := runBuild(check); err == nil {
		t.Fatal("check passed although the vendored schema was edited")
	}
	assertTestFile(t, vendored, edited)

	if err := runBuild(build); err != nil {
		t.Fatalf("build: %v", err)
	}
	assertTestFile(t, vendored, string(original))
}

func TestSchemaDiagnosticsForValidate(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "cue.mod", "module.cue"), "module: \"example.com/app\"\nlanguage: version: \"v0.9.0\"\n")
	writeTestFile(t, filepath.Join(root, "cue", "api", "a.cue"), "package api\n\nimport \"github.com/strogmv/ang/cue/schema\"\n\nX: schema.#Operation\n")

	diags := schemaDiagnostics(root, "cue")
	if len(diags) != 1 || diags[0].Code != codeSchemaStale || diags[0].Severity != "error" || !strings.Contains(diags[0].Message, "create cue.mod/pkg/github.com/strogmv/ang/cue/schema/types.cue") {
		t.Fatalf("a missing vendored schema is SCHEMA_STALE: %+v", diags)
	}
	if _, err := schemavendor.Sync(root, schemavendor.Options{}); err != nil {
		t.Fatal(err)
	}
	if diags := schemaDiagnostics(root, "cue"); len(diags) != 0 {
		t.Fatalf("a synced project has no schema diagnostics: %+v", diags)
	}

	writeTestFile(t, filepath.Join(root, "cue", "schema", "types.cue"), "package schema\n\n#Operation: {}\n\n#FlowStep: {}\n")
	diags = schemaDiagnostics(root, "cue")
	if len(diags) != 1 || diags[0].Code != codeSchemaProjectCopy || diags[0].Severity != "warn" {
		t.Fatalf("a project copy of the schema is SCHEMA_PROJECT_COPY: %+v", diags)
	}
}
