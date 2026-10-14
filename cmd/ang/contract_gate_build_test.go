package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Item 13 of docs/tech-debt-backlog.md: what `ang build` does after a contract
// change without --accept-contract, and again on the next run. A build that
// removes an operation is refused before anything is written, every time,
// and the refusal names the change; --accept-contract lets it through, after
// which the new contract is the baseline.
func TestRunBuildContractGateRefusesUntilAccepted(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "contract-app")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "contract-app",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/contract-app",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	repoRoot, _ := filepath.Abs("../..")
	writeTestFile(t, filepath.Join(projectDir, "go.mod"), "module github.com/example/contract-app\n\ngo 1.25\n\nreplace github.com/strogmv/ang => "+repoRoot+"\n")
	build := []string{projectDir, "--mode=in_place", "--backend-dir=.", "--skip-go-verify", "--skip-frontend"}

	extraPath := filepath.Join(projectDir, "cue", "api", "tenant_status.cue")
	writeTestFile(t, extraPath, `package api

HTTP: GetTenantStatus: {
	method: "GET"
	path:   "/tenants/status"
}

GetTenantStatus: {
	service: "auth"
	input: {tenantID: string}
	output: {status: string}
}
`)
	if err := runBuild(build); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	contract := filepath.Join(projectDir, "api", "openapi.yaml")
	if !fileContains(t, contract, "/tenants/status") {
		t.Fatalf("%s must hold GET /tenants/status after the first build", contract)
	}

	// The operation is deleted from CUE: refused, twice, with nothing written.
	if err := os.Remove(extraPath); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, projectDir)
	for attempt := 1; attempt <= 2; attempt++ {
		out := captureStdout(t, func() {
			if err := runBuild(build); err == nil {
				t.Fatalf("attempt %d: a build that removes an operation must be refused without --accept-contract", attempt)
			}
		})
		for _, want := range []string{"removed operation GET /tenants/status", "--accept-contract"} {
			if !strings.Contains(out, want) {
				t.Errorf("attempt %d: the refusal does not mention %q:\n%s", attempt, want, out)
			}
		}
		if changed := diffTrees(before, snapshotTree(t, projectDir)); len(changed) > 0 {
			t.Fatalf("attempt %d: a refused build changed the project: %v", attempt, changed)
		}
	}

	// A dry run (and --check, which implies it) predicts the refusal instead
	// of reporting a build the real run would refuse.
	for _, flag := range []string{"--dry-run", "--check"} {
		out := captureStdout(t, func() {
			if err := runBuild(append(append([]string(nil), build...), flag)); err == nil {
				t.Errorf("%s must fail like the real build would", flag)
			}
		})
		if !strings.Contains(out, "removed operation GET /tenants/status") || !strings.Contains(out, "--accept-contract") {
			t.Errorf("%s does not report the breaking change:\n%s", flag, out)
		}
	}
	if err := runBuild(append(append([]string(nil), build...), "--dry-run", "--accept-contract")); err != nil {
		t.Errorf("--dry-run --accept-contract: %v", err)
	}
	if changed := diffTrees(before, snapshotTree(t, projectDir)); len(changed) > 0 {
		t.Fatalf("a dry run changed the project: %v", changed)
	}

	if err := runBuild(append(append([]string(nil), build...), "--accept-contract")); err != nil {
		t.Fatalf("build with --accept-contract: %v", err)
	}
	if fileContains(t, contract, "/tenants/status") {
		t.Fatal("with --accept-contract the operation must be gone from the contract")
	}
	// The accepted contract is the new baseline.
	if err := runBuild(build); err != nil {
		t.Fatalf("the build after an accepted change must pass without the flag: %v", err)
	}
}

// snapshotTree hashes every file of the project except ANG's own state
// (.ang/: build logs, conflicts) and the build scratch directories.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if rel == ".ang" || strings.HasPrefix(d.Name(), ".ang-build") {
				return filepath.SkipDir
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func diffTrees(before, after map[string]string) []string {
	var changed []string
	for path, sum := range before {
		if after[path] != sum {
			changed = append(changed, path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			changed = append(changed, path+" (new)")
		}
	}
	return changed
}
