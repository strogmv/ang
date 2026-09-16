package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeGeneratedFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func generatedFixtureExists(root, rel string) bool {
	_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

func listedGeneratedFiles(t *testing.T, root string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, generatedFilesName))
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, line := range strings.Split(string(data), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			paths = append(paths, line)
		}
	}
	return paths
}

// A file the previous build listed and this one did not produce is removed,
// together with the directories that emptied; a hand-written file next to it
// is not, and a listed file outside the managed directories is only reported.
func TestSyncGeneratedFilesRemovesListedFilesNoLongerGenerated(t *testing.T) {
	project, workspace := t.TempDir(), t.TempDir()
	writeGeneratedFixture(t, project, generatedFilesName, generatedFilesHeader+"internal/a.go\ninternal/old.go\ninternal/sub/gone.go\nscripts/outside.sh\n")
	writeGeneratedFixture(t, project, "scripts/outside.sh", "#!/bin/sh\n")
	for _, rel := range []string{"internal/a.go", "internal/old.go", "internal/hand.go", "internal/sub/gone.go"} {
		writeGeneratedFixture(t, workspace, rel, "package x\n")
	}
	produced := map[string]struct{}{"internal/a.go": {}, "internal/new.go": {}}

	result, err := syncGeneratedFiles(project, workspace, []string{filepath.Join(project, "internal")}, produced, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Created {
		t.Error("the project had a list")
	}
	if want := []string{"internal/old.go", "internal/sub/gone.go"}; !reflect.DeepEqual(result.Removed, want) {
		t.Errorf("removed %v, want %v", result.Removed, want)
	}
	if want := []string{"scripts/outside.sh"}; !reflect.DeepEqual(result.Outside, want) {
		t.Errorf("outside %v, want %v", result.Outside, want)
	}
	for _, gone := range []string{"internal/old.go", "internal/sub"} {
		if generatedFixtureExists(workspace, gone) {
			t.Errorf("%s must be removed", gone)
		}
	}
	for _, kept := range []string{"internal/a.go", "internal/hand.go"} {
		if !generatedFixtureExists(workspace, kept) {
			t.Errorf("%s must stay", kept)
		}
	}
	if got, want := listedGeneratedFiles(t, workspace), []string{"internal/a.go", "internal/new.go", "scripts/outside.sh"}; !reflect.DeepEqual(got, want) {
		t.Errorf("list %v, want %v", got, want)
	}
}

// A partial build did not produce what it skipped, so it removes nothing.
func TestSyncGeneratedFilesPartialBuildOnlyAddsToTheList(t *testing.T) {
	project, workspace := t.TempDir(), t.TempDir()
	writeGeneratedFixture(t, project, generatedFilesName, generatedFilesHeader+"sdk/index.ts\n")
	writeGeneratedFixture(t, workspace, "sdk/index.ts", "export {};\n")

	result, err := syncGeneratedFiles(project, workspace, []string{filepath.Join(project, "sdk")}, map[string]struct{}{"internal/a.go": {}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 0 || !generatedFixtureExists(workspace, "sdk/index.ts") {
		t.Fatalf("a partial build removed %v", result.Removed)
	}
	if got, want := listedGeneratedFiles(t, workspace), []string{"internal/a.go", "sdk/index.ts"}; !reflect.DeepEqual(got, want) {
		t.Errorf("list %v, want %v", got, want)
	}
}

func TestSyncGeneratedFilesWithoutListRemovesNothing(t *testing.T) {
	project, workspace := t.TempDir(), t.TempDir()
	writeGeneratedFixture(t, workspace, "internal/leftover.go", "package x\n")

	result, err := syncGeneratedFiles(project, workspace, []string{filepath.Join(project, "internal")}, map[string]struct{}{"internal/a.go": {}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || len(result.Removed) != 0 || !generatedFixtureExists(workspace, "internal/leftover.go") {
		t.Fatalf("first build: created=%v removed=%v", result.Created, result.Removed)
	}
}

// --check reports listed files that are no longer generated and an outdated
// list, and passes once both are in order.
func TestPlanGeneratedFilesReportsDeletionsAndStaleList(t *testing.T) {
	project := t.TempDir()
	writeGeneratedFixture(t, project, generatedFilesName, generatedFilesHeader+"internal/a.go\ninternal/old.go\n")
	writeGeneratedFixture(t, project, "internal/a.go", "package a\n")
	writeGeneratedFixture(t, project, "internal/old.go", "package a\n")
	owned := []string{filepath.Join(project, "internal")}
	manifest := func() dryRunManifest {
		return dryRunManifest{Targets: []dryRunTargetManifest{{Changes: []dryRunFileChange{
			{Path: filepath.ToSlash(filepath.Join(project, "internal", "a.go")), Action: "unchanged"},
		}}}}
	}

	man := manifest()
	if err := planGeneratedFiles(project, owned, &man, false); err != nil {
		t.Fatal(err)
	}
	want := []dryRunFileChange{{Path: generatedFilesName, Action: "update"}, {Path: "internal/old.go", Action: "delete"}}
	if got := dryRunDifferences(man, project); !reflect.DeepEqual(got, want) {
		t.Fatalf("differences %+v, want %+v", got, want)
	}

	if err := os.Remove(filepath.Join(project, "internal", "old.go")); err != nil {
		t.Fatal(err)
	}
	writeGeneratedFixture(t, project, generatedFilesName, string(renderGeneratedFiles(map[string]struct{}{"internal/a.go": {}})))
	man = manifest()
	if err := planGeneratedFiles(project, owned, &man, false); err != nil {
		t.Fatal(err)
	}
	if got := dryRunDifferences(man, project); len(got) != 0 {
		t.Fatalf("an up-to-date project differs: %+v", got)
	}
}

func TestApplyGeneratedOutputCopiesAndRecordsProjectPaths(t *testing.T) {
	project, gen, dest := t.TempDir(), t.TempDir(), t.TempDir()
	writeGeneratedFixture(t, gen, "internal/x.go", "package x\n")
	writeGeneratedFixture(t, dest, "internal/hand.go", "package x\n")
	produced := map[string]struct{}{}

	if err := applyGeneratedOutput(gen, dest, filepath.Join(project, "backend"), project, produced); err != nil {
		t.Fatal(err)
	}
	if _, ok := produced["backend/internal/x.go"]; !ok || len(produced) != 1 {
		t.Fatalf("produced %v", produced)
	}
	data, err := os.ReadFile(filepath.Join(dest, "internal", "x.go"))
	if err != nil || string(data) != "package x\n" {
		t.Fatalf("copied %q, %v", data, err)
	}
	if !generatedFixtureExists(dest, "internal/hand.go") {
		t.Fatal("applying output must not touch other files")
	}
}

func TestGeneratedFilesListIsOwnedByTheBuild(t *testing.T) {
	project := t.TempDir()
	want := filepath.Join(project, generatedFilesName)
	for _, p := range generatedTransactionPaths(project, "in_place", []string{project}, nil) {
		if p == want {
			return
		}
	}
	t.Fatalf("%s is not a transaction path", generatedFilesName)
}

func TestReadGeneratedFilesRejectsPathsOutsideTheProject(t *testing.T) {
	project := t.TempDir()
	writeGeneratedFixture(t, project, generatedFilesName, "../escape.go\n")
	if _, _, err := readGeneratedFiles(project); err == nil {
		t.Fatal("a path leaving the project must be rejected")
	}
}
