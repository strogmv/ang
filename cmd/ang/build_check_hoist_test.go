package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/golit"
)

// slugCUE declares operations (name → the CUE value their logic.Call runs).
// _slugify is a closed literal big enough for golit to hoist once two
// operations paste it. regexp is imported for that literal alone, so hoisting
// drops the import from the operation files and moves their //line directives
// up a line: what a build writes differs from the emitters' output, and
// --check must compare against the former.
func slugCUE(ops ...[2]string) string {
	var b strings.Builder
	b.WriteString("package api\n\n_slugify: \"\"\"\n\t(func(v string) string {\n\t\t// A slug: lower case, runs of other characters become one dash.\n")
	for i := 0; i < 24; i++ {
		b.WriteString("\t\tv = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(strings.TrimSpace(v)), \"-\")\n")
	}
	b.WriteString("\t\treturn strings.Trim(v, \"-\")\n\t})\n\t\"\"\"\n\n_plain: \"(func(v string) string { return v })\"\n")
	for _, op := range ops {
		name, literal := op[0], op[1]
		b.WriteString("\n" + name + ": {\n\tservice: \"auth\"\n\tinput: {name: string}\n\toutput: {slug: string}\n\tflow: [\n" +
			"\t\t{action: \"logic.Check\", condition: \"req.Name != \\\"\\\"\", throw: \"name is required\"},\n" +
			"\t\t{action: \"logic.Call\", func: " + literal + ", args: [\"req.Name\"], output: \"slug\"},\n" +
			"\t\t{action: \"mapping.Assign\", to: \"resp.Slug\", value: \"slug\"},\n\t]\n}\n")
		b.WriteString("\nHTTP: " + name + ": {method: \"POST\", path: \"/slugs/" + strings.ToLower(name) + "\"}\n")
	}
	return b.String()
}

func TestRunBuildCheckMatchesHoistedLiterals(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "hoist-app")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "hoist-app",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/hoist-app",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	repoRoot, _ := filepath.Abs("../..")
	writeTestFile(t, filepath.Join(projectDir, "go.mod"), "module github.com/example/hoist-app\n\ngo 1.25\n\nreplace github.com/strogmv/ang => "+repoRoot+"\n")
	cuePath := filepath.Join(projectDir, "cue", "api", "slugs.cue")
	shared := slugCUE([2]string{"SlugA", "_slugify"}, [2]string{"SlugB", "_slugify"})
	writeTestFile(t, cuePath, shared)

	// --accept-contract: the last step removes an operation.
	build := []string{projectDir, "--mode=in_place", "--backend-dir=.", "--skip-go-verify", "--skip-frontend", "--accept-contract"}
	check := append(append([]string(nil), build...), "--check")

	if err := runBuild(build); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	service := filepath.Join(projectDir, "internal", "service")
	literals := filepath.Join(service, golit.File)
	if _, err := os.Stat(literals); err != nil {
		t.Fatalf("the build must hoist the shared literal: %v", err)
	}
	opFile := filepath.Join(service, "auth__slug_a.gen.go")
	opSrc, err := os.ReadFile(opFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(opSrc), `"regexp"`) || !strings.Contains(string(opSrc), "//line auth__slug_a.gen.go:") {
		t.Fatalf("the operation file must lose the literal's import and keep its //line directives:\n%s", opSrc)
	}

	// Twice: the second build reads its own literals file back.
	for i := 0; i < 2; i++ {
		if err := runBuild(check); err != nil {
			t.Fatalf("check right after a build must pass: %v", err)
		}
		if err := runBuild(build); err != nil {
			t.Fatalf("rebuild: %v", err)
		}
	}
	assertTestFile(t, opFile, string(opSrc))

	// SlugB stops pasting the literal: nothing is repeated. The dry run
	// reports the literals file a build deletes and deletes nothing itself.
	writeTestFile(t, cuePath, slugCUE([2]string{"SlugA", "_slugify"}, [2]string{"SlugB", "_plain"}))
	if !dryRunReports(t, projectDir, build, "internal/service/"+golit.File, "delete") {
		t.Fatal("the dry run must report the literals file a build deletes")
	}
	if _, err := os.Stat(literals); err != nil {
		t.Fatalf("a dry run must not delete the literals file: %v", err)
	}
	if err := runBuild(build); err != nil {
		t.Fatalf("build without the repeat: %v", err)
	}
	assertTestFileMissing(t, literals)
	if err := runBuild(check); err != nil {
		t.Fatalf("check after that build must pass: %v", err)
	}

	// SlugB goes. A partial build (--skip-frontend) keeps its stale file, so
	// the literal is still repeated: a check that hoisted only what the
	// emitters wrote would predict the literals file gone.
	writeTestFile(t, cuePath, shared)
	if err := runBuild(build); err != nil {
		t.Fatalf("build with the repeat again: %v", err)
	}
	writeTestFile(t, cuePath, slugCUE([2]string{"SlugA", "_slugify"}))
	if err := runBuild(build); err != nil {
		t.Fatalf("partial build without SlugB: %v", err)
	}
	if _, err := os.Stat(filepath.Join(service, "auth__slug_b.gen.go")); err != nil {
		t.Fatalf("a partial build keeps the stale operation file: %v", err)
	}
	if _, err := os.Stat(literals); err != nil {
		t.Fatalf("the kept file still repeats the literal: %v", err)
	}
	if err := runBuild(check); err != nil {
		t.Fatalf("check after the partial build must pass: %v", err)
	}
}

// dryRunReports runs a dry run of build and reports whether --check would
// list rel (relative to the project) with action.
func dryRunReports(t *testing.T, projectDir string, build []string, rel, action string) bool {
	t.Helper()
	report := filepath.Join(t.TempDir(), "dry-run.json")
	args := append(append([]string(nil), build...), "--dry-run", "--dry-run-report", report)
	if err := runBuild(args); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	var man dryRunManifest
	if err := json.Unmarshal(data, &man); err != nil {
		t.Fatal(err)
	}
	for _, d := range dryRunDifferences(man, projectDir) {
		if d.Path == rel && d.Action == action {
			return true
		}
	}
	t.Logf("differences: %+v", dryRunDifferences(man, projectDir))
	return false
}
