package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/normalizer"
)

func runValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "output diagnostics as JSON (ang/diags/v1)")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("Validate FAILED: %v\n", err)
		os.Exit(1)
	}

	projectPath := "."
	if fs.NArg() > 0 {
		projectPath = fs.Arg(0)
	}

	if !*asJSON {
		fmt.Println("Validating architecture...")
	}

	_, compileErr := compiler.CompileForEmit(projectPath, compiler.PipelineOptions{}, compiler.CompileForEmitOptions{})

	if *asJSON {
		diags := append([]normalizer.Warning(nil), compiler.LatestDiagnostics...)
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
		for _, d := range compiler.LatestDiagnostics {
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

	hasErrors := emitDiagnostics(os.Stderr, compiler.LatestDiagnostics)
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

func emitDiagnostics(w io.Writer, diagnostics []normalizer.Warning) bool {
	hasErrors := false
	for _, d := range diagnostics {
		severity := "WARN"
		if d.Severity != "" {
			severity = strings.ToUpper(d.Severity)
		}
		if severity == "ERROR" {
			hasErrors = true
		}
		if d.Code != "" {
			fmt.Fprintf(w, "⚠️  %s [%s]: %s\n", severity, d.Code, d.Message)
		} else {
			fmt.Fprintf(w, "⚠️  %s: %s\n", severity, d.Message)
		}
		if d.File != "" {
			fmt.Fprintf(w, "   at %s:%d:%d\n", d.File, d.Line, d.Column)
		}
		if d.Hint != "" {
			fmt.Fprintf(w, "   💡 Hint: %s\n", d.Hint)
		}
	}
	return hasErrors
}
