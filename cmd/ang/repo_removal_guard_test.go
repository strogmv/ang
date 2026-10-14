package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDiffRepositoryRemovals(t *testing.T) {
	before := repositoryMethods{
		"UserRepository":    {"Save": true, "FindByID": true, "ListUsers": true},
		"InvoiceRepository": {"Save": true},
	}
	after := repositoryMethods{
		"UserRepository": {"Save": true, "FindByID": true, "ListActive": true},
	}
	got := diffRepositoryRemovals(before, after)
	want := []string{"port.InvoiceRepository (the whole interface)", "port.UserRepository.ListUsers"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("removals = %v, want %v", got, want)
	}
	if extra := diffRepositoryRemovals(after, after); len(extra) != 0 {
		t.Fatalf("same generation must remove nothing: %v", extra)
	}
	msg := repositoryRemovalError("app", got).Error()
	for _, want := range []string{"removes 2 repository method(s)", "port.UserRepository.ListUsers", "--accept-removals", "check cue/repo"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal does not mention %q: %s", want, msg)
		}
	}
}

func TestCollectRepositoryMethods(t *testing.T) {
	backend := t.TempDir()
	writeTestFile(t, filepath.Join(backend, "internal", "port", "userrepository.go"), `package port

import "context"

type UserRepository interface {
	Save(ctx context.Context, id string) error
	ListUsers(ctx context.Context) ([]string, error)
}

type Auth interface {
	Login(ctx context.Context) error
}
`)
	got, err := collectRepositoryMethods(backend)
	if err != nil {
		t.Fatal(err)
	}
	want := repositoryMethods{"UserRepository": {"Save": true, "ListUsers": true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("methods = %v, want %v", got, want)
	}
	if none, err := collectRepositoryMethods(filepath.Join(backend, "missing")); err != nil || none != nil {
		t.Fatalf("a backend without internal/port must give nil, nil: %v, %v", none, err)
	}
}

// Item 1 of docs/tech-debt-backlog.md, end to end: a cue/repo that does not
// parse fails the build with file:line and leaves the ports alone; a build
// that drops a finder is refused until --accept-removals.
func TestRunBuildGuardsRepositoryFinders(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "finder-app")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "finder-app",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/finder-app",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	repoRoot, _ := filepath.Abs("../..")
	writeTestFile(t, filepath.Join(projectDir, "go.mod"), "module github.com/example/finder-app\n\ngo 1.25\n\nreplace github.com/strogmv/ang => "+repoRoot+"\n")
	build := []string{projectDir, "--mode=in_place", "--backend-dir=.", "--skip-go-verify", "--skip-frontend"}
	reposPath := filepath.Join(projectDir, "cue", "repo", "repositories.cue")
	original, err := os.ReadFile(reposPath)
	if err != nil {
		t.Fatal(err)
	}
	const listInvoices = `finders: [{name: "ListInvoices", returns: "many"}]`
	if !strings.Contains(string(original), listInvoices) {
		t.Fatalf("template changed, %s has no %s", reposPath, listInvoices)
	}
	withExtra := strings.Replace(string(original), listInvoices, `finders: [{name: "ListInvoices", returns: "many"}, {name: "ListRecentInvoices", returns: "many"}]`, 1)
	writeTestFile(t, reposPath, withExtra)
	if err := runBuild(build); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	portFile := filepath.Join(projectDir, "internal", "port", "invoicerepository.go")
	if !fileContains(t, portFile, "ListRecentInvoices(") {
		t.Fatalf("%s must declare ListRecentInvoices after the first build", portFile)
	}

	// 1. `\s` inside sql: the build fails and names the file, line and finder.
	writeTestFile(t, filepath.Join(projectDir, "cue", "repo", "search.cue"), "package repo\n\nRepositories: User: finders: [{\n\tname: \"SearchUsers\"\n\treturns: \"many\"\n\tsql: \"\"\"\n\t\tSELECT * FROM users WHERE regexp_replace(name, '\\s+', ' ') = $1\n\t\t\"\"\"\n}]\n")
	err = runBuild(build)
	if err == nil {
		t.Fatal("a build with an invalid escape in a finder's sql must fail")
	}
	if !fileContains(t, portFile, "ListRecentInvoices(") {
		t.Fatal("a failed build must leave the generated ports alone")
	}
	if err := os.Remove(filepath.Join(projectDir, "cue", "repo", "search.cue")); err != nil {
		t.Fatal(err)
	}

	// 2. A finder deleted from CUE: refused, then accepted with the flag.
	writeTestFile(t, reposPath, string(original))
	err = runBuild(build)
	if err == nil {
		t.Fatal("a build that removes a repository method must be refused without --accept-removals")
	}
	if !fileContains(t, portFile, "ListRecentInvoices(") {
		t.Fatal("a refused build must leave the generated ports alone")
	}
	if err := runBuild(append(append([]string(nil), build...), "--accept-removals")); err != nil {
		t.Fatalf("build with --accept-removals: %v", err)
	}
	if fileContains(t, portFile, "ListRecentInvoices(") {
		t.Fatal("with --accept-removals the finder must be gone")
	}
}

func fileContains(t *testing.T, path, needle string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Contains(string(data), needle)
}
