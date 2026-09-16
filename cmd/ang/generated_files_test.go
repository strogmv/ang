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

// stagedUnder maps project paths to a staging directory, the way a build maps
// them into its workspace.
func stagedUnder(t *testing.T, projectRoot, stageRoot string) stagedPathFunc {
	t.Helper()
	projectAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	return func(p string) string {
		rel, err := filepath.Rel(projectAbs, p)
		if err != nil {
			t.Fatal(err)
		}
		return filepath.Join(stageRoot, rel)
	}
}

// A file the previous build listed and this one did not produce is removed,
// together with the directories that emptied; a hand-written file next to it
// is not, and a listed file outside the owned directories is only reported.
func TestSyncGeneratedFilesRemovesListedFilesNoLongerGenerated(t *testing.T) {
	root := t.TempDir()
	project, stage := filepath.Join(root, "project"), filepath.Join(root, "stage")
	writeGeneratedFixture(t, project, generatedFilesName, generatedFilesHeader+"internal/a.go\ninternal/old.go\ninternal/sub/gone.go\nscripts/outside.sh\n")
	writeGeneratedFixture(t, project, "scripts/outside.sh", "#!/bin/sh\n")
	for _, rel := range []string{"internal/a.go", "internal/old.go", "internal/hand.go", "internal/sub/gone.go"} {
		writeGeneratedFixture(t, stage, rel, "package x\n")
	}
	produced := map[string]struct{}{"internal/a.go": {}, "internal/new.go": {}}

	result, err := syncGeneratedFiles(project, stagedUnder(t, project, stage), []string{filepath.Join(project, "internal")}, produced, false)
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
		if generatedFixtureExists(stage, gone) {
			t.Errorf("%s must be removed", gone)
		}
	}
	for _, kept := range []string{"internal/a.go", "internal/hand.go"} {
		if !generatedFixtureExists(stage, kept) {
			t.Errorf("%s must stay", kept)
		}
	}
	if got, want := listedGeneratedFiles(t, stage), []string{"internal/a.go", "internal/new.go", "scripts/outside.sh"}; !reflect.DeepEqual(got, want) {
		t.Errorf("list %v, want %v", got, want)
	}
}

// The frontend SDK is generated into a sibling repository: the list carries
// those paths too, and stale files there are removed like any other.
func TestSyncGeneratedFilesRemovesStaleFilesOutsideTheProject(t *testing.T) {
	root := t.TempDir()
	project, stage := filepath.Join(root, "project"), filepath.Join(root, "stage")
	sdk := filepath.Join(root, "app", "src", "@sdk")
	writeGeneratedFixture(t, project, generatedFilesName, generatedFilesHeader+"../app/src/@sdk/index.ts\n../app/src/@sdk/routes.ts\n")
	for _, rel := range []string{"index.ts", "routes.ts"} {
		writeGeneratedFixture(t, sdk, rel, "export {};\n")
	}
	// A build stages an owned directory outside the project in a scratch copy of
	// its own, and the project itself under the workspace.
	stagedSDK := filepath.Join(root, "stage-sdk")
	for _, rel := range []string{"index.ts", "routes.ts"} {
		writeGeneratedFixture(t, stagedSDK, rel, "export {};\n")
	}
	staged := func(p string) string {
		if rel, err := filepath.Rel(sdk, p); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.Join(stagedSDK, rel)
		}
		return stagedUnder(t, project, stage)(p)
	}

	result, err := syncGeneratedFiles(project, staged, []string{sdk}, map[string]struct{}{"../app/src/@sdk/index.ts": {}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"../app/src/@sdk/routes.ts"}; !reflect.DeepEqual(result.Removed, want) {
		t.Fatalf("removed %v, want %v", result.Removed, want)
	}
	if generatedFixtureExists(stagedSDK, "routes.ts") || !generatedFixtureExists(stagedSDK, "index.ts") {
		t.Error("only the file that is no longer generated must go")
	}
	if !generatedFixtureExists(sdk, "routes.ts") {
		t.Error("the sibling repository itself is changed by the transaction commit, not here")
	}
	if got, want := listedGeneratedFiles(t, stage), []string{"../app/src/@sdk/index.ts"}; !reflect.DeepEqual(got, want) {
		t.Errorf("list %v, want %v", got, want)
	}
}

// A partial build did not produce what it skipped, so it removes nothing.
func TestSyncGeneratedFilesPartialBuildOnlyAddsToTheList(t *testing.T) {
	root := t.TempDir()
	project, stage := filepath.Join(root, "project"), filepath.Join(root, "stage")
	writeGeneratedFixture(t, project, generatedFilesName, generatedFilesHeader+"sdk/index.ts\n")
	writeGeneratedFixture(t, stage, "sdk/index.ts", "export {};\n")

	result, err := syncGeneratedFiles(project, stagedUnder(t, project, stage), []string{filepath.Join(project, "sdk")}, map[string]struct{}{"internal/a.go": {}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 0 || !generatedFixtureExists(stage, "sdk/index.ts") {
		t.Fatalf("a partial build removed %v", result.Removed)
	}
	if got, want := listedGeneratedFiles(t, stage), []string{"internal/a.go", "sdk/index.ts"}; !reflect.DeepEqual(got, want) {
		t.Errorf("list %v, want %v", got, want)
	}
}

func TestSyncGeneratedFilesWithoutListRemovesNothing(t *testing.T) {
	root := t.TempDir()
	project, stage := filepath.Join(root, "project"), filepath.Join(root, "stage")
	writeGeneratedFixture(t, stage, "internal/leftover.go", "package x\n")

	result, err := syncGeneratedFiles(project, stagedUnder(t, project, stage), []string{filepath.Join(project, "internal")}, map[string]struct{}{"internal/a.go": {}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || len(result.Removed) != 0 || !generatedFixtureExists(stage, "internal/leftover.go") {
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
	root := t.TempDir()
	project, gen, dest := filepath.Join(root, "project"), filepath.Join(root, "gen"), filepath.Join(root, "dest")
	writeGeneratedFixture(t, gen, "index.ts", "export {};\n")
	writeGeneratedFixture(t, dest, "hand.ts", "export const mine = 1;\n")
	produced := map[string]struct{}{}

	// The generated frontend SDK of a project whose SDK lives in a sibling repo.
	if err := applyGeneratedOutput(gen, dest, filepath.Join(root, "app", "src", "@sdk"), project, produced); err != nil {
		t.Fatal(err)
	}
	if _, ok := produced["../app/src/@sdk/index.ts"]; !ok || len(produced) != 1 {
		t.Fatalf("produced %v", produced)
	}
	data, err := os.ReadFile(filepath.Join(dest, "index.ts"))
	if err != nil || string(data) != "export {};\n" {
		t.Fatalf("copied %q, %v", data, err)
	}
	if !generatedFixtureExists(dest, "hand.ts") {
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

func TestReadGeneratedFilesAcceptsSiblingPathsAndRejectsAbsoluteOnes(t *testing.T) {
	project := t.TempDir()
	writeGeneratedFixture(t, project, generatedFilesName, "../app/src/@sdk/index.ts\n")
	listed, _, err := readGeneratedFiles(project)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := listed["../app/src/@sdk/index.ts"]; !ok {
		t.Fatalf("a sibling path must be kept: %v", listed)
	}
	writeGeneratedFixture(t, project, generatedFilesName, "/etc/passwd\n")
	if _, _, err := readGeneratedFiles(project); err == nil {
		t.Fatal("an absolute path must be rejected")
	}
}
