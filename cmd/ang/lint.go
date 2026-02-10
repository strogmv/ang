package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/parser"
)

type lintReport struct {
	OK       bool                 `json:"ok"`
	Warnings []normalizer.Warning `json:"warnings,omitempty"`
	Errors   []lintError          `json:"errors,omitempty"`
}

type lintError struct {
	Message string `json:"message"`
}

func runLint(args []string) {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "emit structured JSON output")
	checkTestCov := fs.Bool("check-test-coverage", false, "check test coverage for endpoints")
	generateStubs := fs.Bool("generate-stubs", false, "generate stubs for missing tests")
	testDir := fs.String("test-dir", "tests", "directory containing test files")
	minCoverage := fs.Float64("min-coverage", 0, "minimum required coverage percentage (0-100)")
	verbose := fs.Bool("verbose", false, "show detailed output")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("\n❌ Lint FAILED: %v\n", err)
		os.Exit(1)
	}

	if *checkTestCov {
		runTestCoverageCheck(*testDir, *minCoverage, *verbose, *jsonOut, *generateStubs)
		return
	}

	projectPath := "."
	if fs.NArg() > 0 {
		projectPath = fs.Arg(0)
	}

	if *jsonOut {
		var warnings []normalizer.Warning
		_, _, _, _, _, _, _, _, err := compiler.RunPipelineWithOptions(projectPath, compiler.PipelineOptions{
			WarningSink: func(w normalizer.Warning) { warnings = append(warnings, w) },
		})

		violations, lintErr := runCueLint(projectPath)

		report := lintReport{
			OK:       err == nil && lintErr == nil && len(violations) == 0,
			Warnings: warnings,
		}
		if err != nil {
			report.Errors = append(report.Errors, lintError{
				Message: formatStageFailure("Lint FAILED", compiler.StageCUE, compiler.ErrCodeCUEPipeline, "run pipeline", err),
			})
		}
		if lintErr != nil {
			report.Errors = append(report.Errors, lintError{
				Message: formatStageFailure("Lint FAILED", compiler.StageCUE, compiler.ErrCodeCUELintLoad, "load cue/lint", lintErr),
			})
		}
		for _, v := range violations {
			report.Errors = append(report.Errors, v)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(report); encErr != nil {
			fmt.Fprintf(os.Stderr, "\n❌ Lint FAILED: %v\n", encErr)
			os.Exit(1)
		}
		if err != nil || lintErr != nil || len(violations) > 0 {
			os.Exit(1)
		}
		return
	}

	fmt.Println("Linting intent...")
	_, _, _, _, _, _, _, _, err := compiler.RunPipeline(projectPath)
	if err != nil {
		fmt.Println()
		printStageFailure("❌ Lint FAILED", compiler.StageCUE, compiler.ErrCodeCUEPipeline, "run pipeline", err)
		os.Exit(1)
	}

	violations, lintErr := runCueLint(projectPath)
	if lintErr != nil {
		fmt.Println()
		printStageFailure("❌ Lint FAILED", compiler.StageCUE, compiler.ErrCodeCUELintLoad, "load cue/lint", lintErr)
		os.Exit(1)
	}
	if len(violations) > 0 {
		fmt.Println("\n❌ Lint FAILED. Violations:")
		for _, v := range violations {
			fmt.Printf("  - %s\n", v.Message)
		}
		os.Exit(1)
	}

	fmt.Println("\n✅ Lint SUCCESSFUL.")
}

func runTestCoverageCheck(testDir string, minCoverage float64, verbose bool, jsonOut bool, generateStubs bool) {
	if !jsonOut {
		fmt.Println("Checking test coverage...")
	}

	_, _, endpoints, _, _, _, _, _, err := compiler.RunPipeline(".")
	if err != nil {
		fmt.Println()
		printStageFailure("❌ Test coverage check FAILED", compiler.StageCUE, compiler.ErrCodeCUETestCoveragePipeline, "run pipeline", err)
		os.Exit(1)
	}

	report, err := checkTestCoverage(endpoints, testDir)
	if err != nil {
		fmt.Printf("\n❌ Test coverage check FAILED: %v\n", err)
		os.Exit(1)
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "\n❌ Test coverage check FAILED: %v\n", err)
			os.Exit(1)
		}
	} else {
		printTestCoverageReport(report, verbose)
	}

	if generateStubs && len(report.MissingTests) > 0 {
		fmt.Println("\nGenerating test stubs for missing endpoints...")
		em := emitter.New(".", "sdk", "templates")

		var missing []normalizer.Endpoint
		missingMap := make(map[string]bool)
		for _, m := range report.MissingTests {
			missingMap[m.Method+" "+m.Path] = true
		}
		for _, ep := range endpoints {
			if missingMap[ep.Method+" "+ep.Path] {
				missing = append(missing, ep)
			}
		}

		if err := em.EmitTestStubs(missing, "NEW-endpoint-stubs.test.ts"); err != nil {
			fmt.Printf("❌ Failed to generate test stubs: %v\n", err)
		}
	}

	if minCoverage > 0 && report.CoveragePercent < minCoverage {
		fmt.Printf("\n❌ Test coverage %.1f%% is below minimum required %.1f%%\n", report.CoveragePercent, minCoverage)
		os.Exit(1)
	}

	if len(report.MissingTests) > 0 {
		fmt.Printf("\n⚠️  %d endpoints without test coverage\n", len(report.MissingTests))
	} else {
		fmt.Println("\n✅ All endpoints have test coverage!")
	}
}

func runCueLint(projectPath string) ([]lintError, error) {
	p := parser.New()
	val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, "cue/lint"))
	if err != nil || !ok {
		return nil, err
	}
	if err := val.Validate(); err != nil {
		return nil, err
	}

	errVal := val.LookupPath(cue.ParsePath("validation_errors"))
	if !errVal.Exists() {
		return nil, nil
	}

	var violations []lintError
	iter, _ := errVal.Fields()
	for iter.Next() {
		violations = append(violations, lintError{
			Message: fmt.Sprintf("%s: %s", iter.Selector(), iter.Value()),
		})
	}
	sort.Slice(violations, func(i, j int) bool {
		return violations[i].Message < violations[j].Message
	})
	return violations, nil
}

func runOptionalMCPGeneration(projectPath string) error {
	root := projectPath
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	script := filepath.Join(root, "scripts", "gen_mcp_server.sh")
	if _, err := os.Stat(script); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cmd := exec.Command("bash", script)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mcp generation failed: %w", err)
	}
	return nil
}
