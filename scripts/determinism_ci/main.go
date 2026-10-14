package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
)

type artifactHashRecord struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

type artifactHashManifest struct {
	SchemaVersion   string               `json:"schemaVersion"`
	CompilerVersion string               `json:"compilerVersion"`
	IRVersion       string               `json:"irVersion"`
	InputHash       string               `json:"inputHash,omitempty"`
	TemplateHash    string               `json:"templateHash,omitempty"`
	Artifacts       []artifactHashRecord `json:"artifacts"`
}

type buildPlan struct {
	SchemaVersion  string `json:"schemaVersion"`
	PlanVersion    string `json:"planVersion"`
	GeneratedAtUTC string `json:"generatedAtUtc"`
	WorkspaceRoot  string `json:"workspaceRoot"`
	InputHash      string `json:"inputHash"`
	CompilerHash   string `json:"compilerHash"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "determinism check failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("OK: determinism checks passed for phases plan/apply/all")
}

// run builds a scratch copy of examples/blog (with the repository's cue.mod and
// cue/schema, which its CUE imports) so the tracked sample is never rewritten.
func run() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", "ang-determinism-ci-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	angBin := filepath.Join(tmpDir, "ang")
	if err := runCmd(repoRoot, "go", "build", "-o", angBin, "./cmd/ang"); err != nil {
		return err
	}
	sampleRoot := filepath.Join(tmpDir, "sample")
	for _, rel := range []string{"cue.mod", filepath.Join("cue", "schema"), filepath.Join("examples", "blog")} {
		if err := copyTree(filepath.Join(repoRoot, rel), filepath.Join(sampleRoot, rel)); err != nil {
			return err
		}
	}
	project := filepath.Join(sampleRoot, "examples", "blog")
	build := func(args ...string) error {
		base := []string{"build", "--project", project, "--accept-contract", "--accept-removals", "--skip-go-verify"}
		return runCmd(project, angBin, append(base, args...)...)
	}
	manifestPath := filepath.Join(project, ".ang", "cache", "manifest.json")

	// phase=all determinism
	if err := build("--phase=all"); err != nil {
		return err
	}
	m1, err := readManifest(manifestPath)
	if err != nil {
		return err
	}
	if err := build("--phase=all"); err != nil {
		return err
	}
	m2, err := readManifest(manifestPath)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(m1, m2) {
		return fmt.Errorf("phase=all drift detected between two runs")
	}

	// phase=plan determinism
	plan1 := filepath.Join(tmpDir, "plan1.json")
	plan2 := filepath.Join(tmpDir, "plan2.json")
	if err := build("--phase=plan", "--out-plan", plan1); err != nil {
		return err
	}
	if err := build("--phase=plan", "--out-plan", plan2); err != nil {
		return err
	}
	p1, err := readPlan(plan1)
	if err != nil {
		return err
	}
	p2, err := readPlan(plan2)
	if err != nil {
		return err
	}
	p1.GeneratedAtUTC = ""
	p2.GeneratedAtUTC = ""
	if !reflect.DeepEqual(p1, p2) {
		return fmt.Errorf("phase=plan drift detected between two runs")
	}

	// phase=apply determinism
	if err := build("--phase=apply", "--plan-file", plan1); err != nil {
		return err
	}
	ma, err := readManifest(manifestPath)
	if err != nil {
		return err
	}
	if err := build("--phase=apply", "--plan-file", plan1); err != nil {
		return err
	}
	mb, err := readManifest(manifestPath)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(ma, mb) {
		return fmt.Errorf("phase=apply drift detected when reapplying same plan")
	}

	return nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func runCmd(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return nil
}

func readManifest(path string) (artifactHashManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return artifactHashManifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var out artifactHashManifest
	if err := json.Unmarshal(data, &out); err != nil {
		return artifactHashManifest{}, fmt.Errorf("unmarshal manifest: %w", err)
	}
	return out, nil
}

func readPlan(path string) (buildPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return buildPlan{}, fmt.Errorf("read plan: %w", err)
	}
	var out buildPlan
	if err := json.Unmarshal(data, &out); err != nil {
		return buildPlan{}, fmt.Errorf("unmarshal plan: %w", err)
	}
	return out, nil
}
