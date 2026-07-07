package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
)

type projectDoctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	Code   string `json:"code,omitempty"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
}

type projectDoctorReport struct {
	Schema      string               `json:"schema"`
	Status      string               `json:"status"`
	ProjectPath string               `json:"project_path"`
	Checks      []projectDoctorCheck `json:"checks"`
}

func analyzeProjectHealth(projectPath string) projectDoctorReport {
	report := projectDoctorReport{Schema: "ang/doctor/v1", Status: "ok", ProjectPath: filepath.Clean(projectPath)}
	var diagnostics []normalizer.Warning
	_, err := compiler.RunSemanticPhasesWithOptions(projectPath, compiler.PipelineOptions{
		WarningSink: func(w normalizer.Warning) { diagnostics = append(diagnostics, w) },
	})
	if err != nil {
		report.Checks = append(report.Checks, projectDoctorCheck{Name: "semantic-validation", Status: "error", Detail: err.Error()})
		report.Status = "error"
	} else {
		report.Checks = append(report.Checks, projectDoctorCheck{Name: "semantic-validation", Status: "ok"})
	}
	seen := map[string]struct{}{}
	for _, diagnostic := range diagnostics {
		key := fmt.Sprintf("%s|%s|%d|%s", diagnostic.Code, diagnostic.File, diagnostic.Line, diagnostic.Message)
		if _, ok := seen[key]; ok || diagnosticSuppressed(diagnostic) {
			continue
		}
		seen[key] = struct{}{}
		status := strings.ToLower(diagnostic.Severity)
		if status == "" {
			status = "warn"
		}
		report.Checks = append(report.Checks, projectDoctorCheck{Name: "diagnostic", Status: status, Detail: diagnostic.Message, Code: diagnostic.Code, File: diagnostic.File, Line: diagnostic.Line})
		if status == "error" {
			report.Status = "error"
		} else if report.Status == "ok" {
			report.Status = "warn"
		}
	}

	manifest, manifestErr := readArtifactHashManifest(projectPath)
	if manifestErr != nil {
		report.Checks = append(report.Checks, projectDoctorCheck{Name: "artifact-manifest", Status: "warn", Detail: manifestErr.Error()})
		if report.Status == "ok" {
			report.Status = "warn"
		}
	} else {
		drift := 0
		for _, artifact := range manifest.Artifacts {
			hash, hashErr := fileSHA256(filepath.Join(projectPath, filepath.FromSlash(artifact.Path)))
			if hashErr != nil || hash != artifact.Hash {
				drift++
				detail := "content hash differs from manifest"
				if hashErr != nil {
					detail = hashErr.Error()
				}
				report.Checks = append(report.Checks, projectDoctorCheck{Name: "artifact-integrity", Status: "error", File: artifact.Path, Detail: detail})
			}
		}
		if drift == 0 {
			report.Checks = append(report.Checks, projectDoctorCheck{Name: "artifact-integrity", Status: "ok", Detail: fmt.Sprintf("%d artifacts verified", len(manifest.Artifacts))})
		} else {
			report.Status = "error"
		}
	}

	if bundle, bundleErr := compiler.LoadInfraBundle(projectPath); bundleErr == nil {
		if diErr := emitter.ValidateGeneratedDI(projectPath, emitter.MainContext{}, bundle.Auth); diErr != nil {
			report.Checks = append(report.Checks, projectDoctorCheck{Name: "dependency-injection", Status: "error", Detail: diErr.Error()})
			report.Status = "error"
		} else {
			report.Checks = append(report.Checks, projectDoctorCheck{Name: "dependency-injection", Status: "ok"})
		}
	}

	currentContract := filepath.Join(projectPath, "api", "openapi.yaml")
	if current, readErr := os.ReadFile(currentContract); readErr == nil {
		cmd := exec.Command("git", "show", "HEAD:api/openapi.yaml")
		cmd.Dir = projectPath
		if baseline, gitErr := cmd.Output(); gitErr == nil {
			diff, diffErr := diffOpenAPIContracts(baseline, current)
			if diffErr != nil {
				status := "error"
				if strings.Contains(diffErr.Error(), "parse previous OpenAPI") {
					status = "warn"
				}
				report.Checks = append(report.Checks, projectDoctorCheck{Name: "contract-diff", Status: status, Detail: diffErr.Error(), File: "api/openapi.yaml"})
				if status == "error" {
					report.Status = "error"
				} else if report.Status == "ok" {
					report.Status = "warn"
				}
			} else if len(diff.BreakingChanges) != 0 {
				report.Checks = append(report.Checks, projectDoctorCheck{Name: "contract-diff", Status: "error", Detail: strings.Join(diff.BreakingChanges, "; "), File: "api/openapi.yaml"})
				report.Status = "error"
			} else {
				report.Checks = append(report.Checks, projectDoctorCheck{Name: "contract-diff", Status: "ok", Detail: fmt.Sprintf("+%d operations, 0 breaking", len(diff.AddedOperations))})
			}
		} else {
			report.Checks = append(report.Checks, projectDoctorCheck{Name: "contract-diff", Status: "warn", Detail: "no committed api/openapi.yaml baseline"})
			if report.Status == "ok" {
				report.Status = "warn"
			}
		}
	}

	if strings.HasSuffix(filepath.Base(os.Args[0]), ".test") {
		report.Checks = append(report.Checks, projectDoctorCheck{Name: "generation-idempotency", Status: "skip", Detail: "subprocess check is disabled inside a Go test binary"})
	} else if detail, idempotencyErr := checkGenerationIdempotency(projectPath); idempotencyErr != nil {
		report.Checks = append(report.Checks, projectDoctorCheck{Name: "generation-idempotency", Status: "error", Detail: idempotencyErr.Error()})
		report.Status = "error"
	} else {
		report.Checks = append(report.Checks, projectDoctorCheck{Name: "generation-idempotency", Status: "ok", Detail: detail})
	}
	return report
}

func checkGenerationIdempotency(projectPath string) (string, error) {
	opts := OutputOptions{BackendDir: ".", FrontendDir: "sdk", DryRun: true, SkipGoVerify: true}
	first, cleanupFirst, err := buildTemplateSnapshot(projectPath, opts)
	if err != nil {
		return "", fmt.Errorf("first dry-run: %w", err)
	}
	defer cleanupFirst()
	second, cleanupSecond, err := buildTemplateSnapshot(projectPath, opts)
	if err != nil {
		return "", fmt.Errorf("second dry-run: %w", err)
	}
	defer cleanupSecond()
	left, err := hashDoctorTree(first.DryRunRoot)
	if err != nil {
		return "", err
	}
	right, err := hashDoctorTree(second.DryRunRoot)
	if err != nil {
		return "", err
	}
	keys := make([]string, 0, len(left)+len(right))
	seen := map[string]struct{}{}
	for key := range left {
		seen[key] = struct{}{}
	}
	for key := range right {
		seen[key] = struct{}{}
	}
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if left[key] != right[key] {
			return "", fmt.Errorf("non-deterministic generated artifact %s", key)
		}
	}
	return fmt.Sprintf("two dry-runs reproduced %d identical artifacts", len(left)), nil
}

func hashDoctorTree(root string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() == "dry-run-report.json" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		hash, err := fileSHA256(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = hash
		return nil
	})
	return out, err
}
