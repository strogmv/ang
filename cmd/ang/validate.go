package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler"
)

func runValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "output diagnostics as JSON (ang/diags/v1)")
	var opFilters []string
	fs.Func("op", "filter diagnostics to a specific operation (repeatable: --op=PlaceBid --op=CreateBid)", func(v string) error {
		opFilters = append(opFilters, strings.ToLower(strings.TrimSpace(v)))
		return nil
	})
	if err := fs.Parse(args); err != nil {
		fmt.Printf("Validate FAILED: %v\n", err)
		os.Exit(1)
	}

	projectPath := "."
	if fs.NArg() > 0 {
		projectPath = fs.Arg(0)
	}

	filterDiags := func(diags []normalizer.Warning) []normalizer.Warning {
		if len(opFilters) == 0 {
			return diags
		}
		var out []normalizer.Warning
		for _, d := range diags {
			if d.Op == "" {
				// include non-op diagnostics (file-level, architecture-level)
				out = append(out, d)
				continue
			}
			for _, f := range opFilters {
				if strings.EqualFold(d.Op, f) {
					out = append(out, d)
					break
				}
			}
		}
		return out
	}

	infraBundle, infraErr := compiler.LoadInfraBundle(projectPath)
	if infraErr != nil {
		if *asJSON {
			diags := []normalizer.Warning{{
				Kind:     "pipeline",
				Code:     string(compiler.ErrCodeCUEInfraConfigParse),
				Severity: "error",
				Message:  infraErr.Error(),
			}}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(vetDiagsEnvelope{
				Schema:      "ang/diags/v1",
				Valid:       false,
				Diagnostics: diags,
			})
			os.Exit(1)
		}
		if ce, ok := infraErr.(*compiler.ContractError); ok {
			printStageFailure("Validation FAILED", ce.Stage, ce.Code, ce.Op, ce.Err)
		} else {
			printStageFailure("Validation FAILED", compiler.StageCUE, compiler.ErrCodeCUEInfraConfigParse, "load infrastructure bundle", infraErr)
		}
		os.Exit(1)
	}

	if !*asJSON {
		if len(opFilters) > 0 {
			fmt.Printf("Validating operation(s): %s...\n", strings.Join(opFilters, ", "))
		} else {
			fmt.Println("Validating architecture...")
		}
	}

	_, compileErr := compiler.CompileForEmit(projectPath, compiler.PipelineOptions{}, compiler.CompileForEmitOptions{
		Config:      derefConfig(infraBundle.Config),
		Auth:        infraBundle.Auth,
		InfraValues: infraBundle.Values,
		Templates:   infraBundle.Templates,
	})

	if *asJSON {
		diags := machineReadableDiagnostics(filterDiags(append([]normalizer.Warning(nil), compiler.LatestDiagnostics...)))
		hasErrors := false
		if compileErr != nil {
			var ce *compiler.ContractError
			if errors.As(compileErr, &ce) {
				diags = append(diags, normalizer.Warning{
					Kind:     "pipeline",
					Code:     string(ce.Code),
					Severity: "error",
					Message:  ce.Error(),
				})
			} else {
				diags = append(diags, normalizer.Warning{
					Kind:     "pipeline",
					Code:     string(compiler.ErrCodeCUEPipeline),
					Severity: "error",
					Message:  compileErr.Error(),
				})
			}
			hasErrors = true
		}
		for _, d := range diags {
			if strings.ToLower(d.Severity) == "error" {
				hasErrors = true
				break
			}
		}
		if diags == nil {
			diags = []normalizer.Warning{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(vetDiagsEnvelope{
			Schema:      "ang/diags/v1",
			Valid:       !hasErrors,
			Diagnostics: diags,
		})
		if hasErrors {
			os.Exit(1)
		}
		return
	}

	if compileErr != nil {
		if ce, ok := compileErr.(*compiler.ContractError); ok {
			printBootstrapGuidanceIfNeeded(projectPath, ce.Stage, ce.Code, ce.Err)
			printStageFailure("Validation FAILED", ce.Stage, ce.Code, ce.Op, ce.Err)
		} else {
			printBootstrapGuidanceIfNeeded(projectPath, compiler.StageCUE, compiler.ErrCodeCUEPipeline, compileErr)
			printStageFailure("Validation FAILED", compiler.StageCUE, compiler.ErrCodeCUEPipeline, "compile for emit", compileErr)
		}
		os.Exit(1)
	}

	hasErrors := emitDiagnostics(os.Stderr, filterDiags(compiler.LatestDiagnostics))
	if hasErrors {
		fmt.Println("Validation FAILED due to diagnostic errors.")
		os.Exit(1)
	}
	if msg, risky := detectReleaseRootModuleMismatch(projectPath); risky {
		printStageFailure("Validation FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "release output/module guard", errors.New(msg))
		os.Exit(1)
	}
	fmt.Println("Validation SUCCESSFUL.")
}

func derefConfig(cfg *normalizer.ConfigDef) normalizer.ConfigDef {
	if cfg == nil {
		return normalizer.ConfigDef{}
	}
	return *cfg
}

func emitDiagnostics(w io.Writer, diagnostics []normalizer.Warning) bool {
	hasErrors := false
	seen := make(map[string]struct{}, len(diagnostics))
	warnings := 0
	suppressed := 0
	for _, d := range diagnostics {
		key := strings.Join([]string{d.Code, d.File, fmt.Sprint(d.Line), d.CUEPath, d.Op, d.Action, d.Message}, "|")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		if diagnosticSuppressed(d) {
			suppressed++
			continue
		}
		severity := "WARN"
		if d.Severity != "" {
			severity = strings.ToUpper(d.Severity)
		}
		if severity == "ERROR" {
			hasErrors = true
		} else if severity == "WARN" {
			warnings++
		}
		if d.Code != "" {
			fmt.Fprintf(w, "⚠️  %s [%s]: %s\n", severity, d.Code, d.Message)
		} else {
			fmt.Fprintf(w, "⚠️  %s: %s\n", severity, d.Message)
		}
		if d.File != "" {
			fmt.Fprintf(w, "   at %s:%d:%d\n", d.File, d.Line, d.Column)
		} else if d.Path != "" {
			fmt.Fprintf(w, "   at %s\n", d.Path)
		}
		if d.Hint != "" {
			fmt.Fprintf(w, "   💡 Hint: %s\n", d.Hint)
		}
	}
	if warnings > 0 || suppressed > 0 {
		fmt.Fprintf(w, "%d warnings, %d suppressed\n", warnings, suppressed)
	}
	return hasErrors
}

func diagnosticSuppressed(d normalizer.Warning) bool {
	if d.File == "" || d.Line <= 0 || strings.TrimSpace(d.Code) == "" {
		return false
	}
	data, err := os.ReadFile(d.File)
	if err != nil {
		return false
	}
	lines := strings.Split(string(data), "\n")
	start := d.Line - 1
	if start >= len(lines) {
		start = len(lines) - 1
	}
	for index := start; index >= 0 && index >= start-2; index-- {
		line := strings.ToLower(lines[index])
		marker := strings.Index(line, "ang:nolint")
		if marker < 0 {
			continue
		}
		codes := strings.Fields(strings.NewReplacer(",", " ", "//", " ", "#", " ").Replace(line[marker+len("ang:nolint"):]))
		if len(codes) == 0 {
			return true
		}
		for _, code := range codes {
			if strings.EqualFold(code, d.Code) {
				return true
			}
		}
	}
	return false
}

// machineReadableDiagnostics keeps JSON validation consistent with the human
// output: diagnostics are deduplicated and per-site nolint directives apply.
func machineReadableDiagnostics(diagnostics []normalizer.Warning) []normalizer.Warning {
	seen := make(map[string]struct{}, len(diagnostics))
	out := make([]normalizer.Warning, 0, len(diagnostics))
	for _, d := range diagnostics {
		key := strings.Join([]string{d.Code, d.File, fmt.Sprint(d.Line), d.CUEPath, d.Op, d.Action, d.Message}, "|")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		if diagnosticSuppressed(d) {
			continue
		}
		out = append(out, d)
	}
	return out
}
